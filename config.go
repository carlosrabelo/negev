package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type SwitchConfig struct {
	Target         string            `yaml:"target"`
	Username       string            `yaml:"username"`
	Password       string            `yaml:"password"`
	EnablePassword string            `yaml:"enable_password"`
	MacToVlan      map[string]string `yaml:"mac_to_vlan"`
	ExcludeMacs    []string          `yaml:"exclude_macs"`
	DefaultVlan    string            `yaml:"default_vlan"` // VLAN padrão por switch
	NoDataVlan     string            `yaml:"no_data_vlan"` // VLAN de quarentena por switch
	Sandbox        bool
	Debug          bool
	SkipVlanCheck  bool
	CreateVLANs    bool
}

type Config struct {
	ServerIP       string            `yaml:"server_ip"`
	Username       string            `yaml:"username"`
	Password       string            `yaml:"password"`
	EnablePassword string            `yaml:"enable_password"`
	SnmpCommunity  string            `yaml:"snmp_community"` // Comunidade SNMP
	DefaultVlan    string            `yaml:"default_vlan"`
	NoDataVlan     string            `yaml:"no_data_vlan"` // VLAN de quarentena global
	ExcludeMacs    []string          `yaml:"exclude_macs"`
	MacToVlan      map[string]string `yaml:"mac_to_vlan"`
	Switches       []SwitchConfig    `yaml:"switches"`
}

type Port struct {
	Interface string
	Vlan      string
}

type Device struct {
	Vlan      string
	Mac       string
	MacFull   string
	Interface string
}

func normalizeMac(mac string) string {
	return strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(mac, ":", ""), ".", ""))
}

func getMacList(devices []Device) []string {
	var macs []string
	for _, d := range devices {
		macs = append(macs, d.MacFull)
	}
	return macs
}

