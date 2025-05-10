package main

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ziutek/telnet"
)

const (
	DefaultTimeout    = 30 * time.Second
	BufferSize        = 4096
	PromptUsername    = "Username:"
	PromptPassword    = "Password:"
	PromptEnable      = ">"
	PromptPrivileged  = "#"
	TerminalLengthCmd = "terminal length 0\n"
)

type TelnetClient struct {
	conn   *telnet.Conn
	config SwitchConfig
}

func NewTelnetClient(config SwitchConfig) *TelnetClient {
	return &TelnetClient{config: config}
}

func (tc *TelnetClient) Connect() error {
	conn, err := telnet.Dial("tcp", tc.config.Target+":23")
	if err != nil {
		return fmt.Errorf("falha ao conectar a %s: %v", tc.config.Target, err)
	}
	tc.conn = conn
	tc.conn.SetReadDeadline(time.Now().Add(DefaultTimeout))
	tc.conn.SetWriteDeadline(time.Now().Add(DefaultTimeout))
	if tc.config.Verbose {
		fmt.Printf("DEBUG: Conectado a %s\n", tc.config.Target)
	}
	prompts := []struct {
		prompt string
		input  string
	}{
		{PromptUsername, tc.config.Username + "\n"},
		{PromptPassword, tc.config.Password + "\n"},
		{PromptEnable, "enable\n"},
		{PromptPassword, tc.config.EnablePassword + "\n"},
		{PromptPrivileged, TerminalLengthCmd},
		{PromptPrivileged, ""},
	}
	for _, p := range prompts {
		output, err := tc.readUntil(p.prompt, DefaultTimeout)
		if err != nil {
			return fmt.Errorf("falha ao esperar por %s: %v, saída: %s", p.prompt, err, output)
		}
		if p.input != "" {
			tc.conn.Write([]byte(p.input))
			if tc.config.Verbose {
				fmt.Printf("DEBUG: Enviado %s para prompt %s\n", strings.TrimSpace(p.input), p.prompt)
			}
		}
	}
	return nil
}

