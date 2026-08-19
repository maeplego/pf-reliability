package postgres

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/portfolio/pf-reliability/apps/api/internal/incident"
)

type scanner interface {
	Scan(dest ...any) error
}

//go:embed schema.sql
var schemaFS embed.FS

type Store struct {
	pool *pgxpool.Pool
}

func Open(ctx context.Context, url string) (*Store, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	s := &Store{pool: pool}
	if err := s.migrate(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() { s.pool.Close() }

func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

func (s *Store) migrate(ctx context.Context) error {
	raw, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		return err
	}
	for _, stmt := range splitSQL(string(raw)) {
		if _, err := s.pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func splitSQL(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ";") {
		s := strings.TrimSpace(part)
		if s == "" {
			continue
		}
		out = append(out, s)
	}
	return out
}

func isUnique(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505"
}

func (s *Store) CreateService(ctx context.Context, ms incident.MonitoredService) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO reliability_services (id, code, name, description, created_at)
		VALUES ($1,$2,$3,$4,$5)`,
		ms.ID, ms.Code, ms.Name, ms.Description, ms.CreatedAt)
	if isUnique(err) {
		return incident.ErrConflict
	}
	return err
}

func (s *Store) GetService(ctx context.Context, id string) (incident.MonitoredService, error) {
	return scanService(s.pool.QueryRow(ctx, `SELECT id, code, name, description, created_at FROM reliability_services WHERE id=$1`, id))
}

func (s *Store) GetServiceByCode(ctx context.Context, code string) (incident.MonitoredService, error) {
	return scanService(s.pool.QueryRow(ctx, `SELECT id, code, name, description, created_at FROM reliability_services WHERE code=$1`, code))
}

func (s *Store) ListServices(ctx context.Context) ([]incident.MonitoredService, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, code, name, description, created_at FROM reliability_services ORDER BY code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []incident.MonitoredService
	for rows.Next() {
		ms, err := scanService(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, ms)
	}
	return out, rows.Err()
}

func scanService(row scanner) (incident.MonitoredService, error) {
	var ms incident.MonitoredService
	err := row.Scan(&ms.ID, &ms.Code, &ms.Name, &ms.Description, &ms.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return incident.MonitoredService{}, incident.ErrServiceNotFound
	}
	return ms, err
}

func (s *Store) CreateIntegration(ctx context.Context, in incident.Integration) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO reliability_integrations (id, service_id, public_key, secret, created_at)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (public_key) DO UPDATE SET service_id=EXCLUDED.service_id, secret=EXCLUDED.secret`,
		in.ID, in.ServiceID, in.PublicKey, in.Secret, in.CreatedAt)
	if isUnique(err) {
		return incident.ErrConflict
	}
	return err
}

func (s *Store) GetIntegrationByPublicKey(ctx context.Context, key string) (incident.Integration, error) {
	var in incident.Integration
	err := s.pool.QueryRow(ctx, `SELECT id, service_id, public_key, secret, created_at FROM reliability_integrations WHERE public_key=$1`, key).
		Scan(&in.ID, &in.ServiceID, &in.PublicKey, &in.Secret, &in.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return incident.Integration{}, incident.ErrIntegrationMissing
	}
	return in, err
}

