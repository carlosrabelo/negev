package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gosnmp/gosnmp"
)

const (
	SNMPPort     = 162              // Porta padrão para SNMP traps
	DebounceTime = 10 * time.Second // Tempo mínimo entre configurações de VLAN
)

// TrapState mantém o estado do último trap processado para debounce
type TrapState struct {
	lastTrapTime time.Time
	lastVlan     string
	mutex        sync.Mutex
}

// trapStates armazena o estado de traps por switch e porta
var trapStates = make(map[string]*TrapState)
var trapStateMutex sync.Mutex

// RunSNMP inicia o daemon para escutar SNMP traps e configurar VLANs
func RunSNMP(cfg *Config, verbose, extra bool) error {
	// Mapa para buscar configurações de switch por IP
	switchMap := make(map[string]SwitchConfig)
	for _, sw := range cfg.Switches {
		sw.Verbose = verbose // Aplicar verbosidade a todas as configurações de switch
		sw.Extra = extra     // Aplicar exibição de saídas brutas
		switchMap[sw.Target] = sw
	}

	// Inicializar o listener de traps SNMP
	listener := gosnmp.NewTrapListener()
	listener.Params = &gosnmp.GoSNMP{
		Port:      SNMPPort,
		Community: cfg.SnmpCommunity,
		Version:   gosnmp.Version2c,
		Timeout:   time.Duration(5) * time.Second,
		Transport: "udp", // Garantir IPv4
	}

	// Definir handler para processar traps
	listener.OnNewTrap = func(packet *gosnmp.SnmpPacket, addr *net.UDPAddr) {
		// Logar todos os traps recebidos
		if verbose {
			fmt.Printf("DEBUG: Recebido trap de %s, OID=%s\n", addr.IP.String(), packet.Variables[0].Name)
		}

		// Verificar se o IP do remetente está na lista de switches
		switchCfg, exists := switchMap[addr.IP.String()]
		if !exists {
			log.Printf("Trap de %s não registrado no YAML", addr.IP.String())
			return
		}

		// Logar a recepção do trap apenas se verbose estiver ativado
		if switchCfg.Verbose {
			fmt.Printf("DEBUG: Trap recebido de %s\n", addr.IP.String())
		}

		// Logar todas as variáveis do trap
		var macAddress string
		var port uint16
		var operation uint8
		for _, variable := range packet.Variables {
			oid := variable.Name
			valueStr := fmt.Sprintf("%v", variable.Value)
			if switchCfg.Extra {
				fmt.Printf("Saída do switch: Trap variável: OID=%s, Valor=%s\n", oid, valueStr)
			}

			// Processar apenas traps cmnMacChangedNotification
			if oid == ".1.3.6.1.6.3.1.1.4.1.0" && variable.Value == ".1.3.6.1.4.1.9.9.215.2.0.1" {
				// Procurar cmnHistMacChangedMsg
				for _, v := range packet.Variables {
					if strings.HasPrefix(v.Name, ".1.3.6.1.4.1.9.9.215.1.1.8.1.2") {
						if bytes, ok := v.Value.([]byte); ok && len(bytes) >= 11 {
							operation = bytes[0]
							// Extrair VLAN, MAC, port
							vlan := binary.BigEndian.Uint16(bytes[1:3])
							macBytes := bytes[3:9]
							macAddress = fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x",
								macBytes[0], macBytes[1], macBytes[2], macBytes[3], macBytes[4], macBytes[5])
							port = binary.BigEndian.Uint16(bytes[9:11])
							if switchCfg.Verbose {
								fmt.Printf("DEBUG: Trap processado: MAC=%s, VLAN=%d, dot1dBasePort=%d, Operação=%d\n", macAddress, vlan, port, operation)
							}
						} else {
							log.Printf("Erro: cmnHistMacChangedMsg inválido ou muito curto: %v", v.Value)
							return
						}
					}
				}

				if macAddress == "" {
					log.Printf("Erro: Não foi possível extrair MAC do trap")
					return
				}

				// Chave para rastrear o estado do trap
				trapKey := fmt.Sprintf("%s:%d", switchCfg.Target, port)

				// Obter ou criar estado do trap
				trapStateMutex.Lock()
				state, exists := trapStates[trapKey]
				if !exists {
					state = &TrapState{}
					trapStates[trapKey] = state
				}
				trapStateMutex.Unlock()

				// Verificar debounce
				state.mutex.Lock()
				defer state.mutex.Unlock()
				now := time.Now()
				if now.Sub(state.lastTrapTime) < DebounceTime && state.lastVlan != "" {
					if switchCfg.Verbose {
						fmt.Printf("DEBUG: Ignorando trap para %s devido a debounce (última VLAN: %s, tempo desde último trap: %v)\n", trapKey, state.lastVlan, now.Sub(state.lastTrapTime))
					}
					return
				}

				// Processar com base na operação
				if operation == 1 { // MAC learnt
					// Configurar VLAN para o MAC e dot1dBasePort via SNMP
					err := configureVlanForTrap(switchCfg, *cfg, macAddress, port)
					if err != nil {
						log.Printf("Erro ao configurar VLAN para MAC %s com dot1dBasePort %d do switch %s: %v", macAddress, port, switchCfg.Target, err)
					} else {
						state.lastTrapTime = now
						state.lastVlan = switchCfg.DefaultVlan
					}
				} else if operation == 2 { // MAC removed
					// Reverter a VLAN para no_data_vlan
					err := revertVlanForTrap(switchCfg, *cfg, macAddress, port)
					if err != nil {
						log.Printf("Erro ao reverter VLAN para MAC %s com dot1dBasePort %d do switch %s: %v", macAddress, port, switchCfg.Target, err)
					} else {
						state.lastTrapTime = now
						state.lastVlan = switchCfg.NoDataVlan
						// Adicionar delay para garantir aplicação antes de novos traps
						time.Sleep(2 * time.Second)
					}
				} else {
					if switchCfg.Verbose {
						fmt.Printf("DEBUG: Ignorando trap com operação desconhecida %d\n", operation)
					}
				}
			}
		}
	}

	// Iniciar o listener no IP especificado no YAML
	listenerAddress := fmt.Sprintf("%s:%d", cfg.ServerIP, SNMPPort)
	fmt.Printf("Daemon SNMP iniciado, escutando traps em %s com comunidade %s...\n", listenerAddress, cfg.SnmpCommunity)
	err := listener.Listen(listenerAddress)
	if err != nil {
		return fmt.Errorf("falha ao iniciar listener SNMP em %s: %v", listenerAddress, err)
	}

	// Manter o programa rodando
	select {}
}

