package storage

import (
	"strings"
	"testing"
)

func TestNormalizeMinQueryTimeSec(t *testing.T) {
	if got := normalizeMinQueryTimeSec(-1); got != 0 {
		t.Fatalf("expected negative threshold to normalize to 0, got %v", got)
	}
	if got := normalizeMinQueryTimeSec(0); got != 0 {
		t.Fatalf("expected zero threshold to stay 0, got %v", got)
	}
	if got := normalizeMinQueryTimeSec(1.5); got != 1.5 {
		t.Fatalf("expected positive threshold to be preserved, got %v", got)
	}
}

func TestBuildFingerprintAggregationQueryIncludesThresholdAndDBName(t *testing.T) {
	query, args := buildFingerprintAggregationQuery(7, "app_db", 1.25)

	for _, expected := range []string{
		"r.source_id = ?",
		"r.query_time_sec >= ?",
		"r.db_name = ?",
		"GROUP BY r.fingerprint_id",
	} {
		if !strings.Contains(query, expected) {
			t.Fatalf("expected query to contain %q, got:\n%s", expected, query)
		}
	}

	if len(args) != 3 {
		t.Fatalf("expected 3 args, got %d", len(args))
	}
	if args[0] != int64(7) || args[1] != 1.25 || args[2] != "app_db" {
		t.Fatalf("unexpected args: %#v", args)
	}
}
