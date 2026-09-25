-- +goose Up
-- Skema awal (PRD §11): tabel master_* untuk reference, trans_* untuk transaksi/event.
-- Master memakai code TEXT sebagai PK agar seed & FK terbaca (NEW, S1, P1, ...).
-- Transaksi memakai UUID v4. Waktu memakai timestamptz (UTC).

-- ============ MASTER ============

CREATE TABLE master_incident_status (
  code        TEXT PRIMARY KEY,
  name        TEXT NOT NULL,
  sort_order  SMALLINT NOT NULL,
  is_terminal BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE master_incident_severity (
  code       TEXT PRIMARY KEY,
  name       TEXT NOT NULL,
  sort_order SMALLINT NOT NULL
);

CREATE TABLE master_incident_priority (
  code       TEXT PRIMARY KEY,
  name       TEXT NOT NULL,
  sort_order SMALLINT NOT NULL
);

CREATE TABLE master_environment (
  code TEXT PRIMARY KEY,
  name TEXT NOT NULL
);

CREATE TABLE master_incident_source (
  code TEXT PRIMARY KEY,
  name TEXT NOT NULL
);

CREATE TABLE master_team (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  code       TEXT NOT NULL UNIQUE,
  name       TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE master_application (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  code       TEXT NOT NULL UNIQUE,
  name       TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE master_user (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email      TEXT NOT NULL UNIQUE,
  name       TEXT NOT NULL,
  role       TEXT NOT NULL,
  team_id    UUID REFERENCES master_team (id),
  is_active  BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============ TRANSAKSI ============

CREATE TABLE trans_incident (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  incident_no     TEXT NOT NULL UNIQUE,
  title           TEXT NOT NULL CHECK (char_length(title) >= 5),
  description     TEXT NOT NULL DEFAULT '',
  source_code     TEXT NOT NULL REFERENCES master_incident_source (code),
  severity_code   TEXT NOT NULL REFERENCES master_incident_severity (code),
  priority_code   TEXT NOT NULL REFERENCES master_incident_priority (code),
  status_code     TEXT NOT NULL DEFAULT 'NEW' REFERENCES master_incident_status (code),
  application_id  UUID REFERENCES master_application (id),
  environment_code TEXT REFERENCES master_environment (code),
  reporter_id     UUID REFERENCES master_user (id),
  current_team_id UUID REFERENCES master_team (id),
  current_pic_id  UUID REFERENCES master_user (id),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  resolved_at     TIMESTAMPTZ,
  closed_at       TIMESTAMPTZ,
  closed_by       UUID REFERENCES master_user (id)
);
CREATE INDEX idx_trans_incident_status ON trans_incident (status_code);
CREATE INDEX idx_trans_incident_severity ON trans_incident (severity_code);
CREATE INDEX idx_trans_incident_priority ON trans_incident (priority_code);
CREATE INDEX idx_trans_incident_application ON trans_incident (application_id);
CREATE INDEX idx_trans_incident_pic ON trans_incident (current_pic_id);

CREATE TABLE trans_incident_assignment (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  incident_id UUID NOT NULL REFERENCES trans_incident (id) ON DELETE CASCADE,
  team_id     UUID REFERENCES master_team (id),
  pic_id      UUID REFERENCES master_user (id),
  assigned_by UUID REFERENCES master_user (id),
  note        TEXT NOT NULL DEFAULT '',
  assigned_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_trans_assignment_incident ON trans_incident_assignment (incident_id, assigned_at);

CREATE TABLE trans_incident_comment (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  incident_id UUID NOT NULL REFERENCES trans_incident (id) ON DELETE CASCADE,
  author_id   UUID REFERENCES master_user (id),
  body        TEXT NOT NULL CHECK (char_length(body) >= 1),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_trans_comment_incident ON trans_incident_comment (incident_id, created_at);

CREATE TABLE trans_incident_activity (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  incident_id UUID NOT NULL REFERENCES trans_incident (id) ON DELETE CASCADE,
  type        TEXT NOT NULL,
  actor_id    UUID REFERENCES master_user (id),
  from_status TEXT REFERENCES master_incident_status (code),
  to_status   TEXT REFERENCES master_incident_status (code),
  payload     JSONB NOT NULL DEFAULT '{}',
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_trans_activity_incident ON trans_incident_activity (incident_id, created_at);

CREATE TABLE trans_incident_investigation (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  incident_id UUID NOT NULL REFERENCES trans_incident (id) ON DELETE CASCADE,
  author_id   UUID REFERENCES master_user (id),
  notes       TEXT NOT NULL DEFAULT '',
  findings    TEXT NOT NULL DEFAULT '',
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_trans_investigation_incident ON trans_incident_investigation (incident_id, created_at);

CREATE TABLE trans_incident_fix (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  incident_id UUID NOT NULL REFERENCES trans_incident (id) ON DELETE CASCADE,
  author_id   UUID REFERENCES master_user (id),
  description TEXT NOT NULL DEFAULT '',
  reference   TEXT NOT NULL DEFAULT '',
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_trans_fix_incident ON trans_incident_fix (incident_id, created_at);

CREATE TABLE trans_incident_verification (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  incident_id UUID NOT NULL REFERENCES trans_incident (id) ON DELETE CASCADE,
  verifier_id UUID REFERENCES master_user (id),
  result      TEXT NOT NULL CHECK (result IN ('PASS', 'FAIL')),
  reason      TEXT NOT NULL DEFAULT '',
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_trans_verification_incident ON trans_incident_verification (incident_id, created_at);

CREATE TABLE trans_incident_attachment (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  incident_id UUID NOT NULL REFERENCES trans_incident (id) ON DELETE CASCADE,
  file_name   TEXT NOT NULL,
  storage_key TEXT NOT NULL,
  mime_type   TEXT NOT NULL DEFAULT 'application/octet-stream',
  size_bytes  BIGINT NOT NULL DEFAULT 0,
  uploaded_by UUID REFERENCES master_user (id),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_trans_attachment_incident ON trans_incident_attachment (incident_id, created_at);

-- ============ SEED MASTER ============

INSERT INTO master_incident_status (code, name, sort_order, is_terminal) VALUES
  ('NEW', 'New', 1, FALSE),
  ('ASSIGNED', 'Assigned', 2, FALSE),
  ('INVESTIGATING', 'Investigating', 3, FALSE),
  ('FIXING', 'Fixing', 4, FALSE),
  ('VERIFYING', 'Verifying', 5, FALSE),
  ('RESOLVED', 'Resolved', 6, FALSE),
  ('CLOSED', 'Closed', 7, TRUE);

INSERT INTO master_incident_severity (code, name, sort_order) VALUES
  ('S1', 'Critical', 1),
  ('S2', 'High', 2),
  ('S3', 'Medium', 3),
  ('S4', 'Low', 4);

INSERT INTO master_incident_priority (code, name, sort_order) VALUES
  ('P1', 'Critical', 1),
  ('P2', 'High', 2),
  ('P3', 'Normal', 3),
  ('P4', 'Low', 4);

INSERT INTO master_environment (code, name) VALUES
  ('production', 'Production'),
  ('staging', 'Staging'),
  ('development', 'Development');

INSERT INTO master_incident_source (code, name) VALUES
  ('user', 'User / Customer'),
  ('helpdesk', 'Help Desk'),
  ('email', 'Email'),
  ('whatsapp', 'WhatsApp'),
  ('monitoring', 'System / Monitoring'),
  ('other', 'Other');

-- +goose Down
DROP TABLE IF EXISTS trans_incident_attachment;
DROP TABLE IF EXISTS trans_incident_verification;
DROP TABLE IF EXISTS trans_incident_fix;
DROP TABLE IF EXISTS trans_incident_investigation;
DROP TABLE IF EXISTS trans_incident_activity;
DROP TABLE IF EXISTS trans_incident_comment;
DROP TABLE IF EXISTS trans_incident_assignment;
DROP TABLE IF EXISTS trans_incident;
DROP TABLE IF EXISTS master_user;
DROP TABLE IF EXISTS master_application;
DROP TABLE IF EXISTS master_team;
DROP TABLE IF EXISTS master_incident_source;
DROP TABLE IF EXISTS master_environment;
DROP TABLE IF EXISTS master_incident_priority;
DROP TABLE IF EXISTS master_incident_severity;
DROP TABLE IF EXISTS master_incident_status;
