package transport

import (
	"bufio"
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ziutek/telnet"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
)

type directTCPIPPayload struct {
	DestAddr   string
	DestPort   uint32
	OriginAddr string
	OriginPort uint32
}

type testJumpServer struct {
	addr     string
	ln       net.Listener
	sshConns atomic.Int32
}

type jumpServerOpts struct {
	hostSigners         []ssh.Signer
	username            string
	password            string
	keyboardInteractive bool
	authorizedKeys      []ssh.PublicKey
}

func generateED25519Key(t *testing.T) (ed25519.PrivateKey, ssh.Signer) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ed25519 key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("new ed25519 signer: %v", err)
	}
	return priv, signer
}

func generateECDSAKey(t *testing.T) (*ecdsa.PrivateKey, ssh.Signer) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate ecdsa key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("new ecdsa signer: %v", err)
	}
	return priv, signer
}

func writePrivateKeyPEM(t *testing.T, path string, priv any, passphrase string) {
	t.Helper()
	var block *pem.Block
	var err error
	if passphrase != "" {
		block, err = ssh.MarshalPrivateKeyWithPassphrase(priv, "", []byte(passphrase))
	} else {
		block, err = ssh.MarshalPrivateKey(priv, "")
	}
	if err != nil {
		t.Fatalf("marshal private key: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir key dir: %v", err)
	}
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatalf("write private key: %v", err)
	}
}

func writeKnownHostsFile(t *testing.T, path, addr string, pub ssh.PublicKey) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir known_hosts dir: %v", err)
	}
	line := knownhosts.Line([]string{knownhosts.Normalize(addr)}, pub) + "\n"
	if err := os.WriteFile(path, []byte(line), 0o600); err != nil {
		t.Fatalf("write known_hosts: %v", err)
	}
}

