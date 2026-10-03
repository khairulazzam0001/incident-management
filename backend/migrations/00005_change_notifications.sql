-- +goose Up
-- Change Management CM-2 (PRD_Change_Management.md §13, Q7): perluas
-- trans_notification agar bisa merujuk incident ATAU change.

ALTER TABLE trans_notification ALTER COLUMN incident_id DROP NOT NULL;
ALTER TABLE trans_notification
  ADD COLUMN change_id UUID REFERENCES trans_change (id) ON DELETE CASCADE;
ALTER TABLE trans_notification
  ADD CONSTRAINT chk_trans_notification_subject
  CHECK (incident_id IS NOT NULL OR change_id IS NOT NULL);
CREATE INDEX idx_trans_notification_change ON trans_notification (change_id);

-- +goose Down
DELETE FROM trans_notification WHERE incident_id IS NULL;
DROP INDEX IF EXISTS idx_trans_notification_change;
ALTER TABLE trans_notification DROP CONSTRAINT IF EXISTS chk_trans_notification_subject;
ALTER TABLE trans_notification DROP COLUMN IF EXISTS change_id;
ALTER TABLE trans_notification ALTER COLUMN incident_id SET NOT NULL;
