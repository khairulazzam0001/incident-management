package repository

import (
	"context"
	"fmt"

	"github.com/khairulazzam0001/incident-management/backend/internal/model"
)

// NotificationInput is one recipient row for an event.
type NotificationInput struct {
	IncidentID  string
	Type        string
	RecipientID string
	ActorID     *string
	EmailStatus string
	Payload     string
}

// CreateNotifications inserts one row per recipient (in-app unread by default).
func (r *Repository) CreateNotifications(ctx context.Context, rows []NotificationInput) ([]string, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	ids := make([]string, 0, len(rows))
	for _, n := range rows {
		var id string
		err := tx.QueryRow(ctx, `
			INSERT INTO trans_notification (incident_id, type, recipient_id, actor_id, email_status, payload)
			VALUES ($1,$2,$3,$4,$5,$6::jsonb) RETURNING id`,
			n.IncidentID, n.Type, n.RecipientID, n.ActorID, n.EmailStatus, n.Payload).Scan(&id)
		if err != nil {
			return nil, fmt.Errorf("insert notification: %w", err)
		}
		ids = append(ids, id)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return ids, nil
}

// ListNotifications returns the recipient's items newest-first.
func (r *Repository) ListNotifications(ctx context.Context, recipientID string, unreadOnly bool, limit int) ([]*model.Notification, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	query := `
		SELECT id, incident_id, type, recipient_id, actor_id,
			TO_CHAR(read_at, 'YYYY-MM-DD"T"HH24:MI:SSOF"TZ"'), email_status,
			TO_CHAR(created_at, 'YYYY-MM-DD"T"HH24:MI:SSOF"TZ"')
		FROM trans_notification WHERE recipient_id = $1`
	args := []any{recipientID}
	if unreadOnly {
		query += ` AND read_at IS NULL`
	}
	query += ` ORDER BY created_at DESC LIMIT $2`
	args = append(args, limit)
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	defer rows.Close()
	items := []*model.Notification{}
	for rows.Next() {
		n := &model.Notification{}
		if err := rows.Scan(&n.ID, &n.IncidentID, &n.Type, &n.RecipientID, &n.ActorID,
			&n.ReadAt, &n.EmailStatus, &n.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan notification: %w", err)
		}
		items = append(items, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate notifications: %w", err)
	}
	return items, nil
}

// UnreadCount counts unread in-app notifications.
func (r *Repository) UnreadCount(ctx context.Context, recipientID string) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM trans_notification WHERE recipient_id = $1 AND read_at IS NULL`,
		recipientID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("unread count: %w", err)
	}
	return n, nil
}

// MarkNotificationRead marks one item read if owned by the recipient.
func (r *Repository) MarkNotificationRead(ctx context.Context, id, recipientID string) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE trans_notification SET read_at = now()
		WHERE id = $1 AND recipient_id = $2 AND read_at IS NULL`, id, recipientID)
	if err != nil {
		return false, fmt.Errorf("mark read: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// UpdateEmailStatus records email delivery outcome.
func (r *Repository) UpdateEmailStatus(ctx context.Context, id, status, errMsg string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	_, err := r.pool.Exec(ctx, `
		UPDATE trans_notification SET email_status = $2, email_error = $3 WHERE id = $1`,
		id, status, errMsg)
	if err != nil {
		return fmt.Errorf("update email status: %w", err)
	}
	return nil
}

// CoordinatorIDs returns active coordinator user ids (HelpDesk, SystemAnalyst, ManagerLead).
func (r *Repository) CoordinatorIDs(ctx context.Context) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id FROM master_user
		WHERE is_active = TRUE AND role IN ('HelpDesk','SystemAnalyst','ManagerLead')`)
	if err != nil {
		return nil, fmt.Errorf("coordinator ids: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan coordinator: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate coordinators: %w", err)
	}
	return ids, nil
}

// EmailsByIDs returns id → email for the given users.
func (r *Repository) EmailsByIDs(ctx context.Context, ids []string) (map[string]string, error) {
	out := map[string]string{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, `SELECT id, email FROM master_user WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, fmt.Errorf("emails by ids: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, email string
		if err := rows.Scan(&id, &email); err != nil {
			return nil, fmt.Errorf("scan email: %w", err)
		}
		out[id] = email
	}
	return out, rows.Err()
}
