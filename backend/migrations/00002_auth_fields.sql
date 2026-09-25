-- +goose Up
-- Auth Fase 1: sequence nomor incident (race-safe via nextval),
-- kolom password_hash, dan seed user DEV ONLY.

CREATE SEQUENCE incident_no_seq START WITH 1;

ALTER TABLE master_user ADD COLUMN password_hash TEXT NOT NULL DEFAULT '';

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Seed user DEV ONLY. Password: Password123!
-- GANTI/HAPUS di environment bersama. Login asli menyusul hardening SSO (PRD §15).
INSERT INTO master_user (email, name, role, password_hash) VALUES
  ('helpdesk@example.com',  'Help Desk',      'HelpDesk',      crypt('Password123!', gen_salt('bf'))),
  ('analyst@example.com',   'System Analyst', 'SystemAnalyst', crypt('Password123!', gen_salt('bf'))),
  ('developer@example.com', 'Developer',      'Developer',     crypt('Password123!', gen_salt('bf'))),
  ('qa@example.com',        'QA',             'QA',            crypt('Password123!', gen_salt('bf'))),
  ('manager@example.com',   'Manager',        'ManagerLead',   crypt('Password123!', gen_salt('bf'))),
  ('user@example.com',      'Customer',       'User',          crypt('Password123!', gen_salt('bf')))
ON CONFLICT (email) DO NOTHING;

-- +goose Down
DELETE FROM master_user WHERE email IN (
  'helpdesk@example.com', 'analyst@example.com', 'developer@example.com',
  'qa@example.com', 'manager@example.com', 'user@example.com'
);
ALTER TABLE master_user DROP COLUMN password_hash;
DROP SEQUENCE IF EXISTS incident_no_seq;
