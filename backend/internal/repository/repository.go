// Package repository runs database queries with pgx. No business logic here.
package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/khairulazzam0001/incident-management/backend/internal/model"
)

// Repository wraps the connection pool.
type Repository struct {
	pool *pgxpool.Pool
}

// New creates a repository bound to the pool.
func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// Pool exposes the pool for health checks and tests.
func (r *Repository) Pool() *pgxpool.Pool { return r.pool }

const incidentColumns = `id, incident_no, title, description, source_code, severity_code,
	priority_code, status_code, application_id, environment_code, reporter_id,
	current_team_id, current_pic_id, created_at, updated_at, resolved_at, closed_at, closed_by`

func scanIncident(row pgx.Row) (*model.Incident, error) {
	in := &model.Incident{}
	err := row.Scan(
		&in.ID, &in.IncidentNo, &in.Title, &in.Description, &in.Source,
		&in.Severity, &in.Priority, &in.Status, &in.ApplicationID, &in.Environment,
		&in.ReporterID, &in.TeamID, &in.PICID, &in.CreatedAt, &in.UpdatedAt,
		&in.ResolvedAt, &in.ClosedAt, &in.ClosedBy,
	)
	if err != nil {
		return nil, err
	}
	return in, nil
}

// NextIncidentNo generates a race-safe human-readable number (INC-2026-000123).
func (r *Repository) NextIncidentNo(ctx context.Context) (string, error) {
	var seq int64
	if err := r.pool.QueryRow(ctx, `SELECT nextval('incident_no_seq')`).Scan(&seq); err != nil {
		return "", fmt.Errorf("next incident seq: %w", err)
	}
	return fmt.Sprintf("INC-%d-%06d", time.Now().UTC().Year(), seq), nil
}

