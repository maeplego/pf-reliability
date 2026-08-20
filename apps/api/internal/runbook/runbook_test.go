package runbook_test

import (
	"testing"
	"time"

	"github.com/portfolio/pf-reliability/apps/api/internal/runbook"
)

func TestRunbookCRUD(t *testing.T) {
	s := runbook.NewStore()
	if err := s.Upsert(runbook.Book{ID: "a", Title: "A", Body: "steps"}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get("a")
	if err != nil || got.Title != "A" {
		t.Fatalf("%+v %v", got, err)
	}
	if err := s.Delete("a"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get("a"); err != runbook.ErrNotFound {
		t.Fatalf("got %v", err)
	}
}

func TestOnCallIsVirtual(t *testing.T) {
	oc := runbook.OnCallAt(time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC))
	if !oc.VirtualOnly || oc.Primary == "" {
		t.Fatalf("%+v", oc)
	}
}
