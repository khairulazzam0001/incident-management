-- +goose Up
-- SLA & Escalation (PRD_SLA_Escalation.md §9): kebijakan per priority,
-- kalender jam kerja + hari libur, instance SLA per incident, audit eskalasi.

CREATE TABLE master_business_calendar (
  code     TEXT PRIMARY KEY,
  name     TEXT NOT NULL,
  timezone TEXT NOT NULL DEFAULT 'Asia/Jakarta',
  is_24x7  BOOLEAN NOT NULL DEFAULT FALSE
);

-- weekday mengikuti Go time.Weekday (0 = Minggu). Menit sejak tengah malam.
CREATE TABLE master_business_hours (
  calendar_code TEXT NOT NULL REFERENCES master_business_calendar (code) ON DELETE CASCADE,
  weekday       SMALLINT NOT NULL CHECK (weekday BETWEEN 0 AND 6),
  start_minute  SMALLINT NOT NULL CHECK (start_minute BETWEEN 0 AND 1440),
  end_minute    SMALLINT NOT NULL CHECK (end_minute BETWEEN 0 AND 1440),
  PRIMARY KEY (calendar_code, weekday),
  CHECK (end_minute > start_minute)
);

CREATE TABLE master_holiday (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  calendar_code TEXT NOT NULL REFERENCES master_business_calendar (code) ON DELETE CASCADE,
  date          DATE NOT NULL,
  name          TEXT NOT NULL,
  created_by    UUID REFERENCES master_user (id),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (calendar_code, date)
);

CREATE TABLE master_sla_policy (
  priority_code           TEXT PRIMARY KEY REFERENCES master_incident_priority (code),
  response_minutes        INTEGER NOT NULL CHECK (response_minutes > 0),
  resolution_minutes      INTEGER NOT NULL CHECK (resolution_minutes > 0),
  calendar_code           TEXT NOT NULL REFERENCES master_business_calendar (code),
  warn_percent            SMALLINT NOT NULL DEFAULT 75 CHECK (warn_percent BETWEEN 1 AND 99),
  breach_reminder_minutes INTEGER CHECK (breach_reminder_minutes IS NULL OR breach_reminder_minutes > 0),
  updated_by              UUID REFERENCES master_user (id),
  updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE trans_incident_sla (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  incident_id      UUID NOT NULL REFERENCES trans_incident (id) ON DELETE CASCADE,
  metric           TEXT NOT NULL CHECK (metric IN ('RESPONSE', 'RESOLUTION')),
  cycle            SMALLINT NOT NULL DEFAULT 1,
  status           TEXT NOT NULL DEFAULT 'RUNNING'
    CHECK (status IN ('RUNNING', 'MET', 'BREACHED', 'CANCELLED')),
  -- Snapshot kebijakan saat instance dibuat (perubahan policy tidak mengubah histori).
  priority_code    TEXT NOT NULL REFERENCES master_incident_priority (code),
  target_minutes   INTEGER NOT NULL,
  calendar_code    TEXT NOT NULL REFERENCES master_business_calendar (code),
  warn_percent     SMALLINT NOT NULL,
  reminder_minutes INTEGER,
  started_at       TIMESTAMPTZ NOT NULL,
  warn_at          TIMESTAMPTZ NOT NULL,
  target_at        TIMESTAMPTZ NOT NULL,
  next_check_at    TIMESTAMPTZ,
  warned_at        TIMESTAMPTZ,
  breached_at      TIMESTAMPTZ,
  reminded_at      TIMESTAMPTZ,
  stopped_at       TIMESTAMPTZ,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (incident_id, metric, cycle)
);
CREATE INDEX idx_trans_incident_sla_due ON trans_incident_sla (next_check_at)
  WHERE next_check_at IS NOT NULL;
CREATE INDEX idx_trans_incident_sla_incident ON trans_incident_sla (incident_id);

CREATE TABLE trans_incident_escalation (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  incident_id UUID NOT NULL REFERENCES trans_incident (id) ON DELETE CASCADE,
  sla_id      UUID NOT NULL REFERENCES trans_incident_sla (id) ON DELETE CASCADE,
  level       TEXT NOT NULL CHECK (level IN ('WARNING', 'BREACH', 'REMINDER')),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (sla_id, level)
);
CREATE INDEX idx_trans_incident_escalation_incident ON trans_incident_escalation (incident_id);

INSERT INTO master_business_calendar (code, name, timezone, is_24x7) VALUES
  ('24x7', '24x7', 'Asia/Jakarta', TRUE),
  ('BUSINESS_HOURS', 'Jam kerja (Sen–Jum 08:00–17:00)', 'Asia/Jakarta', FALSE);

INSERT INTO master_business_hours (calendar_code, weekday, start_minute, end_minute)
SELECT 'BUSINESS_HOURS', d, 480, 1020 FROM generate_series(1, 5) AS d;

-- Default Q1: 1 hari kerja = 540 menit kerja.
INSERT INTO master_sla_policy
  (priority_code, response_minutes, resolution_minutes, calendar_code, warn_percent, breach_reminder_minutes) VALUES
  ('P1', 15, 240, '24x7', 75, 60),
  ('P2', 60, 480, '24x7', 75, 240),
  ('P3', 240, 1620, 'BUSINESS_HOURS', 75, NULL),
  ('P4', 540, 2700, 'BUSINESS_HOURS', 75, NULL);

-- +goose Down
DROP TABLE IF EXISTS trans_incident_escalation;
DROP TABLE IF EXISTS trans_incident_sla;
DROP TABLE IF EXISTS master_sla_policy;
DROP TABLE IF EXISTS master_holiday;
DROP TABLE IF EXISTS master_business_hours;
DROP TABLE IF EXISTS master_business_calendar;