func (tc *TelnetClient) readUntil(pattern string, timeout time.Duration) (string, error) {
	buffer := make([]byte, BufferSize)
	var output strings.Builder
	output.Grow(BufferSize)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		n, err := tc.conn.Read(buffer)
		if err != nil {
			return output.String(), fmt.Errorf("erro de leitura: %v", err)
		}
		if n > 0 {
			output.Write(buffer[:n])
			if tc.config.Extra {
				fmt.Printf("Saída do switch: Lido: %s\n", string(buffer[:n]))
			}
			if strings.Contains(output.String(), pattern) {
				return output.String(), nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return output.String(), fmt.Errorf("timeout esperando por %s", pattern)
}

func (tc *TelnetClient) Disconnect() {
	if tc.conn != nil {
		tc.conn.Close()
		if tc.config.Verbose {
			fmt.Println("DEBUG: Desconectado")
		}
	}
}

func (tc *TelnetClient) ExecuteCommand(cmd string) (string, error) {
	if tc.config.Verbose {
		fmt.Printf("DEBUG: Executando: %s\n", cmd)
	}
	tc.conn.Write([]byte(cmd + "\n"))
	output, err := tc.readUntil(PromptPrivileged, DefaultTimeout)
	if err != nil {
		return "", fmt.Errorf("erro ao executar %s: %v", cmd, err)
	}
	lines := strings.Split(output, "\n")
	if len(lines) > 1 {
		output = strings.Join(lines[1:len(lines)-1], "\n")
	} else {
		output = ""
	}
	if tc.config.Extra {
		fmt.Printf("Saída do switch para '%s':\n%s\n", cmd, output)
	}
	return output, nil
}

type SwitchManager struct {
	config       SwitchConfig
	globalConfig Config
	telnetClient *TelnetClient
}

func NewSwitchManager(config SwitchConfig, globalConfig Config) *SwitchManager {
	return &SwitchManager{
		config:       config,
		globalConfig: globalConfig,
		telnetClient: NewTelnetClient(config),
	}
}

func (sm *SwitchManager) ProcessPorts() error {
	if sm.config.Verbose {
		fmt.Printf("DEBUG: Configuração do switch %s: DefaultVlan=%s, ExcludeMacs=%v\n", sm.config.Target, sm.config.DefaultVlan, sm.config.ExcludeMacs)
	}
	err := sm.telnetClient.Connect()
	if err != nil {
		return err
	}
	defer sm.telnetClient.Disconnect()

	existingVLANs, err := sm.getVlanList()
	if err != nil {
		return err
	}

	if sm.config.CreateVLANs {
		requiredVLANs := sm.getRequiredVLANs()
		for vlan := range requiredVLANs {
			if !existingVLANs[vlan] {
				fmt.Printf("Criando VLAN %s no switch\n", vlan)
				if err := sm.createVLAN(vlan); err != nil {
					return err
				}
				existingVLANs[vlan] = true
			}
		}
	}

	trunks, err := sm.getTrunkInterfaces()
	if err != nil {
		return err
	}
	activePorts, err := sm.getActivePorts()
	if err != nil {
		return err
	}
	if len(activePorts) == 0 {
		fmt.Println("Nenhuma porta ativa encontrada no switch")
		return nil
	}

	devices, err := sm.getMacTable()
	if err != nil {
		return err
	}
	if len(devices) == 0 {
		fmt.Println("Nenhum dispositivo encontrado na tabela de endereços MAC")
		return nil
	}

	var commands []string
	changed := false
	for _, port := range activePorts {
		if trunks[port.Interface] {
			if sm.config.Verbose {
				fmt.Printf("DEBUG: Ignorando interface trunk %s\n", port.Interface)
			}
			continue
		}
		var portDevices []Device
		for _, d := range devices {
			if d.Interface == port.Interface {
				portDevices = append(portDevices, d)
			}
		}
		var targetVlan string
		if len(portDevices) == 0 {
			targetVlan = sm.config.DefaultVlan
			if sm.config.Verbose {
				fmt.Printf("DEBUG: Porta %s sem dispositivo ativo, usando default_vlan %s\n", port.Interface, targetVlan)
			}
		} else if len(portDevices) > 1 {
			log.Printf("Aviso: Múltiplos MACs detectados na porta %s: %v. Ignorando porta para evitar ambiguidade.",
				port.Interface, getMacList(portDevices))
			continue
		} else {
			// Um dispositivo encontrado
			dev := portDevices[0]
			normDevMac := normalizeMac(dev.MacFull)
			if sm.config.Verbose {
				fmt.Printf("DEBUG: Verificando MAC %s na porta %s contra exclude_macs: %v\n", dev.MacFull, port.Interface, sm.config.ExcludeMacs)
			}
			isExcluded := false
			for _, excludeMac := range sm.config.ExcludeMacs {
				if normDevMac == excludeMac {
					if sm.config.Verbose {
						fmt.Printf("DEBUG: MAC %s excluído, ignorando porta %s\n", dev.MacFull, port.Interface)
					}
					isExcluded = true
					break
				}
			}
			if isExcluded {
				continue
			}
			// Apenas prosseguir se o MAC não foi excluído
			if sm.config.Verbose {
				fmt.Printf("DEBUG: MAC %s não excluído, verificando mac_to_vlan para prefixo %s\n", dev.MacFull, normDevMac[:6])
			}
			macPrefix := normDevMac[:6]
			targetVlan = sm.config.MacToVlan[macPrefix]
			if targetVlan == "" || targetVlan == "0" || targetVlan == "00" {
				targetVlan = sm.config.DefaultVlan
				if sm.config.Verbose {
					if targetVlan == "0" || targetVlan == "00" {
						fmt.Printf("DEBUG: Ignorando mapeamento inválido para VLAN %s para MAC %s (prefixo %s) na porta %s, usando default_vlan do switch %s\n", targetVlan, dev.MacFull, macPrefix, port.Interface, sm.config.DefaultVlan)
					} else {
						fmt.Printf("DEBUG: Nenhum mapeamento de VLAN para %s (prefixo %s) na porta %s, usando default_vlan do switch %s\n", dev.MacFull, macPrefix, port.Interface, sm.config.DefaultVlan)
					}
				}
			} else {
				if sm.config.Verbose {
					fmt.Printf("DEBUG: MAC %s (prefixo %s) mapeado para VLAN %s na porta %s\n", dev.MacFull, macPrefix, targetVlan, port.Interface)
				}
			}
		}

		if !sm.config.SkipVlanCheck && !existingVLANs[targetVlan] {
			log.Printf("Erro: VLAN %s não existe no switch, ignorando porta %s\n", targetVlan, port.Interface)
			continue
		}

		if targetVlan != port.Vlan {
			if sm.config.Verbose {
				fmt.Printf("DEBUG: Alterando %s de VLAN %s para %s\n", port.Interface, port.Vlan, targetVlan)
			}
			cmds := sm.configureVlan(port.Interface, targetVlan)
			commands = append(commands, cmds...)
			changed = true
		} else {
			if sm.config.Verbose {
				fmt.Printf("DEBUG: Porta %s já configurada para VLAN %s, nenhuma alteração necessária\n", port.Interface, targetVlan)
			}
		}
	}
	fmt.Printf("Estado antes de gravação: Sandbox=%v, Changed=%v\n", sm.config.Sandbox, changed)
	if !sm.config.Sandbox && changed {
		_, err := sm.telnetClient.ExecuteCommand("write memory")
		if err != nil {
			log.Printf("Erro ao salvar configuração: %v", err)
		} else {
			fmt.Println("Configuração salva")
		}
	} else {
		if !changed {
			fmt.Println("Nenhuma alteração necessária")
		} else if sm.config.Sandbox {
			fmt.Println("Alterações simuladas (modo sandbox ativado, use -w para gravar)")
		}
	}
	return nil
}

func (sm *SwitchManager) getVlanList() (map[string]bool, error) {
	output, err := sm.telnetClient.ExecuteCommand("show vlan brief")
	if err != nil {
		return nil, fmt.Errorf("falha ao obter lista de VLANs: %v", err)
	}
	re := regexp.MustCompile(`(?m)^(\d+)\s+\S+`)
	matches := re.FindAllStringSubmatch(output, -1)
	vlans := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 1 {
			vlans[match[1]] = true
		}
	}
	if sm.config.Verbose {
		fmt.Printf("DEBUG: VLANs existentes: %v\n", vlans)
	}
	if len(vlans) == 0 {
		fmt.Println("Aviso: Nenhuma VLAN encontrada no switch. Pode ser necessário criar as VLANs necessárias.")
	}
	return vlans, nil
}

func (sm *SwitchManager) getTrunkInterfaces() (map[string]bool, error) {
	output, err := sm.telnetClient.ExecuteCommand("show interfaces trunk")
	if err != nil {
		return nil, fmt.Errorf("falha ao obter interfaces trunk: %v", err)
	}
	re := regexp.MustCompile(`(?m)^\s*([A-Za-z]+\d+\/\d+(?:\/\d+)?)`)
	matches := re.FindAllStringSubmatch(output, -1)
	trunks := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 1 {
			trunks[match[1]] = true
		}
	}
	if sm.config.Verbose {
		fmt.Printf("DEBUG: Interfaces trunk: %v\n", trunks)
	}
	return trunks, nil
}

