package scenario_test

import (
	"testing"

	"github.com/portfolio/pf-reliability/packages/scenario"
)

func TestScaleDoesNotRecover(t *testing.T) {
	next, err := scenario.Apply(scenario.StateDegraded, scenario.ActionScale)
	if err != nil {
		t.Fatal(err)
	}
	if next != scenario.StateScaled {
		t.Fatalf("got %s", next)
	}
	snap := scenario.Metrics(next)
	if snap.Points[0].Value < 0.1 {
		t.Fatalf("scale should leave error ratio high: %+v", snap.Points)
	}
	if !snap.VirtualOnly {
		t.Fatal("metrics must stay virtual")
	}
}

func TestRollbackRecovers(t *testing.T) {
	next, err := scenario.Apply(scenario.StateDegraded, scenario.ActionRollback)
	if err != nil {
		t.Fatal(err)
	}
	if next != scenario.StateRecovered {
		t.Fatalf("got %s", next)
	}
	snap := scenario.Metrics(next)
	if snap.Points[0].Value >= 0.01 {
		t.Fatalf("rollback should drop error ratio: %+v", snap.Points)
	}
}

func TestDisallowedAction(t *testing.T) {
	if _, err := scenario.Apply(scenario.StateDegraded, scenario.Action("kubectl_delete")); err != scenario.ErrUnknownAction {
		t.Fatalf("got %v", err)
	}
	if _, err := scenario.Apply(scenario.StateDegraded, scenario.ActionDeclareResolved); err != scenario.ErrUnknownAction {
		t.Fatalf("declare while degraded: %v", err)
	}
}

func TestObserveKeepsState(t *testing.T) {
	next, err := scenario.Apply(scenario.StateScaled, scenario.ActionObserve)
	if err != nil || next != scenario.StateScaled {
		t.Fatalf("%s %v", next, err)
	}
}
