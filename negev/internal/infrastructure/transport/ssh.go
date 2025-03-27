package transport

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
)

type SSHClient struct {
	config         entities.SwitchConfig
	client         *ssh.Client
	session        *ssh.Session
	stdin          io.WriteCloser
	reader         *bufio.Reader
	netConn        net.Conn
	authSequence   []entities.AuthPrompt
	sessionProfile entities.SessionProfile
}

func NewSSHClient(cfg entities.SwitchConfig) *SSHClient {
	return &SSHClient{config: cfg}
}

func (sc *SSHClient) SetAuthSequence(prompts []entities.AuthPrompt) {
	sc.authSequence = prompts
}

func (sc *SSHClient) SetSessionProfile(profile entities.SessionProfile) {
	sc.sessionProfile = profile
}

func sshAuthMethods(password string) []ssh.AuthMethod {
	return []ssh.AuthMethod{
		ssh.Password(password),
		ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
			answers := make([]string, len(questions))
			for i := range questions {
				answers[i] = password
			}
			return answers, nil
		}),
	}
}

func (sc *SSHClient) Connect() error {
	if sc.IsConnected() {
		return nil
	}
	addr := net.JoinHostPort(sc.config.Target, "22")
	slog.Warn("SSH host key verification is disabled — use only on trusted networks", "target", sc.config.Target)
	sshConfig := &ssh.ClientConfig{
		User:            sc.config.Username,
		Auth:            sshAuthMethods(sc.config.Password),
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec
		Timeout:         DefaultTimeout,
	}

	dialer := &net.Dialer{Timeout: DefaultTimeout}
	rawConn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to connect to %s via SSH: %v", sc.config.Target, err)
	}

	clientConn, chans, reqs, err := ssh.NewClientConn(rawConn, addr, sshConfig)
	if err != nil {
		rawConn.Close()
		return fmt.Errorf("failed to establish SSH client connection to %s: %v", sc.config.Target, err)
	}

	client := ssh.NewClient(clientConn, chans, reqs)

	session, err := client.NewSession()
	if err != nil {
		client.Close()
		rawConn.Close()
		return fmt.Errorf("failed to create SSH session for %s: %v", sc.config.Target, err)
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          0,
		ssh.TTY_OP_ISPEED: 9600,
		ssh.TTY_OP_OSPEED: 9600,
	}
	if err := session.RequestPty("vt100", 80, 40, modes); err != nil {
		session.Close()
		client.Close()
		rawConn.Close()
		return fmt.Errorf("failed to request PTY for %s: %v", sc.config.Target, err)
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		session.Close()
		client.Close()
		rawConn.Close()
		return fmt.Errorf("failed to get stdin pipe for %s: %v", sc.config.Target, err)
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		client.Close()
		rawConn.Close()
		return fmt.Errorf("failed to get stdout pipe for %s: %v", sc.config.Target, err)
	}

	if err := session.Shell(); err != nil {
		session.Close()
		client.Close()
		rawConn.Close()
		return fmt.Errorf("failed to start shell for %s: %v", sc.config.Target, err)
	}

	sc.client = client
	sc.session = session
	sc.stdin = stdin
	sc.reader = bufio.NewReader(stdout)
	sc.netConn = rawConn

	if sc.config.IsDebugEnabled() {
		fmt.Printf("DEBUG: Connected to %s via SSH\n", sc.config.Target)
	}

	initial, err := sc.readUntilAny([]string{PromptPrivileged, PromptEnable}, DefaultTimeout)
	if err != nil {
		sc.Disconnect()
		return err
	}

	if len(sc.authSequence) > 0 {
		var prompts []entities.AuthPrompt
		for _, p := range sc.authSequence {
			if strings.Contains(p.SendCmd, "USERNAME_PLACEHOLDER") || (strings.Contains(p.SendCmd, "PASSWORD_PLACEHOLDER") && !strings.Contains(p.SendCmd, "ENABLE_PASSWORD_PLACEHOLDER")) {
				continue
			}
			cmd := p.SendCmd
			cmd = strings.ReplaceAll(cmd, "USERNAME_PLACEHOLDER", sc.config.Username)
			cmd = strings.ReplaceAll(cmd, "ENABLE_PASSWORD_PLACEHOLDER", sc.config.EnablePassword)
			cmd = strings.ReplaceAll(cmd, "PASSWORD_PLACEHOLDER", sc.config.Password)
			prompts = append(prompts, entities.AuthPrompt{
				WaitFor: p.WaitFor,
				SendCmd: cmd,
			})
		}

		alreadyPrivileged := strings.Contains(initial, PromptPrivileged)
		currentOutput := initial
		for _, p := range prompts {
			if shouldSkipSSHEnableStep(alreadyPrivileged, p.WaitFor, p.SendCmd, sc.config.EnablePassword) {
				continue
			}
			if !strings.Contains(currentOutput, p.WaitFor) {
				var err error
				currentOutput, err = sc.readUntil(p.WaitFor, DefaultTimeout)
				if err != nil {
					sc.Disconnect()
					return fmt.Errorf("failed to wait for %s: %v, output: %s", p.WaitFor, err, currentOutput)
				}
			}
			if p.SendCmd != "" {
				if err := sc.send(p.SendCmd); err != nil {
					sc.Disconnect()
					return fmt.Errorf("failed to send command %s: %v", p.SendCmd, err)
				}
				if sc.config.IsDebugEnabled() {
					displayCmd := strings.TrimSpace(p.SendCmd)
					if strings.Contains(strings.ToLower(p.WaitFor), "password") {
						displayCmd = "********"
					}
					fmt.Printf("DEBUG: Sent %s for prompt %s\n", displayCmd, p.WaitFor)
				}
				currentOutput = ""
			}
		}
	} else {
		if !strings.Contains(initial, PromptPrivileged) {
			if sc.config.IsDebugEnabled() {
				fmt.Printf("DEBUG: Elevating to privileged mode on %s\n", sc.config.Target)
			}
			if err := sc.send("enable\n"); err != nil {
				sc.Disconnect()
				return fmt.Errorf("failed to send enable command to %s: %v", sc.config.Target, err)
			}
			if _, err := sc.readUntil(PromptPassword, DefaultTimeout); err != nil {
				sc.Disconnect()
				return err
			}
			if err := sc.send(sc.config.EnablePassword + "\n"); err != nil {
				sc.Disconnect()
				return fmt.Errorf("failed to send enable password to %s: %v", sc.config.Target, err)
			}
			if _, err := sc.readUntil(PromptPrivileged, DefaultTimeout); err != nil {
				sc.Disconnect()
				return err
			}
		}

		if err := sc.send(TerminalLengthCmd); err != nil {
			sc.Disconnect()
			return fmt.Errorf("failed to send terminal length command to %s: %v", sc.config.Target, err)
		}
		if _, err := sc.readUntil(PromptPrivileged, DefaultTimeout); err != nil {
			sc.Disconnect()
			return err
		}
	}

	return nil
}

