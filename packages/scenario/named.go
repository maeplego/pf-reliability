package scenario

import (
	"errors"
	"strings"
)

var ErrUnknownScenario = errors.New("unknown scenario")

const (
	NoisyNeighbor      = "noisy-neighbor"
	DependencyTimeout  = "dependency-timeout"
)

func NormalizeName(name string) string {
	n := strings.TrimSpace(strings.ToLower(name))
	if n == "" {
		return BadDeploy
	}
	return n
}

func Known(name string) bool {
	switch NormalizeName(name) {
	case BadDeploy, NoisyNeighbor, DependencyTimeout:
		return true
	default:
		return false
	}
}

func Names() []string {
	return []string{BadDeploy, NoisyNeighbor, DependencyTimeout}
}

func ApplyNamed(name string, state State, action Action) (State, error) {
	switch NormalizeName(name) {
	case BadDeploy:
		return Apply(state, action)
	case NoisyNeighbor:
		return applyNoisy(state, action)
	case DependencyTimeout:
		return applyTimeout(state, action)
	default:
		return state, ErrUnknownScenario
	}
}

func applyNoisy(state State, action Action) (State, error) {
	switch action {
	case ActionObserve, ActionEscalate:
		return state, nil
	case ActionScale:
		return StateRecovered, nil
	case ActionRollback:
		if state == StateRecovered {
			return StateDegraded, nil
		}
		return state, nil
	case ActionDeclareResolved:
		if state != StateRecovered {
			return state, ErrUnknownAction
		}
		return StateRecovered, nil
	default:
		return state, ErrUnknownAction
	}
}

func applyTimeout(state State, action Action) (State, error) {
	switch action {
	case ActionObserve:
		return state, nil
	case ActionEscalate:
		return StateRecovered, nil
	case ActionScale:
		if state == StateRecovered {
			return StateRecovered, nil
		}
		return StateScaled, nil
	case ActionRollback:
		return state, nil
	case ActionDeclareResolved:
		if state != StateRecovered {
			return state, ErrUnknownAction
		}
		return StateRecovered, nil
	default:
		return state, ErrUnknownAction
	}
}

func MetricsNamed(name string, state State) Snapshot {
	name = NormalizeName(name)
	snap := Metrics(state)
	snap.Scenario = name
	switch name {
	case NoisyNeighbor:
		snap.Note = "Virtual noisy-neighbor: scale is the intended recovery. This product never talks to a cluster."
		if state == StateRecovered {
			snap.Points = []Point{
				{Name: "error_ratio", Unit: "ratio", Value: 0.005},
				{Name: "cpu_steal_ratio", Unit: "ratio", Value: 0.04},
			}
		} else {
			snap.Points = []Point{
				{Name: "error_ratio", Unit: "ratio", Value: 0.12},
				{Name: "cpu_steal_ratio", Unit: "ratio", Value: 0.55},
			}
		}
	case DependencyTimeout:
		snap.Note = "Virtual dependency timeout: escalate (fictional on-call) is the intended move. No paging vendor."
		if state == StateRecovered {
			snap.Points = []Point{
				{Name: "error_ratio", Unit: "ratio", Value: 0.003},
				{Name: "upstream_p99_ms", Unit: "ms", Value: 120},
			}
		} else {
			snap.Points = []Point{
				{Name: "error_ratio", Unit: "ratio", Value: 0.18},
				{Name: "upstream_p99_ms", Unit: "ms", Value: 4800},
			}
		}
	}
	return snap
}

func AfterNamed(name string, action Action) (Snapshot, error) {
	next, err := ApplyNamed(name, StateDegraded, action)
	if err != nil {
		return Snapshot{}, err
	}
	return MetricsNamed(name, next), nil
}

func ScoreNamed(name string, actions []Action) (ScoreResult, error) {
	name = NormalizeName(name)
	if !Known(name) {
		return ScoreResult{}, ErrUnknownScenario
	}
	state := StateDegraded
	score := 100
	var penalties []string
	var notes []string
	usedRollback := false
	usedScale := false
	usedEscalate := false
	copied := make([]Action, 0, len(actions))
	for _, action := range actions {
		copied = append(copied, action)
		next, err := ApplyNamed(name, state, action)
		if err != nil {
			return ScoreResult{}, err
		}
		switch action {
		case ActionScale:
			usedScale = true
			if name == BadDeploy && state != StateRecovered {
				score -= 25
				penalties = append(penalties, "scale does not fix a bad inventory deploy")
			}
			if name == DependencyTimeout && state != StateRecovered {
				score -= 25
				penalties = append(penalties, "scale does not fix an upstream timeout")
			}
			if name == NoisyNeighbor {
				notes = append(notes, "scale recovered the virtual noisy-neighbor")
			}
		case ActionRollback:
			usedRollback = true
			if name == BadDeploy {
				notes = append(notes, "rollback recovered the virtual error ratio")
			}
			if name == NoisyNeighbor {
				score -= 25
				penalties = append(penalties, "rollback is the wrong move for noisy-neighbor")
			}
			if name == DependencyTimeout {
				score -= 15
				penalties = append(penalties, "rollback does not restore the fictional upstream")
			}
		case ActionObserve:
			notes = append(notes, "observe only; virtual metrics")
		case ActionEscalate:
			usedEscalate = true
			if name == DependencyTimeout {
				notes = append(notes, "escalate recovered the virtual dependency timeout")
			} else {
				notes = append(notes, "escalate is logged; this demo does not page a human")
			}
		}
		state = next
	}
	switch name {
	case BadDeploy:
		if !usedRollback {
			score -= 40
			penalties = append(penalties, "rollback was not used")
		}
	case NoisyNeighbor:
		if !usedScale {
			score -= 40
			penalties = append(penalties, "scale was not used")
		}
	case DependencyTimeout:
		if !usedEscalate {
			score -= 40
			penalties = append(penalties, "escalate was not used")
		}
	}
	if state != StateRecovered {
		score -= 40
		penalties = append(penalties, "session ended without recovery")
	}
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	passed := state == StateRecovered
	switch name {
	case BadDeploy:
		passed = passed && usedRollback
	case NoisyNeighbor:
		passed = passed && usedScale
	case DependencyTimeout:
		passed = passed && usedEscalate
	}
	return ScoreResult{
		Scenario:    name,
		Actions:     copied,
		FinalState:  state,
		Score:       score,
		Passed:      passed,
		Penalties:   penalties,
		Notes:       notes,
		VirtualOnly: true,
	}, nil
}
