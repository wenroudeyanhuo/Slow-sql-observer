package acquisition

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"slow-sql-observer/internal/config"
)

func runSSHCommand(ctx context.Context, source config.SourceConfig, script string) ([]byte, error) {
	signer, err := loadSigner(source.SSHPrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("load ssh private key: %w", err)
	}
	hostKeyCallback, err := knownhosts.New(expandPath(source.SSHKnownHostsPath))
	if err != nil {
		return nil, fmt.Errorf("load known_hosts: %w", err)
	}
	clientConfig := &ssh.ClientConfig{
		User:            source.RemoteUser,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: hostKeyCallback,
	}
	address := fmt.Sprintf("%s:%d", source.RemoteHost, source.RemotePort)
	conn, err := ssh.Dial("tcp", address, clientConfig)
	if err != nil {
		return nil, fmt.Errorf("ssh dial %s: %w", address, err)
	}
	defer conn.Close()

	session, err := conn.NewSession()
	if err != nil {
		return nil, fmt.Errorf("ssh session: %w", err)
	}
	defer session.Close()

	done := make(chan struct{})
	var output []byte
	var runErr error
	go func() {
		defer close(done)
		output, runErr = session.Output("sh -lc " + shellQuote(script))
	}()
	select {
	case <-ctx.Done():
		_ = session.Close()
		<-done
		return nil, ctx.Err()
	case <-done:
		if runErr != nil {
			return nil, fmt.Errorf("run remote command: %w", runErr)
		}
		return output, nil
	}
}

func loadSigner(path string) (ssh.Signer, error) {
	keyPath := expandPath(path)
	content, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}
	return ssh.ParsePrivateKey(content)
}

func expandPath(path string) string {
	if path == "" {
		return path
	}
	if strings.HasPrefix(path, "~") {
		currentUser, err := user.Current()
		if err == nil {
			return filepath.Join(currentUser.HomeDir, strings.TrimPrefix(path, "~"))
		}
	}
	return path
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func parseInt64(value string) (int64, error) {
	return strconv.ParseInt(strings.TrimSpace(value), 10, 64)
}
