package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/khairulazzam0001/incident-management/backend/internal/model"
)

// Monitoring Integration queries (PRD_Monitoring_Integration.md §9).

const integrationColumns = `i.id, i.type, i.name, i.token_last4, i.is_active, i.default_application_id,
	i.default_environment_code, i.min_severity, i.mapping, i.ip_allowlist, i.rate_limit_per_min,
	i.created_by, i.created_at, i.rotated_at, i.last_event_at`

func scanIntegration(row pgx.Row, extra ...any) (*model.Integration, error) {
	it := &model.Integration{}
	var mapping []byte
	dest := []any{&it.ID, &it.Type, &it.Name, &it.TokenLast4, &it.IsActive, &it.DefaultApplicationID,
		&it.DefaultEnvironmentCode, &it.MinSeverity, &mapping, &it.IPAllowlist, &it.RateLimitPerMin,
		&it.CreatedBy, &it.CreatedAt, &it.RotatedAt, &it.LastEventAt}
	if err := row.Scan(append(dest, extra...)...); err != nil {
		return nil, err
	}
	it.Mapping = json.RawMessage(mapping)
	if it.IPAllowlist == nil {
		it.IPAllowlist = []string{}
	}
	return it, nil
}

// IntegrationRow is a validated integration to store.
type IntegrationRow struct {
	Type                 string
	Name                 string
	IsActive             bool
	DefaultApplicationID *string
	DefaultEnvironment   *string
	MinSeverity          string
	Mapping              []byte
	IPAllowlist          []string
	RateLimitPerMin      int
}

// CreateIntegration inserts an integration with its token hash.
func (r *Repository) CreateIntegration(ctx context.Context, in IntegrationRow, tokenHash, last4, createdBy string) (*model.Integration, error) {
	return scanIntegration(r.pool.QueryRow(ctx, `
		INSERT INTO master_integration AS i
			(type, name, token_hash, token_last4, is_active, default_application_id, default_environment_code,
			 min_severity, mapping, ip_allowlist, rate_limit_per_min, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING `+integrationColumns,
		in.Type, in.Name, tokenHash, last4, in.IsActive, in.DefaultApplicationID, in.DefaultEnvironment,
		in.MinSeverity, in.Mapping, in.IPAllowlist, in.RateLimitPerMin, createdBy))
}

// UpdateIntegration overwrites the editable fields (type is immutable).
func (r *Repository) UpdateIntegration(ctx context.Context, id string, in IntegrationRow) (*model.Integration, error) {
	return scanIntegration(r.pool.QueryRow(ctx, `
		UPDATE master_integration AS i
		SET name = $2, is_active = $3, default_application_id = $4, default_environment_code = $5,
			min_severity = $6, mapping = $7, ip_allowlist = $8, rate_limit_per_min = $9
		WHERE id = $1
		RETURNING `+integrationColumns,
		id, in.Name, in.IsActive, in.DefaultApplicationID, in.DefaultEnvironment, in.MinSeverity,
		in.Mapping, in.IPAllowlist, in.RateLimitPerMin))
}

// RotateIntegrationToken replaces the token hash; the old token stops working.
func (r *Repository) RotateIntegrationToken(ctx context.Context, id, tokenHash, last4 string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE master_integration SET token_hash = $2, token_last4 = $3, rotated_at = now() WHERE id = $1`,
		id, tokenHash, last4)
	if err != nil {
		return fmt.Errorf("rotate token: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// GetIntegration returns the integration plus its token hash (for auth only).
func (r *Repository) GetIntegration(ctx context.Context, id string) (*model.Integration, string, error) {
	var hash string
	it, err := scanIntegration(r.pool.QueryRow(ctx, `SELECT `+integrationColumns+`, i.token_hash
		FROM master_integration i WHERE i.id = $1`, id), &hash)
	if err != nil {
		return nil, "", err
	}
	return it, hash, nil
}

// ListIntegrations returns integrations with their event count of 24 hours.
func (r *Repository) ListIntegrations(ctx context.Context) ([]*model.Integration, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+integrationColumns+`,
		(SELECT COUNT(*) FROM trans_alert_event e
			WHERE e.integration_id = i.id AND e.received_at >= now() - INTERVAL '24 hours')
		FROM master_integration i ORDER BY i.created_at`)
	if err != nil {
		return nil, fmt.Errorf("list integrations: %w", err)
	}
	defer rows.Close()
	items := []*model.Integration{}
	for rows.Next() {
		var n int
		it, err := scanIntegration(rows, &n)
		if err != nil {
			return nil, fmt.Errorf("scan integration: %w", err)
		}
		it.Events24h = n
		items = append(items, it)
	}
	return items, rows.Err()
}

// TouchIntegration records the last received event time.
func (r *Repository) TouchIntegration(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `UPDATE master_integration SET last_event_at = now() WHERE id = $1`, id)
	return err
}

// AlertEventRow is one received alert to audit.
type AlertEventRow struct {
	IntegrationID string
	PayloadHash   string
	Payload       []byte
	Fingerprint   string
	Status        string
	Result        string
	Warning       string
	Error         string
	IncidentID    *string
}

// InsertAlertEvent stores the raw alert outcome.
func (r *Repository) InsertAlertEvent(ctx context.Context, e AlertEventRow) error {
	if len(e.Payload) == 0 {
		e.Payload = []byte(`{}`)
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO trans_alert_event
			(integration_id, payload_hash, payload, fingerprint, status, result, warning, error, incident_id)
		VALUES ($1,$2,$3::jsonb,$4,$5,$6,$7,$8,$9)`,
		e.IntegrationID, e.PayloadHash, string(e.Payload), e.Fingerprint, e.Status, e.Result, e.Warning, e.Error, e.IncidentID)
	if err != nil {
		return fmt.Errorf("insert alert event: %w", err)
	}
	return nil
}

