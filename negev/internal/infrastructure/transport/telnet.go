package transport

import (
	"fmt"
	"log/slog"
	"net"
	"strings"
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
	tc.conn.SetReadDeadline(time.Now().Add(DefaultTimeout))
	tc.conn.SetWriteDeadline(time.Now().Add(DefaultTimeout))
	if tc.config.IsDebugEnabled() {
		fmt.Printf("DEBUG: Connected to %s\n", tc.config.Target)
	}
	if err := tc.authenticate(); err != nil {
		tc.Disconnect()
		return err
	}
	return nil
}

func (tc *TelnetClient) authenticate() error {
	if len(tc.authSequence) > 0 {
		return tc.runAuthSequence(tc.authSequence)
	}
	return tc.defaultAuthenticate()
}

func (tc *TelnetClient) defaultAuthenticate() error {
	output, err := tc.readUntilAny([]string{PromptUsername, "login:", PromptPrivileged, PromptEnable}, DefaultTimeout)
	if err != nil {
		return fmt.Errorf("failed to wait for login prompt: %v, output: %s", err, output)
	}
	switch {
	case strings.Contains(output, "login:"):
		return tc.runAuthSequence([]entities.AuthPrompt{
			{WaitFor: "", SendCmd: tc.config.Username + "\n"},
			{WaitFor: PromptPassword, SendCmd: tc.config.Password + "\n"},
			{WaitFor: PromptPrivileged, SendCmd: TerminalLengthCmd},
			{WaitFor: PromptPrivileged, SendCmd: ""},
		})
	case strings.Contains(output, PromptUsername):
		return tc.runAuthSequence([]entities.AuthPrompt{
			{WaitFor: "", SendCmd: tc.config.Username + "\n"},
			{WaitFor: PromptPassword, SendCmd: tc.config.Password + "\n"},
			{WaitFor: PromptEnable, SendCmd: "enable\n"},
			{WaitFor: PromptPassword, SendCmd: tc.config.EnablePassword + "\n"},
			{WaitFor: PromptPrivileged, SendCmd: TerminalLengthCmd},
			{WaitFor: PromptPrivileged, SendCmd: ""},
		})
	case strings.Contains(output, PromptPrivileged):
		return tc.runAuthSequence([]entities.AuthPrompt{
			{WaitFor: "", SendCmd: TerminalLengthCmd},
			{WaitFor: PromptPrivileged, SendCmd: ""},
		})
	case strings.Contains(output, PromptEnable):
		return tc.runAuthSequence([]entities.AuthPrompt{
			{WaitFor: "", SendCmd: "enable\n"},
			{WaitFor: PromptPassword, SendCmd: tc.config.EnablePassword + "\n"},
			{WaitFor: PromptPrivileged, SendCmd: TerminalLengthCmd},
			{WaitFor: PromptPrivileged, SendCmd: ""},
		})
	default:
		return fmt.Errorf("unrecognized login banner: %s", output)
	}
}

func (tc *TelnetClient) runAuthSequence(prompts []entities.AuthPrompt) error {
	for _, p := range prompts {
		cmd := p.SendCmd
		if p.WaitFor != "" {
			output, err := tc.readUntil(p.WaitFor, DefaultTimeout)
			if err != nil {
				return fmt.Errorf("failed to wait for %s: %v, output: %s", p.WaitFor, err, output)
			}
		}
		if cmd != "" {
			_ = tc.conn.SetWriteDeadline(time.Now().Add(DefaultTimeout))
			if _, err := tc.conn.Write([]byte(cmd)); err != nil {
				return fmt.Errorf("failed to send auth command for prompt %s: %v", p.WaitFor, err)
			}
		}
	}
	return nil
}

func (tc *TelnetClient) readUntilAny(patterns []string, timeout time.Duration) (string, error) {
	buffer := make([]byte, BufferSize)
	var output strings.Builder
	output.Grow(BufferSize)
	deadline := time.Now().Add(timeout)
	_ = tc.conn.SetReadDeadline(deadline)
	for {
		n, err := tc.conn.Read(buffer)
		if n > 0 {
			output.Write(buffer[:n])
			got := output.String()
			for _, pattern := range patterns {
				if pattern != "" && strings.Contains(got, pattern) {
					return got, nil
				}
			}
		}
		if err != nil {
			return output.String(), fmt.Errorf("read error: %v", err)
		}
		if time.Now().After(deadline) {
			break
		}
	}
	return output.String(), fmt.Errorf("timeout waiting for %v", patterns)
}

func (tc *TelnetClient) readUntil(pattern string, timeout time.Duration) (string, error) {
	buffer := make([]byte, BufferSize)
	var output strings.Builder
	output.Grow(BufferSize)
	deadline := time.Now().Add(timeout)
	_ = tc.conn.SetReadDeadline(deadline)
	for {
		n, err := tc.conn.Read(buffer)
		if n > 0 {
			output.Write(buffer[:n])
			if strings.Contains(output.String(), pattern) {
				return output.String(), nil
			}
		}
		if err != nil {
			return output.String(), fmt.Errorf("read error: %v", err)
		}
		if time.Now().After(deadline) {
			break
		}
	}
	return output.String(), fmt.Errorf("timeout waiting for %s", pattern)
}

func (tc *TelnetClient) Disconnect() {
	if tc.conn != nil {
		tc.conn.Close()
		if tc.config.IsDebugEnabled() {
			fmt.Println("DEBUG: Disconnected")
		}
		tc.conn = nil
	}
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

func (tc *TelnetClient) ExecuteCommand(cmd string) (string, error) {
	if tc.conn == nil {
		return "", fmt.Errorf("not connected")
	}
	if tc.config.IsDebugEnabled() {
		fmt.Printf("DEBUG: Executing: %s\n", cmd)
	}
	_ = tc.conn.SetWriteDeadline(time.Now().Add(DefaultTimeout))
	if _, err := tc.conn.Write([]byte(cmd + "\n")); err != nil {
		return "", fmt.Errorf("failed to send command %s: %v", cmd, err)
	}
	output, err := tc.readUntil(PromptPrivileged, DefaultTimeout)
	if err != nil {
		return "", fmt.Errorf("error executing %s: %v", cmd, err)
	}
	lines := strings.Split(output, "\n")
	if len(lines) > 1 {
		output = strings.Join(lines[1:len(lines)-1], "\n")
	} else {
		output = ""
	}
	return output, nil
}

func (tc *TelnetClient) IsConnected() bool {
	return tc.conn != nil
}