// configureVlanForTrap configura a VLAN para um MAC com base no dot1dBasePort via SNMP
func configureVlanForTrap(switchCfg SwitchConfig, cfg Config, mac string, port uint16) error {
	normMac := normalizeMac(mac)
	macPrefix := normMac[:6]

	// Verificar se o MAC está excluído
	for _, excludeMac := range switchCfg.ExcludeMacs {
		if normMac == excludeMac {
			if switchCfg.Verbose {
				fmt.Printf("DEBUG: Ignorando MAC %s com dot1dBasePort %d devido à exclusão\n", mac, port)
			}
			return nil
		}
	}

	// Determinar VLAN alvo
	targetVlan := switchCfg.MacToVlan[macPrefix]
	if switchCfg.Verbose {
		fmt.Printf("DEBUG: Prefixo MAC %s mapeado para VLAN %s em MacToVlan\n", macPrefix, targetVlan)
	}
	if targetVlan == "" {
		targetVlan = switchCfg.DefaultVlan
		if switchCfg.Verbose {
			fmt.Printf("DEBUG: Nenhum mapeamento de VLAN para %s com dot1dBasePort %d, usando default_vlan do switch %s\n", mac, port, targetVlan)
			fmt.Printf("DEBUG: Valor final de default_vlan para switch %s: %s\n", switchCfg.Target, targetVlan)
		}
	}

	// Inicializar cliente SNMP
	client := &gosnmp.GoSNMP{
		Target:    switchCfg.Target,
		Port:      161,
		Community: cfg.SnmpCommunity,
		Version:   gosnmp.Version2c,
		Timeout:   time.Duration(5) * time.Second,
	}
	err := client.Connect()
	if err != nil {
		return fmt.Errorf("falha ao conectar ao switch %s via SNMP: %v", switchCfg.Target, err)
	}
	defer client.Conn.Close()

	// Obter ifIndex com retentativas
	var ifIndex int
	const maxRetries = 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		ifIndex, err = getIfIndexFromPort(client, port, switchCfg.Verbose)
		if err == nil {
			break
		}
		if switchCfg.Verbose {
			fmt.Printf("DEBUG: Tentativa %d de %d falhou ao obter ifIndex para dot1dBasePort %d: %v\n", attempt, maxRetries, port, err)
		}
		if attempt < maxRetries {
			time.Sleep(2 * time.Second)
		}
	}
	if err != nil {
		// Fallback para ifTable
		iface := fmt.Sprintf("GigabitEthernet1/0/%d", port)
		if switchCfg.Verbose {
			fmt.Printf("DEBUG: Fallback para ifTable com interface inferida %s\n", iface)
		}
		ifIndex, err = getIfIndexFromIfTable(client, iface, switchCfg.Verbose)
		if err != nil {
			return fmt.Errorf("falha ao obter ifIndex para dot1dBasePort %d ou interface %s: %v", port, iface, err)
		}
	}

	// Configurar VLAN via SNMP (usando CISCO-VLAN-MEMBERSHIP-MIB)
	vNum, err := parseVlanNumber(targetVlan)
	if err != nil {
		return fmt.Errorf("VLAN %s inválida: %v", targetVlan, err)
	}

	// Tentar configurar VLAN com retries
	const setRetries = 3
	for attempt := 1; attempt <= setRetries; attempt++ {
		oid := fmt.Sprintf(".1.3.6.1.4.1.9.9.68.1.2.2.1.2.%d", ifIndex)
		pdu := gosnmp.SnmpPDU{
			Name:  oid,
			Type:  gosnmp.Integer,
			Value: vNum,
		}
		result, err := client.Set([]gosnmp.SnmpPDU{pdu})
		if err == nil {
			if switchCfg.Verbose {
				fmt.Printf("DEBUG: Resultado do SET para VLAN %s (tentativa %d): %v\n", targetVlan, attempt, result)
			}
			break
		}
		if switchCfg.Verbose {
			fmt.Printf("DEBUG: Tentativa %d de %d falhou ao configurar VLAN %s para dot1dBasePort %d (ifIndex %d): %v\n", attempt, setRetries, targetVlan, port, ifIndex, err)
		}
		if attempt < setRetries {
			time.Sleep(2 * time.Second)
		}
	}
	if err != nil {
		return fmt.Errorf("falha ao configurar VLAN %s para dot1dBasePort %d (ifIndex %d) após %d tentativas: %v", targetVlan, port, ifIndex, setRetries, err)
	}

	// Verificar se a VLAN foi configurada corretamente
	currentVlan, err := getCurrentVlan(client, ifIndex, switchCfg.Verbose)
	if err != nil {
		log.Printf("Aviso: Falha ao verificar VLAN atual para dot1dBasePort %d (ifIndex %d): %v", port, ifIndex, err)
	} else if currentVlan != vNum {
		log.Printf("Aviso: VLAN configurada (%s) não corresponde à VLAN atual (%d) para dot1dBasePort %d (ifIndex %d)", targetVlan, currentVlan, port, ifIndex)
	} else {
		if switchCfg.Verbose {
			fmt.Printf("DEBUG: Verificado: VLAN %s configurada corretamente para dot1dBasePort %d (ifIndex %d)\n", targetVlan, port, ifIndex)
		}
	}

	// Exibir mensagem quando a VLAN for alterada
	fmt.Printf("VLAN %s configurada para a interface com dot1dBasePort %d (ifIndex %d) no switch %s\n", targetVlan, port, ifIndex, switchCfg.Target)

	if switchCfg.Verbose {
		fmt.Printf("DEBUG: Configurado VLAN %s para dot1dBasePort %d (ifIndex %d) via SNMP\n", targetVlan, port, ifIndex)
	}

	return nil
}

