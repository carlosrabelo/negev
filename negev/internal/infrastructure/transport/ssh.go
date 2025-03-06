package transport

import (
	"fmt"
	"log/slog"
	"net"

	"golang.org/x/crypto/ssh"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
)

type SSHClient struct {
	config  entities.SwitchConfig
	client  *ssh.Client
	session *ssh.Session
	netConn net.Conn
}

func NewSSHClient(cfg entities.SwitchConfig) *SSHClient {
	return &SSHClient{config: cfg}
}

func (sc *SSHClient) Connect() error {
	addr := net.JoinHostPort(sc.config.Target, "22")
	slog.Warn("SSH host key verification is disabled — use only on trusted networks", "target", sc.config.Target)
	sshConfig := &ssh.ClientConfig{
		User:            sc.config.Username,
		Auth:            []ssh.AuthMethod{ssh.Password(sc.config.Password)},
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
	sc.client = client
	sc.netConn = rawConn
	return nil
}
