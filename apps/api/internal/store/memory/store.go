package memory

import (
	"context"
	"sort"
	"sync"

	"github.com/portfolio/pf-reliability/apps/api/internal/incident"
)

type Store struct {
	mu             sync.Mutex
	services       map[string]incident.MonitoredService
	servicesByCode map[string]string
	integrations   map[string]incident.Integration
	intByKey       map[string]string
	incidents      map[string]incident.Incident
	events         map[string][]incident.Event
	eventIDs       map[string]string
}

func New() *Store {
	return &Store{
		services:       map[string]incident.MonitoredService{},
		servicesByCode: map[string]string{},
		integrations:   map[string]incident.Integration{},
		intByKey:       map[string]string{},
		incidents:      map[string]incident.Incident{},
		events:         map[string][]incident.Event{},
		eventIDs:       map[string]string{},
	}
}

func (s *Store) Ping(context.Context) error { return nil }

func (s *Store) CreateService(_ context.Context, ms incident.MonitoredService) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id, ok := s.servicesByCode[ms.Code]; ok && id != ms.ID {
		return incident.ErrConflict
	}
	s.services[ms.ID] = ms
	s.servicesByCode[ms.Code] = ms.ID
	return nil
}

func (s *Store) GetService(_ context.Context, id string) (incident.MonitoredService, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ms, ok := s.services[id]
	if !ok {
		return incident.MonitoredService{}, incident.ErrServiceNotFound
	}
	return ms, nil
}

func (s *Store) GetServiceByCode(_ context.Context, code string) (incident.MonitoredService, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.servicesByCode[code]
	if !ok {
		return incident.MonitoredService{}, incident.ErrServiceNotFound
	}
	return s.services[id], nil
}

func (s *Store) ListServices(context.Context) ([]incident.MonitoredService, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]incident.MonitoredService, 0, len(s.services))
	for _, ms := range s.services {
		out = append(out, ms)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out, nil
}

func (s *Store) CreateIntegration(_ context.Context, in incident.Integration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id, ok := s.intByKey[in.PublicKey]; ok && id != in.ID {
		return incident.ErrConflict
	}
	s.integrations[in.ID] = in
	s.intByKey[in.PublicKey] = in.ID
	return nil
}

func (s *Store) GetIntegrationByPublicKey(_ context.Context, key string) (incident.Integration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.intByKey[key]
	if !ok {
		return incident.Integration{}, incident.ErrIntegrationMissing
	}
	return s.integrations[id], nil
}

func (s *Store) ListIntegrationsByService(_ context.Context, serviceID string) ([]incident.Integration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []incident.Integration
	for _, in := range s.integrations {
		if in.ServiceID == serviceID {
			out = append(out, in)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PublicKey < out[j].PublicKey })
	return out, nil
}

func (s *Store) CreateIncident(_ context.Context, inc incident.Incident) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := inc
	s.incidents[inc.ID] = cp
	return nil
}

func (s *Store) GetIncident(_ context.Context, id string) (incident.Incident, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	inc, ok := s.incidents[id]
	if !ok {
		return incident.Incident{}, incident.ErrNotFound
	}
	return inc, nil
}

func (s *Store) ListIncidents(context.Context) ([]incident.Incident, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]incident.Incident, 0, len(s.incidents))
	for _, inc := range s.incidents {
		out = append(out, inc)
	}
	sort.Slice(out, func(i, j int) bool {
		if ri, rj := incident.StatusRank(out[i].Status), incident.StatusRank(out[j].Status); ri != rj {
			return ri < rj
		}
		if si, sj := incident.SeverityRank(out[i].Severity), incident.SeverityRank(out[j].Severity); si != sj {
			return si < sj
		}
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out, nil
}

func (s *Store) FindOpenByDedup(_ context.Context, serviceID, dedupKey string) (incident.Incident, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var found incident.Incident
	ok := false
	for _, inc := range s.incidents {
		if inc.ServiceID == serviceID && inc.DedupKey == dedupKey && inc.Status != incident.StatusResolved {
			if !ok || inc.CreatedAt.After(found.CreatedAt) {
				found = inc
				ok = true
			}
		}
	}
	if !ok {
		return incident.Incident{}, incident.ErrNotFound
	}
	return found, nil
}

func (s *Store) UpdateIncident(_ context.Context, inc incident.Incident) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.incidents[inc.ID]; !ok {
		return incident.ErrNotFound
	}
	s.incidents[inc.ID] = inc
	return nil
}

func (s *Store) AppendEvent(_ context.Context, ev incident.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events[ev.IncidentID] = append(s.events[ev.IncidentID], ev)
	return nil
}

func (s *Store) ListEvents(_ context.Context, incidentID string) ([]incident.Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	src := s.events[incidentID]
	out := make([]incident.Event, len(src))
	copy(out, src)
	sort.SliceStable(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
	return out, nil
}

func eventKey(integrationID, eventID string) string {
	return integrationID + "\x00" + eventID
}

func (s *Store) SeenEventID(_ context.Context, integrationID, eventID string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.eventIDs[eventKey(integrationID, eventID)]
	return id, ok
}

func (s *Store) RememberEventID(_ context.Context, integrationID, eventID, incidentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eventIDs[eventKey(integrationID, eventID)] = incidentID
	return nil
}
