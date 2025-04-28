package transport

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
)

type jumpPoolEntry struct {
	mu     sync.Mutex
	client *ssh.Client
}

var (
	jumpMu   sync.Mutex
	jumpPool = make(map[string]*jumpPoolEntry)
)

type jumpCacheKeyData struct {
	Addr          string `json:"addr"`
	Username      string `json:"username"`
	Password      string `json:"password"`
	KeyFile       string `json:"key_file"`
	KeyPassphrase string `json:"key_passphrase"`
	KnownHosts    string `json:"known_hosts"`
}

func jumpCacheKey(cfg entities.JumpHostConfig, resolvedUser string) string {
	data, err := json.Marshal(jumpCacheKeyData{
		Addr:          cfg.Addr(),
		Username:      resolvedUser,
		Password:      cfg.Password,
		KeyFile:       cfg.KeyFile,
		KeyPassphrase: cfg.KeyPassphrase,
		KnownHosts:    cfg.KnownHosts,
	})
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum)
}

func getJumpEntry(key string) *jumpPoolEntry {
	jumpMu.Lock()
	defer jumpMu.Unlock()
	if e, ok := jumpPool[key]; ok {
		return e
	}
	e := &jumpPoolEntry{}
	jumpPool[key] = e
	return e
}

func closeJumpPool() {
	jumpMu.Lock()
	entries := make([]*jumpPoolEntry, 0, len(jumpPool))
	for k, e := range jumpPool {
		entries = append(entries, e)
		delete(jumpPool, k)
	}
	jumpMu.Unlock()

	for _, e := range entries {
		e.mu.Lock()
		if e.client != nil {
			_ = e.client.Close()
			e.client = nil
		}
		e.mu.Unlock()
	}
}

func expandUserPath(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", nil
	}
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, "~\\") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to resolve home directory for %q: %w", p, err)
		}
		if p == "~" {
			return home, nil
		}
		return filepath.Join(home, p[2:]), nil
	}
	return p, nil
}

func resolveJumpUsername(cfg entities.JumpHostConfig) (string, error) {
	if u := strings.TrimSpace(cfg.Username); u != "" {
		return u, nil
	}
	if u, err := user.Current(); err == nil && strings.TrimSpace(u.Username) != "" {
		return strings.TrimSpace(u.Username), nil
	}
	for _, env := range []string{"USER", "USERNAME"} {
		if u := strings.TrimSpace(os.Getenv(env)); u != "" {
			return u, nil
		}
	}
	return "", fmt.Errorf("jump host username is required (set username in jump_host or --jump user@host)")
}

func loadJumpHostKeyCallback(cfg entities.JumpHostConfig) (ssh.HostKeyCallback, []string, error) {
	var knownHostsPath string
	if strings.TrimSpace(cfg.KnownHosts) != "" {
		var err error
		knownHostsPath, err = expandUserPath(cfg.KnownHosts)
		if err != nil {
			return nil, nil, err
		}
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to resolve home directory for known_hosts: %w", err)
		}
		knownHostsPath = filepath.Join(home, ".ssh", "known_hosts")
	}

	info, err := os.Stat(knownHostsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, fmt.Errorf(
				"jump host known_hosts file %q not found (run 'ssh -p %d %s' or 'ssh-keyscan -p %d -H %s >> %s', or pass --jump-known-hosts)",
				knownHostsPath, cfg.EffectivePort(), cfg.Host, cfg.EffectivePort(), cfg.Host, knownHostsPath,
			)
		}
		return nil, nil, fmt.Errorf("failed to access known_hosts file %q: %w", knownHostsPath, err)
	}
	if info.IsDir() {
		return nil, nil, fmt.Errorf("known_hosts path %q is a directory", knownHostsPath)
	}

	rawCB, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load known_hosts file %q: %w", knownHostsPath, err)
	}

	algos := knownHostKeyAlgorithms(rawCB, cfg.Addr())
	cb := func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := rawCB(hostname, remote, key)
		if err == nil {
			return nil
		}
		var keyErr *knownhosts.KeyError
		if errors.As(err, &keyErr) {
			fp := ssh.FingerprintSHA256(key)
			if len(keyErr.Want) == 0 {
				return fmt.Errorf(
					"jump host %s (%s %s) is not trusted in %s; add it first (e.g. ssh-keyscan -p %d -H %s >> %s)",
					hostname, key.Type(), fp, knownHostsPath, cfg.EffectivePort(), cfg.Host, knownHostsPath,
				)
			}
			return fmt.Errorf(
				"REMOTE HOST IDENTIFICATION HAS CHANGED for jump host %s (%s %s); mismatch in %s: %w",
				hostname, key.Type(), fp, knownHostsPath, err,
			)
		}
		var revErr *knownhosts.RevokedError
		if errors.As(err, &revErr) {
			return fmt.Errorf("jump host %s key is revoked in %s: %w", hostname, knownHostsPath, err)
		}
		return err
	}
	return cb, algos, nil
}