// CreateIncident inserts the incident and its "created" activity atomically.
func (r *Repository) CreateIncident(ctx context.Context, in model.CreateIncidentInput, incidentNo, reporterID string) (*model.Incident, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	created, err := scanIncident(tx.QueryRow(ctx, `
		INSERT INTO trans_incident
			(incident_no, title, description, source_code, severity_code, priority_code,
			 application_id, environment_code, reporter_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING `+incidentColumns,
		incidentNo, in.Title, in.Description, in.Source,
		in.Severity, in.Priority, in.ApplicationID, in.Environment, reporterID,
	))
	if err != nil {
		return nil, fmt.Errorf("insert incident: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO trans_incident_activity (incident_id, type, actor_id, to_status)
		VALUES ($1,'created',$2,'NEW')`, created.ID, reporterID); err != nil {
		return nil, fmt.Errorf("insert created activity: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return created, nil
}

// GetIncident fetches one incident by id.
func (r *Repository) GetIncident(ctx context.Context, id string) (*model.Incident, error) {
	return scanIncident(r.pool.QueryRow(ctx,
		`SELECT `+incidentColumns+` FROM trans_incident WHERE id = $1`, id))
}

// IncidentFilter filters listing. Page starts at 1.
type IncidentFilter struct {
	Status   string
	Severity string
	Priority string
	Q        string
	Sort     string
	Order    string
	Page     int
	Limit    int
}

// ListIncidents returns a page plus the total count.
func (r *Repository) ListIncidents(ctx context.Context, f IncidentFilter) ([]*model.Incident, int, error) {
	conds := []string{"1=1"}
	args := []any{}
	add := func(cond string, v any) {
		args = append(args, v)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}
	if f.Status != "" {
		add("status_code = $%d", f.Status)
	}
	if f.Severity != "" {
		add("severity_code = $%d", f.Severity)
	}
	if f.Priority != "" {
		add("priority_code = $%d", f.Priority)
	}
	if f.Q != "" {
		args = append(args, "%"+f.Q+"%")
		conds = append(conds, fmt.Sprintf("(title ILIKE $%d OR description ILIKE $%d OR incident_no ILIKE $%d)",
			len(args), len(args), len(args)))
	}
	sortCols := map[string]string{
		"created_at": "created_at", "updated_at": "updated_at",
		"severity": "severity_code", "priority": "priority_code", "status": "status_code",
	}
	sortCol, ok := sortCols[f.Sort]
	if !ok {
		sortCol = "created_at"
	}
	dir := "DESC"
	if strings.ToUpper(f.Order) == "ASC" {
		dir = "ASC"
	}
	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	page := f.Page
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit
	args = append(args, limit, offset)

	query := fmt.Sprintf(`SELECT %s FROM trans_incident
		WHERE %s ORDER BY %s %s LIMIT $%d OFFSET $%d`,
		incidentColumns, strings.Join(conds, " AND "), sortCol, dir, len(args)-1, len(args))

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list incidents: %w", err)
	}
	defer rows.Close()

	items := []*model.Incident{}
	for rows.Next() {
		in, err := scanIncident(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan incident: %w", err)
		}
		items = append(items, in)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate incidents: %w", err)
	}
	total, err := r.countIncidents(ctx, conds, args[:len(args)-2])
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *Repository) countIncidents(ctx context.Context, conds []string, args []any) (int, error) {
	var total int
	err := r.pool.QueryRow(ctx, fmt.Sprintf(
		`SELECT COUNT(*) FROM trans_incident WHERE %s`, strings.Join(conds, " AND ")), args...).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("count incidents: %w", err)
	}
	return total, nil
}

// UpdateStatus changes status (plus resolved/closed timestamps) and writes the
// audit activity atomically.
func (r *Repository) UpdateStatus(ctx context.Context, id, from, to, actorID string) (*model.Incident, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	updated, err := scanIncident(tx.QueryRow(ctx, `
		UPDATE trans_incident
		SET status_code = $2,
			updated_at = now(),
			resolved_at = CASE WHEN $2 = 'RESOLVED' AND resolved_at IS NULL THEN now() ELSE resolved_at END,
			closed_at = CASE WHEN $2 = 'CLOSED' THEN now() ELSE closed_at END,
			closed_by = CASE WHEN $2 = 'CLOSED' THEN $3 ELSE closed_by END
		WHERE id = $1
		RETURNING `+incidentColumns, id, to, actorID))
	if err != nil {
		return nil, fmt.Errorf("update status: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO trans_incident_activity (incident_id, type, actor_id, from_status, to_status)
		VALUES ($1,'status_change',$2,$3,$4)`, id, actorID, from, to); err != nil {
		return nil, fmt.Errorf("insert status activity: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return updated, nil
}

// AssignIncident sets team/PIC (and status when it changes) plus history rows.
func (r *Repository) AssignIncident(ctx context.Context, id string, teamID, picID *string, by, fromStatus, toStatus string, note string) (*model.Incident, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	updated, err := scanIncident(tx.QueryRow(ctx, `
		UPDATE trans_incident
		SET current_team_id = $2, current_pic_id = $3, status_code = $4, updated_at = now()
		WHERE id = $1
		RETURNING `+incidentColumns, id, teamID, picID, toStatus))
	if err != nil {
		return nil, fmt.Errorf("update assignment: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO trans_incident_assignment (incident_id, team_id, pic_id, assigned_by, note)
		VALUES ($1,$2,$3,$4,$5)`, id, teamID, picID, by, note); err != nil {
		return nil, fmt.Errorf("insert assignment history: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO trans_incident_activity (incident_id, type, actor_id, from_status, to_status, payload)
		VALUES ($1,'assignment',$2,$3,$4,jsonb_build_object('team_id',$5::text,'pic_id',$6::text))`,
		id, by, fromStatus, toStatus, teamID, picID); err != nil {
		return nil, fmt.Errorf("insert assignment activity: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return updated, nil
}

// AddComment inserts a comment and its timeline activity atomically.
func (r *Repository) AddComment(ctx context.Context, incidentID, authorID, body string) (*model.Comment, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	c := &model.Comment{}
	err = tx.QueryRow(ctx, `
		INSERT INTO trans_incident_comment (incident_id, author_id, body)
		VALUES ($1,$2,$3) RETURNING id, incident_id, author_id, body, created_at`,
		incidentID, authorID, body).Scan(&c.ID, &c.IncidentID, &c.AuthorID, &c.Body, &c.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert comment: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO trans_incident_activity (incident_id, type, actor_id, payload)
		VALUES ($1,'comment',$2,jsonb_build_object('comment_id',$3::text))`,
		incidentID, authorID, c.ID); err != nil {
		return nil, fmt.Errorf("insert comment activity: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return c, nil
}

// ListComments returns comments newest-last for one incident.
func (r *Repository) ListComments(ctx context.Context, incidentID string) ([]*model.Comment, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, incident_id, author_id, body, created_at
		FROM trans_incident_comment WHERE incident_id = $1 ORDER BY created_at ASC`, incidentID)
	if err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}
	defer rows.Close()
	items := []*model.Comment{}
	for rows.Next() {
		c := &model.Comment{}
		if err := rows.Scan(&c.ID, &c.IncidentID, &c.AuthorID, &c.Body, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan comment: %w", err)
		}
		items = append(items, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate comments: %w", err)
	}
	return items, nil
}

// AddInvestigation records notes/findings and moves ASSIGNED → INVESTIGATING.
func (r *Repository) AddInvestigation(ctx context.Context, incidentID, authorID, notes, findings, from, to string) (*model.Investigation, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	inv := &model.Investigation{}
	err = tx.QueryRow(ctx, `
		INSERT INTO trans_incident_investigation (incident_id, author_id, notes, findings)
		VALUES ($1,$2,$3,$4) RETURNING id, incident_id, author_id, notes, findings, created_at`,
		incidentID, authorID, notes, findings).
		Scan(&inv.ID, &inv.IncidentID, &inv.AuthorID, &inv.Notes, &inv.Findings, &inv.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert investigation: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE trans_incident SET status_code = $2, updated_at = now() WHERE id = $1`,
		incidentID, to); err != nil {
		return nil, fmt.Errorf("update status: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO trans_incident_activity (incident_id, type, actor_id, from_status, to_status, payload)
		VALUES ($1,'investigation',$2,$3,$4,jsonb_build_object('investigation_id',$5::text))`,
		incidentID, authorID, from, to, inv.ID); err != nil {
		return nil, fmt.Errorf("insert investigation activity: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return inv, nil
}

// ListInvestigations returns investigation records oldest-first.
func (r *Repository) ListInvestigations(ctx context.Context, incidentID string) ([]*model.Investigation, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, incident_id, author_id, notes, findings, created_at
		FROM trans_incident_investigation WHERE incident_id = $1 ORDER BY created_at ASC`, incidentID)
	if err != nil {
		return nil, fmt.Errorf("list investigations: %w", err)
	}
	defer rows.Close()
	items := []*model.Investigation{}
	for rows.Next() {
		inv := &model.Investigation{}
		if err := rows.Scan(&inv.ID, &inv.IncidentID, &inv.AuthorID, &inv.Notes, &inv.Findings, &inv.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan investigation: %w", err)
		}
		items = append(items, inv)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate investigations: %w", err)
	}
	return items, nil
}

// AddFix records the fix and moves INVESTIGATING → FIXING.
func (r *Repository) AddFix(ctx context.Context, incidentID, authorID, description, reference, from, to string) (*model.Fix, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	f := &model.Fix{}
	err = tx.QueryRow(ctx, `
		INSERT INTO trans_incident_fix (incident_id, author_id, description, reference)
		VALUES ($1,$2,$3,$4) RETURNING id, incident_id, author_id, description, reference, created_at`,
		incidentID, authorID, description, reference).
		Scan(&f.ID, &f.IncidentID, &f.AuthorID, &f.Description, &f.Reference, &f.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert fix: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE trans_incident SET status_code = $2, updated_at = now() WHERE id = $1`,
		incidentID, to); err != nil {
		return nil, fmt.Errorf("update status: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO trans_incident_activity (incident_id, type, actor_id, from_status, to_status, payload)
		VALUES ($1,'fix',$2,$3,$4,jsonb_build_object('fix_id',$5::text))`,
		incidentID, authorID, from, to, f.ID); err != nil {
		return nil, fmt.Errorf("insert fix activity: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return f, nil
}

// ListFixes returns fix records oldest-first.
func (r *Repository) ListFixes(ctx context.Context, incidentID string) ([]*model.Fix, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, incident_id, author_id, description, reference, created_at
		FROM trans_incident_fix WHERE incident_id = $1 ORDER BY created_at ASC`, incidentID)
	if err != nil {
		return nil, fmt.Errorf("list fixes: %w", err)
	}
	defer rows.Close()
	items := []*model.Fix{}
	for rows.Next() {
		f := &model.Fix{}
		if err := rows.Scan(&f.ID, &f.IncidentID, &f.AuthorID, &f.Description, &f.Reference, &f.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan fix: %w", err)
		}
		items = append(items, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate fixes: %w", err)
	}
	return items, nil
}

// AddVerification records PASS/FAIL and moves the status accordingly:
// PASS → RESOLVED, FAIL → FIXING.
func (r *Repository) AddVerification(ctx context.Context, incidentID, verifierID, result, reason, from, to string) (*model.Verification, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	v := &model.Verification{}
	err = tx.QueryRow(ctx, `
		INSERT INTO trans_incident_verification (incident_id, verifier_id, result, reason)
		VALUES ($1,$2,$3,$4) RETURNING id, incident_id, verifier_id, result, reason, created_at`,
		incidentID, verifierID, result, reason).
		Scan(&v.ID, &v.IncidentID, &v.VerifierID, &v.Result, &v.Reason, &v.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert verification: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE trans_incident
		SET status_code = $2, updated_at = now(),
			resolved_at = CASE WHEN $2 = 'RESOLVED' AND resolved_at IS NULL THEN now() ELSE resolved_at END
		WHERE id = $1`, incidentID, to); err != nil {
		return nil, fmt.Errorf("update status: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO trans_incident_activity (incident_id, type, actor_id, from_status, to_status, payload)
		VALUES ($1,'verification',$2,$3,$4,jsonb_build_object('verification_id',$5::text,'result',$6::text))`,
		incidentID, verifierID, from, to, v.ID, result); err != nil {
		return nil, fmt.Errorf("insert verification activity: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return v, nil
}

// ListVerifications returns verification records oldest-first.
func (r *Repository) ListVerifications(ctx context.Context, incidentID string) ([]*model.Verification, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, incident_id, verifier_id, result, reason, created_at
		FROM trans_incident_verification WHERE incident_id = $1 ORDER BY created_at ASC`, incidentID)
	if err != nil {
		return nil, fmt.Errorf("list verifications: %w", err)
	}
	defer rows.Close()
	items := []*model.Verification{}
	for rows.Next() {
		v := &model.Verification{}
		if err := rows.Scan(&v.ID, &v.IncidentID, &v.VerifierID, &v.Result, &v.Reason, &v.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan verification: %w", err)
		}
		items = append(items, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate verifications: %w", err)
	}
	return items, nil
}

// Reopen moves RESOLVED/CLOSED back to INVESTIGATING with a reason (FR-12).
func (r *Repository) Reopen(ctx context.Context, id, from, to, actorID, reason string) (*model.Incident, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	updated, err := scanIncident(tx.QueryRow(ctx, `
		UPDATE trans_incident
		SET status_code = $2, updated_at = now(),
			resolved_at = CASE WHEN $2 <> 'RESOLVED' THEN NULL ELSE resolved_at END,
			closed_at = NULL, closed_by = NULL
		WHERE id = $1
		RETURNING `+incidentColumns, id, to))
	if err != nil {
		return nil, fmt.Errorf("reopen incident: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO trans_incident_activity (incident_id, type, actor_id, from_status, to_status, payload)
		VALUES ($1,'reopen',$2,$3,$4,jsonb_build_object('reason',$5::text))`,
		id, actorID, from, to, reason); err != nil {
		return nil, fmt.Errorf("insert reopen activity: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return updated, nil
}

// ListActivities returns the audit timeline oldest-first (no N+1: single query).
func (r *Repository) ListActivities(ctx context.Context, incidentID string) ([]*model.Activity, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, incident_id, type, actor_id, from_status, to_status, payload, created_at
		FROM trans_incident_activity WHERE incident_id = $1 ORDER BY created_at ASC, id ASC`, incidentID)
	if err != nil {
		return nil, fmt.Errorf("list activities: %w", err)
	}
	defer rows.Close()
	items := []*model.Activity{}
	for rows.Next() {
		a := &model.Activity{}
		var payload any
		if err := rows.Scan(&a.ID, &a.IncidentID, &a.Type, &a.ActorID, &a.FromStatus, &a.ToStatus, &payload, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan activity: %w", err)
		}
		a.Payload = payload
		items = append(items, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate activities: %w", err)
	}
	return items, nil
}

// FindUserByEmail returns the user plus password hash for login.
func (r *Repository) FindUserByEmail(ctx context.Context, email string) (*model.User, string, error) {
	u := &model.User{}
	var hash string
	err := r.pool.QueryRow(ctx, `
		SELECT id, email, name, role, team_id, is_active, password_hash
		FROM master_user WHERE email = $1`, email).
		Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.TeamID, &u.IsActive, &hash)
	if err != nil {
		return nil, "", err
	}
	return u, hash, nil
}

// GetUserByID fetches a user for existence/role checks.
func (r *Repository) GetUserByID(ctx context.Context, id string) (*model.User, error) {
	u := &model.User{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, email, name, role, team_id, is_active FROM master_user WHERE id = $1`, id).
		Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.TeamID, &u.IsActive)
	if err != nil {
		return nil, err
	}
	return u, nil
}

// TeamExists reports whether the team id exists (nil id counts as absent-ok).
func (r *Repository) TeamExists(ctx context.Context, id *string) (bool, error) {
	if id == nil || *id == "" {
		return true, nil
	}
	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM master_team WHERE id = $1)`, *id).Scan(&exists)
	return exists, err
}

// ListUsers returns active users for assignment lookup.
func (r *Repository) ListUsers(ctx context.Context) ([]*model.User, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, email, name, role, team_id, is_active
		FROM master_user WHERE is_active = TRUE ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()
	users := []*model.User{}
	for rows.Next() {
		u := &model.User{}
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.TeamID, &u.IsActive); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users: %w", err)
	}
	return users, nil
}

// ListMeta returns all master data for forms and filters.
func (r *Repository) ListMeta(ctx context.Context) (*model.Meta, error) {
	meta := &model.Meta{
		Statuses: []model.MasterItem{}, Severities: []model.MasterItem{},
		Priorities: []model.MasterItem{}, Environments: []model.MasterItem{},
		Sources: []model.MasterItem{}, Applications: []model.MasterIDItem{},
		Teams: []model.MasterIDItem{},
	}
	collect := func(query, order string, dest *[]model.MasterItem) error {
		rows, err := r.pool.Query(ctx, query+` ORDER BY `+order)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var it model.MasterItem
			if err := rows.Scan(&it.Code, &it.Name); err != nil {
				return err
			}
			*dest = append(*dest, it)
		}
		return rows.Err()
	}
	if err := collect(`SELECT code, name FROM master_incident_status`, "sort_order", &meta.Statuses); err != nil {
		return nil, fmt.Errorf("list statuses: %w", err)
	}
	if err := collect(`SELECT code, name FROM master_incident_severity`, "sort_order", &meta.Severities); err != nil {
		return nil, fmt.Errorf("list severities: %w", err)
	}
	if err := collect(`SELECT code, name FROM master_incident_priority`, "sort_order", &meta.Priorities); err != nil {
		return nil, fmt.Errorf("list priorities: %w", err)
	}
	if err := collect(`SELECT code, name FROM master_environment`, "name", &meta.Environments); err != nil {
		return nil, fmt.Errorf("list environments: %w", err)
	}
	if err := collect(`SELECT code, name FROM master_incident_source`, "name", &meta.Sources); err != nil {
		return nil, fmt.Errorf("list sources: %w", err)
	}
	rows, err := r.pool.Query(ctx, `SELECT id, code, name FROM master_application ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list applications: %w", err)
	}
	for rows.Next() {
		var it model.MasterIDItem
		if err := rows.Scan(&it.ID, &it.Code, &it.Name); err != nil {
			rows.Close()
			return nil, err
		}
		meta.Applications = append(meta.Applications, it)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows, err = r.pool.Query(ctx, `SELECT id, code, name FROM master_team ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list teams: %w", err)
	}
	for rows.Next() {
		var it model.MasterIDItem
		if err := rows.Scan(&it.ID, &it.Code, &it.Name); err != nil {
			rows.Close()
			return nil, err
		}
		meta.Teams = append(meta.Teams, it)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return meta, nil
}