func startTestJumpServer(t *testing.T, opts jumpServerOpts) *testJumpServer {
	t.Helper()
	srvCfg := &ssh.ServerConfig{}
	if opts.password != "" && !opts.keyboardInteractive {
		srvCfg.PasswordCallback = func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if opts.username != "" && c.User() != opts.username {
				return nil, fmt.Errorf("unexpected user %s", c.User())
			}
			if string(pass) != opts.password {
				return nil, errors.New("invalid password")
			}
			return nil, nil
		}
	}
	if opts.password != "" && opts.keyboardInteractive {
		srvCfg.KeyboardInteractiveCallback = func(c ssh.ConnMetadata, client ssh.KeyboardInteractiveChallenge) (*ssh.Permissions, error) {
			if opts.username != "" && c.User() != opts.username {
				return nil, fmt.Errorf("unexpected user %s", c.User())
			}
			ans, err := client("", "", []string{"Password: "}, []bool{false})
			if err != nil || len(ans) != 1 || ans[0] != opts.password {
				return nil, errors.New("invalid keyboard-interactive password")
			}
			return nil, nil
		}
	}
	if len(opts.authorizedKeys) > 0 {
		srvCfg.PublicKeyCallback = func(c ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if opts.username != "" && c.User() != opts.username {
				return nil, fmt.Errorf("unexpected user %s", c.User())
			}
			for _, allowed := range opts.authorizedKeys {
				if bytes.Equal(key.Marshal(), allowed.Marshal()) {
					return nil, nil
				}
			}
			return nil, errors.New("unauthorized public key")
		}
	}
	for _, s := range opts.hostSigners {
		srvCfg.AddHostKey(s)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen jump server: %v", err)
	}
	srv := &testJumpServer{
		addr: ln.Addr().String(),
		ln:   ln,
	}

	var wg sync.WaitGroup
	var connMu sync.Mutex
	active := make(map[io.Closer]struct{})
	track := func(c io.Closer) {
		connMu.Lock()
		active[c] = struct{}{}
		connMu.Unlock()
	}
	untrack := func(c io.Closer) {
		connMu.Lock()
		delete(active, c)
		connMu.Unlock()
	}

	t.Cleanup(func() {
		_ = ln.Close()
		connMu.Lock()
		for c := range active {
			_ = c.Close()
		}
		connMu.Unlock()
		wg.Wait()
	})

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			tcpConn, err := ln.Accept()
			if err != nil {
				return
			}
			track(tcpConn)
			wg.Add(1)
			go func(c net.Conn) {
				defer wg.Done()
				defer untrack(c)
				sConn, chans, reqs, err := ssh.NewServerConn(c, srvCfg)
				if err != nil {
					_ = c.Close()
					return
				}
				srv.sshConns.Add(1)
				track(sConn)
				defer func() {
					untrack(sConn)
					_ = sConn.Close()
				}()
				go ssh.DiscardRequests(reqs)

				for newChan := range chans {
					if newChan.ChannelType() != "direct-tcpip" {
						_ = newChan.Reject(ssh.UnknownChannelType, "unsupported channel")
						continue
					}
					var payload directTCPIPPayload
					if err := ssh.Unmarshal(newChan.ExtraData(), &payload); err != nil {
						_ = newChan.Reject(ssh.Prohibited, "invalid payload")
						continue
					}
					dest := net.JoinHostPort(payload.DestAddr, fmt.Sprintf("%d", payload.DestPort))
					remote, err := net.DialTimeout("tcp", dest, 5*time.Second)
					if err != nil {
						_ = newChan.Reject(ssh.ConnectionFailed, err.Error())
						continue
					}
					ch, chReqs, err := newChan.Accept()
					if err != nil {
						_ = remote.Close()
						continue
					}
					track(ch)
					track(remote)
					go ssh.DiscardRequests(chReqs)
					wg.Add(1)
					go func(a io.ReadWriteCloser, b io.ReadWriteCloser) {
						defer wg.Done()
						defer untrack(a)
						defer untrack(b)
						var cpWg sync.WaitGroup
						cpWg.Add(2)
						go func() {
							defer cpWg.Done()
							_, _ = io.Copy(b, a)
							if tc, ok := b.(interface{ CloseWrite() error }); ok {
								_ = tc.CloseWrite()
							}
						}()
						go func() {
							defer cpWg.Done()
							_, _ = io.Copy(a, b)
							if tc, ok := a.(interface{ CloseWrite() error }); ok {
								_ = tc.CloseWrite()
							}
						}()
						cpWg.Wait()
						_ = a.Close()
						_ = b.Close()
					}(ch, remote)
				}
			}(tcpConn)
		}
	}()

	return srv
}

func startMockTelnetSwitch(t *testing.T, promptName string) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen mock telnet switch: %v", err)
	}
	var wg sync.WaitGroup
	t.Cleanup(func() {
		_ = ln.Close()
		wg.Wait()
	})

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			wg.Add(1)
			go func(c net.Conn) {
				defer wg.Done()
				defer c.Close()
				r := bufio.NewReader(c)
				_, _ = c.Write([]byte("Username: "))
				_, _ = r.ReadString('\n')
				_, _ = c.Write([]byte("Password: "))
				_, _ = r.ReadString('\n')
				_, _ = fmt.Fprintf(c, "\r\n%s#", promptName)
				for {
					line, err := r.ReadString('\n')
					if err != nil {
						return
					}
					cmd := strings.TrimSpace(line)
					switch cmd {
					case "terminal length 0":
						_, _ = fmt.Fprintf(c, "%s\r\n%s#", cmd, promptName)
					case "show version":
						_, _ = fmt.Fprintf(c, "%s\r\nCisco IOS Software\r\n%s#", cmd, promptName)
					default:
						_, _ = fmt.Fprintf(c, "%s\r\n%s#", cmd, promptName)
					}
				}
			}(conn)
		}
	}()
	return ln.Addr().String()
}

