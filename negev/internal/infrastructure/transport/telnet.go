package transport

import (
	"fmt"
	"log/slog"
	"net"
	"os"
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
	conn           *telnet.Conn
	config         entities.SwitchConfig
	authSequence   []entities.AuthPrompt
	sessionProfile entities.SessionProfile
}

func NewTelnetClient(cfg entities.SwitchConfig) *TelnetClient {
	return &TelnetClient{config: cfg}
}

func (tc *TelnetClient) SetAuthSequence(prompts []entities.AuthPrompt) {
	tc.authSequence = prompts
}

func (tc *TelnetClient) SetSessionProfile(profile entities.SessionProfile) {
	tc.sessionProfile = profile
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
		fmt.Fprintf(os.Stderr, "DEBUG: Connected to %s\n", tc.config.Target)
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
	alreadyPrivileged := false
	sentEnable := false
	currentOutput := ""
	for _, p := range prompts {
		cmd := p.SendCmd
		cmd = strings.ReplaceAll(cmd, "USERNAME_PLACEHOLDER", tc.config.Username)
		cmd = strings.ReplaceAll(cmd, "ENABLE_PASSWORD_PLACEHOLDER", tc.config.EnablePassword)
		cmd = strings.ReplaceAll(cmd, "PASSWORD_PLACEHOLDER", tc.config.Password)

		if shouldSkipEnableStep(alreadyPrivileged, p.WaitFor, cmd, tc.config.EnablePassword) {
			continue
		}

		if p.WaitFor != "" {
			switch {
			case p.WaitFor == PromptEnable && strings.TrimSpace(cmd) == "enable":
				if !strings.Contains(currentOutput, PromptEnable) && !strings.Contains(currentOutput, PromptPrivileged) {
					var err error
					currentOutput, err = tc.readUntilAny([]string{PromptEnable, PromptPrivileged}, DefaultTimeout)
					if err != nil {
						return fmt.Errorf("failed to wait for %s: %v, output: %s", p.WaitFor, err, currentOutput)
					}
				}
				if strings.Contains(currentOutput, PromptPrivileged) {
					alreadyPrivileged = true
					continue
				}
			case p.WaitFor == PromptPassword && sentEnable:
				// enable may elevate straight to # with no password prompt
				if !strings.Contains(currentOutput, PromptPassword) && !strings.Contains(currentOutput, PromptPrivileged) {
					var err error
					currentOutput, err = tc.readUntilAny([]string{PromptPassword, PromptPrivileged}, DefaultTimeout)
					if err != nil {
						return fmt.Errorf("failed to wait for %s: %v, output: %s", p.WaitFor, err, currentOutput)
					}
				}
				if strings.Contains(currentOutput, PromptPrivileged) && !strings.Contains(currentOutput, PromptPassword) {
					alreadyPrivileged = true
					continue
				}
			default:
				if !strings.Contains(currentOutput, p.WaitFor) {
					var err error
					currentOutput, err = tc.readUntil(p.WaitFor, DefaultTimeout)
					if err != nil {
						return fmt.Errorf("failed to wait for %s: %v, output: %s", p.WaitFor, err, currentOutput)
					}
				}
			}
		}
		if cmd != "" {
			_ = tc.conn.SetWriteDeadline(time.Now().Add(DefaultTimeout))
			if _, err := tc.conn.Write([]byte(cmd)); err != nil {
				return fmt.Errorf("failed to send auth command for prompt %s: %v", p.WaitFor, err)
			}
			if tc.config.IsDebugEnabled() {
				displayCmd := strings.TrimSpace(cmd)
				if strings.Contains(strings.ToLower(p.WaitFor), "password") {
					displayCmd = "********"
				}
				fmt.Fprintf(os.Stderr, "DEBUG: Sent %s for prompt %s\n", displayCmd, p.WaitFor)
			}
			if strings.TrimSpace(cmd) == "enable" {
				sentEnable = true
			}
			currentOutput = ""
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
			if tc.config.IsRawOutputEnabled() {
				fmt.Fprintf(os.Stderr, "Switch output: Read: %s\n", string(buffer[:n]))
			}
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
			if tc.config.IsRawOutputEnabled() {
				fmt.Fprintf(os.Stderr, "Switch output: Read: %s\n", string(buffer[:n]))
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
			fmt.Fprintln(os.Stderr, "DEBUG: Disconnected")
		}
		tc.conn = nil
	}
}

func (tc *TelnetClient) IsConnected() bool {
	return tc.conn != nil
}

func (tc *TelnetClient) ExecuteCommand(cmd string) (string, error) {
	if tc.conn == nil {
		return "", fmt.Errorf("not connected")
	}
	if tc.config.IsDebugEnabled() {
		fmt.Fprintf(os.Stderr, "DEBUG: Executing: %s\n", cmd)
	}
	_ = tc.conn.SetWriteDeadline(time.Now().Add(DefaultTimeout))
	if _, err := tc.conn.Write([]byte(cmd + "\n")); err != nil {
		return "", fmt.Errorf("failed to send command %s: %v", cmd, err)
	}
	raw, err := tc.readUntilPrompt(tc.sessionProfile, DefaultTimeout)
	if err != nil {
		return "", fmt.Errorf("error executing %s: %v", cmd, err)
	}
	output := stripCommandEchoAndPrompt(raw)
	if tc.config.IsRawOutputEnabled() {
		fmt.Fprintf(os.Stderr, "Switch output for '%s':\n%s\n", cmd, output)
	}
	return output, nil
}

func (tc *TelnetClient) readUntilPrompt(profile entities.SessionProfile, timeout time.Duration) (string, error) {
	profile = effectiveProfile(profile)
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
				fmt.Fprintf(os.Stderr, "Switch output: Read: %s\n", string(buffer[:n]))
			}
			text := output.String()
			for {
				loc := pagerMatchLoc(profile, text)
				if loc == nil {
					break
				}
				_ = tc.conn.SetWriteDeadline(time.Now().Add(DefaultTimeout))
				if _, werr := tc.conn.Write([]byte(profile.PagerResponse)); werr != nil {
					return text, fmt.Errorf("failed to advance pager: %v", werr)
				}
				text = stripPager(text, loc)
				output.Reset()
				output.WriteString(text)
			}
			if promptMatched(profile, text) {
				return text, nil
			}
		}
		if err != nil {
			return output.String(), fmt.Errorf("read error: %v", err)
		}
		if time.Now().After(deadline) {
			break
		}
	}
	return output.String(), fmt.Errorf("timeout waiting for session prompt")
}
