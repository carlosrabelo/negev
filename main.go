package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/ziutek/telnet"
	"gopkg.in/yaml.v3"
)

// Config armazena as configurações do switch
type Config struct {
	Host           string            `yaml:"host"`
	Username       string            `yaml:"username"`
	Password       string            `yaml:"password"`
	EnablePassword string            `yaml:"enable_password"`
	Sandbox        bool              // Controlado por -x, padrão true
	Debug          bool              // Controlado por -d, padrão false
	YamlFile       string            `yaml:"yaml_file"`
	DefaultVlan    string            `yaml:"default_vlan"`
	MacToVlan      map[string]string `yaml:"mac_to_vlan"`
	ExcludeMacs    []string          `yaml:"exclude_macs"` // Lista de MACs a excluir
	ReplaceVlan    string            // Controlado por -rv, formato "old,new"
}

// Device representa um dispositivo na tabela MAC
type Device struct {
	Vlan      string
	Mac       string
	MacFull   string
	Interface string
}

// CiscoSwitchManager gerencia a interação com o switch
type CiscoSwitchManager struct {
	config Config
	conn   *telnet.Conn
}

// readUntil lê até encontrar um padrão ou atingir timeout
func (m *CiscoSwitchManager) readUntil(pattern string, timeout time.Duration) (string, error) {
	buffer := make([]byte, 4096)
	var output strings.Builder
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		n, err := m.conn.Read(buffer)
		if err != nil {
			return output.String(), fmt.Errorf("erro ao ler saída: %v", err)
		}
		if n > 0 {
			output.Write(buffer[:n])
			if m.config.Debug {
				fmt.Printf("[DEBUG] Dados lidos: %s\n", string(buffer[:n]))
			}
		}
		if strings.Contains(output.String(), pattern) {
			return output.String(), nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return output.String(), fmt.Errorf("timeout de %v esperando por '%s'", timeout, pattern)
}

func NewCiscoSwitchManager(config Config) (*CiscoSwitchManager, error) {
	data, err := os.ReadFile(config.YamlFile)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler arquivo YAML %s: %v", config.YamlFile, err)
	}

	var yamlConfig Config
	err = yaml.Unmarshal(data, &yamlConfig)
	if err != nil {
		return nil, fmt.Errorf("erro ao parsear YAML: %v", err)
	}

	// Aplicar valores do YAML
	yamlConfig.YamlFile = config.YamlFile

	// Garantir que as flags de linha de comando tenham precedência
	yamlConfig.Debug = config.Debug
	yamlConfig.Sandbox = config.Sandbox
	// Sobrescrever o host do YAML com o valor da flag -h, se fornecido
	if config.Host != "" {
		yamlConfig.Host = config.Host
	}
	// Aplicar a substituição de VLAN, se fornecida via -rv
	if config.ReplaceVlan != "" {
		parts := strings.Split(config.ReplaceVlan, ",")
		if len(parts) != 2 {
			return nil, fmt.Errorf("formato inválido para -rv, use 'old,new' (ex.: '10,100')")
		}
		oldVlan, newVlan := parts[0], parts[1]
		if yamlConfig.DefaultVlan == oldVlan {
			yamlConfig.DefaultVlan = newVlan
			if yamlConfig.Debug {
				fmt.Printf("[DEBUG] Substituindo default_vlan %s por %s\n", oldVlan, newVlan)
			}
		}
		for mac, vlan := range yamlConfig.MacToVlan {
			if vlan == oldVlan {
				yamlConfig.MacToVlan[mac] = newVlan
				if yamlConfig.Debug {
					fmt.Printf("[DEBUG] Substituindo VLAN %s por %s para MAC prefix %s\n", oldVlan, newVlan, mac)
				}
			}
		}
	}

	if yamlConfig.Host == "" || yamlConfig.Username == "" || yamlConfig.Password == "" || yamlConfig.EnablePassword == "" {
		return nil, fmt.Errorf("host, username, password e enable_password são obrigatórios (verifique o YAML ou forneça via -h)")
	}

	return &CiscoSwitchManager{
		config: yamlConfig,
	}, nil
}

