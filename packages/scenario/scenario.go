// Package scenario is a pure virtual-metrics engine.
// It never talks to a cluster, kubectl, or a real commerce API.
package scenario

import "errors"

var ErrUnknownAction = errors.New("unknown or disallowed action")

type State string

const (
	StateDegraded  State = "degraded"
	StateScaled    State = "scaled"
	StateRecovered State = "recovered"
)

type Action string

const (
	ActionObserve         Action = "observe"
	ActionRollback        Action = "rollback"
	ActionScale           Action = "scale"
	ActionEscalate        Action = "escalate"
	ActionDeclareResolved Action = "declare_resolved"
)

type Point struct {
	Name  string  `json:"name"`
	Unit  string  `json:"unit"`
	Value float64 `json:"value"`
}

type Snapshot struct {
	Scenario    string  `json:"scenario"`
	State       State   `json:"state"`
	Note        string  `json:"note"`
	VirtualOnly bool    `json:"virtualOnly"`
	Points      []Point `json:"points"`
}

const BadDeploy = "bad-deploy"

func Apply(state State, action Action) (State, error) {
	switch action {
	case ActionObserve, ActionEscalate:
		return state, nil
	case ActionScale:
		if state == StateRecovered {
			return StateRecovered, nil
		}
		return StateScaled, nil
	case ActionRollback:
		return StateRecovered, nil
	case ActionDeclareResolved:
		if state != StateRecovered {
			return state, ErrUnknownAction
		}
		return StateRecovered, nil
	default:
		return state, ErrUnknownAction
	}
}

func Metrics(state State) Snapshot {
	snap := Snapshot{
		Scenario:    BadDeploy,
		State:       state,
		VirtualOnly: true,
		Note:        "Synthetic series only. This product never runs kubectl, rollback bots, or production remediations.",
	}
	switch state {
	case StateRecovered:
		snap.Points = []Point{
			{Name: "error_ratio", Unit: "ratio", Value: 0.004},
			{Name: "p99_latency_ms", Unit: "ms", Value: 180},
			{Name: "minutes_since_deploy", Unit: "min", Value: 12},
		}
	case StateScaled:
		snap.Points = []Point{
			{Name: "error_ratio", Unit: "ratio", Value: 0.20},
			{Name: "p99_latency_ms", Unit: "ms", Value: 2100},
			{Name: "minutes_since_deploy", Unit: "min", Value: 10},
			{Name: "replica_count", Unit: "count", Value: 8},
		}
		snap.Note += " Scale-out did not fix a bad inventory deploy."
	default:
		snap.State = StateDegraded
		snap.Points = []Point{
			{Name: "error_ratio", Unit: "ratio", Value: 0.20},
			{Name: "p99_latency_ms", Unit: "ms", Value: 2000},
			{Name: "minutes_since_deploy", Unit: "min", Value: 10},
			{Name: "replica_count", Unit: "count", Value: 3},
		}
	}
	return snap
}

func After(action Action) (Snapshot, error) {
	next, err := Apply(StateDegraded, action)
	if err != nil {
		return Snapshot{}, err
	}
	return Metrics(next), nil
}

type ScoreResult struct {
	Scenario    string   `json:"scenario"`
	Actions     []Action `json:"actions"`
	FinalState  State    `json:"finalState"`
	Score       int      `json:"score"`
	Passed      bool     `json:"passed"`
	Penalties   []string `json:"penalties"`
	Notes       []string `json:"notes"`
	VirtualOnly bool     `json:"virtualOnly"`
}

// Score grades a training session. Scale-out is a known wrong move for bad-deploy
// (penalty) but rollback can still recover. The product never talks to a cluster.
// Score grades a bad-deploy training session.
func Score(actions []Action) (ScoreResult, error) {
	return ScoreNamed(BadDeploy, actions)
}