func loadConfig(yamlFile string, sandbox, debug, skipVlanCheck, createVLANs bool) (*Config, error) {
	data, err := os.ReadFile(yamlFile)
	if err != nil {
		return nil, fmt.Errorf("falha ao ler arquivo YAML %s: %v", yamlFile, err)
	}
	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, fmt.Errorf("falha ao parsear YAML: %v", err)
	}

	validateVLAN := func(vlan string, context string) error {
		vlanNum, err := strconv.Atoi(vlan)
		if err != nil {
			return fmt.Errorf("número de VLAN inválido em %s: %s deve ser um número", context, vlan)
		}
		if vlanNum < 1 || vlanNum > 4094 {
			return fmt.Errorf("número de VLAN inválido em %s: %s deve estar entre 1 e 4094", context, vlan)
		}
		return nil
	}

	// Validar server_ip
	if cfg.ServerIP == "" {
		return nil, fmt.Errorf("server_ip é obrigatório no YAML")
	}
	if net.ParseIP(cfg.ServerIP) == nil {
		return nil, fmt.Errorf("server_ip %s não é um endereço IP válido", cfg.ServerIP)
	}
	// Verificar se é IPv4
	ip := net.ParseIP(cfg.ServerIP).To4()
	if ip == nil {
		return nil, fmt.Errorf("server_ip %s deve ser um endereço IPv4", cfg.ServerIP)
	}

	// Validar snmp_community
	if cfg.SnmpCommunity == "" {
		cfg.SnmpCommunity = "public" // Valor padrão
		if debug {
			fmt.Printf("DEBUG: Nenhum snmp_community definido, usando padrão 'public'\n")
		}
	}

	// Validar default_vlan global
	if cfg.DefaultVlan == "" {
		return nil, fmt.Errorf("default_vlan global é obrigatório")
	}
	if err := validateVLAN(cfg.DefaultVlan, "default_vlan global"); err != nil {
		return nil, err
	}

	// Validar no_data_vlan global
	if cfg.NoDataVlan == "" {
		return nil, fmt.Errorf("no_data_vlan global é obrigatório")
	}
	if err := validateVLAN(cfg.NoDataVlan, "no_data_vlan global"); err != nil {
		return nil, err
	}

	// Logar valores globais
	if debug {
		fmt.Printf("DEBUG: Valores globais: ServerIP=%s, SnmpCommunity=%s, DefaultVlan=%s, NoDataVlan=%s, ExcludeMacs=%v\n", cfg.ServerIP, cfg.SnmpCommunity, cfg.DefaultVlan, cfg.NoDataVlan, cfg.ExcludeMacs)
	}

	// Validar campos globais usados como fallback
	if cfg.Username == "" {
		return nil, fmt.Errorf("username global é obrigatório")
	}
	if cfg.Password == "" {
		return nil, fmt.Errorf("password global é obrigatório")
	}
	if cfg.EnablePassword == "" {
		return nil, fmt.Errorf("enable_password global é obrigatório")
	}

	// Processar cada configuração de switch
	for i, sw := range cfg.Switches {
		// Validar campos obrigatórios
		if sw.Target == "" {
			return nil, fmt.Errorf("target é obrigatório para o switch %d", i)
		}
		if sw.Username == "" && cfg.Username == "" {
			return nil, fmt.Errorf("username é obrigatório para o switch %d", i)
		}
		if sw.Password == "" && cfg.Password == "" {
			return nil, fmt.Errorf("password é obrigatório para o switch %d", i)
		}
		if sw.EnablePassword == "" && cfg.EnablePassword == "" {
			return nil, fmt.Errorf("enable_password é obrigatório para o switch %d", i)
		}

		// Aplicar padrões globais
		if sw.Username == "" {
			sw.Username = cfg.Username
			if debug {
				fmt.Printf("DEBUG: Nenhum username definido para switch %s, usando global %s\n", sw.Target, cfg.Username)
			}
		}
		if sw.Password == "" {
			sw.Password = cfg.Password
			if debug {
				fmt.Printf("DEBUG: Nenhum password definido para switch %s, usando global %s\n", sw.Target, cfg.Password)
			}
		}
		if sw.EnablePassword == "" {
			sw.EnablePassword = cfg.EnablePassword
			if debug {
				fmt.Printf("DEBUG: Nenhum enable_password definido para switch %s, usando global %s\n", sw.Target, cfg.EnablePassword)
			}
		}
		// Alteração: Herdar exclude_macs global se a lista local estiver vazia ou não definida
		if sw.ExcludeMacs == nil || len(sw.ExcludeMacs) == 0 {
			sw.ExcludeMacs = cfg.ExcludeMacs
			if debug {
				fmt.Printf("DEBUG: Nenhum exclude_macs definido para switch %s, herdando global %v\n", sw.Target, cfg.ExcludeMacs)
			}
		}
		if sw.MacToVlan == nil {
			sw.MacToVlan = make(map[string]string)
			for k, v := range cfg.MacToVlan {
				sw.MacToVlan[k] = v
			}
			if debug {
				fmt.Printf("DEBUG: Nenhum mac_to_vlan definido para switch %s, herdando global %v\n", sw.Target, sw.MacToVlan)
			}
		}

		// Herdar default_vlan global se não especificado
		if sw.DefaultVlan == "" {
			sw.DefaultVlan = cfg.DefaultVlan
			if debug {
				fmt.Printf("DEBUG: Nenhum default_vlan definido para switch %s, herdando global %s\n", sw.Target, cfg.DefaultVlan)
			}
		}
		// Validar default_vlan do switch
		if err := validateVLAN(sw.DefaultVlan, fmt.Sprintf("default_vlan do switch %s", sw.Target)); err != nil {
			return nil, err
		}

		// Herdar no_data_vlan global se não especificado
		if sw.NoDataVlan == "" {
			sw.NoDataVlan = cfg.NoDataVlan
			if debug {
				fmt.Printf("DEBUG: Nenhum no_data_vlan definido para switch %s, herdando global %s\n", sw.Target, cfg.NoDataVlan)
			}
		}
		// Validar no_data_vlan do switch
		if err := validateVLAN(sw.NoDataVlan, fmt.Sprintf("no_data_vlan do switch %s", sw.Target)); err != nil {
			return nil, err
		}

		// Normalizar e validar mac_to_vlan
		newMacToVlan := make(map[string]string)
		for mac, vlan := range sw.MacToVlan {
			if vlan == "0" || vlan == "00" {
				if debug {
					fmt.Printf("DEBUG: Ignorando mapeamento inválido para VLAN %s no mac_to_vlan para MAC %s no switch %s\n", vlan, mac, sw.Target)
				}
				continue
			}
			if err := validateVLAN(vlan, fmt.Sprintf("mac_to_vlan para MAC %s no switch %s", mac, sw.Target)); err != nil {
				return nil, err
			}
			normalizedMac := normalizeMac(mac)
			newMacToVlan[normalizedMac[:6]] = vlan
		}
		sw.MacToVlan = newMacToVlan
		if debug {
			fmt.Printf("DEBUG: Após normalização para switch %s: MacToVlan=%v, DefaultVlan=%s, NoDataVlan=%s, ExcludeMacs=%v\n", sw.Target, sw.MacToVlan, sw.DefaultVlan, sw.NoDataVlan, sw.ExcludeMacs)
		}

		// Normalizar exclude_macs
		for j, mac := range sw.ExcludeMacs {
			sw.ExcludeMacs[j] = normalizeMac(mac)
		}

		// Aplicar flags da linha de comando
		sw.Sandbox = sandbox
		sw.Debug = debug
		sw.SkipVlanCheck = skipVlanCheck
		sw.CreateVLANs = createVLANs

		// Log final para confirmar configuração do switch
		if debug {
			fmt.Printf("DEBUG: Configuração final para switch %s: DefaultVlan=%s, NoDataVlan=%s, ExcludeMacs=%v\n", sw.Target, sw.DefaultVlan, sw.NoDataVlan, sw.ExcludeMacs)
		}

		// Atualizar a configuração do switch na lista
		cfg.Switches[i] = sw
	}

	if len(cfg.Switches) == 0 {
		return nil, fmt.Errorf("nenhum switch definido no YAML")
	}

	return &cfg, nil
}
