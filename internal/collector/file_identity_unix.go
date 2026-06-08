//go:build !windows

package collector

import (
	"fmt"
	"os"
	"syscall"
)

func fileIdentity(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", fmt.Errorf("unsupported stat type")
	}
	return fmt.Sprintf("%d-%d", stat.Dev, stat.Ino), nil
}
