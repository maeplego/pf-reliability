package incident

import (
	"context"
	"strings"
	"time"

	"github.com/portfolio/pf-reliability/apps/api/internal/id"
)

type Repository interface {
	CreateService(ctx context.Context, s MonitoredService) error
	GetService(ctx context.Context, id string) (MonitoredService, error)
	GetServiceByCode(ctx context.Context, code string) (MonitoredService, error)
	ListServices(ctx context.Context) ([]MonitoredService, error)

	CreateIntegration(ctx context.Context, in Integration) error
	GetIntegrationByPublicKey(ctx context.Context, key string) (Integration, error)
	ListIntegrationsByService(ctx context.Context, serviceID string) ([]Integration, error)

	CreateIncident(ctx context.Context, inc Incident) error
	GetIncident(ctx context.Context, id string) (Incident, error)
	ListIncidents(ctx context.Context) ([]Incident, error)
	FindOpenByDedup(ctx context.Context, serviceID, dedupKey string) (Incident, error)
	UpdateIncident(ctx context.Context, inc Incident) error

	AppendEvent(ctx context.Context, ev Event) error
	ListEvents(ctx context.Context, incidentID string) ([]Event, error)

	SeenEventID(ctx context.Context, integrationID, eventID string) (incidentID string, ok bool)
	RememberEventID(ctx context.Context, integrationID, eventID, incidentID string) error
}

type Service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository, now func() time.Time) *Service {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Service{repo: repo, now: now}
}

func (s *Service) CreateMonitored(ctx context.Context, code, name, description string) (MonitoredService, error) {
	code = strings.ToLower(strings.TrimSpace(code))
	name = strings.TrimSpace(name)
	if code == "" || name == "" {
		return MonitoredService{}, ErrInvalid
	}
	if existing, err := s.repo.GetServiceByCode(ctx, code); err == nil {
		return existing, nil
	} else if err != ErrServiceNotFound {
		return MonitoredService{}, err
	}
	now := s.now()
	ms := MonitoredService{
		ID: id.New(), Code: code, Name: name,
		Description: strings.TrimSpace(description), CreatedAt: now,
	}
	if err := s.repo.CreateService(ctx, ms); err != nil {
		if err == ErrConflict {
			if existing, gerr := s.repo.GetServiceByCode(ctx, code); gerr == nil {
				return existing, nil
			}
		}
		return MonitoredService{}, err
	}
	return ms, nil
}

func (s *Service) EnsureIntegration(ctx context.Context, serviceID, publicKey, secret string) (Integration, error) {
	publicKey = strings.TrimSpace(publicKey)
	secret = strings.TrimSpace(secret)
	if publicKey == "" || secret == "" {
		return Integration{}, ErrInvalid
	}
	if existing, err := s.repo.GetIntegrationByPublicKey(ctx, publicKey); err == nil {
		existing.Secret = secret
		existing.ServiceID = serviceID
		if err := s.repo.CreateIntegration(ctx, existing); err != nil {
			return Integration{}, err
		}
		return existing, nil
	} else if err != ErrIntegrationMissing {
		return Integration{}, err
	}
	in := Integration{
		ID: id.New(), ServiceID: serviceID, PublicKey: publicKey, Secret: secret, CreatedAt: s.now(),
	}
	if err := s.repo.CreateIntegration(ctx, in); err != nil {
		return Integration{}, err
	}
	return in, nil
}

func (s *Service) ListServices(ctx context.Context) ([]MonitoredService, error) {
	return s.repo.ListServices(ctx)
}

func (s *Service) GetService(ctx context.Context, serviceID string) (MonitoredService, error) {
	return s.repo.GetService(ctx, serviceID)
}

func (s *Service) ListIntegrations(ctx context.Context, serviceID string) ([]Integration, error) {
	return s.repo.ListIntegrationsByService(ctx, serviceID)
}

func (s *Service) GetIntegration(ctx context.Context, publicKey string) (Integration, error) {
	return s.repo.GetIntegrationByPublicKey(ctx, publicKey)
}

type CreateInput struct {
	ServiceID string
	DedupKey  string
	Severity  string
	Summary   string
	Actor     string
}

func (s *Service) Create(ctx context.Context, in CreateInput) (Incident, bool, error) {
	svc, err := s.repo.GetService(ctx, strings.TrimSpace(in.ServiceID))
	if err != nil {
		return Incident{}, false, err
	}
	sev, err := ParseSeverity(strings.TrimSpace(in.Severity))
	if err != nil {
		return Incident{}, false, err
	}
	summary := strings.TrimSpace(in.Summary)
	if summary == "" {
		return Incident{}, false, ErrInvalid
	}
	dedup := strings.TrimSpace(in.DedupKey)
	if dedup == "" {
		dedup = id.New()
	}
	return s.openOrAggregate(ctx, svc.ID, dedup, sev, summary, strings.TrimSpace(in.Actor), "")
}

type IngestInput struct {
	Integration Integration
	EventID     string
	DedupKey    string
	Severity    string
	ServiceCode string
	Summary     string
}

