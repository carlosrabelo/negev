package transport

import (
	"fmt"
	"strings"
	"time"

	"github.com/ziutek/telnet"

	"github.com/carlosrabelo/negev/core/domain/entities"
)

const (
	DefaultTimeout    = 120 * time.Second // Increased for slow DmOS commands (switchport, MAC table)
	BufferSize        = 4096
	PromptUsername    = "Username:"
	PromptPassword    = "Password:"
	PromptEnable      = ">"
	PromptPrivileged  = "#"
	TerminalLengthCmd = "terminal length 0\n"
)

// TelnetClient manages a Telnet connection to a switch
type TelnetClient struct {
	conn         *telnet.Conn
	config       entities.SwitchConfig
	authSequence []entities.AuthPrompt
}

// NewTelnetClient creates a new Telnet client with the given configuration
func NewTelnetClient(cfg entities.SwitchConfig) *TelnetClient {
	return &TelnetClient{config: cfg}
}

// SetAuthSequence configures the authentication sequence for this client
func (tc *TelnetClient) SetAuthSequence(prompts []entities.AuthPrompt) {
	tc.authSequence = prompts
}

// Connect establishes a Telnet connection to the switch
func (tc *TelnetClient) Connect() error {
	if tc.conn != nil {
		return nil
	}
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

	// Use custom auth sequence if configured, otherwise use default IOS sequence
	var prompts []entities.AuthPrompt
	if len(tc.authSequence) > 0 {
		prompts = tc.authSequence
	} else {
		// Default to Cisco IOS authentication sequence
		prompts = []entities.AuthPrompt{
			{WaitFor: PromptUsername, SendCmd: tc.config.Username + "\n"},
			{WaitFor: PromptPassword, SendCmd: tc.config.Password + "\n"},
			{WaitFor: PromptEnable, SendCmd: "enable\n"},
			{WaitFor: PromptPassword, SendCmd: tc.config.EnablePassword + "\n"},
			{WaitFor: PromptPrivileged, SendCmd: TerminalLengthCmd},
			{WaitFor: PromptPrivileged, SendCmd: ""},
		}
	}

	for _, p := range prompts {
		output, err := tc.readUntil(p.WaitFor, DefaultTimeout)
		if err != nil {
			return fmt.Errorf("failed to wait for %s: %v, output: %s", p.WaitFor, err, output)
		}
		if p.SendCmd != "" {
			tc.conn.Write([]byte(p.SendCmd))
			if tc.config.IsDebugEnabled() {
				fmt.Printf("DEBUG: Sent %s for prompt %s\n", strings.TrimSpace(p.SendCmd), p.WaitFor)
			}
		}
	}
	return nil
}

// readUntil reads from the Telnet connection until the specified pattern is found
func (tc *TelnetClient) readUntil(pattern string, timeout time.Duration) (string, error) {
	buffer := make([]byte, BufferSize)
	var output strings.Builder
	output.Grow(BufferSize)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		n, err := tc.conn.Read(buffer)
		if err != nil {
			return output.String(), fmt.Errorf("read error: %v", err)
		}
		if n > 0 {
			output.Write(buffer[:n])
			if tc.config.IsRawOutputEnabled() {
				fmt.Printf("Switch output: Read: %s\n", string(buffer[:n]))
			}
			if strings.Contains(output.String(), pattern) {
				return output.String(), nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return output.String(), fmt.Errorf("timeout waiting for %s", pattern)
}

// Disconnect closes the Telnet connection
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

// ExecuteCommand sends a command to the switch and returns its output
func (tc *TelnetClient) ExecuteCommand(cmd string) (string, error) {
	if tc.config.IsDebugEnabled() {
		fmt.Printf("DEBUG: Executing: %s\n", cmd)
	}
	tc.conn.Write([]byte(cmd + "\n"))
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
