package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// openCondition matches non-terminal incidents for "open" metrics.
const openCondition = `status_code NOT IN ('RESOLVED','CLOSED')`

// Dashboard aggregates PRD §14 metrics over open incidents unless noted.
type Dashboard struct {
	OpenTotal               int            `json:"open_total"`
	ByStatus                map[string]int `json:"by_status"`
	BySeverity              map[string]int `json:"by_severity"`
	ByPriority              map[string]int `json:"by_priority"`
	ByApplication           []NamedCount   `json:"by_application"`
	ByTeam                  []NamedCount   `json:"by_team"`
	MyOpen                  int            `json:"my_open"`
	AvgHoursCreateToAssign  *float64       `json:"avg_hours_create_to_assign"`
	AvgHoursCreateToResolve *float64       `json:"avg_hours_create_to_resolve"`
	AvgHoursCreateToClose   *float64       `json:"avg_hours_create_to_close"`
	ReopenRate              *float64       `json:"reopen_rate"`
	VerificationFailureRate *float64       `json:"verification_failure_rate"`
}

// NamedCount is an id/name/count bucket.
type NamedCount struct {
	ID    *string `json:"id"`
	Name  string  `json:"name"`
	Count int     `json:"count"`
}

func codeCounts(ctx context.Context, pool *pgxpool.Pool, column string) (map[string]int, error) {
	out := map[string]int{}
	rows, err := pool.Query(ctx, fmt.Sprintf(
		`SELECT %s, COUNT(*) FROM trans_incident WHERE %s GROUP BY %s`, column, openCondition, column))
	if err != nil {
		return nil, fmt.Errorf("dashboard %s: %w", column, err)
	}
	defer rows.Close()
	for rows.Next() {
		var code string
		var n int
		if err := rows.Scan(&code, &n); err != nil {
			return nil, fmt.Errorf("scan dashboard %s: %w", column, err)
		}
		out[code] = n
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dashboard %s: %w", column, err)
	}
	return out, nil
}

func avgHours(ctx context.Context, pool *pgxpool.Pool, query string, args ...any) (*float64, error) {
	var v *float64
	if err := pool.QueryRow(ctx, query, args...).Scan(&v); err != nil {
		return nil, fmt.Errorf("dashboard avg: %w", err)
	}
	return v, nil
}

// GetDashboard computes all metrics in a handful of indexed aggregate queries.
func (r *Repository) GetDashboard(ctx context.Context, actorID string) (*Dashboard, error) {
	d := &Dashboard{
		ByStatus: map[string]int{}, BySeverity: map[string]int{}, ByPriority: map[string]int{},
		ByApplication: []NamedCount{}, ByTeam: []NamedCount{},
	}
	pool := r.pool
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM trans_incident WHERE `+openCondition).Scan(&d.OpenTotal); err != nil {
		return nil, fmt.Errorf("dashboard open total: %w", err)
	}
	var err error
	if d.ByStatus, err = codeCounts(ctx, pool, "status_code"); err != nil {
		return nil, err
	}
	if d.BySeverity, err = codeCounts(ctx, pool, "severity_code"); err != nil {
		return nil, err
	}
	if d.ByPriority, err = codeCounts(ctx, pool, "priority_code"); err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `
		SELECT i.application_id, COALESCE(a.name, '(tanpa aplikasi)'), COUNT(*)
		FROM trans_incident i LEFT JOIN master_application a ON a.id = i.application_id
		WHERE `+openCondition+` GROUP BY i.application_id, a.name ORDER BY COUNT(*) DESC`)
	if err != nil {
		return nil, fmt.Errorf("dashboard by application: %w", err)
	}
	for rows.Next() {
		var b NamedCount
		if err := rows.Scan(&b.ID, &b.Name, &b.Count); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan by application: %w", err)
		}
		d.ByApplication = append(d.ByApplication, b)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate by application: %w", err)
	}
	rows, err = pool.Query(ctx, `
		SELECT i.current_team_id, COALESCE(t.name, '(tanpa team)'), COUNT(*)
		FROM trans_incident i LEFT JOIN master_team t ON t.id = i.current_team_id
		WHERE `+openCondition+` GROUP BY i.current_team_id, t.name ORDER BY COUNT(*) DESC`)
	if err != nil {
		return nil, fmt.Errorf("dashboard by team: %w", err)
	}
	for rows.Next() {
		var b NamedCount
		if err := rows.Scan(&b.ID, &b.Name, &b.Count); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan by team: %w", err)
		}
		d.ByTeam = append(d.ByTeam, b)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate by team: %w", err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM trans_incident WHERE `+openCondition+`
		AND current_pic_id = $1`, actorID).Scan(&d.MyOpen); err != nil {
		return nil, fmt.Errorf("dashboard my open: %w", err)
	}
	if d.AvgHoursCreateToAssign, err = avgHours(ctx, pool, `
		SELECT AVG(EXTRACT(EPOCH FROM (a.assigned_at - i.created_at)) / 3600)
		FROM trans_incident i JOIN (
			SELECT incident_id, MIN(assigned_at) AS assigned_at
			FROM trans_incident_assignment GROUP BY incident_id
		) a ON a.incident_id = i.id`); err != nil {
		return nil, err
	}
	if d.AvgHoursCreateToResolve, err = avgHours(ctx, pool, `
		SELECT AVG(EXTRACT(EPOCH FROM (resolved_at - created_at)) / 3600)
		FROM trans_incident WHERE resolved_at IS NOT NULL`); err != nil {
		return nil, err
	}
	if d.AvgHoursCreateToClose, err = avgHours(ctx, pool, `
		SELECT AVG(EXTRACT(EPOCH FROM (closed_at - created_at)) / 3600)
		FROM trans_incident WHERE closed_at IS NOT NULL`); err != nil {
		return nil, err
	}
	if d.ReopenRate, err = avgHours(ctx, pool, `
		SELECT CASE WHEN COUNT(*) = 0 THEN NULL
			ELSE (SELECT COUNT(DISTINCT incident_id) FROM trans_incident_activity WHERE type = 'reopen')::float / COUNT(*) END
		FROM trans_incident`); err != nil {
		return nil, err
	}
	if d.VerificationFailureRate, err = avgHours(ctx, pool, `
		SELECT CASE WHEN COUNT(*) = 0 THEN NULL
			ELSE SUM(CASE WHEN result = 'FAIL' THEN 1 ELSE 0 END)::float / COUNT(*) END
		FROM trans_incident_verification`); err != nil {
		return nil, err
	}
	return d, nil
}