func (s *Service) Ingest(ctx context.Context, in IngestInput) (Incident, bool, error) {
	eventID := strings.TrimSpace(in.EventID)
	if eventID != "" {
		if incidentID, ok := s.repo.SeenEventID(ctx, in.Integration.ID, eventID); ok {
			inc, err := s.repo.GetIncident(ctx, incidentID)
			return inc, false, err
		}
	}
	svc, err := s.repo.GetService(ctx, in.Integration.ServiceID)
	if err != nil {
		return Incident{}, false, err
	}
	code := strings.ToLower(strings.TrimSpace(in.ServiceCode))
	if code != "" && code != svc.Code {
		return Incident{}, false, ErrInvalid
	}
	dedup := strings.TrimSpace(in.DedupKey)
	if dedup == "" {
		return Incident{}, false, ErrInvalid
	}
	sev, err := ParseSeverity(strings.TrimSpace(in.Severity))
	if err != nil {
		return Incident{}, false, err
	}
	summary := strings.TrimSpace(in.Summary)
	if summary == "" {
		return Incident{}, false, ErrInvalid
	}
	inc, created, err := s.openOrAggregate(ctx, svc.ID, dedup, sev, summary, "webhook", eventID)
	if err != nil {
		return Incident{}, false, err
	}
	if eventID != "" {
		if err := s.repo.RememberEventID(ctx, in.Integration.ID, eventID, inc.ID); err != nil {
			return Incident{}, false, err
		}
	}
	return inc, created, nil
}

func (s *Service) openOrAggregate(ctx context.Context, serviceID, dedup string, sev Severity, summary, actor, eventID string) (Incident, bool, error) {
	existing, err := s.repo.FindOpenByDedup(ctx, serviceID, dedup)
	if err == nil {
		existing.AlertCount++
		existing.Severity = sev
		existing.Summary = summary
		existing.UpdatedAt = s.now()
		if err := s.repo.UpdateIncident(ctx, existing); err != nil {
			return Incident{}, false, err
		}
		msg := "repeat alert"
		if eventID != "" {
			msg = "repeat alert event " + eventID
		}
		if err := s.repo.AppendEvent(ctx, Event{
			ID: id.New(), IncidentID: existing.ID, Kind: KindAlertRepeat,
			Actor: actor, Message: msg, At: s.now(),
		}); err != nil {
			return Incident{}, false, err
		}
		return existing, false, nil
	}
	if err != ErrNotFound {
		return Incident{}, false, err
	}
	now := s.now()
	inc := Incident{
		ID: id.New(), ServiceID: serviceID, DedupKey: dedup,
		Severity: sev, Status: StatusTriggered, Summary: summary,
		AlertCount: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repo.CreateIncident(ctx, inc); err != nil {
		return Incident{}, false, err
	}
	if err := s.repo.AppendEvent(ctx, Event{
		ID: id.New(), IncidentID: inc.ID, Kind: KindOpened,
		Actor: actor, Message: summary, At: now,
	}); err != nil {
		return Incident{}, false, err
	}
	return inc, true, nil
}

func (s *Service) Get(ctx context.Context, incidentID string) (Incident, []Event, error) {
	inc, err := s.repo.GetIncident(ctx, incidentID)
	if err != nil {
		return Incident{}, nil, err
	}
	evs, err := s.repo.ListEvents(ctx, incidentID)
	if err != nil {
		return Incident{}, nil, err
	}
	return inc, evs, nil
}

func (s *Service) List(ctx context.Context, statusFilter string) ([]Incident, error) {
	list, err := s.repo.ListIncidents(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(statusFilter) == "" {
		return list, nil
	}
	st, err := ParseStatus(statusFilter)
	if err != nil {
		return nil, err
	}
	out := make([]Incident, 0, len(list))
	for _, inc := range list {
		if inc.Status == st {
			out = append(out, inc)
		}
	}
	return out, nil
}

func (s *Service) Ack(ctx context.Context, incidentID, actor string) (Incident, error) {
	inc, err := s.repo.GetIncident(ctx, incidentID)
	if err != nil {
		return Incident{}, err
	}
	next, err := ApplyAck(inc.Status)
	if err != nil {
		return Incident{}, err
	}
	changed := inc.Status != next || inc.AssigneeSub != actor
	inc.Status = next
	inc.AssigneeSub = actor
	now := s.now()
	inc.UpdatedAt = now
	if inc.AcknowledgedAt.IsZero() {
		inc.AcknowledgedAt = now
	}
	if err := s.repo.UpdateIncident(ctx, inc); err != nil {
		return Incident{}, err
	}
	if changed {
		if err := s.repo.AppendEvent(ctx, Event{
			ID: id.New(), IncidentID: inc.ID, Kind: KindAcknowledged,
			Actor: actor, Message: "acknowledged", At: now,
		}); err != nil {
			return Incident{}, err
		}
	}
	return inc, nil
}

func (s *Service) Resolve(ctx context.Context, incidentID, actor string) (Incident, error) {
	inc, err := s.repo.GetIncident(ctx, incidentID)
	if err != nil {
		return Incident{}, err
	}
	next, err := ApplyResolve(inc.Status)
	if err != nil {
		return Incident{}, err
	}
	changed := inc.Status != next
	inc.Status = next
	now := s.now()
	inc.UpdatedAt = now
	if inc.ResolvedAt.IsZero() {
		inc.ResolvedAt = now
	}
	if err := s.repo.UpdateIncident(ctx, inc); err != nil {
		return Incident{}, err
	}
	if changed {
		if err := s.repo.AppendEvent(ctx, Event{
			ID: id.New(), IncidentID: inc.ID, Kind: KindResolved,
			Actor: actor, Message: "resolved", At: now,
		}); err != nil {
			return Incident{}, err
		}
	}
	return inc, nil
}

func (s *Service) Comment(ctx context.Context, incidentID, actor, body string) (Event, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return Event{}, ErrInvalid
	}
	if _, err := s.repo.GetIncident(ctx, incidentID); err != nil {
		return Event{}, err
	}
	ev := Event{
		ID: id.New(), IncidentID: incidentID, Kind: KindComment,
		Actor: actor, Message: body, At: s.now(),
	}
	if err := s.repo.AppendEvent(ctx, ev); err != nil {
		return Event{}, err
	}
	return ev, nil
}
