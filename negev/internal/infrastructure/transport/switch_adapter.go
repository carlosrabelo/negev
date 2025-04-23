package transport

import (
	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
	"github.com/carlosrabelo/negev/negev/internal/domain/ports"
)

type SwitchAdapter struct {
	config entities.SwitchConfig
	client Client
}

func NewSwitchAdapter(cfg entities.SwitchConfig) *SwitchAdapter {
	return &SwitchAdapter{config: cfg}
}

func (sa *SwitchAdapter) Connect() error {
	if sa.client == nil {
		sa.client = GetClient(sa.config)
	}
	return sa.client.Connect()
}

func (sa *SwitchAdapter) Disconnect() {
	if sa.client != nil {
		sa.client.Disconnect()
	}
}

func (sa *SwitchAdapter) ExecuteCommand(cmd string) (string, error) {
	if err := sa.Connect(); err != nil {
		return "", err
	}
	return sa.client.ExecuteCommand(cmd)
}

func (sa *SwitchAdapter) IsConnected() bool {
	return sa.client != nil && sa.client.IsConnected()
}

func (sa *SwitchAdapter) SetAuthSequence(prompts []entities.AuthPrompt) {
	if sa.client == nil {
		sa.client = GetClient(sa.config)
	}
	if authCfg, ok := sa.client.(AuthConfigurable); ok {
		authCfg.SetAuthSequence(prompts)
	}
}

func (sa *SwitchAdapter) GetTarget() string {
	return sa.config.Target
}

var _ ports.SwitchRepository = (*SwitchAdapter)(nil)
var _ AuthConfigurable = (*SwitchAdapter)(nil)
