package config

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unicode"
)

const passwordCommandTimeout = 5 * time.Second

func resolvePasswordCommands(password *string, passwordCmd string, enable *string, enableCmd string) error {
	if *password == "" && passwordCmd != "" {
		out, err := runPasswordCommand(passwordCmd)
		if err != nil {
			return err
		}
		*password = out
	}
	if *enable == "" && enableCmd != "" {
		out, err := runPasswordCommand(enableCmd)
		if err != nil {
			return err
		}
		*enable = out
	}
	return nil
}

func runPasswordCommand(command string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), passwordCommandTimeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", command)
	} else {
		cmd = exec.Command("/bin/sh", "-c", command)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}
	cmd.Env = minimalPasswordCommandEnv()
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &bytes.Buffer{}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("password command failed: %w", err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		if err != nil {
			return "", fmt.Errorf("password command failed: %w", err)
		}
		return strings.TrimRight(stdout.String(), "\r\n"), nil
	case <-ctx.Done():
		killPasswordCommand(cmd)
		<-done
		return "", fmt.Errorf("password command failed: %w", context.DeadlineExceeded)
	}
}

func killPasswordCommand(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	if runtime.GOOS == "windows" {
		_ = cmd.Process.Kill()
		return
	}
	// Negative PID kills the process group started with Setpgid.
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}

func minimalPasswordCommandEnv() []string {
	keys := []string{"PATH", "HOME", "USER", "LANG", "LC_ALL", "SystemRoot", "ComSpec"}
	var env []string
	for _, key := range keys {
		if val, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+val)
		}
	}
	return env
}

// expandEnv resolves ${VAR} references in s. $${VAR} becomes the literal ${VAR}.
// Unset variables return an error: environment variable "VAR" not set.
func expandEnv(s string) (string, error) {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if s[i] != '$' || i+1 >= len(s) {
			b.WriteByte(s[i])
			i++
			continue
		}

		// Escaped form: $${NAME} → ${NAME}
		if s[i+1] == '$' && i+2 < len(s) && s[i+2] == '{' {
			name, end, ok := readEnvName(s, i+3)
			if !ok {
				b.WriteByte(s[i])
				i++
				continue
			}
			b.WriteString("${")
			b.WriteString(name)
			b.WriteByte('}')
			i = end
			continue
		}

		if s[i+1] == '{' {
			name, end, ok := readEnvName(s, i+2)
			if !ok {
				b.WriteByte(s[i])
				i++
				continue
			}
			val, found := os.LookupEnv(name)
			if !found {
				return "", fmt.Errorf("environment variable %q not set", name)
			}
			b.WriteString(val)
			i = end
			continue
		}

		b.WriteByte(s[i])
		i++
	}
	return b.String(), nil
}

func readEnvName(s string, start int) (name string, end int, ok bool) {
	if start >= len(s) {
		return "", start, false
	}
	j := start
	if c := rune(s[j]); c != '_' && !unicode.IsLetter(c) {
		return "", start, false
	}
	j++
	for j < len(s) {
		c := rune(s[j])
		if c == '_' || unicode.IsLetter(c) || unicode.IsDigit(c) {
			j++
			continue
		}
		break
	}
	if j >= len(s) || s[j] != '}' || j == start {
		return "", start, false
	}
	return s[start:j], j + 1, true
}

func expandCredentialFields(username, password, enable string) (string, string, string, error) {
	u, err := expandEnv(username)
	if err != nil {
		return "", "", "", err
	}
	p, err := expandEnv(password)
	if err != nil {
		return "", "", "", err
	}
	e, err := expandEnv(enable)
	if err != nil {
		return "", "", "", err
	}
	return u, p, e, nil
}
