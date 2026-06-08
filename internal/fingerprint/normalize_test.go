package fingerprint

import "testing"

func TestEquivalentStatementsNormalizeToSameFingerprint(t *testing.T) {
	n := NewNormalizer()

	first := n.Process("SELECT * FROM orders WHERE id = 100 AND status IN ('new', 'paid') LIMIT 10;")
	second := n.Process("select * from orders where id = 200 and status in ('cancelled') limit 20;")

	if first.NormalizedSQL != second.NormalizedSQL {
		t.Fatalf("expected normalized SQL to match\nfirst: %s\nsecond: %s", first.NormalizedSQL, second.NormalizedSQL)
	}
	if first.Hash != second.Hash {
		t.Fatalf("expected fingerprint hashes to match")
	}
}

func TestDifferentStructureStaysSeparate(t *testing.T) {
	n := NewNormalizer()

	first := n.Process("SELECT * FROM orders WHERE id = 1")
	second := n.Process("SELECT * FROM orders WHERE status = 1")

	if first.Hash == second.Hash {
		t.Fatalf("expected different statements to produce different hashes")
	}
}
