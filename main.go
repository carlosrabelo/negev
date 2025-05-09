package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
)

func main() {
	yamlFile := flag.String("y", "config.yaml", "Arquivo de configuração YAML")
	write := flag.Bool("w", false, "Gravar alterações (desativa sandbox)")
	verbose := flag.Bool("v", false, "Ativar logs detalhados (debug)")
	daemon := flag.Bool("d", false, "Ativar modo daemon para SNMP traps")
	host := flag.String("t", "", "Alvo do switch (deve corresponder a um target no YAML)")
	skipVlanCheck := flag.Bool("s", false, "Ignorar verificação de VLAN (use com cautela)")
	createVLANs := flag.Bool("c", false, "Criar VLANs ausentes no switch")
	flag.Parse()

	// Determinar o caminho do arquivo de configuração
	configPath := *yamlFile
	if runtime.GOOS == "linux" && *yamlFile == "config.yaml" {
		// Em Linux, se o caminho padrão não foi sobrescrito, procurar em locais específicos
		possiblePaths := []string{
			filepath.Join(".", "config.yaml"), // Diretório local
		}

		// Adicionar caminho do usuário (~/.config/negev/config.yaml)
		if userConfigDir, err := os.UserConfigDir(); err == nil {
			possiblePaths = append(possiblePaths, filepath.Join(userConfigDir, "negev", "config.yaml"))
		}

		// Adicionar caminho global (/etc/negev/config.yaml)
		possiblePaths = append(possiblePaths, "/etc/negev/config.yaml")

		// Tentar encontrar o primeiro arquivo existente
		found := false
		for _, path := range possiblePaths {
			if _, err := os.Stat(path); err == nil {
				configPath = path
				found = true
				if *verbose {
					fmt.Printf("DEBUG: Arquivo de configuração encontrado em %s\n", path)
				}
				break
			}
		}

		if !found {
			log.Fatal("Erro: Nenhum arquivo config.yaml encontrado em ./, ~/.config/negev/ ou /etc/negev/")
		}
	}

	cfg, err := loadConfig(configPath, !*write, *verbose, *skipVlanCheck, *createVLANs)
	if err != nil {
		log.Fatal(err)
	}

	if *daemon {
		// Modo daemon: escuta SNMP traps
		fmt.Println("Iniciando Negev em modo daemon para SNMP traps...")
		err = RunSNMP(cfg, *verbose)
		if err != nil {
			log.Fatal(err)
		}
		return
	}

	// Modo normal: processa switches
	if *host != "" {
		found := false
		for _, switchCfg := range cfg.Switches {
			if switchCfg.Target == *host {
				found = true
				fmt.Printf("Iniciando Negev para switch %s\n", switchCfg.Target)
				sm := NewSwitchManager(switchCfg, *cfg)
				err = sm.ProcessPorts()
				if err != nil {
					log.Printf("Erro ao processar switch %s: %v", switchCfg.Target, err)
				}
				break
			}
		}
		if !found {
			fmt.Fprintf(os.Stderr, "Erro: target %s não registrado na configuração YAML\n", *host)
			os.Exit(1)
		}
		return
	}

	// Processa todos os switches se -t não for fornecido
	for _, switchCfg := range cfg.Switches {
		fmt.Printf("Iniciando Negev para switch %s\n", switchCfg.Target)
		sm := NewSwitchManager(switchCfg, *cfg)
		err = sm.ProcessPorts()
		if err != nil {
			log.Printf("Erro ao processar switch %s: %v", switchCfg.Target, err)
			continue
		}
	}
}