// revertVlanForTrap reverte a VLAN para no_data_vlan quando um dispositivo é desconectado
func revertVlanForTrap(switchCfg SwitchConfig, cfg Config, mac string, port uint16) error {
	// Determinar VLAN de quarentena
	targetVlan := switchCfg.NoDataVlan
	if switchCfg.Verbose {
		fmt.Printf("DEBUG: Valor de no_data_vlan para switch %s: %s\n", switchCfg.Target, targetVlan)
		fmt.Printf("DEBUG: Revertendo para no_data_vlan %s para MAC %s com dot1dBasePort %d\n", targetVlan, mac, port)
	}

	// Inicializar cliente SNMP
	client := &gosnmp.GoSNMP{
		Target:    switchCfg.Target,
		Port:      161,
		Community: cfg.SnmpCommunity,
		Version:   gosnmp.Version2c,
		Timeout:   time.Duration(5) * time.Second,
	}
	err := client.Connect()
	if err != nil {
		return fmt.Errorf("falha ao conectar ao switch %s via SNMP: %v", switchCfg.Target, err)
	}
	defer client.Conn.Close()

	// Obter ifIndex com retentativas
	var ifIndex int
	const maxRetries = 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		ifIndex, err = getIfIndexFromPort(client, port, switchCfg.Verbose)
		if err == nil {
			break
		}
		if switchCfg.Verbose {
			fmt.Printf("DEBUG: Tentativa %d de %d falhou ao obter ifIndex para dot1dBasePort %d: %v\n", attempt, maxRetries, port, err)
		}
		if attempt < maxRetries {
			time.Sleep(2 * time.Second)
		}
	}
	if err != nil {
		// Fallback para ifTable
		iface := fmt.Sprintf("GigabitEthernet1/0/%d", port)
		if switchCfg.Verbose {
			fmt.Printf("DEBUG: Fallback para ifTable com interface inferida %s\n", iface)
		}
		ifIndex, err = getIfIndexFromIfTable(client, iface, switchCfg.Verbose)
		if err != nil {
			return fmt.Errorf("falha ao obter ifIndex para dot1dBasePort %d ou interface %s: %v", port, iface, err)
		}
	}

	// Configurar VLAN via SNMP (usando CISCO-VLAN-MEMBERSHIP-MIB)
	vNum, err := parseVlanNumber(targetVlan)
	if err != nil {
		return fmt.Errorf("no_data_vlan %s inválida: %v", targetVlan, err)
	}

	// Tentar configurar VLAN com retries
	const setRetries = 5
	for attempt := 1; attempt <= setRetries; attempt++ {
		oid := fmt.Sprintf(".1.3.6.1.4.1.9.9.68.1.2.2.1.2.%d", ifIndex)
		pdu := gosnmp.SnmpPDU{
			Name:  oid,
			Type:  gosnmp.Integer,
			Value: vNum,
		}
		result, err := client.Set([]gosnmp.SnmpPDU{pdu})
		if err == nil {
			if switchCfg.Verbose {
				fmt.Printf("DEBUG: Resultado do SET para no_data_vlan %s (tentativa %d): %v\n", targetVlan, attempt, result)
			}
			break
		}
		if switchCfg.Verbose {
			fmt.Printf("DEBUG: Tentativa %d de %d falhou ao configurar no_data_vlan %s para dot1dBasePort %d (ifIndex %d): %v\n", attempt, setRetries, targetVlan, port, ifIndex, err)
		}
		if attempt < maxRetries {
			time.Sleep(2 * time.Second)
		}
	}
	if err != nil {
		return fmt.Errorf("falha ao reverter para no_data_vlan %s para dot1dBasePort %d (ifIndex %d) após %d tentativas: %v", targetVlan, port, ifIndex, setRetries, err)
	}

	// Verificar se a VLAN foi configurada corretamente
	currentVlan, err := getCurrentVlan(client, ifIndex, switchCfg.Verbose)
	if err != nil {
		log.Printf("Aviso: Falha ao verificar VLAN atual para dot1dBasePort %d (ifIndex %d): %v", port, ifIndex, err)
	} else if currentVlan != vNum {
		log.Printf("Aviso: VLAN configurada (%s) não corresponde à VLAN atual (%d) para dot1dBasePort %d (ifIndex %d)", targetVlan, currentVlan, port, ifIndex)
	} else {
		if switchCfg.Verbose {
			fmt.Printf("DEBUG: Verificado: no_data_vlan %s configurada corretamente para dot1dBasePort %d (ifIndex %d)\n", targetVlan, port, ifIndex)
		}
	}

	// Exibir mensagem quando a VLAN for alterada
	fmt.Printf("Revertido para no_data_vlan %s na interface com dot1dBasePort %d (ifIndex %d) no switch %s\n", targetVlan, port, ifIndex, switchCfg.Target)

	if switchCfg.Verbose {
		fmt.Printf("DEBUG: Revertido para no_data_vlan %s para dot1dBasePort %d (ifIndex %d) via SNMP\n", targetVlan, port, ifIndex)
	}

	return nil
}

