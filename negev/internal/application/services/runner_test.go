package services

import (
	"errors"
	"strings"
	"testing"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
	"github.com/carlosrabelo/negev/negev/internal/infrastructure/config"
	"github.com/carlosrabelo/negev/negev/internal/infrastructure/transport"

	_ "github.com/carlosrabelo/negev/negev/internal/platform/dmos"
	_ "github.com/carlosrabelo/negev/negev/internal/platform/ios"
)

type scriptedClient struct {
	connected   bool
	prompts     []entities.AuthPrompt
	responses   map[string]string
	failConnect error
}

func (c *scriptedClient) Connect() error {
	if c.failConnect != nil {
		return c.failConnect
	}
	c.connected = true
	return nil
}

func (c *scriptedClient) Disconnect() { c.connected = false }

func (c *scriptedClient) ExecuteCommand(cmd string) (string, error) {
	if out, ok := c.responses[cmd]; ok {
		return out, nil
	}
	for key, out := range c.responses {
		if strings.HasPrefix(cmd, key) {
			return out, nil
		}
	}
	return "", nil
}

func (c *scriptedClient) IsConnected() bool { return c.connected }

func (c *scriptedClient) SetAuthSequence(prompts []entities.AuthPrompt) {
	c.prompts = prompts
}

func iosScriptedClient() *scriptedClient {
	return &scriptedClient{
		responses: map[string]string{
			"show version": `Cisco IOS Software, C2960 Software`,
			"show vlan brief": `
VLAN Name                             Status    Ports
---- -------------------------------- --------- -------------------------------
1    default                          active    Gi1/0/1
10   VLAN_10                          active
`,
			"show interfaces trunk": `
Port        Mode             Encapsulation  Status        Native vlan
Gi1/0/24    on               802.1q         trunking      1
`,
			"show interfaces status": `
Port      Name               Status       Vlan       Duplex  Speed Type
Gi1/0/1                      connected    1          a-full  a-1000 10/100/1000BaseTX
`,
			"show mac address-table dynamic": `
Vlan    Mac Address       Type        Ports
----    -----------       ----        -----
   1    aabb.ccdd.eeff    DYNAMIC     Gi1/0/1
`,
		},
	}
}

func TestRunTargetNotFound(t *testing.T) {
	cfg := &config.Config{Switches: []entities.SwitchConfig{{Target: "10.0.0.1"}}}
	svc := NewVLANApplicationService(cfg, "10.0.0.2")
	if err := svc.Run(true, 0, false); err == nil {
		t.Fatal("expected target not found error")
	}
}

func TestRunUnknownPlatform(t *testing.T) {
	cfg := &config.Config{Switches: []entities.SwitchConfig{{
		Target:   "10.0.0.1",
		Platform: "unknown-os",
	}}}
	svc := NewVLANApplicationService(cfg, "10.0.0.1")
	svc.newAdapter = func(sc entities.SwitchConfig) *transport.SwitchAdapter {
		return transport.NewSwitchAdapterWithClient(sc, iosScriptedClient())
	}
	if err := svc.Run(true, 0, false); err == nil {
		t.Fatal("expected unknown platform error")
	}
}

func TestRunIOSSandboxSuccess(t *testing.T) {
	cli := iosScriptedClient()
	cfg := &config.Config{Switches: []entities.SwitchConfig{{
		Target:      "10.0.0.1",
		Platform:    "ios",
		DefaultVlan: "10",
		MacToVlan:   map[string]string{"aabbcc": "10"},
	}}}
	svc := NewVLANApplicationService(cfg, "10.0.0.1")
	svc.newAdapter = func(sc entities.SwitchConfig) *transport.SwitchAdapter {
		return transport.NewSwitchAdapterWithClient(sc, cli)
	}
	if err := svc.Run(true, 1, false); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if len(cli.prompts) == 0 {
		t.Fatal("expected auth sequence to be applied")
	}
	if cfg.Switches[0].Sandbox != true || cfg.Switches[0].VerbosityLevel != 1 {
		t.Fatalf("expected sandbox/verbosity flags applied, got %+v", cfg.Switches[0])
	}
}

func TestRunAutoDetectSuccess(t *testing.T) {
	cli := iosScriptedClient()
	cfg := &config.Config{Switches: []entities.SwitchConfig{{
		Target:      "10.0.0.1",
		Platform:    "auto",
		DefaultVlan: "10",
		MacToVlan:   map[string]string{"aabbcc": "10"},
	}}}
	svc := NewVLANApplicationService(cfg, "10.0.0.1")
	svc.newAdapter = func(sc entities.SwitchConfig) *transport.SwitchAdapter {
		return transport.NewSwitchAdapterWithClient(sc, cli)
	}
	if err := svc.Run(true, 0, false); err != nil {
		t.Fatalf("Run auto failed: %v", err)
	}
}

func TestRunAutoDetectConnectFailure(t *testing.T) {
	cli := &scriptedClient{failConnect: errors.New("unreachable")}
	cfg := &config.Config{Switches: []entities.SwitchConfig{{
		Target:   "10.0.0.1",
		Platform: "auto",
	}}}
	svc := NewVLANApplicationService(cfg, "10.0.0.1")
	svc.newAdapter = func(sc entities.SwitchConfig) *transport.SwitchAdapter {
		return transport.NewSwitchAdapterWithClient(sc, cli)
	}
	if err := svc.Run(true, 0, false); err == nil {
		t.Fatal("expected auto-detect connect failure")
	}
}

func TestRunAutoDetectNoMatch(t *testing.T) {
	cli := &scriptedClient{
		responses: map[string]string{
			"show version": "SomeUnknownOS 1.0",
		},
	}
	cfg := &config.Config{Switches: []entities.SwitchConfig{{
		Target:   "10.0.0.1",
		Platform: "auto",
	}}}
	svc := NewVLANApplicationService(cfg, "10.0.0.1")
	svc.newAdapter = func(sc entities.SwitchConfig) *transport.SwitchAdapter {
		return transport.NewSwitchAdapterWithClient(sc, cli)
	}
	if err := svc.Run(true, 0, false); err == nil {
		t.Fatal("expected platform detection failure")
	}
}
