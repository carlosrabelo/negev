package transport

import (
	"fmt"
	"net"

	"github.com/ziutek/telnet"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
)

type TelnetClient struct {
	conn   *telnet.Conn
	config entities.SwitchConfig
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
	conn, err := telnet.Dial("tcp", telnetDialAddr(tc.config.Target))
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %v", tc.config.Target, err)
	}
	tc.conn = conn
	return nil
}
