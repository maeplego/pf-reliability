package incident

import "errors"

var (
	ErrNotFound           = errors.New("incident not found")
	ErrInvalid            = errors.New("invalid incident")
	ErrConflict           = errors.New("incident conflict")
	ErrInvalidTransition  = errors.New("invalid status transition")
	ErrServiceNotFound    = errors.New("service not found")
	ErrIntegrationMissing = errors.New("integration not found")
)

type Status string

const (
	StatusTriggered    Status = "triggered"
	StatusAcknowledged Status = "acknowledged"
	StatusResolved     Status = "resolved"
)

type Severity string

const (
	SEV1 Severity = "SEV1"
	SEV2 Severity = "SEV2"
	SEV3 Severity = "SEV3"
	SEV4 Severity = "SEV4"
)

func ParseSeverity(s string) (Severity, error) {
	switch Severity(s) {
	case SEV1, SEV2, SEV3, SEV4:
		return Severity(s), nil
	default:
		return "", ErrInvalid
	}
}

func ParseStatus(s string) (Status, error) {
	switch Status(s) {
	case StatusTriggered, StatusAcknowledged, StatusResolved:
		return Status(s), nil
	default:
		return "", ErrInvalid
	}
}

// ApplyAck is the incident lifecycle: triggered → acknowledged.
// A second ack on an already-acknowledged incident is idempotent.
func ApplyAck(from Status) (Status, error) {
	switch from {
	case StatusTriggered:
		return StatusAcknowledged, nil
	case StatusAcknowledged:
		return StatusAcknowledged, nil
	case StatusResolved:
		return "", ErrInvalidTransition
	default:
		return "", ErrInvalid
	}
}

// ApplyResolve is triggered|acknowledged → resolved. Resolve on resolved is idempotent.
func ApplyResolve(from Status) (Status, error) {
	switch from {
	case StatusTriggered, StatusAcknowledged:
		return StatusResolved, nil
	case StatusResolved:
		return StatusResolved, nil
	default:
		return "", ErrInvalid
	}
}

func StatusRank(s Status) int {
	switch s {
	case StatusTriggered:
		return 0
	case StatusAcknowledged:
		return 1
	case StatusResolved:
		return 2
	default:
		return 9
	}
}

func SeverityRank(s Severity) int {
	switch s {
	case SEV1:
		return 0
	case SEV2:
		return 1
	case SEV3:
		return 2
	case SEV4:
		return 3
	default:
		return 9
	}
}

func OpenStatuses() []Status {
	return []Status{StatusTriggered, StatusAcknowledged}
}