func knownHostKeyAlgorithms(cb ssh.HostKeyCallback, addr string) []string {
	priv := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	pub, err := ssh.NewPublicKey(priv.Public())
	if err != nil {
		return nil
	}
	err = cb(addr, &net.TCPAddr{IP: net.IPv4zero}, pub)
	var keyErr *knownhosts.KeyError
	if !errors.As(err, &keyErr) || len(keyErr.Want) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	var algos []string
	add := func(a string) {
		if a != "" && !seen[a] {
			seen[a] = true
			algos = append(algos, a)
		}
	}
	for _, want := range keyErr.Want {
		if want.Key == nil {
			continue
		}
		kt := want.Key.Type()
		if kt == ssh.KeyAlgoRSA {
			add(ssh.KeyAlgoRSASHA512)
			add(ssh.KeyAlgoRSASHA256)
			add(ssh.KeyAlgoRSA)
		} else {
			add(kt)
		}
	}
	return algos
}

func parsePrivateKeyFile(path, passphrase string) (ssh.Signer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if passphrase != "" {
		return ssh.ParsePrivateKeyWithPassphrase(data, []byte(passphrase))
	}
	return ssh.ParsePrivateKey(data)
}

func buildJumpAuthMethods(cfg entities.JumpHostConfig, debug bool) ([]ssh.AuthMethod, io.Closer, error) {
	var signers []ssh.Signer
	seenPubs := make(map[string]bool)
	addSigner := func(s ssh.Signer) {
		if s == nil {
			return
		}
		blob := string(s.PublicKey().Marshal())
		if !seenPubs[blob] {
			seenPubs[blob] = true
			signers = append(signers, s)
		}
	}

	explicitKey := strings.TrimSpace(cfg.KeyFile) != ""
	if explicitKey {
		keyPath, err := expandUserPath(cfg.KeyFile)
		if err != nil {
			return nil, nil, err
		}
		signer, err := parsePrivateKeyFile(keyPath, cfg.KeyPassphrase)
		if err != nil {
			var passErr *ssh.PassphraseMissingError
			if errors.As(err, &passErr) && cfg.KeyPassphrase == "" {
				return nil, nil, fmt.Errorf(
					"jump host private key %q requires a passphrase (set jump_host.key_passphrase or key_passphrase_command, or add it to ssh-agent)",
					keyPath,
				)
			}
			return nil, nil, fmt.Errorf("failed to load jump host private key %q: %w", keyPath, err)
		}
		addSigner(signer)
	}

	var agentCloser io.Closer
	if sock := strings.TrimSpace(os.Getenv("SSH_AUTH_SOCK")); sock != "" {
		conn, err := net.DialTimeout("unix", sock, 2*time.Second)
		if err != nil {
			if debug {
				fmt.Fprintf(os.Stderr, "DEBUG: Jump ssh-agent unavailable (%s): %v\n", sock, err)
			}
		} else {
			ag := agent.NewClient(conn)
			agSigners, err := ag.Signers()
			if err == nil && len(agSigners) > 0 {
				for _, s := range agSigners {
					addSigner(s)
				}
				agentCloser = conn
			} else {
				_ = conn.Close()
			}
		}
	}

	if !explicitKey {
		if home, err := os.UserHomeDir(); err == nil {
			for _, name := range []string{"id_ed25519", "id_ecdsa", "id_rsa"} {
				p := filepath.Join(home, ".ssh", name)
				s, err := parsePrivateKeyFile(p, cfg.KeyPassphrase)
				if err == nil {
					addSigner(s)
				} else if !os.IsNotExist(err) && debug {
					fmt.Fprintf(os.Stderr, "DEBUG: Skipping default jump key %s: %v\n", p, err)
				}
			}
		}
	}

	var methods []ssh.AuthMethod
	if len(signers) > 0 {
		methods = append(methods, ssh.PublicKeys(signers...))
	}
	if cfg.Password != "" {
		pw := cfg.Password
		methods = append(methods,
			ssh.Password(pw),
			ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
				answers := make([]string, len(questions))
				for i := range questions {
					answers[i] = pw
				}
				return answers, nil
			}),
		)
	}

	if len(methods) == 0 {
		if agentCloser != nil {
			_ = agentCloser.Close()
		}
		return nil, nil, fmt.Errorf(
			"no jump host authentication method available (start ssh-agent, set --jump-key / jump_host.key_file, set jump_host.password / password_command, or pass --jump-ask-pass)",
		)
	}

	return methods, agentCloser, nil
}

