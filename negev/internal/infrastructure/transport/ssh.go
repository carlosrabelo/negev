package transport

import (
	"fmt"
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
	sshConfig := &ssh.ClientConfig{
		User:            sc.config.Username,
		Auth:            []ssh.AuthMethod{ssh.Password(sc.config.Password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec
		Timeout:         DefaultTimeout,
	}
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to %s via SSH: %v", sc.config.Target, err)
	}
	sc.client = client
	return nil
}
