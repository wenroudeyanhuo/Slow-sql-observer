package acquisition

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"slow-sql-observer/internal/collector"
	"slow-sql-observer/internal/config"
	"slow-sql-observer/internal/model"
)

type fakeSSHClient struct {
	validateErr error
	statState   collector.FileState
	statErr     error
	readBytes   []byte
	readErr     error
}

func (f *fakeSSHClient) ValidateLinux(context.Context, config.SourceConfig) error {
	return f.validateErr
}
func (f *fakeSSHClient) Stat(context.Context, config.SourceConfig) (collector.FileState, error) {
	return f.statState, f.statErr
}
func (f *fakeSSHClient) ReadFrom(context.Context, config.SourceConfig, int64) ([]byte, error) {
	return f.readBytes, f.readErr
}

func TestAcquireLocalReturnsParsePath(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "slow.log")
	if err := os.WriteFile(logPath, []byte("# Time: demo;"), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}
	service := NewService(nil)
	result, err := service.Acquire(context.Background(), config.SourceConfig{
		LogMode:     model.LogModeLocalFile,
		SlowLogPath: logPath,
	}, nil)
	if err != nil {
		t.Fatalf("Acquire returned error: %v", err)
	}
	if result.ParsePath != logPath {
		t.Fatalf("expected parse path %q, got %q", logPath, result.ParsePath)
	}
	if !result.ShouldParse {
		t.Fatalf("expected local mode to be parseable")
	}
}

func TestAcquireRemoteDefaultsToEndWithoutCheckpoint(t *testing.T) {
	dir := t.TempDir()
	service := NewService(&fakeSSHClient{
		statState: collector.FileState{Identity: "dev:inode", Size: 200},
		readBytes: []byte{},
	})
	source := config.SourceConfig{
		LogMode:            model.LogModeSSHPull,
		InstanceName:       "db-a",
		RemoteHost:         "example",
		RemotePort:         22,
		RemoteUser:         "mysql",
		RemoteSlowLogPath:  "/var/log/mysql/slow.log",
		SSHPrivateKeyPath:  filepath.Join(dir, "id_rsa"),
		SSHKnownHostsPath:  filepath.Join(dir, "known_hosts"),
		LocalSpoolDir:      dir,
		InitialPosition:    model.InitialPositionEnd,
		LocalSpoolMaxBytes: 1024,
	}
	if err := os.WriteFile(source.SSHPrivateKeyPath, []byte("dummy"), 0o644); err != nil {
		t.Fatalf("write key: %v", err)
	}
	if err := os.WriteFile(source.SSHKnownHostsPath, []byte("dummy"), 0o644); err != nil {
		t.Fatalf("write known_hosts: %v", err)
	}

	result, err := service.Acquire(context.Background(), source, nil)
	if err != nil && result.AcquisitionState != model.AcquisitionStateHealthy {
		t.Fatalf("expected healthy no-op first acquire, got state=%s err=%v", result.AcquisitionState, err)
	}
	if result.RemoteOffsetStart != 200 {
		t.Fatalf("expected first pull to start at end, got %d", result.RemoteOffsetStart)
	}
}

func TestAcquireRemoteBlocksWhenSpoolLimitWouldBeExceeded(t *testing.T) {
	dir := t.TempDir()
	service := NewService(&fakeSSHClient{
		statState: collector.FileState{Identity: "dev:inode", Size: 50},
		readBytes: make([]byte, 50),
	})
	source := config.SourceConfig{
		LogMode:            model.LogModeSSHPull,
		InstanceName:       "db-a",
		RemoteHost:         "example",
		RemotePort:         22,
		RemoteUser:         "mysql",
		RemoteSlowLogPath:  "/var/log/mysql/slow.log",
		SSHPrivateKeyPath:  filepath.Join(dir, "id_rsa"),
		SSHKnownHostsPath:  filepath.Join(dir, "known_hosts"),
		LocalSpoolDir:      dir,
		InitialPosition:    model.InitialPositionStart,
		LocalSpoolMaxBytes: 10,
	}
	if err := os.WriteFile(source.SSHPrivateKeyPath, []byte("dummy"), 0o644); err != nil {
		t.Fatalf("write key: %v", err)
	}
	if err := os.WriteFile(source.SSHKnownHostsPath, []byte("dummy"), 0o644); err != nil {
		t.Fatalf("write known_hosts: %v", err)
	}

	result, err := service.Acquire(context.Background(), source, nil)
	if err == nil {
		t.Fatalf("expected spool limit error")
	}
	if result.AcquisitionState != model.AcquisitionStateBlocked {
		t.Fatalf("expected blocked state, got %q", result.AcquisitionState)
	}
}

func TestAcquireRemoteRestartsFromZeroAfterRotation(t *testing.T) {
	dir := t.TempDir()
	service := NewService(&fakeSSHClient{
		statState: collector.FileState{Identity: "new-dev:new-inode", Size: 80},
		readBytes: make([]byte, 80),
	})
	source := config.SourceConfig{
		LogMode:            model.LogModeSSHPull,
		InstanceName:       "db-a",
		RemoteHost:         "example",
		RemotePort:         22,
		RemoteUser:         "mysql",
		RemoteSlowLogPath:  "/var/log/mysql/slow.log",
		SSHPrivateKeyPath:  filepath.Join(dir, "id_rsa"),
		SSHKnownHostsPath:  filepath.Join(dir, "known_hosts"),
		LocalSpoolDir:      dir,
		InitialPosition:    model.InitialPositionEnd,
		LocalSpoolMaxBytes: 1024,
	}
	if err := os.WriteFile(source.SSHPrivateKeyPath, []byte("dummy"), 0o644); err != nil {
		t.Fatalf("write key: %v", err)
	}
	if err := os.WriteFile(source.SSHKnownHostsPath, []byte("dummy"), 0o644); err != nil {
		t.Fatalf("write known_hosts: %v", err)
	}

	checkpointIdentity := "old-dev:old-inode"
	result, err := service.Acquire(context.Background(), source, &model.AcquisitionCheckpoint{
		SourceID:           1,
		TransportMode:      model.LogModeSSHPull,
		RemoteFileIdentity: &checkpointIdentity,
		LastRemoteOffset:   200,
	})
	if err != nil {
		t.Fatalf("Acquire returned error: %v", err)
	}
	if result.RemoteOffsetStart != 0 {
		t.Fatalf("expected rotated file to restart from 0, got %d", result.RemoteOffsetStart)
	}
	if result.RemoteOffsetEnd != 80 {
		t.Fatalf("expected remote end offset 80, got %d", result.RemoteOffsetEnd)
	}
}
