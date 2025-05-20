package main

import (
	"fmt"
	"log"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
)

func isPortTypeAllowed(port string, cfg *Config) bool {
	for _, portType := range cfg.PortTypes {
		if strings.HasPrefix(port, portType) {
			return true
		}
	}
	return false
}

func isPortIgnored(port string, sw *SwitchConfig) bool {
	for _, ignored := range sw.IgnoredPorts {
		if port == ignored {
			return true
		}
	}
	return false
}

func getVLANFromOUI(oui string, sw *SwitchConfig, cfg *Config) int {
	oui = strings.ToUpper(strings.ReplaceAll(oui, ":", ""))

	// Primeiro, verifica o mapeamento específico do switch
	for _, mapping := range sw.OUIMappings {
		for _, mappedOUI := range mapping.OUIs {
			if oui == strings.ToUpper(strings.ReplaceAll(mappedOUI, ":", "")) {
				return mapping.VLAN
			}
		}
	}

	// Depois, verifica o mapeamento global
	for _, mapping := range cfg.DefaultOUIMappings {
		for _, mappedOUI := range mapping.OUIs {
			if oui == strings.ToUpper(strings.ReplaceAll(mappedOUI, ":", "")) {
				return mapping.VLAN
			}
		}
	}

	return 0 // Retorna 0 se não houver mapeamento, para usar default_vlan
}

func getMACFromPort(port string, sw *SwitchConfig, cfg *Config) (string, error) {
	sm := &SwitchManager{config: *sw}
	if err := sm.connect(cfg); err != nil {
		return "", err
	}
	defer sm.disconnect()

	output, err := sm.executeCommand("show mac address-table dynamic interface "+port, cfg)
	if err != nil {
		return "", fmt.Errorf("failed to get MAC table for %s: %v", port, err)
	}
	re := regexp.MustCompile(`(\d+)\s+([0-9A-Fa-f]{4}\.[0-9A-Fa-f]{4}\.[0-9A-Fa-f]{4})\s+\w+\s+(\S+)`)
	matches := re.FindAllStringSubmatch(output, -1)
	if len(matches) == 0 {
		return "", fmt.Errorf("no MAC found for port %s", port)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("multiple MACs found for port %s", port)
	}
	match := matches[0]
	mac := match[2]
	macNoDots := strings.ReplaceAll(mac, ".", "")
	var macFull strings.Builder
	for i := 0; i < len(macNoDots); i += 2 {
		if i > 0 {
			macFull.WriteString(":")
		}
		macFull.WriteString(macNoDots[i : i+2])
	}
	return macFull.String(), nil
}

func getSwitchConfig(sourceIP string, cfg *Config) *SwitchConfig {
	for _, sw := range cfg.Switches {
		if sw.Target == sourceIP {
			return &sw
		}
	}
	return nil
}

