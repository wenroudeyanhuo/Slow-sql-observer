package tableingest

import (
	"database/sql"
	"strings"
	"testing"
	"time"
)

func TestParseUserHostFullFormat(t *testing.T) {
	user, host := parseUserHost("root[root] @ db-host [192.168.1.1]")
	if user == nil || *user != "root" {
		t.Fatalf("expected user 'root', got %v", user)
	}
	if host == nil || *host != "db-host" {
		t.Fatalf("expected host 'db-host', got %v", host)
	}
}

func TestParseUserHostUserOnly(t *testing.T) {
	user, host := parseUserHost("admin[admin] @")
	if user == nil || *user != "admin" {
		t.Fatalf("expected user 'admin', got %v", user)
	}
	if host != nil {
		t.Fatalf("expected nil host, got %v", host)
	}
}

func TestParseUserHostEmpty(t *testing.T) {
	user, host := parseUserHost("")
	if user != nil {
		t.Fatalf("expected nil user, got %v", user)
	}
	if host != nil {
		t.Fatalf("expected nil host, got %v", host)
	}
}

func TestParseTimeToSecondsNormal(t *testing.T) {
	result := parseTimeToSeconds("00:00:01.500000")
	if result < 1.4 || result > 1.6 {
		t.Fatalf("expected ~1.5 seconds, got %f", result)
	}
}

func TestParseTimeToSecondsHours(t *testing.T) {
	result := parseTimeToSeconds("01:02:03.000000")
	expected := 3600.0 + 120.0 + 3.0
	if result < expected-0.1 || result > expected+0.1 {
		t.Fatalf("expected ~%f seconds, got %f", expected, result)
	}
}

func TestParseTimeToSecondsInvalid(t *testing.T) {
	result := parseTimeToSeconds("invalid")
	if result != 0 {
		t.Fatalf("expected 0 for invalid input, got %f", result)
	}
}

func TestBuildRawBlockContainsEssentialParts(t *testing.T) {
	row := slowLogRow{
		StartTime:    time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC),
		UserHost:     "root[root] @ localhost [127.0.0.1]",
		QueryTime:    "00:00:02.000000",
		LockTime:     "00:00:00.000000",
		RowsSent:     1,
		RowsExamined: 100,
		DB:           sql.NullString{String: "testdb", Valid: true},
		SQLText:      "SELECT * FROM users WHERE id = 1",
	}
	block := buildRawBlock(row)
	for _, expected := range []string{
		"# Time:",
		"# User@Host:",
		"# Schema: testdb",
		"# Query_time:",
		"SET timestamp=",
		"SELECT * FROM users WHERE id = 1;",
	} {
		if !strings.Contains(block, expected) {
			t.Fatalf("expected block to contain %q, got:\n%s", expected, block)
		}
	}
}

func TestBuildRawBlockAppendsSemicolon(t *testing.T) {
	row := slowLogRow{
		StartTime: time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC),
		SQLText:   "SELECT 1",
	}
	block := buildRawBlock(row)
	if !strings.Contains(block, "SELECT 1;") {
		t.Fatalf("expected semicolon to be appended, got:\n%s", block)
	}
}

func TestBuildRawBlockNoDoubleSemicolon(t *testing.T) {
	row := slowLogRow{
		StartTime: time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC),
		SQLText:   "SELECT 1;",
	}
	block := buildRawBlock(row)
	if strings.Contains(block, "SELECT 1;;") {
		t.Fatalf("expected no double semicolons, got:\n%s", block)
	}
}
