package transport

import (
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/ziutek/telnet"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
)

const (
	DefaultTimeout    = 120 * time.Second
	BufferSize        = 4096
	PromptUsername    = "Username:"
	PromptPassword    = "Password:"
	PromptEnable      = ">"
	PromptPrivileged  = "#"
	TerminalLengthCmd = "terminal length 0\n"
)

type TelnetClient struct {
	conn         *telnet.Conn
	config       entities.SwitchConfig
	authSequence []entities.AuthPrompt
}

func NewTelnetClient(cfg entities.SwitchConfig) *TelnetClient {
	return &TelnetClient{config: cfg}
}

func telnetDialAddr(target string) string {
	if _, _, err := net.SplitHostPort(target); err == nil {
		return target
	}
	return net.JoinHostPort(target, "23")
}

func (tc *TelnetClient) Connect() error {
	if tc.conn != nil {
		return nil
	}
	slog.Warn("Connecting via Telnet — credentials are transmitted in cleartext", "target", tc.config.Target)
	conn, err := telnet.Dial("tcp", telnetDialAddr(tc.config.Target))
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %v", tc.config.Target, err)
	}
	tc.conn = conn
	return nil
}

func (tc *TelnetClient) SetAuthSequence(prompts []entities.AuthPrompt) {
	tc.authSequence = prompts
}

func defaultIOSAuthSequence(cfg entities.SwitchConfig) []entities.AuthPrompt {
	return []entities.AuthPrompt{
		{WaitFor: PromptUsername, SendCmd: cfg.Username + "\n"},
		{WaitFor: PromptPassword, SendCmd: cfg.Password + "\n"},
		{WaitFor: PromptEnable, SendCmd: "enable\n"},
		{WaitFor: PromptPassword, SendCmd: cfg.EnablePassword + "\n"},
		{WaitFor: PromptPrivileged, SendCmd: TerminalLengthCmd},
		{WaitFor: PromptPrivileged, SendCmd: ""},
	}
}
