package collector

import (
	"testing"

	"slow-sql-observer/internal/model"
)

func TestFrameBlocksPreservesIncompleteTail(t *testing.T) {
	data := []byte(`# Time: 2026-06-04T10:10:10.123456Z
# Query_time: 1.000000  Lock_time: 0.000000 Rows_sent: 1  Rows_examined: 10
SELECT 1;
# Time: 2026-06-04T10:11:10.123456Z
# Query_time: 2.000000  Lock_time: 0.000000 Rows_sent: 1  Rows_examined: 20
SELECT`)

	blocks, remainder, offset := frameBlocks(data, 0)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 complete block, got %d", len(blocks))
	}
	if len(remainder) == 0 {
		t.Fatalf("expected incomplete remainder to be preserved")
	}
	if offset != blocks[0].EndOffset {
		t.Fatalf("expected remainder offset to match end of first block")
	}
}

func TestResolveStartOffset(t *testing.T) {
	state := FileState{Identity: "same", Size: 200}
	checkpoint := &struct {
		FileIdentity string
		LastOffset   int64
	}{FileIdentity: "same", LastOffset: 100}

	start := ResolveStartOffset(&model.CollectorCheckpoint{
		FileIdentity: checkpoint.FileIdentity,
		LastOffset:   checkpoint.LastOffset,
	}, state)
	if start != 100 {
		t.Fatalf("expected to resume at 100, got %d", start)
	}
}
