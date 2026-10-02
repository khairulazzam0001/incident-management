-- +goose Up
-- Change Management CM-FR-13: attachment pada change request (aturan file
-- sama dengan trans_incident_attachment, FR-09).

CREATE TABLE trans_change_attachment (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  change_id   UUID NOT NULL REFERENCES trans_change (id) ON DELETE CASCADE,
  file_name   TEXT NOT NULL,
  storage_key TEXT NOT NULL,
  mime_type   TEXT NOT NULL DEFAULT 'application/octet-stream',
  size_bytes  BIGINT NOT NULL DEFAULT 0,
  uploaded_by UUID REFERENCES master_user (id),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_trans_change_attachment_change ON trans_change_attachment (change_id, created_at);

-- +goose Down
DROP TABLE IF EXISTS trans_change_attachment;
