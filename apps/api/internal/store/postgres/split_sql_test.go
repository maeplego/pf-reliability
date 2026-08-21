package postgres

import "testing"

func TestSplitSQLKeepsQuotedSemicolons(t *testing.T) {
	raw := `-- Incident rows are the current state; events is the timeline.
CREATE INDEX x ON t (a) WHERE status <> 'resolved';
CREATE TABLE reliability_events (id TEXT);
`
	got := splitSQL(raw)
	if len(got) != 2 {
		t.Fatalf("got %d stmts: %#v", len(got), got)
	}
	if got[0] != "CREATE INDEX x ON t (a) WHERE status <> 'resolved'" {
		t.Fatalf("stmt0 %q", got[0])
	}
	if got[1] != "CREATE TABLE reliability_events (id TEXT)" {
		t.Fatalf("stmt1 %q", got[1])
	}
}
