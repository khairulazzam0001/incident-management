package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/khairulazzam0001/incident-management/backend/internal/model"
)

// ErrStaleStatus means the change no longer had the expected status when the
// guarded UPDATE ran (concurrent action or double submit).
var ErrStaleStatus = errors.New("change status changed concurrently")

const changeColumns = `id, change_no, title, description, justification, type_code, risk_code,
	status_code, application_id, environment_code, implementation_plan, rollback_plan,
	test_plan, requester_id, implementer_id, team_id, revision, planned_start, planned_end,
	actual_start, actual_end, outcome, outcome_notes, review_notes, created_at, updated_at,
	closed_at, closed_by`

func scanChange(row pgx.Row) (*model.Change, error) {
	c := &model.Change{}
	err := row.Scan(
		&c.ID, &c.ChangeNo, &c.Title, &c.Description, &c.Justification, &c.Type, &c.Risk,
		&c.Status, &c.ApplicationID, &c.Environment, &c.ImplementationPlan, &c.RollbackPlan,
		&c.TestPlan, &c.RequesterID, &c.ImplementerID, &c.TeamID, &c.Revision, &c.PlannedStart,
		&c.PlannedEnd, &c.ActualStart, &c.ActualEnd, &c.Outcome, &c.OutcomeNotes, &c.ReviewNotes,
		&c.CreatedAt, &c.UpdatedAt, &c.ClosedAt, &c.ClosedBy,
	)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// NextChangeNo generates a race-safe human-readable number (CHG-2026-000001).
func (r *Repository) NextChangeNo(ctx context.Context) (string, error) {
	var seq int64
	if err := r.pool.QueryRow(ctx, `SELECT nextval('change_no_seq')`).Scan(&seq); err != nil {
		return "", fmt.Errorf("next change seq: %w", err)
	}
	return fmt.Sprintf("CHG-%d-%06d", time.Now().UTC().Year(), seq), nil
}

// CreateChange inserts a DRAFT change plus its "created" activity and, when
// incidentID is set, a FIX_FOR link recorded on both timelines — atomically.
func (r *Repository) CreateChange(ctx context.Context, in model.ChangeInput, changeNo, requesterID string, incident *model.Incident) (*model.Change, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	created, err := scanChange(tx.QueryRow(ctx, `
		INSERT INTO trans_change
			(change_no, title, description, justification, type_code, risk_code,
			 application_id, environment_code, implementation_plan, rollback_plan, test_plan,
			 requester_id, implementer_id, team_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		RETURNING `+changeColumns,
		changeNo, in.Title, in.Description, in.Justification, in.Type, in.Risk,
		in.ApplicationID, in.Environment, in.ImplementationPlan, in.RollbackPlan, in.TestPlan,
		requesterID, in.ImplementerID, in.TeamID,
	))
	if err != nil {
		return nil, fmt.Errorf("insert change: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO trans_change_activity (change_id, type, actor_id, to_status)
		VALUES ($1,'created',$2,'DRAFT')`, created.ID, requesterID); err != nil {
		return nil, fmt.Errorf("insert created activity: %w", err)
	}
	if incident != nil {
		if err := insertLink(ctx, tx, created, incident, model.RelationFixFor, requesterID); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return created, nil
}

func insertLink(ctx context.Context, tx pgx.Tx, c *model.Change, in *model.Incident, relation, actorID string) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO trans_change_incident (change_id, incident_id, relation, created_by)
		VALUES ($1,$2,$3,$4)`, c.ID, in.ID, relation, actorID); err != nil {
		return fmt.Errorf("insert change link: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO trans_change_activity (change_id, type, actor_id, payload)
		VALUES ($1,'incident_link',$2,jsonb_build_object(
			'action','link','incident_id',$3::text,'incident_no',$4::text,'relation',$5::text))`,
		c.ID, actorID, in.ID, in.IncidentNo, relation); err != nil {
		return fmt.Errorf("insert change link activity: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO trans_incident_activity (incident_id, type, actor_id, payload, created_at)
		VALUES ($1,'change_link',$2,jsonb_build_object(
			'action','link','change_id',$3::text,'change_no',$4::text,'relation',$5::text),
			clock_timestamp())`,
		in.ID, actorID, c.ID, c.ChangeNo, relation); err != nil {
		return fmt.Errorf("insert incident link activity: %w", err)
	}
	return nil
}

// GetChange fetches one change by id.
func (r *Repository) GetChange(ctx context.Context, id string) (*model.Change, error) {
	return scanChange(r.pool.QueryRow(ctx,
		`SELECT `+changeColumns+` FROM trans_change WHERE id = $1`, id))
}

// ChangeFilter filters listing. Page starts at 1.
type ChangeFilter struct {
	Status        string
	Type          string
	Risk          string
	ApplicationID string
	Environment   string
	RequesterID   string
	ImplementerID string
	ScheduledFrom string
	ScheduledTo   string
	Q             string
	Page          int
	Limit         int
}

// ListChanges returns a page (newest first) plus the total count.
func (r *Repository) ListChanges(ctx context.Context, f ChangeFilter) ([]*model.Change, int, error) {
	conds := []string{"1=1"}
	args := []any{}
	add := func(cond string, v any) {
		args = append(args, v)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}
	if f.Status != "" {
		add("status_code = $%d", f.Status)
	}
	if f.Type != "" {
		add("type_code = $%d", f.Type)
	}
	if f.Risk != "" {
		add("risk_code = $%d", f.Risk)
	}
	if f.ApplicationID != "" {
		add("application_id = $%d", f.ApplicationID)
	}
	if f.Environment != "" {
		add("environment_code = $%d", f.Environment)
	}
	if f.RequesterID != "" {
		add("requester_id = $%d", f.RequesterID)
	}
	if f.ImplementerID != "" {
		add("implementer_id = $%d", f.ImplementerID)
	}
	if f.ScheduledFrom != "" {
		add("planned_start >= $%d::date", f.ScheduledFrom)
	}
	if f.ScheduledTo != "" {
		add("planned_start < ($%d::date + INTERVAL '1 day')", f.ScheduledTo)
	}
	if f.Q != "" {
		args = append(args, "%"+f.Q+"%")
		conds = append(conds, fmt.Sprintf("(title ILIKE $%d OR description ILIKE $%d OR change_no ILIKE $%d)",
			len(args), len(args), len(args)))
	}
	where := strings.Join(conds, " AND ")

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM trans_change WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count changes: %w", err)
	}

	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	page := f.Page
	if page <= 0 {
		page = 1
	}
	args = append(args, limit, (page-1)*limit)
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT %s FROM trans_change
		WHERE %s ORDER BY created_at DESC, id DESC LIMIT $%d OFFSET $%d`,
		changeColumns, where, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list changes: %w", err)
	}
	defer rows.Close()
	items := []*model.Change{}
	for rows.Next() {
		c, err := scanChange(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan change: %w", err)
		}
		items = append(items, c)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate changes: %w", err)
	}
	return items, total, nil
}

// UpdateDraftChange edits a change still in DRAFT. Returns ErrStaleStatus when
// the change left DRAFT meanwhile.
func (r *Repository) UpdateDraftChange(ctx context.Context, id string, in model.ChangeInput, actorID string) (*model.Change, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	updated, err := scanChange(tx.QueryRow(ctx, `
		UPDATE trans_change
		SET title = $2, description = $3, justification = $4, type_code = $5, risk_code = $6,
			application_id = $7, environment_code = $8, implementation_plan = $9,
			rollback_plan = $10, test_plan = $11, implementer_id = $12, team_id = $13,
			updated_at = now()
		WHERE id = $1 AND status_code = 'DRAFT'
		RETURNING `+changeColumns,
		id, in.Title, in.Description, in.Justification, in.Type, in.Risk,
		in.ApplicationID, in.Environment, in.ImplementationPlan, in.RollbackPlan, in.TestPlan,
		in.ImplementerID, in.TeamID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrStaleStatus
		}
		return nil, fmt.Errorf("update change: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO trans_change_activity (change_id, type, actor_id)
		VALUES ($1,'updated',$2)`, id, actorID); err != nil {
		return nil, fmt.Errorf("insert updated activity: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return updated, nil
}

// ChangeApprovalRow is an approval to record alongside a transition.
type ChangeApprovalRow struct {
	ApproverID *string
	Decision   string
	Reason     string
}

// ChangeTransition is one guarded status move. Fields holds extra column
// updates and must only use column names chosen by the service, never input.
type ChangeTransition struct {
	From         string
	To           string
	ActorID      *string
	ActivityType string
	Payload      map[string]any
	Approval     *ChangeApprovalRow
	BumpRevision bool
	Fields       map[string]any
}

// ApplyChangeTransitions runs the moves in order inside one transaction. Each
// UPDATE is guarded by the expected from-status, so a double submit fails
// with ErrStaleStatus instead of duplicating activity.
func (r *Repository) ApplyChangeTransitions(ctx context.Context, id string, steps ...ChangeTransition) (*model.Change, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var updated *model.Change
	for _, st := range steps {
		sets := []string{"status_code = $3", "updated_at = now()"}
		args := []any{id, st.From, st.To}
		if st.BumpRevision {
			sets = append(sets, "revision = revision + 1")
		}
		for col, v := range st.Fields {
			args = append(args, v)
			sets = append(sets, fmt.Sprintf("%s = $%d", col, len(args)))
		}
		updated, err = scanChange(tx.QueryRow(ctx, `
			UPDATE trans_change SET `+strings.Join(sets, ", ")+`
			WHERE id = $1 AND status_code = $2
			RETURNING `+changeColumns, args...))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, ErrStaleStatus
			}
			return nil, fmt.Errorf("update change status: %w", err)
		}
		if st.Approval != nil {
			// Approval belongs to the revision that was decided on.
			revision := updated.Revision
			if st.BumpRevision {
				revision--
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO trans_change_approval (change_id, approver_id, decision, reason, revision)
				VALUES ($1,$2,$3,$4,$5)`,
				id, st.Approval.ApproverID, st.Approval.Decision, st.Approval.Reason, revision); err != nil {
				return nil, fmt.Errorf("insert approval: %w", err)
			}
		}
		payload := st.Payload
		if payload == nil {
			payload = map[string]any{}
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO trans_change_activity (change_id, type, actor_id, from_status, to_status, payload)
			VALUES ($1,$2,$3,$4,$5,$6)`,
			id, st.ActivityType, st.ActorID, st.From, st.To, payload); err != nil {
			return nil, fmt.Errorf("insert change activity: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return updated, nil
}

// ListChangeApprovals returns approval decisions oldest-first.
func (r *Repository) ListChangeApprovals(ctx context.Context, changeID string) ([]*model.ChangeApproval, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, change_id, approver_id, decision, reason, revision, created_at
		FROM trans_change_approval WHERE change_id = $1 ORDER BY created_at ASC, id ASC`, changeID)
	if err != nil {
		return nil, fmt.Errorf("list approvals: %w", err)
	}
	defer rows.Close()
	items := []*model.ChangeApproval{}
	for rows.Next() {
		a := &model.ChangeApproval{}
		if err := rows.Scan(&a.ID, &a.ChangeID, &a.ApproverID, &a.Decision, &a.Reason, &a.Revision, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan approval: %w", err)
		}
		items = append(items, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate approvals: %w", err)
	}
	return items, nil
}

// ListChangeActivities returns the change audit trail oldest-first (single query).
func (r *Repository) ListChangeActivities(ctx context.Context, changeID string) ([]*model.ChangeActivity, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, change_id, type, actor_id, from_status, to_status, payload, created_at
		FROM trans_change_activity WHERE change_id = $1 ORDER BY created_at ASC, id ASC`, changeID)
	if err != nil {
		return nil, fmt.Errorf("list change activities: %w", err)
	}
	defer rows.Close()
	items := []*model.ChangeActivity{}
	for rows.Next() {
		a := &model.ChangeActivity{}
		var payload any
		if err := rows.Scan(&a.ID, &a.ChangeID, &a.Type, &a.ActorID, &a.FromStatus, &a.ToStatus, &payload, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan change activity: %w", err)
		}
		a.Payload = payload
		items = append(items, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate change activities: %w", err)
	}
	return items, nil
}

// AddChangeComment inserts a comment and its timeline activity atomically.
func (r *Repository) AddChangeComment(ctx context.Context, changeID, authorID, body string) (*model.ChangeComment, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	c := &model.ChangeComment{}
	err = tx.QueryRow(ctx, `
		INSERT INTO trans_change_comment (change_id, author_id, body)
		VALUES ($1,$2,$3) RETURNING id, change_id, author_id, body, created_at`,
		changeID, authorID, body).Scan(&c.ID, &c.ChangeID, &c.AuthorID, &c.Body, &c.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert change comment: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO trans_change_activity (change_id, type, actor_id, payload)
		VALUES ($1,'comment',$2,jsonb_build_object('comment_id',$3::text))`,
		changeID, authorID, c.ID); err != nil {
		return nil, fmt.Errorf("insert change comment activity: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return c, nil
}

// ListChangeComments returns comments oldest-first.
func (r *Repository) ListChangeComments(ctx context.Context, changeID string) ([]*model.ChangeComment, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, change_id, author_id, body, created_at
		FROM trans_change_comment WHERE change_id = $1 ORDER BY created_at ASC`, changeID)
	if err != nil {
		return nil, fmt.Errorf("list change comments: %w", err)
	}
	defer rows.Close()
	items := []*model.ChangeComment{}
	for rows.Next() {
		c := &model.ChangeComment{}
		if err := rows.Scan(&c.ID, &c.ChangeID, &c.AuthorID, &c.Body, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan change comment: %w", err)
		}
		items = append(items, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate change comments: %w", err)
	}
	return items, nil
}

// ListChangeLinks returns change ↔ incident links filtered by change or
// incident id (exactly one should be set), joined in a single query.
func (r *Repository) ListChangeLinks(ctx context.Context, changeID, incidentID string) ([]*model.ChangeIncidentLink, error) {
	col, id := "l.change_id", changeID
	if incidentID != "" {
		col, id = "l.incident_id", incidentID
	}
	rows, err := r.pool.Query(ctx, `
		SELECT l.id, c.id, c.change_no, c.title, c.status_code,
			i.id, i.incident_no, i.title, i.status_code, l.relation, l.created_by, l.created_at
		FROM trans_change_incident l
		JOIN trans_change c ON c.id = l.change_id
		JOIN trans_incident i ON i.id = l.incident_id
		WHERE `+col+` = $1 ORDER BY l.created_at ASC`, id)
	if err != nil {
		return nil, fmt.Errorf("list change links: %w", err)
	}
	defer rows.Close()
	items := []*model.ChangeIncidentLink{}
	for rows.Next() {
		l := &model.ChangeIncidentLink{}
		if err := rows.Scan(&l.ID, &l.ChangeID, &l.ChangeNo, &l.ChangeTitle, &l.ChangeStatus,
			&l.IncidentID, &l.IncidentNo, &l.IncidentTitle, &l.IncidentStatus, &l.Relation,
			&l.CreatedBy, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan change link: %w", err)
		}
		items = append(items, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate change links: %w", err)
	}
	return items, nil
}
