package incident_test

import (
	"testing"

	"github.com/portfolio/pf-reliability/apps/api/internal/incident"
)

func TestAckResolveHappyPath(t *testing.T) {
	next, err := incident.ApplyAck(incident.StatusTriggered)
	if err != nil || next != incident.StatusAcknowledged {
		t.Fatalf("ack triggered: %v %s", err, next)
	}
	next, err = incident.ApplyResolve(incident.StatusAcknowledged)
	if err != nil || next != incident.StatusResolved {
		t.Fatalf("resolve ack: %v %s", err, next)
	}
}

func TestResolveFromTriggered(t *testing.T) {
	next, err := incident.ApplyResolve(incident.StatusTriggered)
	if err != nil || next != incident.StatusResolved {
		t.Fatalf("got %s %v", next, err)
	}
}

func TestAckAndResolveAreIdempotent(t *testing.T) {
	next, err := incident.ApplyAck(incident.StatusAcknowledged)
	if err != nil || next != incident.StatusAcknowledged {
		t.Fatalf("ack ack: %v %s", err, next)
	}
	next, err = incident.ApplyResolve(incident.StatusResolved)
	if err != nil || next != incident.StatusResolved {
		t.Fatalf("resolve resolved: %v %s", err, next)
	}
}

func TestCannotAckResolved(t *testing.T) {
	_, err := incident.ApplyAck(incident.StatusResolved)
	if err != incident.ErrInvalidTransition {
		t.Fatalf("got %v", err)
	}
}

func TestUnknownStatusRejected(t *testing.T) {
	if _, err := incident.ApplyAck(incident.Status("open")); err != incident.ErrInvalid {
		t.Fatalf("ack: %v", err)
	}
	if _, err := incident.ApplyResolve(incident.Status("")); err != incident.ErrInvalid {
		t.Fatalf("resolve: %v", err)
	}
}

func TestParseSeverity(t *testing.T) {
	if _, err := incident.ParseSeverity("SEV2"); err != nil {
		t.Fatal(err)
	}
	if _, err := incident.ParseSeverity("sev2"); err != incident.ErrInvalid {
		t.Fatalf("got %v", err)
	}
}
