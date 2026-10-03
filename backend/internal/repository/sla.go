package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/khairulazzam0001/incident-management/backend/internal/model"
	"github.com/khairulazzam0001/incident-management/backend/internal/sla"
)

// SLA & Escalation queries (PRD_SLA_Escalation.md §9). Rules live in package
// sla and the service; this file only reads/writes rows.

// SLAInstanceInput is one SLA instance to create alongside an incident event.
type SLAInstanceInput struct {
	Metric          string
	PriorityCode    string
	TargetMinutes   int
	CalendarCode    string
	WarnPercent     int
	ReminderMinutes *int
	StartedAt       time.Time
	WarnAt          time.Time
	TargetAt        time.Time
}

const incidentSLAColumns = `id, incident_id, metric, cycle, status, priority_code, target_minutes,
	calendar_code, warn_percent, reminder_minutes, started_at, warn_at, target_at,
	warned_at, breached_at, reminded_at, stopped_at`

func scanIncidentSLA(row pgx.Row) (*model.IncidentSLA, error) {
	s := &model.IncidentSLA{}
	err := row.Scan(&s.ID, &s.IncidentID, &s.Metric, &s.Cycle, &s.Status, &s.PriorityCode,
		&s.TargetMinutes, &s.CalendarCode, &s.WarnPercent, &s.ReminderMinutes, &s.StartedAt,
		&s.WarnAt, &s.TargetAt, &s.WarnedAt, &s.BreachedAt, &s.RemindedAt, &s.StoppedAt)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// insertSLAInstances creates instances (cycle = next per metric) in tx. The
// first check is at warn_at.
func insertSLAInstances(ctx context.Context, tx pgx.Tx, incidentID string, rows []SLAInstanceInput) error {
	for _, r := range rows {
		if _, err := tx.Exec(ctx, `
			INSERT INTO trans_incident_sla
				(incident_id, metric, cycle, priority_code, target_minutes, calendar_code,
				 warn_percent, reminder_minutes, started_at, warn_at, target_at, next_check_at)
			VALUES ($1, $2,
				(SELECT COALESCE(MAX(cycle), 0) + 1 FROM trans_incident_sla WHERE incident_id = $1 AND metric = $2),
				$3, $4, $5, $6, $7, $8, $9, $10, $9)`,
			incidentID, r.Metric, r.PriorityCode, r.TargetMinutes, r.CalendarCode,
			r.WarnPercent, r.ReminderMinutes, r.StartedAt, r.WarnAt, r.TargetAt); err != nil {
			return fmt.Errorf("insert sla %s: %w", r.Metric, err)
		}
	}
	return nil
}

// stopSLA stops the open instance(s) of a metric: RUNNING becomes MET, a
// BREACHED one keeps its status and only records stopped_at.
func stopSLA(ctx context.Context, tx pgx.Tx, incidentID, metric string) error {
	_, err := tx.Exec(ctx, `
		UPDATE trans_incident_sla
		SET status = CASE WHEN status = 'RUNNING' THEN 'MET' ELSE status END,
			stopped_at = now(), next_check_at = NULL
		WHERE incident_id = $1 AND metric = $2 AND stopped_at IS NULL
			AND status IN ('RUNNING','BREACHED')`, incidentID, metric)
	if err != nil {
		return fmt.Errorf("stop sla %s: %w", metric, err)
	}
	return nil
}

// cancelOpenSLA ends every open instance when the incident closes without the
// stop event (RUNNING → CANCELLED; BREACHED keeps status).
func cancelOpenSLA(ctx context.Context, tx pgx.Tx, incidentID string) error {
	_, err := tx.Exec(ctx, `
		UPDATE trans_incident_sla
		SET status = CASE WHEN status = 'RUNNING' THEN 'CANCELLED' ELSE status END,
			stopped_at = now(), next_check_at = NULL
		WHERE incident_id = $1 AND stopped_at IS NULL AND status IN ('RUNNING','BREACHED')`, incidentID)
	if err != nil {
		return fmt.Errorf("cancel sla: %w", err)
	}
	return nil
}

// HasSLA reports whether the incident has any SLA instance (incidents created
// before SLA went live have none and are never backfilled).
func (r *Repository) HasSLA(ctx context.Context, incidentID string) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM trans_incident_sla WHERE incident_id = $1)`, incidentID).Scan(&ok)
	return ok, err
}

// ListIncidentSLA returns all instances of an incident, by cycle then metric.
func (r *Repository) ListIncidentSLA(ctx context.Context, incidentID string) ([]*model.IncidentSLA, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+incidentSLAColumns+` FROM trans_incident_sla
		WHERE incident_id = $1 ORDER BY cycle, metric DESC`, incidentID)
	if err != nil {
		return nil, fmt.Errorf("list incident sla: %w", err)
	}
	defer rows.Close()
	items := []*model.IncidentSLA{}
	for rows.Next() {
		s, err := scanIncidentSLA(rows)
		if err != nil {
			return nil, fmt.Errorf("scan incident sla: %w", err)
		}
		items = append(items, s)
	}
	return items, rows.Err()
}

// SLASummaries returns the latest cycle per metric for the given incidents in
// one query (no N+1 on lists).
func (r *Repository) SLASummaries(ctx context.Context, incidentIDs []string) (map[string]*model.SLASummary, error) {
	out := map[string]*model.SLASummary{}
	if len(incidentIDs) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT ON (incident_id, metric)
			incident_id, metric, status, cycle, warn_at, target_at, stopped_at
		FROM trans_incident_sla WHERE incident_id = ANY($1)
		ORDER BY incident_id, metric, cycle DESC`, incidentIDs)
	if err != nil {
		return nil, fmt.Errorf("sla summaries: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, metric string
		st := &model.SLAState{}
		if err := rows.Scan(&id, &metric, &st.Status, &st.Cycle, &st.WarnAt, &st.TargetAt, &st.StoppedAt); err != nil {
			return nil, fmt.Errorf("scan sla summary: %w", err)
		}
		sum := out[id]
		if sum == nil {
			sum = &model.SLASummary{}
			out[id] = sum
		}
		if metric == sla.MetricResponse {
			sum.Response = st
		} else {
			sum.Resolution = st
		}
	}
	return out, rows.Err()
}

