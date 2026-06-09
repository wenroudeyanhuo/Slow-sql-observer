package model

import "testing"

func TestSourceKeyChangesWithIdentityInputs(t *testing.T) {
	first := SourceKey("orders-db", "/var/log/mysql/slow.log")
	second := SourceKey("orders-db", "/var/log/mysql/slow.log")
	third := SourceKey("orders-db", "/var/log/mysql/other.log")

	if first != second {
		t.Fatalf("expected identical source identity inputs to produce the same key")
	}
	if first == third {
		t.Fatalf("expected a different slow log path to produce a different key")
	}
}
