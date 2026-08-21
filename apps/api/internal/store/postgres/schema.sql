-- Reliability Postgres. Incident rows are the current state; the timeline table is reliability_events.

CREATE TABLE IF NOT EXISTS reliability_services (
  id TEXT PRIMARY KEY,
  code TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS reliability_integrations (
  id TEXT PRIMARY KEY,
  service_id TEXT NOT NULL REFERENCES reliability_services (id),
  public_key TEXT NOT NULL UNIQUE,
  secret TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS reliability_incidents (
  id TEXT PRIMARY KEY,
  service_id TEXT NOT NULL REFERENCES reliability_services (id),
  dedup_key TEXT NOT NULL,
  severity TEXT NOT NULL,
  status TEXT NOT NULL,
  summary TEXT NOT NULL,
  assignee_sub TEXT NOT NULL DEFAULT '',
  alert_count INTEGER NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  resolved_at TIMESTAMPTZ,
  acknowledged_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS reliability_incidents_open_dedup
  ON reliability_incidents (service_id, dedup_key)
  WHERE status <> 'resolved';

CREATE TABLE IF NOT EXISTS reliability_events (
  id TEXT PRIMARY KEY,
  incident_id TEXT NOT NULL REFERENCES reliability_incidents (id),
  kind TEXT NOT NULL,
  actor TEXT NOT NULL DEFAULT '',
  message TEXT NOT NULL,
  at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS reliability_events_incident_at
  ON reliability_events (incident_id, at);

CREATE TABLE IF NOT EXISTS reliability_webhook_event_ids (
  integration_id TEXT NOT NULL,
  event_id TEXT NOT NULL,
  incident_id TEXT NOT NULL,
  PRIMARY KEY (integration_id, event_id)
);
