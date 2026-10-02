-- +goose Up
-- Change Management CM-1 (PRD_Change_Management.md §6, §7, §12):
-- master type/risk/status, trans_change + approval/activity/comment, dan
-- link change ↔ incident.

CREATE TABLE master_change_type (
  code       TEXT PRIMARY KEY,
  name       TEXT NOT NULL,
  sort_order SMALLINT NOT NULL
);

CREATE TABLE master_change_risk (
  code       TEXT PRIMARY KEY,
  name       TEXT NOT NULL,
  sort_order SMALLINT NOT NULL
);

CREATE TABLE master_change_status (
  code        TEXT PRIMARY KEY,
  name        TEXT NOT NULL,
  sort_order  SMALLINT NOT NULL,
  is_terminal BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE SEQUENCE change_no_seq START WITH 1;

CREATE TABLE trans_change (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  change_no           TEXT NOT NULL UNIQUE,
  title               TEXT NOT NULL CHECK (char_length(title) >= 5),
  description         TEXT NOT NULL DEFAULT '',
  justification       TEXT NOT NULL DEFAULT '',
  type_code           TEXT NOT NULL REFERENCES master_change_type (code),
  risk_code           TEXT NOT NULL REFERENCES master_change_risk (code),
  status_code         TEXT NOT NULL DEFAULT 'DRAFT' REFERENCES master_change_status (code),
  application_id      UUID NOT NULL REFERENCES master_application (id),
  environment_code    TEXT NOT NULL REFERENCES master_environment (code),
  implementation_plan TEXT NOT NULL DEFAULT '',
  rollback_plan       TEXT NOT NULL DEFAULT '',
  test_plan           TEXT NOT NULL DEFAULT '',
  requester_id        UUID NOT NULL REFERENCES master_user (id),
  implementer_id      UUID REFERENCES master_user (id),
  team_id             UUID REFERENCES master_team (id),
  revision            SMALLINT NOT NULL DEFAULT 1,
  planned_start       TIMESTAMPTZ,
  planned_end         TIMESTAMPTZ,
  actual_start        TIMESTAMPTZ,
  actual_end          TIMESTAMPTZ,
  outcome             TEXT CHECK (outcome IN ('SUCCESS', 'FAILED', 'ROLLED_BACK')),
  outcome_notes       TEXT NOT NULL DEFAULT '',
  review_notes        TEXT NOT NULL DEFAULT '',
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  closed_at           TIMESTAMPTZ,
  closed_by           UUID REFERENCES master_user (id),
  CHECK (planned_end IS NULL OR planned_start IS NULL OR planned_end > planned_start)
);
CREATE INDEX idx_trans_change_status ON trans_change (status_code);
CREATE INDEX idx_trans_change_type ON trans_change (type_code);
CREATE INDEX idx_trans_change_risk ON trans_change (risk_code);
CREATE INDEX idx_trans_change_window ON trans_change (application_id, environment_code, planned_start);
CREATE INDEX idx_trans_change_requester ON trans_change (requester_id);
CREATE INDEX idx_trans_change_implementer ON trans_change (implementer_id);
CREATE INDEX idx_trans_change_created ON trans_change (created_at DESC);

CREATE TABLE trans_change_approval (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  change_id   UUID NOT NULL REFERENCES trans_change (id) ON DELETE CASCADE,
  approver_id UUID REFERENCES master_user (id),
  decision    TEXT NOT NULL CHECK (decision IN ('APPROVED', 'REJECTED', 'CHANGES_REQUESTED', 'AUTO_APPROVED')),
  reason      TEXT NOT NULL DEFAULT '',
  revision    SMALLINT NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_trans_change_approval_change ON trans_change_approval (change_id, created_at);

CREATE TABLE trans_change_activity (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  change_id   UUID NOT NULL REFERENCES trans_change (id) ON DELETE CASCADE,
  type        TEXT NOT NULL,
  actor_id    UUID REFERENCES master_user (id),
  from_status TEXT REFERENCES master_change_status (code),
  to_status   TEXT REFERENCES master_change_status (code),
  payload     JSONB NOT NULL DEFAULT '{}',
  -- clock_timestamp: beberapa activity dalam satu tx (STANDARD auto-approve,
  -- create + link) tetap berurutan di timeline.
  created_at  TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX idx_trans_change_activity_change ON trans_change_activity (change_id, created_at);

CREATE TABLE trans_change_comment (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  change_id  UUID NOT NULL REFERENCES trans_change (id) ON DELETE CASCADE,
  author_id  UUID REFERENCES master_user (id),
  body       TEXT NOT NULL CHECK (char_length(body) >= 1),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_trans_change_comment_change ON trans_change_comment (change_id, created_at);

CREATE TABLE trans_change_incident (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  change_id   UUID NOT NULL REFERENCES trans_change (id) ON DELETE CASCADE,
  incident_id UUID NOT NULL REFERENCES trans_incident (id) ON DELETE CASCADE,
  relation    TEXT NOT NULL CHECK (relation IN ('FIX_FOR', 'CAUSED_BY')),
  created_by  UUID REFERENCES master_user (id),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (change_id, incident_id, relation)
);
CREATE INDEX idx_trans_change_incident_incident ON trans_change_incident (incident_id);

INSERT INTO master_change_type (code, name, sort_order) VALUES
  ('STANDARD', 'Standard', 1),
  ('NORMAL', 'Normal', 2),
  ('EMERGENCY', 'Emergency', 3);

INSERT INTO master_change_risk (code, name, sort_order) VALUES
  ('LOW', 'Low', 1),
  ('MEDIUM', 'Medium', 2),
  ('HIGH', 'High', 3);

INSERT INTO master_change_status (code, name, sort_order, is_terminal) VALUES
  ('DRAFT', 'Draft', 1, FALSE),
  ('SUBMITTED', 'Submitted', 2, FALSE),
  ('APPROVED', 'Approved', 3, FALSE),
  ('SCHEDULED', 'Scheduled', 4, FALSE),
  ('IMPLEMENTING', 'Implementing', 5, FALSE),
  ('REVIEWING', 'Reviewing', 6, FALSE),
  ('CLOSED', 'Closed', 7, TRUE),
  ('REJECTED', 'Rejected', 8, TRUE),
  ('CANCELLED', 'Cancelled', 9, TRUE);

-- +goose Down
DROP TABLE IF EXISTS trans_change_incident;
DROP TABLE IF EXISTS trans_change_comment;
DROP TABLE IF EXISTS trans_change_activity;
DROP TABLE IF EXISTS trans_change_approval;
DROP TABLE IF EXISTS trans_change;
DROP SEQUENCE IF EXISTS change_no_seq;
DROP TABLE IF EXISTS master_change_status;
DROP TABLE IF EXISTS master_change_risk;
DROP TABLE IF EXISTS master_change_type;