// IsRecentDuplicate reports an identical alert received within `within`.
func (r *Repository) IsRecentDuplicate(ctx context.Context, integrationID, hash string, within time.Duration) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM trans_alert_event
			WHERE integration_id = $1 AND payload_hash = $2 AND result <> 'error'
				AND received_at >= now() - make_interval(secs => $3))`,
		integrationID, hash, within.Seconds()).Scan(&ok)
	return ok, err
}

// AlertLinkState is the latest incident linked to a fingerprint.
type AlertLinkState struct {
	LinkID         string
	IncidentID     string
	IncidentStatus string
	ResolvedAt     *time.Time
	ClosedAt       *time.Time
	PICID          *string
	LastActivityAt time.Time
}

// LatestAlertLink returns the most recent incident for (integration, fingerprint).
func (r *Repository) LatestAlertLink(ctx context.Context, integrationID, fingerprint string) (*AlertLinkState, error) {
	s := &AlertLinkState{}
	err := r.pool.QueryRow(ctx, `
		SELECT l.id, i.id, i.status_code, i.resolved_at, i.closed_at, i.current_pic_id, l.last_activity_at
		FROM trans_incident_alert l JOIN trans_incident i ON i.id = l.incident_id
		WHERE l.integration_id = $1 AND l.fingerprint = $2
		ORDER BY l.first_seen DESC LIMIT 1`, integrationID, fingerprint).
		Scan(&s.LinkID, &s.IncidentID, &s.IncidentStatus, &s.ResolvedAt, &s.ClosedAt, &s.PICID, &s.LastActivityAt)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// AlertLinkInput describes a new incident ↔ alert link.
type AlertLinkInput struct {
	IntegrationID   string
	IntegrationName string
	Fingerprint     string
	Title           string
	Labels          map[string]string
	SourceURL       string
}

func insertAlertLink(ctx context.Context, tx pgx.Tx, incidentID string, l AlertLinkInput, activity string) error {
	labels, _ := json.Marshal(l.Labels)
	if _, err := tx.Exec(ctx, `
		INSERT INTO trans_incident_alert (incident_id, integration_id, fingerprint, title, labels, source_url)
		VALUES ($1,$2,$3,$4,$5::jsonb,$6)
		ON CONFLICT (integration_id, fingerprint, incident_id) DO NOTHING`,
		incidentID, l.IntegrationID, l.Fingerprint, l.Title, string(labels), l.SourceURL); err != nil {
		return fmt.Errorf("insert alert link: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO trans_incident_activity (incident_id, type, payload, created_at)
		VALUES ($1,$2,jsonb_build_object('integration',$3::text,'fingerprint',$4::text,'title',$5::text), clock_timestamp())`,
		incidentID, activity, l.IntegrationName, l.Fingerprint, l.Title); err != nil {
		return fmt.Errorf("insert alert activity: %w", err)
	}
	return nil
}

// AlertLinkStep is a CreateIncident extra step linking the new incident to
// the alert in the same transaction (atomic create + dedup key).
func AlertLinkStep(l AlertLinkInput) IncidentStep {
	return func(ctx context.Context, tx pgx.Tx, created *model.Incident) error {
		return insertAlertLink(ctx, tx, created.ID, l, model.ActivityAlertCreated)
	}
}

// GroupAlert links an alert to a storm parent incident (§6.4).
func (r *Repository) GroupAlert(ctx context.Context, parentIncidentID string, l AlertLinkInput) error {
	return r.inTx(ctx, func(tx pgx.Tx) error {
		return insertAlertLink(ctx, tx, parentIncidentID, l, model.ActivityAlertGrouped)
	})
}