// getIfIndexFromPort obtém o ifIndex correspondente ao dot1dBasePort
func getIfIndexFromPort(client *gosnmp.GoSNMP, port uint16, verbose bool) (int, error) {
	oid := fmt.Sprintf(".1.3.6.1.2.1.17.1.4.1.2.%d", port) // dot1dBasePortToIfIndex
	result, err := client.Get([]string{oid})
	if err != nil {
		return 0, fmt.Errorf("falha ao consultar dot1dBasePortTable: %v", err)
	}

	for _, v := range result.Variables {
		if v.Name == oid && v.Type == gosnmp.Integer {
			ifIndex, ok := v.Value.(int)
			if !ok {
				return 0, fmt.Errorf("ifIndex %v não é um inteiro", v.Value)
			}
			if verbose {
				fmt.Printf("DEBUG: Encontrado ifIndex %d para dot1dBasePort %d\n", ifIndex, port)
			}
			return ifIndex, nil
		}
	}

	return 0, fmt.Errorf("dot1dBasePort %d não encontrado na tabela dot1dBasePortTable", port)
}

// getIfIndexFromIfTable obtém o ifIndex da interface usando a tabela ifTable
func getIfIndexFromIfTable(client *gosnmp.GoSNMP, iface string, verbose bool) (int, error) {
	oids := []string{
		".1.3.6.1.2.1.2.2.1.2",    // ifDescr
		".1.3.6.1.2.1.31.1.1.1.1", // ifName
	}
	var ifIndex int
	for _, baseOid := range oids {
		err := client.Walk(baseOid, func(pdu gosnmp.SnmpPDU) error {
			if pdu.Type == gosnmp.OctetString {
				ifName := string(pdu.Value.([]byte))
				if verbose {
					fmt.Printf("DEBUG: Interface encontrada: OID=%s, Nome=%s\n", pdu.Name, ifName)
				}
				// Correspondência parcial para lidar com variações de formato
				if strings.Contains(strings.ToLower(ifName), strings.ToLower(strings.ReplaceAll(iface, "GigabitEthernet1/0/", "Gi1/0/"))) ||
					strings.Contains(strings.ToLower(ifName), strings.ToLower(iface)) {
					parts := strings.Split(pdu.Name, ".")
					if len(parts) > 0 {
						var err error
						ifIndex, err = strconv.Atoi(parts[len(parts)-1])
						if err != nil {
							return fmt.Errorf("falha ao extrair ifIndex do OID %s: %v", pdu.Name, err)
						}
						return fmt.Errorf("found") // Interromper o Walk
					}
				}
			}
			return nil
		})
		if err != nil && err.Error() == "found" {
			if ifIndex != 0 {
				return ifIndex, nil
			}
		} else if err != nil {
			log.Printf("Aviso: Erro ao consultar %s: %v", baseOid, err)
		}
	}

	return 0, fmt.Errorf("interface %s não encontrada na tabela ifTable", iface)
}