func (sm *SwitchManager) getActivePorts() ([]Port, error) {
	output, err := sm.telnetClient.ExecuteCommand("show interfaces status")
	if err != nil {
		return nil, fmt.Errorf("falha ao obter status das interfaces: %v", err)
	}
	// Regex mais rígida para capturar apenas portas com status "connected"
	re := regexp.MustCompile(`(?m)^([A-Za-z]+\d+\/\d+(?:\/\d+)?)\s+(?:[^\s]*\s+)?connected\s+(\d+|trunk)\s+.*$`)
	matches := re.FindAllStringSubmatch(output, -1)
	var ports []Port
	for _, match := range matches {
		if len(match) < 3 {
			log.Printf("Aviso: Ignorando linha de interface malformada: %s", match[0])
			continue
		}
		// Verificação explícita para evitar status ambíguos
		if !strings.Contains(match[0], "connected") {
			log.Printf("Aviso: Linha %s não contém status connected, ignorando", match[0])
			continue
		}
		if sm.config.Verbose {
			fmt.Printf("DEBUG: Encontrada porta ativa %s com VLAN %s\n", match[1], match[2])
		}
		ports = append(ports, Port{
			Interface: match[1],
			Vlan:      match[2],
		})
	}
	if sm.config.Verbose {
		fmt.Printf("DEBUG: Encontradas %d portas ativas\n", len(ports))
	}
	return ports, nil
}