// ProcessDueSLA locks due instances (SKIP LOCKED: safe with several API
// replicas), applies decide, records escalations idempotently (UNIQUE
// sla_id+level) plus timeline activity, and returns the escalations to notify.
func (r *Repository) ProcessDueSLA(ctx context.Context, now time.Time, limit int,
	decide func(sla.Instance, time.Time) sla.Decision) ([]model.SLAEvent, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, `SELECT `+incidentSLAColumns+` FROM trans_incident_sla
		WHERE next_check_at IS NOT NULL AND next_check_at <= $1 AND stopped_at IS NULL
		ORDER BY next_check_at LIMIT $2 FOR UPDATE SKIP LOCKED`, now, limit)
	if err != nil {
		return nil, fmt.Errorf("select due sla: %w", err)
	}
	due := []*model.IncidentSLA{}
	for rows.Next() {
		s, err := scanIncidentSLA(rows)
		if err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan due sla: %w", err)
		}
		due = append(due, s)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate due sla: %w", err)
	}

	activityType := map[string]string{
		sla.LevelWarning:  model.ActivitySLAWarning,
		sla.LevelBreach:   model.ActivitySLABreached,
		sla.LevelReminder: model.ActivitySLAReminder,
	}
	events := []model.SLAEvent{}
	for _, s := range due {
		d := decide(sla.Instance{
			Status: s.Status, TargetAt: s.TargetAt, WarnAt: s.WarnAt, WarnedAt: s.WarnedAt,
			BreachedAt: s.BreachedAt, RemindedAt: s.RemindedAt, StoppedAt: s.StoppedAt,
			ReminderMinutes: s.ReminderMinutes,
		}, now)
		var warned, reminded bool
		for _, lvl := range d.Levels {
			warned = warned || lvl == sla.LevelWarning
			reminded = reminded || lvl == sla.LevelReminder
		}
		if _, err := tx.Exec(ctx, `
			UPDATE trans_incident_sla
			SET status = $2, next_check_at = $3,
				warned_at = CASE WHEN $4 AND warned_at IS NULL THEN $6::timestamptz ELSE warned_at END,
				breached_at = CASE WHEN $2 = 'BREACHED' AND breached_at IS NULL THEN target_at ELSE breached_at END,
				reminded_at = CASE WHEN $5 AND reminded_at IS NULL THEN $6::timestamptz ELSE reminded_at END
			WHERE id = $1`, s.ID, d.NewStatus, d.NextCheckAt, warned, reminded, now); err != nil {
			return nil, fmt.Errorf("update sla: %w", err)
		}
		for _, lvl := range d.Levels {
			var escID string
			err := tx.QueryRow(ctx, `
				INSERT INTO trans_incident_escalation (incident_id, sla_id, level)
				VALUES ($1, $2, $3) ON CONFLICT (sla_id, level) DO NOTHING RETURNING id`,
				s.IncidentID, s.ID, lvl).Scan(&escID)
			if errors.Is(err, pgx.ErrNoRows) {
				continue // sudah pernah dieskalasi pada level ini
			}
			if err != nil {
				return nil, fmt.Errorf("insert escalation: %w", err)
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO trans_incident_activity (incident_id, type, payload, created_at)
				VALUES ($1, $2, jsonb_build_object('metric', $3::text, 'cycle', $4::int,
					'target_at', $5::timestamptz), clock_timestamp())`,
				s.IncidentID, activityType[lvl], s.Metric, s.Cycle, s.TargetAt); err != nil {
				return nil, fmt.Errorf("insert sla activity: %w", err)
			}
			events = append(events, model.SLAEvent{IncidentID: s.IncidentID, SLAID: s.ID, Metric: s.Metric, Level: lvl})
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return events, nil
}

// ---------- master data ----------

// GetSLAPolicy returns the policy for a priority.
func (r *Repository) GetSLAPolicy(ctx context.Context, priority string) (*model.SLAPolicy, error) {
	p := &model.SLAPolicy{}
	err := r.pool.QueryRow(ctx, `
		SELECT priority_code, response_minutes, resolution_minutes, calendar_code, warn_percent,
			breach_reminder_minutes, updated_by, updated_at
		FROM master_sla_policy WHERE priority_code = $1`, priority).
		Scan(&p.PriorityCode, &p.ResponseMinutes, &p.ResolutionMinutes, &p.CalendarCode,
			&p.WarnPercent, &p.BreachReminderMinutes, &p.UpdatedBy, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// ListSLAPolicies returns policies ordered by priority.
func (r *Repository) ListSLAPolicies(ctx context.Context) ([]*model.SLAPolicy, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT p.priority_code, p.response_minutes, p.resolution_minutes, p.calendar_code,
			p.warn_percent, p.breach_reminder_minutes, p.updated_by, p.updated_at
		FROM master_sla_policy p JOIN master_incident_priority m ON m.code = p.priority_code
		ORDER BY m.sort_order`)
	if err != nil {
		return nil, fmt.Errorf("list sla policies: %w", err)
	}
	defer rows.Close()
	items := []*model.SLAPolicy{}
	for rows.Next() {
		p := &model.SLAPolicy{}
		if err := rows.Scan(&p.PriorityCode, &p.ResponseMinutes, &p.ResolutionMinutes, &p.CalendarCode,
			&p.WarnPercent, &p.BreachReminderMinutes, &p.UpdatedBy, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan sla policy: %w", err)
		}
		items = append(items, p)
	}
	return items, rows.Err()
}

// UpdateSLAPolicy overwrites one policy. Returns pgx.ErrNoRows when unknown.
func (r *Repository) UpdateSLAPolicy(ctx context.Context, p model.SLAPolicy, actorID string) (*model.SLAPolicy, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE master_sla_policy
		SET response_minutes = $2, resolution_minutes = $3, calendar_code = $4, warn_percent = $5,
			breach_reminder_minutes = $6, updated_by = $7, updated_at = now()
		WHERE priority_code = $1`,
		p.PriorityCode, p.ResponseMinutes, p.ResolutionMinutes, p.CalendarCode, p.WarnPercent,
		p.BreachReminderMinutes, actorID)
	if err != nil {
		return nil, fmt.Errorf("update sla policy: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, pgx.ErrNoRows
	}
	return r.GetSLAPolicy(ctx, p.PriorityCode)
}

// ListCalendars returns calendars with their working hours.
func (r *Repository) ListCalendars(ctx context.Context) ([]*model.BusinessCalendar, error) {
	rows, err := r.pool.Query(ctx, `SELECT code, name, timezone, is_24x7 FROM master_business_calendar ORDER BY code`)
	if err != nil {
		return nil, fmt.Errorf("list calendars: %w", err)
	}
	cals := []*model.BusinessCalendar{}
	byCode := map[string]*model.BusinessCalendar{}
	for rows.Next() {
		c := &model.BusinessCalendar{Hours: []model.BusinessHours{}}
		if err := rows.Scan(&c.Code, &c.Name, &c.Timezone, &c.Is24x7); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan calendar: %w", err)
		}
		cals = append(cals, c)
		byCode[c.Code] = c
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate calendars: %w", err)
	}
	rows, err = r.pool.Query(ctx, `SELECT calendar_code, weekday, start_minute, end_minute
		FROM master_business_hours ORDER BY calendar_code, weekday`)
	if err != nil {
		return nil, fmt.Errorf("list business hours: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var code string
		var h model.BusinessHours
		if err := rows.Scan(&code, &h.Weekday, &h.StartMinute, &h.EndMinute); err != nil {
			return nil, fmt.Errorf("scan business hours: %w", err)
		}
		if c := byCode[code]; c != nil {
			c.Hours = append(c.Hours, h)
		}
	}
	return cals, rows.Err()
}

// LoadCalendar builds the pure sla.Calendar (hours + all holidays) for code.
func (r *Repository) LoadCalendar(ctx context.Context, code string) (sla.Calendar, error) {
	var tz string
	var always bool
	if err := r.pool.QueryRow(ctx, `SELECT timezone, is_24x7 FROM master_business_calendar WHERE code = $1`, code).
		Scan(&tz, &always); err != nil {
		return sla.Calendar{}, fmt.Errorf("load calendar %s: %w", code, err)
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return sla.Calendar{}, fmt.Errorf("load timezone %s: %w", tz, err)
	}
	cal := sla.Calendar{Code: code, Location: loc, Always: always,
		Hours: map[time.Weekday]sla.Window{}, Holidays: map[string]bool{}}
	if always {
		return cal, nil
	}
	rows, err := r.pool.Query(ctx, `SELECT weekday, start_minute, end_minute FROM master_business_hours WHERE calendar_code = $1`, code)
	if err != nil {
		return sla.Calendar{}, fmt.Errorf("load hours: %w", err)
	}
	for rows.Next() {
		var wd, start, end int
		if err := rows.Scan(&wd, &start, &end); err != nil {
			rows.Close()
			return sla.Calendar{}, fmt.Errorf("scan hours: %w", err)
		}
		cal.Hours[time.Weekday(wd)] = sla.Window{Start: start, End: end}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return sla.Calendar{}, fmt.Errorf("iterate hours: %w", err)
	}
	rows, err = r.pool.Query(ctx, `SELECT TO_CHAR(date, 'YYYY-MM-DD') FROM master_holiday WHERE calendar_code = $1`, code)
	if err != nil {
		return sla.Calendar{}, fmt.Errorf("load holidays: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return sla.Calendar{}, fmt.Errorf("scan holiday: %w", err)
		}
		cal.Holidays[d] = true
	}
	return cal, rows.Err()
}

// ReplaceBusinessHours swaps a calendar's working windows atomically.
func (r *Repository) ReplaceBusinessHours(ctx context.Context, code string, hours []model.BusinessHours) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `DELETE FROM master_business_hours WHERE calendar_code = $1`, code); err != nil {
		return fmt.Errorf("delete hours: %w", err)
	}
	for _, h := range hours {
		if _, err := tx.Exec(ctx, `
			INSERT INTO master_business_hours (calendar_code, weekday, start_minute, end_minute)
			VALUES ($1, $2, $3, $4)`, code, h.Weekday, h.StartMinute, h.EndMinute); err != nil {
			return fmt.Errorf("insert hours: %w", err)
		}
	}
	return tx.Commit(ctx)
}

// CalendarExists reports whether code is a known calendar and if it is 24x7.
func (r *Repository) CalendarExists(ctx context.Context, code string) (exists, is24x7 bool, err error) {
	err = r.pool.QueryRow(ctx, `SELECT is_24x7 FROM master_business_calendar WHERE code = $1`, code).Scan(&is24x7)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, false, nil
	}
	return err == nil, is24x7, err
}

// ListHolidays returns holidays, optionally of one year.
func (r *Repository) ListHolidays(ctx context.Context, year int) ([]*model.Holiday, error) {
	query := `SELECT id, calendar_code, TO_CHAR(date, 'YYYY-MM-DD'), name FROM master_holiday`
	args := []any{}
	if year > 0 {
		query += ` WHERE EXTRACT(YEAR FROM date) = $1`
		args = append(args, year)
	}
	rows, err := r.pool.Query(ctx, query+` ORDER BY date`, args...)
	if err != nil {
		return nil, fmt.Errorf("list holidays: %w", err)
	}
	defer rows.Close()
	items := []*model.Holiday{}
	for rows.Next() {
		h := &model.Holiday{}
		if err := rows.Scan(&h.ID, &h.CalendarCode, &h.Date, &h.Name); err != nil {
			return nil, fmt.Errorf("scan holiday: %w", err)
		}
		items = append(items, h)
	}
	return items, rows.Err()
}

// CreateHoliday inserts a holiday (unique per calendar+date).
func (r *Repository) CreateHoliday(ctx context.Context, h model.Holiday, actorID string) (*model.Holiday, error) {
	out := &model.Holiday{}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO master_holiday (calendar_code, date, name, created_by) VALUES ($1, $2::date, $3, $4)
		RETURNING id, calendar_code, TO_CHAR(date, 'YYYY-MM-DD'), name`,
		h.CalendarCode, h.Date, h.Name, actorID).Scan(&out.ID, &out.CalendarCode, &out.Date, &out.Name)
	if err != nil {
		return nil, fmt.Errorf("insert holiday: %w", err)
	}
	return out, nil
}