func connectJumpClient(cfg entities.JumpHostConfig, timeout time.Duration, debug bool) (*ssh.Client, error) {
	username, err := resolveJumpUsername(cfg)
	if err != nil {
		return nil, err
	}
	hostKeyCB, algos, err := loadJumpHostKeyCallback(cfg)
	if err != nil {
		return nil, err
	}
	authMethods, agentCloser, err := buildJumpAuthMethods(cfg, debug)
	if err != nil {
		return nil, err
	}
	if agentCloser != nil {
		defer agentCloser.Close()
	}

	sshCfg := &ssh.ClientConfig{
		User:              username,
		Auth:              authMethods,
		HostKeyCallback:   hostKeyCB,
		HostKeyAlgorithms: algos,
		Timeout:           timeout,
	}

	addr := cfg.Addr()
	dialer := &net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}
	tcpConn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to jump host %s: %w", addr, err)
	}

	clientConn, chans, reqs, err := ssh.NewClientConn(tcpConn, addr, sshCfg)
	if err != nil {
		_ = tcpConn.Close()
		return nil, fmt.Errorf("failed to establish SSH session to jump host %s: %w", addr, err)
	}

	if debug {
		fmt.Fprintf(os.Stderr, "DEBUG: Connected to jump host %s as %s\n", addr, username)
	}
	return ssh.NewClient(clientConn, chans, reqs), nil
}

func dialViaJump(cfg entities.JumpHostConfig, targetAddr string, timeout time.Duration, debug bool) (net.Conn, error) {
	username, err := resolveJumpUsername(cfg)
	if err != nil {
		return nil, err
	}

	key := jumpCacheKey(cfg, username)
	var client *ssh.Client
	var entry *jumpPoolEntry
	if key != "" {
		entry = getJumpEntry(key)
		entry.mu.Lock()
		client = entry.client
		if client == nil {
			client, err = connectJumpClient(cfg, timeout, debug)
			if err != nil {
				entry.mu.Unlock()
				return nil, err
			}
			entry.client = client
		}
		entry.mu.Unlock()
	} else {
		client, err = connectJumpClient(cfg, timeout, debug)
		if err != nil {
			return nil, err
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	rawConn, err := client.DialContext(ctx, "tcp", targetAddr)
	if err == nil {
		return newDeadlineConn(rawConn), nil
	}

	var openErr *ssh.OpenChannelError
	if errors.As(err, &openErr) || entry == nil {
		return nil, fmt.Errorf("jump host %s failed to reach %s: %w", cfg.Addr(), targetAddr, err)
	}

	// Reconnect once if the underlying bastion SSH transport dropped.
	entry.mu.Lock()
	if entry.client == client {
		_ = client.Close()
		entry.client = nil
	}
	if entry.client == nil {
		var reconnErr error
		entry.client, reconnErr = connectJumpClient(cfg, timeout, debug)
		if reconnErr != nil {
			entry.mu.Unlock()
			return nil, fmt.Errorf("failed to dial %s via jump host %s (%v); reconnect failed: %w", targetAddr, cfg.Addr(), err, reconnErr)
		}
	}
	client = entry.client
	entry.mu.Unlock()

	ctx2, cancel2 := context.WithTimeout(context.Background(), timeout)
	defer cancel2()
	rawConn, err = client.DialContext(ctx2, "tcp", targetAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to dial %s via jump host %s: %w", targetAddr, cfg.Addr(), err)
	}
	return newDeadlineConn(rawConn), nil
}

func dialTarget(cfg entities.SwitchConfig, addr string, timeout time.Duration) (net.Conn, error) {
	if cfg.JumpHost.IsEnabled() {
		return dialViaJump(*cfg.JumpHost, addr, timeout, cfg.IsDebugEnabled())
	}
	dialer := &net.Dialer{Timeout: timeout}
	return dialer.Dial("tcp", addr)
}
