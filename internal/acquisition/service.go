package acquisition

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"slow-sql-observer/internal/collector"
	"slow-sql-observer/internal/config"
	"slow-sql-observer/internal/model"
)

type SSHClient interface {
	ValidateLinux(ctx context.Context, source config.SourceConfig) error
	Stat(ctx context.Context, source config.SourceConfig) (collector.FileState, error)
	ReadFrom(ctx context.Context, source config.SourceConfig, offset int64) ([]byte, error)
}

type Service struct {
	ssh SSHClient
}

func NewService(ssh SSHClient) *Service {
	if ssh == nil {
		ssh = &remoteSSHClient{}
	}
	return &Service{ssh: ssh}
}

func (s *Service) Acquire(ctx context.Context, source config.SourceConfig, checkpoint *model.AcquisitionCheckpoint) (model.AcquisitionResult, error) {
	switch source.LogMode {
	case model.LogModeLocalFile:
		return s.acquireLocal(ctx, source)
	case model.LogModeSSHPull:
		return s.acquireRemote(ctx, source, checkpoint)
	default:
		return model.AcquisitionResult{
			TransportMode:        source.LogMode,
			AcquisitionState:     model.AcquisitionStateBlocked,
			RemoteAccessState:    model.SourceAccessUnknown,
			BlockedConfiguration: true,
		}, fmt.Errorf("unsupported source log mode: %s", source.LogMode)
	}
}

func (s *Service) acquireLocal(ctx context.Context, source config.SourceConfig) (model.AcquisitionResult, error) {
	state, err := collector.StatFile(source.SlowLogPath)
	if err != nil {
		return model.AcquisitionResult{
			ParsePath:         source.SlowLogPath,
			TransportMode:     model.LogModeLocalFile,
			RemoteAccessState: model.SourceAccessInaccessible,
			AcquisitionState:  model.AcquisitionStateError,
		}, err
	}
	return model.AcquisitionResult{
		ParsePath:          source.SlowLogPath,
		TransportMode:      model.LogModeLocalFile,
		RemoteAccessState:  model.SourceAccessAccessible,
		RemoteFileIdentity: state.Identity,
		RemoteOffsetEnd:    state.Size,
		ShouldParse:        true,
		AcquisitionState:   model.AcquisitionStateHealthy,
	}, nil
}

func (s *Service) acquireRemote(ctx context.Context, source config.SourceConfig, checkpoint *model.AcquisitionCheckpoint) (model.AcquisitionResult, error) {
	spoolPath := source.EffectiveParsePath()
	result := model.AcquisitionResult{
		ParsePath:        spoolPath,
		SpoolPath:        spoolPath,
		TransportMode:    model.LogModeSSHPull,
		ShouldParse:      true,
		AcquisitionState: model.AcquisitionStateHealthy,
	}

	if err := source.Validate(); err != nil {
		result.ShouldParse = fileExists(spoolPath)
		result.BlockedConfiguration = true
		result.AcquisitionState = model.AcquisitionStateBlocked
		result.RemoteAccessState = model.SourceAccessUnknown
		return result, err
	}

	if err := os.MkdirAll(filepath.Dir(spoolPath), 0o755); err != nil {
		result.ShouldParse = fileExists(spoolPath)
		result.BlockedConfiguration = true
		result.AcquisitionState = model.AcquisitionStateBlocked
		result.RemoteAccessState = model.SourceAccessUnknown
		return result, err
	}
	spoolSize, err := ensureSpoolFile(spoolPath)
	if err != nil {
		result.ShouldParse = fileExists(spoolPath)
		result.BlockedConfiguration = true
		result.AcquisitionState = model.AcquisitionStateBlocked
		result.RemoteAccessState = model.SourceAccessUnknown
		return result, err
	}
	result.SpoolSizeBytes = spoolSize
	result.ShouldParse = spoolSize > 0

	if err := s.ssh.ValidateLinux(ctx, source); err != nil {
		result.AcquisitionState = model.AcquisitionStateBlocked
		result.RemoteAccessState = model.SourceAccessInaccessible
		return result, err
	}

	state, err := s.ssh.Stat(ctx, source)
	if err != nil {
		result.AcquisitionState = model.AcquisitionStateError
		result.RemoteAccessState = model.SourceAccessInaccessible
		return result, err
	}
	result.RemoteAccessState = model.SourceAccessAccessible
	result.RemoteFileIdentity = state.Identity

	startOffset := resolveRemoteStartOffset(source, checkpoint, state)
	result.RemoteOffsetStart = startOffset
	result.RemoteOffsetEnd = startOffset

	if startOffset > state.Size {
		startOffset = 0
		result.RemoteOffsetStart = 0
	}
	bytesToPull := state.Size - startOffset
	if bytesToPull < 0 {
		bytesToPull = 0
	}
	if bytesToPull == 0 {
		result.RemoteOffsetEnd = state.Size
		result.ShouldParse = fileExists(spoolPath)
		return result, nil
	}

	if source.LocalSpoolMaxBytes > 0 && spoolSize+bytesToPull > source.LocalSpoolMaxBytes {
		result.AcquisitionState = model.AcquisitionStateBlocked
		result.RemoteOffsetEnd = startOffset
		result.ShouldParse = spoolSize > 0
		return result, fmt.Errorf("local spool size limit exceeded before pull (%d + %d > %d)", spoolSize, bytesToPull, source.LocalSpoolMaxBytes)
	}

	payload, err := s.ssh.ReadFrom(ctx, source, startOffset)
	if err != nil {
		result.AcquisitionState = model.AcquisitionStateError
		result.RemoteOffsetEnd = startOffset
		return result, err
	}
	if int64(len(payload)) != bytesToPull {
		result.AcquisitionState = model.AcquisitionStateError
		result.RemoteOffsetEnd = startOffset
		return result, fmt.Errorf("remote read length mismatch: expected %d bytes, got %d", bytesToPull, len(payload))
	}
	if err := appendSpool(spoolPath, payload); err != nil {
		result.AcquisitionState = model.AcquisitionStateError
		result.RemoteOffsetEnd = startOffset
		return result, err
	}

	result.RemoteOffsetEnd = state.Size
	result.SpoolSizeBytes = spoolSize + int64(len(payload))
	result.ShouldParse = result.SpoolSizeBytes > 0
	return result, nil
}

