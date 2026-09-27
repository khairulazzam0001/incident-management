package repository

import (
	"context"
	"fmt"

	"github.com/khairulazzam0001/incident-management/backend/internal/model"
)

// CreateAttachment inserts the attachment metadata row.
func (r *Repository) CreateAttachment(ctx context.Context, incidentID, fileName, storageKey, mime string, size int64, uploadedBy string) (*model.Attachment, error) {
	a := &model.Attachment{}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO trans_incident_attachment
			(incident_id, file_name, storage_key, mime_type, size_bytes, uploaded_by)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, incident_id, file_name, mime_type, size_bytes, uploaded_by, created_at`,
		incidentID, fileName, storageKey, mime, size, uploadedBy).
		Scan(&a.ID, &a.IncidentID, &a.FileName, &a.MimeType, &a.SizeBytes, &a.UploadedBy, &a.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert attachment: %w", err)
	}
	return a, nil
}

// ListAttachments returns attachments oldest-first.
func (r *Repository) ListAttachments(ctx context.Context, incidentID string) ([]*model.Attachment, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, incident_id, file_name, mime_type, size_bytes, uploaded_by, created_at
		FROM trans_incident_attachment WHERE incident_id = $1 ORDER BY created_at ASC`, incidentID)
	if err != nil {
		return nil, fmt.Errorf("list attachments: %w", err)
	}
	defer rows.Close()
	items := []*model.Attachment{}
	for rows.Next() {
		a := &model.Attachment{}
		if err := rows.Scan(&a.ID, &a.IncidentID, &a.FileName, &a.MimeType, &a.SizeBytes, &a.UploadedBy, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan attachment: %w", err)
		}
		items = append(items, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate attachments: %w", err)
	}
	return items, nil
}

// GetAttachment fetches one attachment scoped to its incident.
func (r *Repository) GetAttachment(ctx context.Context, incidentID, id string) (*model.Attachment, error) {
	a := &model.Attachment{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, incident_id, file_name, mime_type, size_bytes, uploaded_by, created_at
		FROM trans_incident_attachment WHERE id = $1 AND incident_id = $2`, id, incidentID).
		Scan(&a.ID, &a.IncidentID, &a.FileName, &a.MimeType, &a.SizeBytes, &a.UploadedBy, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	// storage_key is intentionally not exposed; download resolves it server-side.
	return a, nil
}

// StorageKey returns the internal storage key for a verified attachment id.
func (r *Repository) StorageKey(ctx context.Context, incidentID, id string) (string, error) {
	var key string
	err := r.pool.QueryRow(ctx, `
		SELECT storage_key FROM trans_incident_attachment WHERE id = $1 AND incident_id = $2`,
		id, incidentID).Scan(&key)
	if err != nil {
		return "", err
	}
	return key, nil
}
