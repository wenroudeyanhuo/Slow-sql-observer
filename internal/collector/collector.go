package collector

import (
	"context"
	"fmt"
	"os"
	"strings"

	"slow-sql-observer/internal/model"
)

type FileState struct {
	Identity string
	Size     int64
}

type FramedBlock struct {
	StartOffset int64
	EndOffset   int64
	Raw         string
}

type Framer struct {
	pending       []byte
	pendingOffset int64
}

func NewFramer() *Framer {
	return &Framer{}
}

func (f *Framer) HasPending() bool {
	return len(f.pending) > 0
}

func (f *Framer) Reset() {
	f.pending = nil
	f.pendingOffset = 0
}

func ResolveStartOffset(checkpoint *model.CollectorCheckpoint, state FileState) int64 {
	if checkpoint == nil {
		return 0
	}
	if checkpoint.FileIdentity != state.Identity {
		return 0
	}
	if state.Size < checkpoint.LastOffset {
		return 0
	}
	return checkpoint.LastOffset
}

func StatFile(path string) (FileState, error) {
	info, err := os.Stat(path)
	if err != nil {
		return FileState{}, err
	}
	identity, err := fileIdentity(path)
	if err != nil {
		return FileState{}, err
	}
	return FileState{
		Identity: identity,
		Size:     info.Size(),
	}, nil
}

func (f *Framer) ReadNewBlocks(ctx context.Context, path string, startOffset int64) (FileState, []FramedBlock, error) {
	state, err := StatFile(path)
	if err != nil {
		return FileState{}, nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return FileState{}, nil, err
	}
	defer file.Close()

	if f.pending == nil || startOffset != f.pendingOffset {
		f.pending = nil
		f.pendingOffset = startOffset
	}

	chunkSize := state.Size - startOffset
	if chunkSize < 0 {
		chunkSize = 0
	}
	buffer := make([]byte, chunkSize)
	if chunkSize > 0 {
		if _, err := file.ReadAt(buffer, startOffset); err != nil {
			return FileState{}, nil, fmt.Errorf("read slow log: %w", err)
		}
	}
	data := append(append([]byte{}, f.pending...), buffer...)
	blocks, remainder, remainderOffset := frameBlocks(data, f.pendingOffset)
	f.pending = remainder
	f.pendingOffset = remainderOffset
	return state, blocks, nil
}

func frameBlocks(data []byte, baseOffset int64) ([]FramedBlock, []byte, int64) {
	normalized := strings.ReplaceAll(string(data), "\r\n", "\n")
	starts := findTimeHeaders(normalized)
	if len(starts) == 0 {
		return nil, data, baseOffset
	}

	var blocks []FramedBlock
	for i := 0; i < len(starts); i++ {
		start := starts[i]
		end := len(normalized)
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		block := strings.TrimSpace(normalized[start:end])
		if block == "" {
			continue
		}
		if i == len(starts)-1 && !isLikelyCompleteBlock(block) {
			remainder := []byte(normalized[start:])
			return blocks, remainder, baseOffset + int64(start)
		}
		blocks = append(blocks, FramedBlock{
			StartOffset: baseOffset + int64(start),
			EndOffset:   baseOffset + int64(end),
			Raw:         block,
		})
	}

	return blocks, nil, baseOffset + int64(len(normalized))
}

func findTimeHeaders(input string) []int {
	var positions []int
	if strings.HasPrefix(input, "# Time:") {
		positions = append(positions, 0)
	}
	for index := 0; ; {
		found := strings.Index(input[index:], "\n# Time:")
		if found == -1 {
			break
		}
		pos := index + found + 1
		positions = append(positions, pos)
		index = pos
	}
	return positions
}

func isLikelyCompleteBlock(block string) bool {
	trimmed := strings.TrimSpace(block)
	return strings.Contains(trimmed, "# Query_time:") && strings.HasSuffix(trimmed, ";")
}
