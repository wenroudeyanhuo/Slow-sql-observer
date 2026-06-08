package parser

import "testing"

const sampleBlock = `# Time: 2026-06-04T10:10:10.123456Z
# User@Host: root[root] @ localhost [127.0.0.1]
# Query_time: 2.345678  Lock_time: 0.000123 Rows_sent: 1  Rows_examined: 100
use app_db;
SET timestamp=1717495810;
SELECT *
FROM orders
WHERE id = 123;
`

func TestParseStandardBlock(t *testing.T) {
	p := New()
	record, err := p.Parse(sampleBlock)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if record.QueryTimeSec != 2.345678 {
		t.Fatalf("unexpected query time: %v", record.QueryTimeSec)
	}
	if record.DBName == nil || *record.DBName != "app_db" {
		t.Fatalf("unexpected db name: %#v", record.DBName)
	}
	if record.RawSQL == "" {
		t.Fatalf("expected raw SQL to be present")
	}
}

func TestParseAllowsMissingOptionalFields(t *testing.T) {
	p := New()
	block := `# Time: 2026-06-04T10:10:10.123456Z
# Query_time: 1.000000  Lock_time: 0.000000 Rows_sent: 0  Rows_examined: 10
SELECT 1;
`
	record, err := p.Parse(block)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if record.UserName != nil || record.ClientHost != nil {
		t.Fatalf("expected optional fields to remain nil")
	}
}