func startMockSSHSwitch(t *testing.T, username, password string) string {
	t.Helper()
	_, hostSigner := generateED25519Key(t)
	srvCfg := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if c.User() == username && string(pass) == password {
				return nil, nil
			}
			return nil, errors.New("denied")
		},
	}
	srvCfg.AddHostKey(hostSigner)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen mock ssh switch: %v", err)
	}
	var wg sync.WaitGroup
	t.Cleanup(func() {
		_ = ln.Close()
		wg.Wait()
	})

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			tcpConn, err := ln.Accept()
			if err != nil {
				return
			}
			wg.Add(1)
			go func(c net.Conn) {
				defer wg.Done()
				sConn, chans, reqs, err := ssh.NewServerConn(c, srvCfg)
				if err != nil {
					_ = c.Close()
					return
				}
				defer sConn.Close()
				go ssh.DiscardRequests(reqs)

				for newChan := range chans {
					if newChan.ChannelType() != "session" {
						_ = newChan.Reject(ssh.UnknownChannelType, "only session")
						continue
					}
					ch, chReqs, err := newChan.Accept()
					if err != nil {
						continue
					}
					wg.Add(1)
					go func(channel ssh.Channel, requests <-chan *ssh.Request) {
						defer wg.Done()
						defer channel.Close()
						for req := range requests {
							switch req.Type {
							case "pty-req":
								if req.WantReply {
									_ = req.Reply(true, nil)
								}
							case "shell":
								if req.WantReply {
									_ = req.Reply(true, nil)
								}
								_, _ = channel.Write([]byte("\r\nsw-ssh#"))
								go func() {
									r := bufio.NewReader(channel)
									for {
										line, err := r.ReadString('\n')
										if err != nil {
											return
										}
										cmd := strings.TrimSpace(line)
										switch cmd {
										case "terminal length 0":
											_, _ = channel.Write([]byte("terminal length 0\r\nsw-ssh#"))
										case "show version":
											_, _ = channel.Write([]byte("show version\r\nCisco IOS Software\r\nsw-ssh#"))
										default:
											_, _ = fmt.Fprintf(channel, "%s\r\nsw-ssh#", cmd)
										}
									}
								}()
							default:
								if req.WantReply {
									_ = req.Reply(false, nil)
								}
							}
						}
					}(ch, chReqs)
				}
			}(tcpConn)
		}
	}()

	return ln.Addr().String()
}

func jumpConfigFromAddr(t *testing.T, addr, user, knownHosts string) entities.JumpHostConfig {
	t.Helper()
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split host port %q: %v", addr, err)
	}
	var port int
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		t.Fatalf("parse port %q: %v", portStr, err)
	}
	return entities.JumpHostConfig{
		Host:       host,
		Port:       port,
		Username:   user,
		KnownHosts: knownHosts,
	}
}

func hermeticJumpEnv(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SSH_AUTH_SOCK", "")
	t.Cleanup(CloseAll)
	return home
}