func (m *CiscoSwitchManager) connect() error {
	conn, err := telnet.Dial("tcp", m.config.Host+":23")
	if err != nil {
		return fmt.Errorf("falha ao conectar ao switch %s via Telnet: %v", m.config.Host, err)
	}
	m.conn = conn
	m.conn.SetReadDeadline(time.Now().Add(30 * time.Second)) // Aumentado para 30 segundos
	m.conn.SetWriteDeadline(time.Now().Add(30 * time.Second))

	if m.config.Debug {
		fmt.Printf("[DEBUG] Conectado ao switch %s via Telnet\n", m.config.Host)
	}

	// Login
	output, err := m.readUntil("Username:", 30*time.Second)
	if err != nil {
		return fmt.Errorf("falha ao esperar prompt 'Username:': %v, saída parcial: %s", err, output)
	}
	if m.config.Debug {
		fmt.Printf("[DEBUG] Prompt 'Username:' recebido\n")
	}
	m.conn.Write([]byte(m.config.Username + "\n"))

	output, err = m.readUntil("Password:", 30*time.Second)
	if err != nil {
		return fmt.Errorf("falha ao esperar prompt 'Password:' após username: %v, saída parcial: %s", err, output)
	}
	if m.config.Debug {
		fmt.Printf("[DEBUG] Prompt 'Password:' recebido\n")
	}
	m.conn.Write([]byte(m.config.Password + "\n"))

	output, err = m.readUntil(">", 30*time.Second)
	if err != nil {
		return fmt.Errorf("falha ao esperar prompt '>' após login: %v, saída parcial: %s", err, output)
	}
	if m.config.Debug {
		fmt.Printf("[DEBUG] Prompt '>' recebido após login\n")
	}

	m.conn.Write([]byte("enable\n"))
	output, err = m.readUntil("Password:", 30*time.Second)
	if err != nil {
		return fmt.Errorf("falha ao esperar prompt 'Password:' para enable: %v, saída parcial: %s", err, output)
	}
	if m.config.Debug {
		fmt.Printf("[DEBUG] Prompt 'Password:' recebido para enable\n")
	}
	m.conn.Write([]byte(m.config.EnablePassword + "\n"))

	output, err = m.readUntil("#", 30*time.Second)
	if err != nil {
		return fmt.Errorf("falha ao esperar prompt '#' após enable: %v, saída parcial: %s", err, output)
	}
	if m.config.Debug {
		fmt.Printf("[DEBUG] Prompt '#' recebido após enable\n")
	}

	m.conn.Write([]byte("terminal length 0\n"))
	output, err = m.readUntil("#", 30*time.Second)
	if err != nil {
		return fmt.Errorf("falha ao esperar prompt '#' após terminal length 0: %v, saída parcial: %s", err, output)
	}
	if m.config.Debug {
		fmt.Printf("[DEBUG] Prompt '#' recebido após terminal length 0\n")
	}

	return nil
}

func (m *CiscoSwitchManager) disconnect() {
	if m.config.Debug {
		fmt.Println("[DEBUG] Desconectando do switch")
	}
	if m.conn != nil {
		m.conn.Close()
	}
}

func (m *CiscoSwitchManager) executeCommand(command string) (string, error) {
	if m.config.Debug {
		fmt.Printf("[DEBUG] Executando comando: %s\n", command)
	}
	m.conn.Write([]byte(command + "\n"))

	// Esperar pela saída até o prompt privilegiado, com timeout maior
	output, err := m.readUntil("#", 30*time.Second)
	if err != nil {
		return "", fmt.Errorf("erro ao ler saída de '%s': %v", command, err)
	}
	// Remover o comando ecoado e o prompt final da saída
	lines := strings.Split(output, "\n")
	if len(lines) > 1 {
		output = strings.Join(lines[1:len(lines)-1], "\n") // Remove a primeira (eco) e última (prompt)
	} else {
		output = ""
	}
	if m.config.Debug {
		fmt.Printf("[DEBUG] Saída bruta de '%s':\n%s\n[DEBUG FIM]\n", command, output)
	}
	return output, nil
}

func (m *CiscoSwitchManager) getDynamicMacTable() ([]Device, error) {
	output, err := m.executeCommand("show mac address-table dynamic")
	if err != nil {
		return nil, err
	}

	re := regexp.MustCompile(`(\d+)\s+([0-9A-Fa-f]{4}\.[0-9A-Fa-f]{4}\.[0-9A-Fa-f]{4})\s+\w+\s+(\S+)`)
	matches := re.FindAllStringSubmatch(output, -1)

	var devices []Device
	for _, match := range matches {
		vlan := match[1]
		mac := match[2] // ex.: 24fd.0d25.d621
		iface := match[3]
		// Converter MAC para formato xx:xx:xx:xx:xx:xx
		macNoDots := strings.ReplaceAll(mac, ".", "") // 24fd0d25d621
		var macFull strings.Builder
		for i := 0; i < len(macNoDots); i += 2 {
			if i > 0 {
				macFull.WriteString(":")
			}
			macFull.WriteString(macNoDots[i : i+2])
		}
		devices = append(devices, Device{
			Vlan:      vlan,
			Mac:       mac,
			MacFull:   macFull.String(), // ex.: 24:fd:0d:25:d6:21
			Interface: iface,
		})
	}
	return devices, nil
}

