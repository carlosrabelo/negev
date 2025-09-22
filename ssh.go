package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHClient manages an SSH session with a switch
type SSHClient struct {
	config  SwitchConfig
	client  *ssh.Client
	session *ssh.Session
	stdin   io.WriteCloser
	reader  *bufio.Reader
	netConn net.Conn
}

// NewSSHClient creates a new SSH client with the given configuration
func NewSSHClient(config SwitchConfig) *SSHClient {
	return &SSHClient{config: config}
}

func (sc *SSHClient) Connect() error {
	if sc.IsConnected() {
		return nil
	}
	addr := net.JoinHostPort(sc.config.Target, "22")
	sshConfig := &ssh.ClientConfig{
		User:            sc.config.Username,
		Auth:            []ssh.AuthMethod{ssh.Password(sc.config.Password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
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
	} else if sc.config.IsDebugEnabled() {
		fmt.Printf("DEBUG: %s already in privileged mode\n", sc.config.Target)
	}

	if err := sc.send(TerminalLengthCmd); err != nil {
		sc.Disconnect()
		return fmt.Errorf("failed to send terminal length command to %s: %v", sc.config.Target, err)
	}

	if _, err := sc.readUntil(PromptPrivileged, DefaultTimeout); err != nil {
		sc.Disconnect()
		return err
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
	if err := sc.send(cmd + "\n"); err != nil {
		return "", fmt.Errorf("failed to send command %s: %v", cmd, err)
	}

	output, err := sc.readUntil(PromptPrivileged, DefaultTimeout)
	if err != nil {
		return "", fmt.Errorf("error executing %s: %v", cmd, err)
	}

	lines := strings.Split(output, "\n")
	if len(lines) > 1 {
		output = strings.Join(lines[1:len(lines)-1], "\n")
	} else {
		output = ""
	}

	if sc.config.IsRawOutputEnabled() {
		fmt.Printf("Switch output for '%s':\n%s\n", cmd, output)
	}

	return output, nil
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

	for {
		if sc.netConn != nil {
			_ = sc.netConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		}

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
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				if time.Now().After(deadline) {
					return output.String(), fmt.Errorf("timeout waiting for prompts %s", strings.Join(patterns, ", "))
				}
				continue
			}
			return output.String(), fmt.Errorf("read error: %v", err)
		}

		if time.Now().After(deadline) {
			return output.String(), fmt.Errorf("timeout waiting for prompts %s", strings.Join(patterns, ", "))
		}
	}
}