func TestTelnetOverJumpHostWithPassword(t *testing.T) {
	home := hermeticJumpEnv(t)
	_, hostSigner := generateED25519Key(t)
	srv := startTestJumpServer(t, jumpServerOpts{
		hostSigners:         []ssh.Signer{hostSigner},
		username:            "bastion",
		password:            "jump-secret",
		keyboardInteractive: true,
	})
	kh := filepath.Join(home, ".ssh", "known_hosts")
	writeKnownHostsFile(t, kh, srv.addr, hostSigner.PublicKey())

	swAddr := startMockTelnetSwitch(t, "sw-pw")
	jump := jumpConfigFromAddr(t, srv.addr, "bastion", "")
	jump.Password = "jump-secret"

	tc := NewTelnetClient(entities.SwitchConfig{
		Target:    swAddr,
		Transport: "telnet",
		Username:  "admin",
		Password:  "swpass",
		JumpHost:  &jump,
	})
	if err := tc.Connect(); err != nil {
		t.Fatalf("Telnet Connect via jump failed: %v", err)
	}
	defer tc.Disconnect()

	out, err := tc.ExecuteCommand("show version")
	if err != nil {
		t.Fatalf("ExecuteCommand failed: %v", err)
	}
	if !strings.Contains(out, "Cisco IOS Software") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestTelnetOverJumpHostWithSSHKey(t *testing.T) {
	for _, passphrase := range []string{"", "key-passphrase"} {
		name := "unencrypted"
		if passphrase != "" {
			name = "passphrase"
		}
		t.Run(name, func(t *testing.T) {
			home := hermeticJumpEnv(t)
			_, hostSigner := generateED25519Key(t)
			userPriv, userSigner := generateED25519Key(t)

			keyPath := filepath.Join(home, "custom_key")
			writePrivateKeyPEM(t, keyPath, userPriv, passphrase)

			srv := startTestJumpServer(t, jumpServerOpts{
				hostSigners:    []ssh.Signer{hostSigner},
				username:       "bastion",
				authorizedKeys: []ssh.PublicKey{userSigner.PublicKey()},
			})
			kh := filepath.Join(home, "known_hosts")
			writeKnownHostsFile(t, kh, srv.addr, hostSigner.PublicKey())

			swAddr := startMockTelnetSwitch(t, "sw-key")
			jump := jumpConfigFromAddr(t, srv.addr, "bastion", kh)
			jump.KeyFile = keyPath
			jump.KeyPassphrase = passphrase

			tc := NewTelnetClient(entities.SwitchConfig{
				Target:    swAddr,
				Transport: "telnet",
				Username:  "admin",
				Password:  "swpass",
				JumpHost:  &jump,
			})
			if err := tc.Connect(); err != nil {
				t.Fatalf("Connect failed: %v", err)
			}
			defer tc.Disconnect()

			out, err := tc.ExecuteCommand("show version")
			if err != nil {
				t.Fatalf("ExecuteCommand failed: %v", err)
			}
			if !strings.Contains(out, "Cisco IOS Software") {
				t.Fatalf("unexpected output: %q", out)
			}
		})
	}
}

func TestTelnetOverJumpHostWithSSHAgent(t *testing.T) {
	home := hermeticJumpEnv(t)
	_, hostSigner := generateED25519Key(t)
	userPriv, userSigner := generateED25519Key(t)

	keyring := agent.NewKeyring()
	if err := keyring.Add(agent.AddedKey{PrivateKey: userPriv}); err != nil {
		t.Fatalf("agent add key: %v", err)
	}

	sockPath := filepath.Join(home, "agent.sock")
	agLn, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("listen unix agent: %v", err)
	}
	var wg sync.WaitGroup
	t.Cleanup(func() {
		_ = agLn.Close()
		wg.Wait()
	})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			c, err := agLn.Accept()
			if err != nil {
				return
			}
			wg.Add(1)
			go func(conn net.Conn) {
				defer wg.Done()
				defer conn.Close()
				_ = agent.ServeAgent(keyring, conn)
			}(c)
		}
	}()
	t.Setenv("SSH_AUTH_SOCK", sockPath)

	srv := startTestJumpServer(t, jumpServerOpts{
		hostSigners:    []ssh.Signer{hostSigner},
		username:       "bastion",
		authorizedKeys: []ssh.PublicKey{userSigner.PublicKey()},
	})
	kh := filepath.Join(home, ".ssh", "known_hosts")
	writeKnownHostsFile(t, kh, srv.addr, hostSigner.PublicKey())

	swAddr := startMockTelnetSwitch(t, "sw-agent")
	jump := jumpConfigFromAddr(t, srv.addr, "bastion", "")

	tc := NewTelnetClient(entities.SwitchConfig{
		Target:    swAddr,
		Transport: "telnet",
		Username:  "admin",
		Password:  "swpass",
		JumpHost:  &jump,
	})
	if err := tc.Connect(); err != nil {
		t.Fatalf("Connect via ssh-agent failed: %v", err)
	}
	defer tc.Disconnect()

	out, err := tc.ExecuteCommand("show version")
	if err != nil {
		t.Fatalf("ExecuteCommand failed: %v", err)
	}
	if !strings.Contains(out, "Cisco IOS Software") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestSSHOverJumpHost(t *testing.T) {
	home := hermeticJumpEnv(t)
	_, hostSigner := generateED25519Key(t)
	srv := startTestJumpServer(t, jumpServerOpts{
		hostSigners: []ssh.Signer{hostSigner},
		username:    "bastion",
		password:    "jump-secret",
	})
	kh := filepath.Join(home, ".ssh", "known_hosts")
	writeKnownHostsFile(t, kh, srv.addr, hostSigner.PublicKey())

	swAddr := startMockSSHSwitch(t, "swadmin", "swpass")
	jump := jumpConfigFromAddr(t, srv.addr, "bastion", kh)
	jump.Password = "jump-secret"

	sc := NewSSHClient(entities.SwitchConfig{
		Target:    swAddr,
		Transport: "ssh",
		Username:  "swadmin",
		Password:  "swpass",
		JumpHost:  &jump,
	})
	if err := sc.Connect(); err != nil {
		t.Fatalf("SSH Connect over jump host failed: %v", err)
	}
	defer sc.Disconnect()

	out, err := sc.ExecuteCommand("show version")
	if err != nil {
		t.Fatalf("ExecuteCommand failed: %v", err)
	}
	if !strings.Contains(out, "Cisco IOS Software") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestJumpHostKeyValidationRejectsUnknownHost(t *testing.T) {
	home := hermeticJumpEnv(t)
	_, hostSigner := generateED25519Key(t)
	_, otherSigner := generateED25519Key(t)
	srv := startTestJumpServer(t, jumpServerOpts{
		hostSigners: []ssh.Signer{hostSigner},
		username:    "bastion",
		password:    "jump-secret",
	})
	kh := filepath.Join(home, "known_hosts")
	writeKnownHostsFile(t, kh, "127.0.0.1:65000", otherSigner.PublicKey())

	jump := jumpConfigFromAddr(t, srv.addr, "bastion", kh)
	jump.Password = "jump-secret"

	_, err := dialViaJump(jump, "127.0.0.1:23", 5*time.Second, false)
	if err == nil {
		t.Fatal("expected unknown host key error")
	}
	if !strings.Contains(err.Error(), "is not trusted") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestJumpHostKeyValidationRejectsMismatch(t *testing.T) {
	home := hermeticJumpEnv(t)
	_, hostSigner := generateED25519Key(t)
	_, wrongSigner := generateED25519Key(t)
	srv := startTestJumpServer(t, jumpServerOpts{
		hostSigners: []ssh.Signer{hostSigner},
		username:    "bastion",
		password:    "jump-secret",
	})
	kh := filepath.Join(home, "known_hosts")
	writeKnownHostsFile(t, kh, srv.addr, wrongSigner.PublicKey())

	jump := jumpConfigFromAddr(t, srv.addr, "bastion", kh)
	jump.Password = "jump-secret"

	_, err := dialViaJump(jump, "127.0.0.1:23", 5*time.Second, false)
	if err == nil {
		t.Fatal("expected host key mismatch error")
	}
	if !strings.Contains(err.Error(), "REMOTE HOST IDENTIFICATION HAS CHANGED") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestJumpHostKeyValidationMissingFile(t *testing.T) {
	home := hermeticJumpEnv(t)
	jump := entities.JumpHostConfig{
		Host:       "127.0.0.1",
		Port:       2222,
		Username:   "bastion",
		Password:   "jump-secret",
		KnownHosts: filepath.Join(home, "missing_known_hosts"),
	}
	_, err := dialViaJump(jump, "127.0.0.1:23", 5*time.Second, false)
	if err == nil {
		t.Fatal("expected missing known_hosts error")
	}
	if !strings.Contains(err.Error(), "known_hosts file") || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestJumpHostKeyAlgorithmAlignment(t *testing.T) {
	home := hermeticJumpEnv(t)
	_, edSigner := generateED25519Key(t)
	_, ecdsaSigner := generateECDSAKey(t)
	// Server offers Ed25519 first, then ECDSA; known_hosts only trusts ECDSA.
	srv := startTestJumpServer(t, jumpServerOpts{
		hostSigners: []ssh.Signer{edSigner, ecdsaSigner},
		username:    "bastion",
		password:    "jump-secret",
	})
	kh := filepath.Join(home, "known_hosts")
	writeKnownHostsFile(t, kh, srv.addr, ecdsaSigner.PublicKey())

	swAddr := startMockTelnetSwitch(t, "sw-ecdsa")
	jump := jumpConfigFromAddr(t, srv.addr, "bastion", kh)
	jump.Password = "jump-secret"

	tc := NewTelnetClient(entities.SwitchConfig{
		Target:    swAddr,
		Transport: "telnet",
		Username:  "admin",
		Password:  "swpass",
		JumpHost:  &jump,
	})
	if err := tc.Connect(); err != nil {
		t.Fatalf("Connect with ECDSA-only known_hosts failed: %v", err)
	}
	tc.Disconnect()
}

func TestJumpHostConnectionMultiplexing(t *testing.T) {
	home := hermeticJumpEnv(t)
	_, hostSigner := generateED25519Key(t)
	srv := startTestJumpServer(t, jumpServerOpts{
		hostSigners: []ssh.Signer{hostSigner},
		username:    "bastion",
		password:    "jump-secret",
	})
	kh := filepath.Join(home, "known_hosts")
	writeKnownHostsFile(t, kh, srv.addr, hostSigner.PublicKey())

	sw1 := startMockTelnetSwitch(t, "sw1")
	sw2 := startMockTelnetSwitch(t, "sw2")
	jump := jumpConfigFromAddr(t, srv.addr, "bastion", kh)
	jump.Password = "jump-secret"

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, target := range []string{sw1, sw2} {
		wg.Add(1)
		go func(addr string) {
			defer wg.Done()
			j := jump
			tc := NewTelnetClient(entities.SwitchConfig{
				Target:    addr,
				Transport: "telnet",
				Username:  "admin",
				Password:  "swpass",
				JumpHost:  &j,
			})
			if err := tc.Connect(); err != nil {
				errs <- err
				return
			}
			defer tc.Disconnect()
			if _, err := tc.ExecuteCommand("show version"); err != nil {
				errs <- err
			}
		}(target)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent switch over jump failed: %v", err)
		}
	}

	if got := srv.sshConns.Load(); got != 1 {
		t.Fatalf("jump server observed %d SSH connections; want 1 multiplexed connection", got)
	}
}

func TestJumpHostReadTimeoutEnforced(t *testing.T) {
	home := hermeticJumpEnv(t)
	_, hostSigner := generateED25519Key(t)
	srv := startTestJumpServer(t, jumpServerOpts{
		hostSigners: []ssh.Signer{hostSigner},
		username:    "bastion",
		password:    "jump-secret",
	})
	kh := filepath.Join(home, "known_hosts")
	writeKnownHostsFile(t, kh, srv.addr, hostSigner.PublicKey())

	silentLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen silent switch: %v", err)
	}
	var silentWg sync.WaitGroup
	t.Cleanup(func() {
		_ = silentLn.Close()
		silentWg.Wait()
	})
	silentWg.Add(1)
	go func() {
		defer silentWg.Done()
		c, err := silentLn.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		_, _ = io.Copy(io.Discard, c)
	}()

	jump := jumpConfigFromAddr(t, srv.addr, "bastion", kh)
	jump.Password = "jump-secret"

	rawConn, err := dialViaJump(jump, silentLn.Addr().String(), 5*time.Second, false)
	if err != nil {
		t.Fatalf("dialViaJump failed: %v", err)
	}
	defer rawConn.Close()

	tconn, err := telnet.NewConn(rawConn)
	if err != nil {
		t.Fatalf("telnet.NewConn failed: %v", err)
	}
	tc := &TelnetClient{
		conn:   tconn,
		config: entities.SwitchConfig{Target: silentLn.Addr().String()},
	}

	start := time.Now()
	_, err = tc.readUntilAny([]string{PromptUsername}, 150*time.Millisecond)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected read timeout error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "timeout") && !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("read timeout took too long: %v", elapsed)
	}
}