func (m *CiscoSwitchManager) getTrunkInterfaces() (map[string]bool, error) {
	output, err := m.executeCommand("show interfaces trunk")
	if err != nil {
		return nil, err
	}

	// Regex mais flexível para capturar interfaces trunk
	re := regexp.MustCompile(`(?m)^\s*([A-Za-z]+\d+\/\d+(?:\/\d+)?(?:\s+\w+)*)`)
	matches := re.FindAllStringSubmatch(output, -1)

	trunkInterfaces := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 1 {
			iface := strings.TrimSpace(match[1])
			ifaceParts := strings.Fields(iface)
			if len(ifaceParts) > 0 {
				trunkInterfaces[ifaceParts[0]] = true
			}
		}
	}
	return trunkInterfaces, nil
}

func (m *CiscoSwitchManager) setVlan(iface, vlan string) (commands []string) {
	configCommands := []string{
		"configure terminal",
		"interface " + iface,
		"switchport mode access",
		"switchport access vlan " + vlan,
		"end",
	}
	if m.config.Sandbox {
		fmt.Printf("[SANDBOX] Simulando configuração em %s para VLAN %s:\n", iface, vlan)
		for _, cmd := range configCommands {
			fmt.Printf("  %s\n", cmd)
		}
	} else {
		for _, cmd := range configCommands {
			_, err := m.executeCommand(cmd)
			if err != nil {
				log.Printf("Erro ao executar comando '%s': %v", cmd, err)
			}
		}
		commands = configCommands
		fmt.Printf("Interface %s configurada para VLAN %s\n", iface, vlan)
	}
	return commands
}

func (m *CiscoSwitchManager) processDevices() {
	if err := m.connect(); err != nil {
		log.Fatal(err)
	}
	defer m.disconnect()

	trunkInterfaces, err := m.getTrunkInterfaces()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Interfaces trunk detectadas: %v\n", trunkInterfaces)

	devices, err := m.getDynamicMacTable()
	if err != nil {
		log.Fatal(err)
	}

	validPrefixes := make(map[string]bool)
	for prefix := range m.config.MacToVlan {
		validPrefixes[prefix] = true
	}

	var commandsToExecute []string
	changed := false

	for _, device := range devices {
		iface := device.Interface
		if trunkInterfaces[iface] {
			continue // Silenciosamente ignorar portas trunk
		}

		// Verifica se o MAC completo está na lista de exclusão
		excluded := false
		for _, excludeMac := range m.config.ExcludeMacs {
			normalizedExcludeMac := strings.ToLower(strings.ReplaceAll(excludeMac, ":", ""))
			normalizedDeviceMac := strings.ToLower(strings.ReplaceAll(device.MacFull, ":", ""))
			if normalizedExcludeMac == normalizedDeviceMac {
				if m.config.Debug {
					fmt.Printf("[DEBUG] MAC %s excluído de alterações\n", device.MacFull)
				}
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}

		macNoDots := strings.ReplaceAll(device.Mac, ".", "") // 24fd0d25d621
		macPrefix := ""
		for i := 0; i < 6; i += 2 {
			if i > 0 {
				macPrefix += ":"
			}
			macPrefix += macNoDots[i : i+2]
		} // 24:fd:0d

		targetVlan := m.config.MacToVlan[macPrefix]
		if targetVlan == "" {
			targetVlan = m.config.DefaultVlan
			fmt.Printf("MAC %s não mapeado, usando VLAN padrão %s\n", device.MacFull, targetVlan)
		}

		currentVlan := device.Vlan
		if targetVlan != "" && targetVlan != currentVlan {
			fmt.Printf("Alterando %s (MAC %s) de VLAN %s para %s\n", iface, device.MacFull, currentVlan, targetVlan)
			commands := m.setVlan(iface, targetVlan)
			if !m.config.Sandbox {
				commandsToExecute = append(commandsToExecute, commands...)
				changed = true
			}
		}
	}

	// Executar write memory uma única vez no final, se houve alterações e não está em modo sandbox
	if !m.config.Sandbox && changed {
		_, err := m.executeCommand("write memory")
		if err != nil {
			log.Printf("Erro ao executar 'write memory': %v", err)
		}
	}
}

func main() {
	// Definir flags
	yamlFile := flag.String("y", "config.yaml", "Arquivo YAML com configurações")
	exec := flag.Bool("x", false, "Executa as configurações no switch (desativa sandbox)")
	debug := flag.Bool("d", false, "Ativa saída de debug do switch")
	host := flag.String("h", "", "Host do switch (sobrescreve o valor do YAML)")
	replaceVlan := flag.String("rv", "", "Substitui uma VLAN por outra (formato 'old,new', ex.: '10,100')")

	// Parsear flags
	flag.Parse()

	config := Config{
		Sandbox:     !(*exec), // Sandbox é false se -x for fornecido
		Debug:       *debug,
		YamlFile:    *yamlFile,
		Host:        *host,        // Valor inicial da flag -h, pode ser vazio
		ReplaceVlan: *replaceVlan, // Valor inicial da flag -rv, pode ser vazio
	}

	manager, err := NewCiscoSwitchManager(config)
	if err != nil {
		log.Fatal(err)
	}
	manager.processDevices()
}