// DeleteHoliday removes a holiday; false when not found.
func (r *Repository) DeleteHoliday(ctx context.Context, id string) (bool, error) {
	tag, err := r.pool.Exec(ctx, `DELETE FROM master_holiday WHERE id = $1`, id)
	if err != nil {
		return false, fmt.Errorf("delete holiday: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// ---------- dashboard ----------

// SLACompliance is one priority × metric bucket.
type SLACompliance struct {
	Priority   string   `json:"priority"`
	Metric     string   `json:"metric"`
	Total      int      `json:"total"`
	Met        int      `json:"met"`
	Breached   int      `json:"breached"`
	Running    int      `json:"running"`
	Compliance *float64 `json:"compliance"`
	AvgMinutes *float64 `json:"avg_minutes"` // MTTA (RESPONSE) / MTTR (RESOLUTION), wall clock
}

// ActiveBreach is an incident whose SLA is breached and still open.
type ActiveBreach struct {
	IncidentID string    `json:"incident_id"`
	IncidentNo string    `json:"incident_no"`
	Title      string    `json:"title"`
	Priority   string    `json:"priority"`
	Metric     string    `json:"metric"`
	TargetAt   time.Time `json:"target_at"`
}

// SLADashboard aggregates instances started in [from, to).
type SLADashboard struct {
	From             string          `json:"from"`
	To               string          `json:"to"`
	Buckets          []SLACompliance `json:"buckets"`
	ActiveBreaches   []ActiveBreach  `json:"active_breaches"`
	WorkerLastTickAt *time.Time      `json:"worker_last_tick_at"`
}

// GetSLADashboard computes compliance per priority/metric and open breaches.
func (r *Repository) GetSLADashboard(ctx context.Context, from, to string) (*SLADashboard, error) {
	d := &SLADashboard{From: from, To: to, Buckets: []SLACompliance{}, ActiveBreaches: []ActiveBreach{}}
	rows, err := r.pool.Query(ctx, `
		SELECT priority_code, metric, COUNT(*),
			COUNT(*) FILTER (WHERE status = 'MET'),
			COUNT(*) FILTER (WHERE status = 'BREACHED'),
			COUNT(*) FILTER (WHERE status = 'RUNNING'),
			AVG(EXTRACT(EPOCH FROM (stopped_at - started_at)) / 60)
				FILTER (WHERE stopped_at IS NOT NULL AND status IN ('MET','BREACHED'))
		FROM trans_incident_sla
		WHERE started_at >= $1::date AND started_at < ($2::date + INTERVAL '1 day')
		GROUP BY priority_code, metric ORDER BY priority_code, metric DESC`, from, to)
	if err != nil {
		return nil, fmt.Errorf("sla dashboard: %w", err)
	}
	for rows.Next() {
		var b SLACompliance
		if err := rows.Scan(&b.Priority, &b.Metric, &b.Total, &b.Met, &b.Breached, &b.Running, &b.AvgMinutes); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan sla bucket: %w", err)
		}
		if decided := b.Met + b.Breached; decided > 0 {
			v := float64(b.Met) / float64(decided)
			b.Compliance = &v
		}
		d.Buckets = append(d.Buckets, b)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sla buckets: %w", err)
	}
	rows, err = r.pool.Query(ctx, `
		SELECT i.id, i.incident_no, i.title, i.priority_code, s.metric, s.target_at
		FROM trans_incident_sla s JOIN trans_incident i ON i.id = s.incident_id
		WHERE s.status = 'BREACHED' AND s.stopped_at IS NULL
		ORDER BY s.target_at LIMIT 20`)
	if err != nil {
		return nil, fmt.Errorf("active breaches: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var b ActiveBreach
		if err := rows.Scan(&b.IncidentID, &b.IncidentNo, &b.Title, &b.Priority, &b.Metric, &b.TargetAt); err != nil {
			return nil, fmt.Errorf("scan breach: %w", err)
		}
		d.ActiveBreaches = append(d.ActiveBreaches, b)
	}
	return d, rows.Err()
}