func resolveRemoteStartOffset(source config.SourceConfig, checkpoint *model.AcquisitionCheckpoint, state collector.FileState) int64 {
	if checkpoint == nil {
		if source.InitialPosition == model.InitialPositionStart {
			return 0
		}
		return state.Size
	}
	if checkpoint.RemoteFileIdentity != nil && *checkpoint.RemoteFileIdentity != "" && *checkpoint.RemoteFileIdentity == state.Identity && state.Size >= checkpoint.LastRemoteOffset {
		return checkpoint.LastRemoteOffset
	}
	return 0
}

func ensureSpoolFile(path string) (int64, error) {
	file, err := os.OpenFile(path, os.O_CREATE, 0o644)
	if err != nil {
		return 0, err
	}
	file.Close()
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func appendSpool(path string, payload []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(payload)
	return err
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

type remoteSSHClient struct{}

func (c *remoteSSHClient) ValidateLinux(ctx context.Context, source config.SourceConfig) error {
	output, err := runSSHCommand(ctx, source, "uname -s")
	if err != nil {
		return err
	}
	if !strings.Contains(strings.TrimSpace(string(output)), "Linux") {
		return fmt.Errorf("unsupported remote platform: expected Linux/OpenSSH source")
	}
	return nil
}

func (c *remoteSSHClient) Stat(ctx context.Context, source config.SourceConfig) (collector.FileState, error) {
	cmd := fmt.Sprintf("FILE=%s; stat -c '%%d:%%i:%%s' -- \"$FILE\"", shellQuote(source.RemoteSlowLogPath))
	output, err := runSSHCommand(ctx, source, cmd)
	if err != nil {
		return collector.FileState{}, err
	}
	parts := strings.Split(strings.TrimSpace(string(output)), ":")
	if len(parts) != 3 {
		return collector.FileState{}, fmt.Errorf("unexpected stat output: %q", strings.TrimSpace(string(output)))
	}
	size, err := parseInt64(parts[2])
	if err != nil {
		return collector.FileState{}, err
	}
	return collector.FileState{
		Identity: strings.TrimSpace(parts[0]) + ":" + strings.TrimSpace(parts[1]),
		Size:     size,
	}, nil
}

func (c *remoteSSHClient) ReadFrom(ctx context.Context, source config.SourceConfig, offset int64) ([]byte, error) {
	cmd := fmt.Sprintf("FILE=%s; dd if=\"$FILE\" bs=1 skip=%d status=none 2>/dev/null", shellQuote(source.RemoteSlowLogPath), offset)
	return runSSHCommand(ctx, source, cmd)
}
