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

func TestScoreScaleThenRollbackStillPassesWithPenalty(t *testing.T) {
	got, err := scenario.Score([]scenario.Action{scenario.ActionScale, scenario.ActionRollback, scenario.ActionDeclareResolved})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Passed || got.FinalState != scenario.StateRecovered {
		t.Fatalf("%+v", got)
	}
	if got.Score >= 100 {
		t.Fatalf("scale should cost points: %d", got.Score)
	}
	if len(got.Penalties) == 0 {
		t.Fatal("expected a scale penalty")
	}
	if !got.VirtualOnly {
		t.Fatal("must stay virtual")
	}
}

func TestScoreRollbackAlonePasses(t *testing.T) {
	got, err := scenario.Score([]scenario.Action{scenario.ActionRollback})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Passed || got.Score != 100 {
		t.Fatalf("%+v", got)
	}
}

func TestScoreScaleOnlyFails(t *testing.T) {
	got, err := scenario.Score([]scenario.Action{scenario.ActionScale})
	if err != nil {
		t.Fatal(err)
	}
	if got.Passed || got.FinalState != scenario.StateScaled {
		t.Fatalf("%+v", got)
	}
}

func TestScoreRejectsClusterFantasy(t *testing.T) {
	if _, err := scenario.Score([]scenario.Action{scenario.Action("kubectl")}); err != scenario.ErrUnknownAction {
		t.Fatalf("got %v", err)
	}
}
