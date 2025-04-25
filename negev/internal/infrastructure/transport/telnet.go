package transport

import (
	"fmt"
	"log/slog"
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

func (tc *TelnetClient) SetAuthSequence(prompts []entities.AuthPrompt) {
	tc.authSequence = prompts
}

func (tc *TelnetClient) Connect() error {
	if tc.conn != nil {
		return nil
	}
	slog.Warn("Connecting via Telnet — credentials are transmitted in cleartext", "target", tc.config.Target)
	conn, err := telnet.Dial("tcp", tc.config.Target+":23")
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %v", tc.config.Target, err)
	}
	tc.conn = conn
	tc.conn.SetReadDeadline(time.Now().Add(DefaultTimeout))
	tc.conn.SetWriteDeadline(time.Now().Add(DefaultTimeout))
	if tc.config.IsDebugEnabled() {
		fmt.Printf("DEBUG: Connected to %s\n", tc.config.Target)
	}

	var prompts []entities.AuthPrompt
	if len(tc.authSequence) > 0 {
		prompts = tc.authSequence
	} else {
		prompts = []entities.AuthPrompt{
			{WaitFor: PromptUsername, SendCmd: tc.config.Username + "\n"},
			{WaitFor: PromptPassword, SendCmd: tc.config.Password + "\n"},
			{WaitFor: PromptEnable, SendCmd: "enable\n"},
			{WaitFor: PromptPassword, SendCmd: tc.config.EnablePassword + "\n"},
			{WaitFor: PromptPrivileged, SendCmd: TerminalLengthCmd},
			{WaitFor: PromptPrivileged, SendCmd: ""},
		}
	}

	var resolvedPrompts []entities.AuthPrompt
	for _, p := range prompts {
		cmd := p.SendCmd
		cmd = strings.ReplaceAll(cmd, "USERNAME_PLACEHOLDER", tc.config.Username)
		cmd = strings.ReplaceAll(cmd, "ENABLE_PASSWORD_PLACEHOLDER", tc.config.EnablePassword)
		cmd = strings.ReplaceAll(cmd, "PASSWORD_PLACEHOLDER", tc.config.Password)
		resolvedPrompts = append(resolvedPrompts, entities.AuthPrompt{
			WaitFor: p.WaitFor,
			SendCmd: cmd,
		})
	}

	for _, p := range resolvedPrompts {
		output, err := tc.readUntil(p.WaitFor, DefaultTimeout)
		if err != nil {
			return fmt.Errorf("failed to wait for %s: %v, output: %s", p.WaitFor, err, output)
		}
		if p.SendCmd != "" {
			_ = tc.conn.SetWriteDeadline(time.Now().Add(DefaultTimeout))
			if _, err := tc.conn.Write([]byte(p.SendCmd)); err != nil {
				return fmt.Errorf("failed to send auth command for prompt %s: %v", p.WaitFor, err)
			}
			if tc.config.IsDebugEnabled() {
				displayCmd := strings.TrimSpace(p.SendCmd)
				if strings.Contains(strings.ToLower(p.WaitFor), "password") {
					displayCmd = "********"
				}
				fmt.Printf("DEBUG: Sent %s for prompt %s\n", displayCmd, p.WaitFor)
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
	_ = tc.conn.SetReadDeadline(deadline)
	for {
		n, err := tc.conn.Read(buffer)
		if n > 0 {
			output.Write(buffer[:n])
			if tc.config.IsRawOutputEnabled() {
				fmt.Printf("Switch output: Read: %s\n", string(buffer[:n]))
			}
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

func (tc *TelnetClient) IsConnected() bool {
	return tc.conn != nil
}

func (tc *TelnetClient) ExecuteCommand(cmd string) (string, error) {
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
	if tc.config.IsRawOutputEnabled() {
		fmt.Printf("Switch output for '%s':\n%s\n", cmd, output)
	}
	return output, nil
}