// TouchAlertLink records another firing/resolved signal on a link and,
// optionally, a timeline activity and a system comment (author NULL).
func (r *Repository) TouchAlertLink(ctx context.Context, linkID, incidentID, status, activity, comment string) error {
	return r.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			UPDATE trans_incident_alert
			SET last_seen = now(), last_status = $2,
				fire_count = fire_count + CASE WHEN $2 = 'firing' THEN 1 ELSE 0 END,
				last_activity_at = CASE WHEN $3 <> '' THEN now() ELSE last_activity_at END
			WHERE id = $1`, linkID, status, activity); err != nil {
			return fmt.Errorf("touch alert link: %w", err)
		}
		if activity != "" {
			if _, err := tx.Exec(ctx, `
				INSERT INTO trans_incident_activity (incident_id, type, payload, created_at)
				VALUES ($1,$2,jsonb_build_object('link_id',$3::text), clock_timestamp())`,
				incidentID, activity, linkID); err != nil {
				return fmt.Errorf("insert alert activity: %w", err)
			}
		}
		if comment != "" {
			if _, err := tx.Exec(ctx, `
				INSERT INTO trans_incident_comment (incident_id, author_id, body) VALUES ($1, NULL, $2)`,
				incidentID, comment); err != nil {
				return fmt.Errorf("insert system comment: %w", err)
			}
		}
		return nil
	})
}

// CountRecentAlertIncidents counts incidents this integration created since t
// (storm detection; storm parents excluded).
func (r *Repository) CountRecentAlertIncidents(ctx context.Context, integrationID string, since time.Time) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT l.incident_id) FROM trans_incident_alert l
		JOIN trans_incident_activity a ON a.incident_id = l.incident_id AND a.type = 'alert_created'
		WHERE l.integration_id = $1 AND l.first_seen >= $2 AND l.fingerprint NOT LIKE 'storm:%'`,
		integrationID, since).Scan(&n)
	return n, err
}

// FindStormParent returns an open storm incident for the key since t.
func (r *Repository) FindStormParent(ctx context.Context, integrationID, key string, since time.Time) (string, error) {
	var id string
	err := r.pool.QueryRow(ctx, `
		SELECT l.incident_id FROM trans_incident_alert l JOIN trans_incident i ON i.id = l.incident_id
		WHERE l.integration_id = $1 AND l.fingerprint = $2 AND l.first_seen >= $3
			AND i.status_code NOT IN ('RESOLVED','CLOSED')
		ORDER BY l.first_seen DESC LIMIT 1`, integrationID, "storm:"+key, since).Scan(&id)
	return id, err
}

// ListAlertEvents returns a page of events (newest first) + total.
func (r *Repository) ListAlertEvents(ctx context.Context, integrationID, result string, page, limit int) ([]*model.AlertEvent, int, error) {
	args := []any{integrationID}
	where := `e.integration_id = $1`
	if result != "" {
		args = append(args, result)
		where += ` AND e.result = $2`
	}
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM trans_alert_event e WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count alert events: %w", err)
	}
	args = append(args, limit, (page-1)*limit)
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT e.id, e.received_at, e.fingerprint, e.status, e.result, e.warning, e.error, e.incident_id,
			i.incident_no, e.payload
		FROM trans_alert_event e LEFT JOIN trans_incident i ON i.id = e.incident_id
		WHERE %s ORDER BY e.received_at DESC, e.id LIMIT $%d OFFSET $%d`, where, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list alert events: %w", err)
	}
	defer rows.Close()
	items := []*model.AlertEvent{}
	for rows.Next() {
		e := &model.AlertEvent{}
		var payload []byte
		if err := rows.Scan(&e.ID, &e.ReceivedAt, &e.Fingerprint, &e.Status, &e.Result, &e.Warning, &e.Error,
			&e.IncidentID, &e.IncidentNo, &payload); err != nil {
			return nil, 0, fmt.Errorf("scan alert event: %w", err)
		}
		e.Payload = json.RawMessage(payload)
		items = append(items, e)
	}
	return items, total, rows.Err()
}

// ListIncidentAlerts returns the alerts linked to an incident.
func (r *Repository) ListIncidentAlerts(ctx context.Context, incidentID string) ([]*model.IncidentAlert, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT l.id, l.incident_id, l.integration_id, m.name, l.fingerprint, l.title, l.labels, l.source_url,
			l.first_seen, l.last_seen, l.fire_count, l.last_status
		FROM trans_incident_alert l JOIN master_integration m ON m.id = l.integration_id
		WHERE l.incident_id = $1 ORDER BY l.first_seen`, incidentID)
	if err != nil {
		return nil, fmt.Errorf("list incident alerts: %w", err)
	}
	defer rows.Close()
	items := []*model.IncidentAlert{}
	for rows.Next() {
		a := &model.IncidentAlert{}
		var labels []byte
		if err := rows.Scan(&a.ID, &a.IncidentID, &a.IntegrationID, &a.IntegrationName, &a.Fingerprint, &a.Title,
			&labels, &a.SourceURL, &a.FirstSeen, &a.LastSeen, &a.FireCount, &a.LastStatus); err != nil {
			return nil, fmt.Errorf("scan incident alert: %w", err)
		}
		a.Labels = map[string]string{}
		_ = json.Unmarshal(labels, &a.Labels)
		items = append(items, a)
	}
	return items, rows.Err()
}

// PurgeAlertEvents deletes raw events older than before (retention, Q6).
func (r *Repository) PurgeAlertEvents(ctx context.Context, before time.Time) (int64, error) {
	tag, err := r.pool.Exec(ctx, `DELETE FROM trans_alert_event WHERE received_at < $1`, before)
	if err != nil {
		return 0, fmt.Errorf("purge alert events: %w", err)
	}
	return tag.RowsAffected(), nil
}

// inTx runs fn in a transaction.
func (r *Repository) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
