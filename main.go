package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

// OUIMapping define o mapeamento de uma VLAN para uma lista de OUIs
type OUIMapping struct {
	VLAN int      `yaml:"vlan"`
	OUIs []string `yaml:"oui"`
}

// SwitchConfig define configurações de um switch
type SwitchConfig struct {
	Target         string       `yaml:"target"`
	Community      string       `yaml:"community"`
	TrapPort       uint16       `yaml:"trap_port"`
	OUIMappings    []OUIMapping `yaml:"oui_mappings,omitempty"`
	IgnoredPorts   []string     `yaml:"ignored_ports,omitempty"`
	Username       string       `yaml:"username,omitempty"`
	Password       string       `yaml:"password,omitempty"`
	EnablePassword string       `yaml:"enable_password,omitempty"`
	CreateVLANs    bool         `yaml:"create_vlans,omitempty"`
}

// Config armazena configurações da aplicação
type Config struct {
	DefaultOUIMappings   []OUIMapping   `yaml:"default_oui_mappings"`
	EnableAccessPortfast bool           `yaml:"enable_access_portfast,omitempty"`
	NullVLAN             int            `yaml:"null_vlan"`
	PortTypes            []string       `yaml:"port_types"`
	DefaultVLAN          int            `yaml:"default_vlan"`
	Username             string         `yaml:"username,omitempty"`
	Password             string         `yaml:"password,omitempty"`
	EnablePassword       string         `yaml:"enable_password,omitempty"`
	ExcludeMacs          []string       `yaml:"exclude_macs,omitempty"`
	Switches             []SwitchConfig `yaml:"switches"`
	Sandbox              bool
	Debug                bool
	SkipVlanCheck        bool
}

func loadConfig(yamlFile string, target string, sandbox, debug, skipVlanCheck, createVLANs bool) (*Config, error) {
	data, err := ioutil.ReadFile(yamlFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read YAML file %s: %v", yamlFile, err)
	}
	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %v", err)
	}
	cfg.Sandbox = sandbox
	cfg.Debug = debug
	cfg.SkipVlanCheck = skipVlanCheck

	// Validação
	if cfg.DefaultVLAN < 1 || cfg.DefaultVLAN > 4094 {
		return nil, fmt.Errorf("invalid default_vlan: %d must be between 1 and 4094", cfg.DefaultVLAN)
	}
	if cfg.NullVLAN < 1 || cfg.NullVLAN > 4094 {
		return nil, fmt.Errorf("invalid null_vlan: %d must be between 1 and 4094", cfg.NullVLAN)
	}
	if len(cfg.PortTypes) == 0 {
		return nil, fmt.Errorf("port_types cannot be empty")
	}

	// Filtra o switch com base no target
	var selectedSwitch *SwitchConfig
	for _, sw := range cfg.Switches {
		if sw.Target == target {
			selectedSwitch = &sw
			break
		}
	}
	if selectedSwitch == nil {
		return nil, fmt.Errorf("target %s not found in switches list", target)
	}

	// Usa credenciais globais se não definidas no switch
	if selectedSwitch.Username == "" {
		selectedSwitch.Username = cfg.Username
	}
	if selectedSwitch.Password == "" {
		selectedSwitch.Password = cfg.Password
	}
	if selectedSwitch.EnablePassword == "" {
		selectedSwitch.EnablePassword = cfg.EnablePassword
	}

	// Valida campos obrigatórios
	if selectedSwitch.Target == "" || selectedSwitch.Community == "" || selectedSwitch.TrapPort == 0 {
		return nil, fmt.Errorf("target, community, and trap_port are required for switch %s", selectedSwitch.Target)
	}
	if selectedSwitch.Username == "" || selectedSwitch.Password == "" || selectedSwitch.EnablePassword == "" {
		return nil, fmt.Errorf("username, password, and enable_password are required for switch %s (set globally or per switch)", selectedSwitch.Target)
	}

	// Valida OUIMappings e verifica duplicatas dentro de cada lista
	switchOUISet := make(map[string]bool)
	for _, mapping := range selectedSwitch.OUIMappings {
		if mapping.VLAN < 1 || mapping.VLAN > 4094 {
			return nil, fmt.Errorf("invalid VLAN %d in oui_mappings for switch %s", mapping.VLAN, selectedSwitch.Target)
		}
		for _, oui := range mapping.OUIs {
			normalizedOUI := normalizeMac(oui)
			if len(normalizedOUI) != 6 {
				return nil, fmt.Errorf("OUI %s in oui_mappings for switch %s must be 6 characters (3 bytes)", oui, selectedSwitch.Target)
			}
			if switchOUISet[normalizedOUI] {
				return nil, fmt.Errorf("duplicate OUI %s in oui_mappings for switch %s", oui, selectedSwitch.Target)
			}
			switchOUISet[normalizedOUI] = true
		}
	}

	defaultOUISet := make(map[string]bool)
	for _, mapping := range cfg.DefaultOUIMappings {
		if mapping.VLAN < 1 || mapping.VLAN > 4094 {
			return nil, fmt.Errorf("invalid VLAN %d in default_oui_mappings", mapping.VLAN)
		}
		for _, oui := range mapping.OUIs {
			normalizedOUI := normalizeMac(oui)
			if len(normalizedOUI) != 6 {
				return nil, fmt.Errorf("OUI %s in default_oui_mappings must be 6 characters (3 bytes)", oui)
			}
			if defaultOUISet[normalizedOUI] {
				return nil, fmt.Errorf("duplicate OUI %s in default_oui_mappings", oui)
			}
			defaultOUISet[normalizedOUI] = true
		}
	}

	// Valida exclude_macs
	for i, mac := range cfg.ExcludeMacs {
		cfg.ExcludeMacs[i] = normalizeMac(mac)
	}

	// Atualiza cfg.Switches para conter apenas o switch selecionado
	cfg.Switches = []SwitchConfig{*selectedSwitch}
	return &cfg, nil
}