func (s *Store) ListIntegrationsByService(ctx context.Context, serviceID string) ([]incident.Integration, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, service_id, public_key, secret, created_at
		FROM reliability_integrations WHERE service_id=$1 ORDER BY public_key`, serviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []incident.Integration
	for rows.Next() {
		var in incident.Integration
		if err := rows.Scan(&in.ID, &in.ServiceID, &in.PublicKey, &in.Secret, &in.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, in)
	}
	return out, rows.Err()
}

func (s *Store) CreateIncident(ctx context.Context, inc incident.Incident) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO reliability_incidents
		(id, service_id, dedup_key, severity, status, summary, assignee_sub, alert_count, created_at, updated_at, resolved_at, acknowledged_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		inc.ID, inc.ServiceID, inc.DedupKey, inc.Severity, inc.Status, inc.Summary, inc.AssigneeSub, inc.AlertCount,
		inc.CreatedAt, inc.UpdatedAt, nullTime(inc.ResolvedAt), nullTime(inc.AcknowledgedAt))
	return err
}

func (s *Store) GetIncident(ctx context.Context, id string) (incident.Incident, error) {
	return scanIncident(s.pool.QueryRow(ctx, `SELECT id, service_id, dedup_key, severity, status, summary, assignee_sub, alert_count, created_at, updated_at, resolved_at, acknowledged_at
		FROM reliability_incidents WHERE id=$1`, id))
}

func (s *Store) ListIncidents(ctx context.Context) ([]incident.Incident, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, service_id, dedup_key, severity, status, summary, assignee_sub, alert_count, created_at, updated_at, resolved_at, acknowledged_at
		FROM reliability_incidents`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []incident.Incident
	for rows.Next() {
		inc, err := scanIncident(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, inc)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sortIncidents(out)
	return out, nil
}

func (s *Store) FindOpenByDedup(ctx context.Context, serviceID, dedupKey string) (incident.Incident, error) {
	return scanIncident(s.pool.QueryRow(ctx, `SELECT id, service_id, dedup_key, severity, status, summary, assignee_sub, alert_count, created_at, updated_at, resolved_at, acknowledged_at
		FROM reliability_incidents
		WHERE service_id=$1 AND dedup_key=$2 AND status <> 'resolved'
		ORDER BY created_at DESC LIMIT 1`, serviceID, dedupKey))
}

func (s *Store) UpdateIncident(ctx context.Context, inc incident.Incident) error {
	tag, err := s.pool.Exec(ctx, `UPDATE reliability_incidents SET
		severity=$2, status=$3, summary=$4, assignee_sub=$5, alert_count=$6, updated_at=$7, resolved_at=$8, acknowledged_at=$9
		WHERE id=$1`,
		inc.ID, inc.Severity, inc.Status, inc.Summary, inc.AssigneeSub, inc.AlertCount, inc.UpdatedAt,
		nullTime(inc.ResolvedAt), nullTime(inc.AcknowledgedAt))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return incident.ErrNotFound
	}
	return nil
}

func (s *Store) AppendEvent(ctx context.Context, ev incident.Event) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO reliability_events (id, incident_id, kind, actor, message, at)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		ev.ID, ev.IncidentID, ev.Kind, ev.Actor, ev.Message, ev.At)
	return err
}

func (s *Store) ListEvents(ctx context.Context, incidentID string) ([]incident.Event, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, incident_id, kind, actor, message, at
		FROM reliability_events WHERE incident_id=$1 ORDER BY at ASC`, incidentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []incident.Event
	for rows.Next() {
		var ev incident.Event
		if err := rows.Scan(&ev.ID, &ev.IncidentID, &ev.Kind, &ev.Actor, &ev.Message, &ev.At); err != nil {
			return nil, err
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}

func (s *Store) SeenEventID(ctx context.Context, integrationID, eventID string) (string, bool) {
	var incidentID string
	err := s.pool.QueryRow(ctx, `SELECT incident_id FROM reliability_webhook_event_ids WHERE integration_id=$1 AND event_id=$2`,
		integrationID, eventID).Scan(&incidentID)
	if err != nil {
		return "", false
	}
	return incidentID, true
}

func (s *Store) RememberEventID(ctx context.Context, integrationID, eventID, incidentID string) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO reliability_webhook_event_ids (integration_id, event_id, incident_id)
		VALUES ($1,$2,$3) ON CONFLICT (integration_id, event_id) DO NOTHING`,
		integrationID, eventID, incidentID)
	return err
}

func scanIncident(row scanner) (incident.Incident, error) {
	var inc incident.Incident
	var resolved, acked sql.NullTime
	err := row.Scan(&inc.ID, &inc.ServiceID, &inc.DedupKey, &inc.Severity, &inc.Status, &inc.Summary, &inc.AssigneeSub,
		&inc.AlertCount, &inc.CreatedAt, &inc.UpdatedAt, &resolved, &acked)
	if errors.Is(err, pgx.ErrNoRows) {
		return incident.Incident{}, incident.ErrNotFound
	}
	if err != nil {
		return incident.Incident{}, err
	}
	if resolved.Valid {
		inc.ResolvedAt = resolved.Time
	}
	if acked.Valid {
		inc.AcknowledgedAt = acked.Time
	}
	return inc, nil
}

func nullTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

func sortIncidents(out []incident.Incident) {
	sort.Slice(out, func(i, j int) bool {
		if ri, rj := incident.StatusRank(out[i].Status), incident.StatusRank(out[j].Status); ri != rj {
			return ri < rj
		}
		if si, sj := incident.SeverityRank(out[i].Severity), incident.SeverityRank(out[j].Severity); si != sj {
			return si < sj
		}
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
}