func handleTrap(trap *gosnmp.SnmpPacket, cfg *Config) {
	sourceIP := trap.AgentAddress
	sw := getSwitchConfig(sourceIP, cfg)
	if sw == nil {
		log.Printf("Switch desconhecido: %s", sourceIP)
		return
	}

	sm := &SwitchManager{config: *sw}
	for _, v := range trap.Variables {
		port := fmt.Sprintf("%v", v.Value)

		// Verifica se a porta é de um tipo permitido
		if !isPortTypeAllowed(port, cfg) {
			log.Printf("Switch %s: Porta %s ignorada, tipo de porta não permitido", sourceIP, port)
			continue
		}

		// Verifica se a porta deve ser ignorada
		if isPortIgnored(port, sw) {
			log.Printf("Switch %s: Porta %s ignorada, nenhuma alteração", sourceIP, port)
			continue
		}

		// Verifica se é um trap de linkUp
		if strings.Contains(v.Name, ".1.3.6.1.6.3.1.1.5.3") {
			log.Printf("Trap linkUp recebido do switch %s na porta: %s", sourceIP, port)

			// Obtém o MAC associado à porta
			mac, err := getMACFromPort(port, sw, cfg)
			if err != nil {
				log.Printf("Erro ao obter MAC do switch %s: %v", sourceIP, err)
				continue
			}
			log.Printf("Switch %s: MAC detectado: %s", sourceIP, mac)

			// Verifica se o MAC está excluído
			normMac := normalizeMac(mac)
			excluded := false
			for _, excludeMac := range cfg.ExcludeMacs {
				if normMac == excludeMac {
					excluded = true
					log.Printf("Switch %s: MAC %s excluído, usando default_vlan %d", sourceIP, mac, cfg.DefaultVLAN)
					break
				}
			}

			// Determina a VLAN
			vlan := cfg.DefaultVLAN
			if !excluded {
				vlan = getVLANFromOUI(normMac[:6], sw, cfg)
				if vlan == 0 {
					vlan = cfg.DefaultVLAN
					log.Printf("Switch %s: Nenhum mapeamento OUI para %s, usando default_vlan %d", sourceIP, mac, vlan)
				} else {
					log.Printf("Switch %s: VLAN mapeada para OUI: %d", sourceIP, vlan)
				}
			}

			// Configura a VLAN via Telnet
			if err := sm.connect(cfg); err != nil {
				log.Printf("Erro ao conectar ao switch %s: %v", sourceIP, err)
				continue
			}
			defer sm.disconnect()

			if sw.CreateVLANs {
				existingVLANs, err := sm.getVlanList(cfg)
				if err != nil {
					log.Printf("Erro ao obter lista de VLANs: %v", err)
					continue
				}
				vlanStr := strconv.Itoa(vlan)
				if !existingVLANs[vlanStr] {
					log.Printf("Creating VLAN %d on switch %s", vlan, sourceIP)
					if err := sm.createVLAN(vlan, cfg); err != nil {
						log.Printf("Erro ao criar VLAN %d: %v", vlan, err)
						continue
					}
				}
			}

			if !cfg.SkipVlanCheck {
				existingVLANs, err := sm.getVlanList(cfg)
				if err != nil {
					log.Printf("Erro ao verificar VLANs: %v", err)
					continue
				}
				vlanStr := strconv.Itoa(vlan)
				if !existingVLANs[vlanStr] {
					log.Printf("Erro: VLAN %d não existe no switch %s", vlan, sourceIP)
					continue
				}
			}

			sm.configureVlan(port, vlan, cfg)
			if !cfg.Sandbox {
				_, err := sm.executeCommand("write memory", cfg)
				if err != nil {
					log.Printf("Erro ao salvar configuração: %v", err)
				} else {
					log.Printf("Configuração salva no switch %s", sourceIP)
				}
			}
		}

		// Verifica se é um trap de linkDown
		if strings.Contains(v.Name, ".1.3.6.1.6.3.1.1.5.4") {
			log.Printf("Trap linkDown recebido do switch %s na porta: %s", sourceIP, port)

			// Configura a VLAN nula via Telnet
			if err := sm.connect(cfg); err != nil {
				log.Printf("Erro ao conectar ao switch %s: %v", sourceIP, err)
				continue
			}
			defer sm.disconnect()

			if sw.CreateVLANs {
				existingVLANs, err := sm.getVlanList(cfg)
				if err != nil {
					log.Printf("Erro ao obter lista de VLANs: %v", err)
					continue
				}
				vlanStr := strconv.Itoa(cfg.NullVLAN)
				if !existingVLANs[vlanStr] {
					log.Printf("Creating VLAN %d on switch %s", cfg.NullVLAN, sourceIP)
					if err := sm.createVLAN(cfg.NullVLAN, cfg); err != nil {
						log.Printf("Erro ao criar VLAN %d: %v", cfg.NullVLAN, err)
						continue
					}
				}
			}

			if !cfg.SkipVlanCheck {
				existingVLANs, err := sm.getVlanList(cfg)
				if err != nil {
					log.Printf("Erro ao verificar VLANs: %v", err)
					continue
				}
				vlanStr := strconv.Itoa(cfg.NullVLAN)
				if !existingVLANs[vlanStr] {
					log.Printf("Erro: VLAN %d não existe no switch %s", cfg.NullVLAN, sourceIP)
					continue
				}
			}

			sm.configureVlan(port, cfg.NullVLAN, cfg)
			if !cfg.Sandbox {
				_, err := sm.executeCommand("write memory", cfg)
				if err != nil {
					log.Printf("Erro ao salvar configuração: %v", err)
				} else {
					log.Printf("Configuração salva no switch %s", sourceIP)
				}
			}
		}
	}
}

func runDaemon(cfg *Config) error {
	listener := gosnmp.NewTrapListener()
	listener.Params = &gosnmp.GoSNMP{
		Port:      cfg.Switches[0].TrapPort,
		Community: "", // Comunidade definida por switch
		Version:   gosnmp.Version2c,
		Timeout:   time.Duration(2) * time.Second,
	}

	listener.OnNewTrap = func(packet *gosnmp.SnmpPacket, addr *net.UDPAddr) {
		handleTrap(packet, cfg)
	}

	log.Printf("Listener de traps iniciado na porta %d", cfg.Switches[0].TrapPort)
	err := listener.Listen(fmt.Sprintf("0.0.0.0:%d", cfg.Switches[0].TrapPort))
	if err != nil {
		return fmt.Errorf("erro ao iniciar listener de traps: %v", err)
	}

	return nil
}