func main() {
	yamlFile := flag.String("y", "config.yaml", "YAML configuration file")
	write := flag.Bool("w", false, "Write changes (disable sandbox)")
	verbose := flag.Bool("v", false, "Enable debug logging")
	daemon := flag.Bool("d", false, "Run in daemon mode (SNMP trap listener)")
	target := flag.String("t", "", "Switch target to process (required, must match a target in YAML)")
	skipVlanCheck := flag.Bool("s", false, "Skip VLAN check (use with caution)")
	createVLANs := flag.Bool("c", false, "Create missing VLANs on the switch")
	help := flag.Bool("h", false, "Show this help message")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  -t <target>  Switch target to process (required, must match a target in YAML)\n")
		fmt.Fprintf(os.Stderr, "  -y <file>    YAML configuration file (default: config.yaml)\n")
		fmt.Fprintf(os.Stderr, "  -w           Write changes (disable sandbox)\n")
		fmt.Fprintf(os.Stderr, "  -v           Enable debug logging\n")
		fmt.Fprintf(os.Stderr, "  -d           Run in daemon mode (SNMP trap listener)\n")
		fmt.Fprintf(os.Stderr, "  -s           Skip VLAN check (use with caution)\n")
		fmt.Fprintf(os.Stderr, "  -c           Create missing VLANs on the switch\n")
		fmt.Fprintf(os.Stderr, "  -h           Show this help message\n")
	}
	flag.Parse()

	if *help {
		flag.Usage()
		os.Exit(0)
	}

	if *target == "" {
		log.Fatal("Flag -t is required to specify the switch target")
	}

	cfg, err := loadConfig(*yamlFile, *target, !*write, *verbose, *skipVlanCheck, *createVLANs)
	if err != nil {
		log.Fatal(err)
	}

	if *daemon {
		log.Printf("Starting daemon mode for SNMP traps on switch %s", *target)
		if err := runDaemon(cfg); err != nil {
			log.Fatal(err)
		}
	} else {
		log.Printf("Starting Negev for switch management on switch %s", *target)
		sw := cfg.Switches[0] // Apenas um switch após filtragem
		sw.CreateVLANs = *createVLANs
		sm := &SwitchManager{config: sw}
		if err := sm.processPorts(cfg); err != nil {
			log.Printf("Error processing switch %s: %v", sw.Target, err)
		}
	}
}
