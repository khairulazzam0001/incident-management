-- +goose Up
-- Fase 3: notifikasi in-app + email (PRD §13, FR-11) dan index untuk
-- filter lanjutan + dashboard.

CREATE TABLE trans_notification (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  incident_id   UUID NOT NULL REFERENCES trans_incident (id) ON DELETE CASCADE,
  type          TEXT NOT NULL,
  recipient_id  UUID NOT NULL REFERENCES master_user (id) ON DELETE CASCADE,
  actor_id      UUID REFERENCES master_user (id),
  read_at       TIMESTAMPTZ,
  email_status  TEXT NOT NULL DEFAULT 'pending'
    CHECK (email_status IN ('pending', 'sent', 'failed', 'skipped')),
  email_error   TEXT NOT NULL DEFAULT '',
  payload       JSONB NOT NULL DEFAULT '{}',
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_trans_notification_recipient ON trans_notification (recipient_id, created_at DESC);
CREATE INDEX idx_trans_notification_incident ON trans_notification (incident_id);

-- Index filter lanjutan (FR-02) dan agregat dashboard (PRD §14).
CREATE INDEX idx_trans_incident_team ON trans_incident (current_team_id);
CREATE INDEX idx_trans_incident_created ON trans_incident (created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_trans_incident_created;
DROP INDEX IF EXISTS idx_trans_incident_team;
DROP TABLE IF EXISTS trans_notification;
