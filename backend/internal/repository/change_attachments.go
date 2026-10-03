package repository

import (
	"context"
	"fmt"

	"github.com/khairulazzam0001/incident-management/backend/internal/model"
)

const changeAttachmentColumns = `id, change_id, file_name, mime_type, size_bytes, uploaded_by, created_at`

func scanChangeAttachment(row interface{ Scan(...any) error }) (*model.ChangeAttachment, error) {
	a := &model.ChangeAttachment{}
	if err := row.Scan(&a.ID, &a.ChangeID, &a.FileName, &a.MimeType, &a.SizeBytes, &a.UploadedBy, &a.CreatedAt); err != nil {
		return nil, err
	}
	return a, nil
}

// CreateChangeAttachment inserts the metadata row plus its timeline activity.
func (r *Repository) CreateChangeAttachment(ctx context.Context, changeID, fileName, storageKey, mime string, size int64, uploadedBy string) (*model.ChangeAttachment, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	a, err := scanChangeAttachment(tx.QueryRow(ctx, `
		INSERT INTO trans_change_attachment
			(change_id, file_name, storage_key, mime_type, size_bytes, uploaded_by)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING `+changeAttachmentColumns,
		changeID, fileName, storageKey, mime, size, uploadedBy))
	if err != nil {
		return nil, fmt.Errorf("insert change attachment: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO trans_change_activity (change_id, type, actor_id, payload)
		VALUES ($1,'attachment',$2,jsonb_build_object('attachment_id',$3::text,'file_name',$4::text))`,
		changeID, uploadedBy, a.ID, a.FileName); err != nil {
		return nil, fmt.Errorf("insert attachment activity: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return a, nil
}

// ListChangeAttachments returns attachments oldest-first.
func (r *Repository) ListChangeAttachments(ctx context.Context, changeID string) ([]*model.ChangeAttachment, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+changeAttachmentColumns+`
		FROM trans_change_attachment WHERE change_id = $1 ORDER BY created_at ASC`, changeID)
	if err != nil {
		return nil, fmt.Errorf("list change attachments: %w", err)
	}
	defer rows.Close()
	items := []*model.ChangeAttachment{}
	for rows.Next() {
		a, err := scanChangeAttachment(rows)
		if err != nil {
			return nil, fmt.Errorf("scan change attachment: %w", err)
		}
		items = append(items, a)
	}
	return items, rows.Err()
}

// GetChangeAttachment returns one attachment scoped to its change plus the
// internal storage key (never serialized).
func (r *Repository) GetChangeAttachment(ctx context.Context, changeID, id string) (*model.ChangeAttachment, string, error) {
	a := &model.ChangeAttachment{}
	var key string
	err := r.pool.QueryRow(ctx, `SELECT `+changeAttachmentColumns+`, storage_key
		FROM trans_change_attachment WHERE id = $1 AND change_id = $2`, id, changeID).
		Scan(&a.ID, &a.ChangeID, &a.FileName, &a.MimeType, &a.SizeBytes, &a.UploadedBy, &a.CreatedAt, &key)
	if err != nil {
		return nil, "", err
	}
	return a, key, nil
}
