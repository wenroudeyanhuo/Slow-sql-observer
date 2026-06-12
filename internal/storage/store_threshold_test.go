package storage

import (
	"strings"
	"testing"
	"time"

	"slow-sql-observer/internal/model"
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

func TestBuildDashboardTrendQueryIncludesThresholdWindowAndDBName(t *testing.T) {
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(7 * 24 * time.Hour)
	query, args := buildDashboardTrendQuery(7, "app_db", 1.25, model.TrendBucketDay, start, end)

	for _, expected := range []string{
		"source_id = ?",
		"occurred_at >= ?",
		"occurred_at < ?",
		"query_time_sec >= ?",
		"db_name = ?",
		"COUNT(DISTINCT fingerprint_id)",
		"DATE_FORMAT(occurred_at, '%Y-%m-%d 00:00:00')",
	} {
		if !strings.Contains(query, expected) {
			t.Fatalf("expected query to contain %q, got:\n%s", expected, query)
		}
	}

	if len(args) != 5 {
		t.Fatalf("expected 5 args, got %d", len(args))
	}
	if args[0] != int64(7) || args[3] != 1.25 || args[4] != "app_db" {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestBuildFingerprintTrendQueryIncludesThresholdAndHourBucket(t *testing.T) {
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(48 * time.Hour)
	query, args := buildFingerprintTrendQuery(7, 9, 2.5, model.TrendBucketHour, start, end)

	for _, expected := range []string{
		"source_id = ?",
		"fingerprint_id = ?",
		"occurred_at >= ?",
		"occurred_at < ?",
		"query_time_sec >= ?",
		"DATE_FORMAT(occurred_at, '%Y-%m-%d %H:00:00')",
	} {
		if !strings.Contains(query, expected) {
			t.Fatalf("expected query to contain %q, got:\n%s", expected, query)
		}
	}

	if len(args) != 5 {
		t.Fatalf("expected 5 args, got %d", len(args))
	}
	if args[0] != int64(7) || args[1] != int64(9) || args[4] != 2.5 {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestTrendWindowBoundsDailyAndHourly(t *testing.T) {
	now := time.Date(2026, 6, 12, 15, 37, 0, 0, time.UTC)

	dayStart, dayEnd := trendWindowBounds(model.TrendBucketDay, 7, now)
	if got, want := dayStart, time.Date(2026, 6, 6, 0, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("expected daily window start %s, got %s", want, got)
	}
	if got, want := dayEnd, time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("expected daily window end %s, got %s", want, got)
	}

	hourStart, hourEnd := trendWindowBounds(model.TrendBucketHour, 2, now)
	if got, want := hourStart, time.Date(2026, 6, 10, 16, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("expected hourly window start %s, got %s", want, got)
	}
	if got, want := hourEnd, time.Date(2026, 6, 12, 16, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("expected hourly window end %s, got %s", want, got)
	}
}
