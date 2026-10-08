-- +goose Up
-- Monitoring Integration (PRD_Monitoring_Integration.md §9): integrasi
-- webhook (token per integrasi), event alert mentah, link incident ↔ alert.

CREATE TABLE master_integration (
  id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  type                     TEXT NOT NULL CHECK (type IN ('ALERTMANAGER', 'GRAFANA', 'GENERIC_JSON')),
  name                     TEXT NOT NULL CHECK (char_length(name) >= 3),
  -- SHA-256 hex dari token; token asli hanya ditampilkan sekali.
  token_hash               TEXT NOT NULL,
  token_last4              TEXT NOT NULL,
  is_active                BOOLEAN NOT NULL DEFAULT TRUE,
  default_application_id   UUID REFERENCES master_application (id),
  default_environment_code TEXT REFERENCES master_environment (code),
  -- Alert dengan severity di bawah ambang ini hanya dicatat (Q2: info → event).
  min_severity             TEXT NOT NULL DEFAULT 'S3' REFERENCES master_incident_severity (code),
  mapping                  JSONB NOT NULL DEFAULT '{}',
  ip_allowlist             TEXT[] NOT NULL DEFAULT '{}',
  rate_limit_per_min       INTEGER NOT NULL DEFAULT 60 CHECK (rate_limit_per_min BETWEEN 1 AND 6000),
  created_by               UUID REFERENCES master_user (id),
  created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
  rotated_at               TIMESTAMPTZ,
  last_event_at            TIMESTAMPTZ
);

CREATE TABLE trans_alert_event (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  integration_id UUID NOT NULL REFERENCES master_integration (id) ON DELETE CASCADE,
  received_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  payload_hash   TEXT NOT NULL,
  payload        JSONB NOT NULL DEFAULT '{}',
  fingerprint    TEXT NOT NULL DEFAULT '',
  status         TEXT NOT NULL DEFAULT 'firing' CHECK (status IN ('firing', 'resolved', 'unknown')),
  result         TEXT NOT NULL
    CHECK (result IN ('created', 'updated', 'grouped', 'ignored', 'duplicate', 'error')),
  warning        TEXT NOT NULL DEFAULT '',
  error          TEXT NOT NULL DEFAULT '',
  incident_id    UUID REFERENCES trans_incident (id) ON DELETE SET NULL
);
CREATE INDEX idx_trans_alert_event_integration ON trans_alert_event (integration_id, received_at DESC);
CREATE INDEX idx_trans_alert_event_hash ON trans_alert_event (integration_id, payload_hash, received_at DESC);

CREATE TABLE trans_incident_alert (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  incident_id      UUID NOT NULL REFERENCES trans_incident (id) ON DELETE CASCADE,
  integration_id   UUID NOT NULL REFERENCES master_integration (id) ON DELETE CASCADE,
  fingerprint      TEXT NOT NULL,
  title            TEXT NOT NULL DEFAULT '',
  labels           JSONB NOT NULL DEFAULT '{}',
  source_url       TEXT NOT NULL DEFAULT '',
  first_seen       TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_seen        TIMESTAMPTZ NOT NULL DEFAULT now(),
  fire_count       INTEGER NOT NULL DEFAULT 1,
  last_status      TEXT NOT NULL DEFAULT 'firing' CHECK (last_status IN ('firing', 'resolved')),
  last_activity_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (integration_id, fingerprint, incident_id)
);
CREATE INDEX idx_trans_incident_alert_fp ON trans_incident_alert (integration_id, fingerprint, first_seen DESC);
CREATE INDEX idx_trans_incident_alert_incident ON trans_incident_alert (incident_id);

-- +goose Down
DROP TABLE IF EXISTS trans_incident_alert;
DROP TABLE IF EXISTS trans_alert_event;
DROP TABLE IF EXISTS master_integration;