func (sc *SSHClient) Disconnect() {
	if sc.session != nil {
		sc.session.Close()
		sc.session = nil
	}
	if sc.client != nil {
		sc.client.Close()
		sc.client = nil
	}
	if sc.netConn != nil {
		sc.netConn.Close()
		sc.netConn = nil
	}
	sc.stdin = nil
	sc.reader = nil
	if sc.config.IsDebugEnabled() {
		fmt.Println("DEBUG: Disconnected")
	}
}

func (sc *SSHClient) IsConnected() bool {
	return sc.session != nil && sc.client != nil
}

func (sc *SSHClient) ExecuteCommand(cmd string) (string, error) {
	if sc.config.IsDebugEnabled() {
		fmt.Printf("DEBUG: Executing: %s\n", cmd)
	}
	if sc.netConn != nil {
		_ = sc.netConn.SetWriteDeadline(time.Now().Add(DefaultTimeout))
	}
	if err := sc.send(cmd + "\n"); err != nil {
		return "", fmt.Errorf("failed to send command %s: %v", cmd, err)
	}

	raw, err := sc.readUntilPrompt(sc.sessionProfile, DefaultTimeout)
	if err != nil {
		return "", fmt.Errorf("error executing %s: %v", cmd, err)
	}
	output := stripCommandEchoAndPrompt(raw)

	if sc.config.IsRawOutputEnabled() {
		fmt.Printf("Switch output for '%s':\n%s\n", cmd, output)
	}

	return output, nil
}

func (sc *SSHClient) readUntilPrompt(profile entities.SessionProfile, timeout time.Duration) (string, error) {
	profile = effectiveProfile(profile)
	buffer := make([]byte, BufferSize)
	var output strings.Builder
	output.Grow(BufferSize)
	deadline := time.Now().Add(timeout)

	if sc.netConn != nil {
		_ = sc.netConn.SetReadDeadline(deadline)
	}

	for {
		n, err := sc.reader.Read(buffer)
		if n > 0 {
			output.Write(buffer[:n])
			if sc.config.IsRawOutputEnabled() {
				fmt.Printf("Switch output: Read: %s\n", string(buffer[:n]))
			}
			text := output.String()
			for {
				loc := pagerMatchLoc(profile, text)
				if loc == nil {
					break
				}
				if werr := sc.send(profile.PagerResponse); werr != nil {
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
			return output.String(), fmt.Errorf("timeout waiting for session prompt")
		}
	}
}

func shouldSkipSSHEnableStep(alreadyPrivileged bool, waitFor, sendCmd, enablePassword string) bool {
	if !alreadyPrivileged {
		return false
	}
	if waitFor == PromptEnable {
		return true
	}
	if waitFor == PromptPassword && strings.TrimSpace(sendCmd) == strings.TrimSpace(enablePassword) {
		return true
	}
	return false
}

func (sc *SSHClient) send(data string) error {
	_, err := sc.stdin.Write([]byte(data))
	return err
}

func (sc *SSHClient) readUntil(pattern string, timeout time.Duration) (string, error) {
	return sc.readUntilAny([]string{pattern}, timeout)
}

func (sc *SSHClient) readUntilAny(patterns []string, timeout time.Duration) (string, error) {
	buffer := make([]byte, BufferSize)
	var output strings.Builder
	output.Grow(BufferSize)
	deadline := time.Now().Add(timeout)

	if sc.netConn != nil {
		_ = sc.netConn.SetReadDeadline(deadline)
	}

	for {
		n, err := sc.reader.Read(buffer)
		if n > 0 {
			output.Write(buffer[:n])
			if sc.config.IsRawOutputEnabled() {
				fmt.Printf("Switch output: Read: %s\n", string(buffer[:n]))
			}
			text := output.String()
			for _, pattern := range patterns {
				if strings.Contains(text, pattern) {
					return text, nil
				}
			}
		}

		if err != nil {
			return output.String(), fmt.Errorf("read error: %v", err)
		}

		if time.Now().After(deadline) {
			return output.String(), fmt.Errorf("timeout waiting for prompts %s", strings.Join(patterns, ", "))
		}
	}
}