// getCurrentVlan obtém a VLAN atual da interface via SNMP
func getCurrentVlan(client *gosnmp.GoSNMP, ifIndex int, verbose bool) (int, error) {
	oid := fmt.Sprintf(".1.3.6.1.4.1.9.9.68.1.2.2.1.2.%d", ifIndex)
	result, err := client.Get([]string{oid})
	if err != nil {
		return 0, fmt.Errorf("falha ao consultar VLAN atual: %v", err)
	}

	for _, v := range result.Variables {
		if v.Name == oid && v.Type == gosnmp.Integer {
			vlan, ok := v.Value.(int)
			if !ok {
				return 0, fmt.Errorf("VLAN atual %v não é um inteiro", v.Value)
			}
			if verbose {
				fmt.Printf("DEBUG: VLAN atual para ifIndex %d: %d\n", ifIndex, vlan)
			}
			return vlan, nil
		}
	}

	return 0, fmt.Errorf("VLAN atual não encontrada para ifIndex %d", ifIndex)
}

// parseVlanNumber converte a string da VLAN para um número
func parseVlanNumber(vlan string) (int, error) {
	vlanNum, err := strconv.Atoi(vlan)
	if err != nil {
		return 0, fmt.Errorf("número de VLAN inválido: %v", err)
	}
	if vlanNum < 1 || vlanNum > 4094 {
		return 0, fmt.Errorf("número de VLAN %d fora do intervalo permitido (1-4094)", vlanNum)
	}
	return vlanNum, nil
}