func (sm *SwitchManager) getMacTable() ([]Device, error) {
	output, err := sm.telnetClient.ExecuteCommand("show mac address-table dynamic")
	if err != nil {
		return nil, fmt.Errorf("falha ao obter tabela MAC: %v", err)
	}
	if sm.config.Extra {
		fmt.Printf("Saída bruta do comando 'show mac address-table dynamic':\n%s\n", output)
	}
	// Regex ajustada para ser mais flexível com espaçamento
	re := regexp.MustCompile(`(?m)^\s*(\d+)\s+([0-9A-Fa-f]{4}\.[0-9A-Fa-f]{4}\.[0-9A-Fa-f]{4})\s+DYNAMIC\s+(\S+)\s*$`)
	matches := re.FindAllStringSubmatch(output, -1)
	var devices []Device
	for _, match := range matches {
		if len(match) < 4 {
			if sm.config.Verbose {
				fmt.Printf("DEBUG: Ignorando linha malformada da tabela MAC: %s\n", match[0])
			}
			continue
		}
		vlan := match[1]
		mac := match[2]
		iface := match[3]
		// Validar VLAN
		if _, err := strconv.Atoi(vlan); err != nil {
			if sm.config.Verbose {
				fmt.Printf("DEBUG: Ignorando linha com VLAN inválido '%s' na tabela MAC: %s\n", vlan, match[0])
			}
			continue
		}
		// Validar MAC
		if !regexp.MustCompile(`^[0-9A-Fa-f]{4}\.[0-9A-Fa-f]{4}\.[0-9A-Fa-f]{4}$`).MatchString(mac) {
			if sm.config.Verbose {
				fmt.Printf("DEBUG: Ignorando linha com MAC inválido '%s' na tabela MAC: %s\n", mac, match[0])
			}
			continue
		}
		// Validar interface
		if !regexp.MustCompile(`^[A-Za-z]+\d+\/\d+(?:\/\d+)?$`).MatchString(iface) {
			if sm.config.Verbose {
				fmt.Printf("DEBUG: Ignorando linha com interface inválida '%s' na tabela MAC: %s\n", iface, match[0])
			}
			continue
		}
		macNoDots := strings.ReplaceAll(mac, ".", "")
		var macFull strings.Builder
		for i := 0; i < len(macNoDots); i += 2 {
			if i > 0 {
				macFull.WriteString(":")
			}
			macFull.WriteString(macNoDots[i : i+2])
		}
		if sm.config.Verbose {
			fmt.Printf("DEBUG: Adicionando dispositivo: VLAN=%s, MAC=%s, Interface=%s\n", vlan, macFull.String(), iface)
		}
		devices = append(devices, Device{
			Vlan:      vlan,
			Mac:       mac,
			MacFull:   macFull.String(),
			Interface: iface,
		})
	}
	if sm.config.Verbose {
		fmt.Printf("DEBUG: Encontrados %d dispositivos na tabela MAC\n", len(devices))
	}
	return devices, nil
}

func (sm *SwitchManager) configureVlan(iface, vlan string) []string {
	commands := []string{
		"configure terminal",
		"interface " + iface,
		"switchport mode access",
		"switchport access vlan " + vlan,
		"end",
	}
	if sm.config.Sandbox {
		fmt.Printf("SANDBOX: Simulando configuração para %s para VLAN %s\n", iface, vlan)
		for _, cmd := range commands {
			fmt.Printf("  %s\n", cmd)
		}
		return commands
	}
	for _, cmd := range commands {
		_, err := sm.telnetClient.ExecuteCommand(cmd)
		if err != nil {
			log.Printf("Erro ao executar %s: %v", cmd, err)
		}
	}
	fmt.Printf("Configurado %s para VLAN %s\n", iface, vlan)
	return commands
}

func (sm *SwitchManager) createVLAN(vlan string) error {
	commands := []string{
		"configure terminal",
		fmt.Sprintf("vlan %s", vlan),
		fmt.Sprintf("name VLAN_%s", vlan),
		"end",
	}
	for _, cmd := range commands {
		_, err := sm.telnetClient.ExecuteCommand(cmd)
		if err != nil {
			return fmt.Errorf("falha ao criar VLAN %s: %v", vlan, err)
		}
	}
	if sm.config.Verbose {
		fmt.Printf("DEBUG: Criada VLAN %s\n", vlan)
	}
	return nil
}

func (sm *SwitchManager) getRequiredVLANs() map[string]bool {
	requiredVLANs := make(map[string]bool)
	requiredVLANs[sm.config.DefaultVlan] = true
	for _, vlan := range sm.config.MacToVlan {
		if vlan == "0" || vlan == "00" {
			continue
		}
		requiredVLANs[vlan] = true
	}
	return requiredVLANs
}
