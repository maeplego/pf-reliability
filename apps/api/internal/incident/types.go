package incident

import "time"

type MonitoredService struct {
	ID          string
	Code        string
	Name        string
	Description string
	CreatedAt   time.Time
}

type Integration struct {
	ID        string
	ServiceID string
	PublicKey string
	Secret    string
	CreatedAt time.Time
}

type Incident struct {
	ID             string
	ServiceID      string
	DedupKey       string
	Severity       Severity
	Status         Status
	Summary        string
	AssigneeSub    string
	AlertCount     int
	CreatedAt      time.Time
	UpdatedAt      time.Time
	ResolvedAt     time.Time
	AcknowledgedAt time.Time
}

type EventKind string

const (
	KindOpened       EventKind = "opened"
	KindAcknowledged EventKind = "acknowledged"
	KindResolved     EventKind = "resolved"
	KindComment      EventKind = "comment"
	KindAlertRepeat  EventKind = "alert_repeat"
)

type Event struct {
	ID         string
	IncidentID string
	Kind       EventKind
	Actor      string
	Message    string
	At         time.Time
}

func MaskSecret(secret string) string {
	if len(secret) <= 4 {
		return "••••"
	}
	return "••••" + secret[len(secret)-4:]
}
