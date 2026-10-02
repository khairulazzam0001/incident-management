package repository

import (
	"context"
	"fmt"

	"github.com/khairulazzam0001/incident-management/backend/internal/model"
)

// activeChangeCondition matches non-terminal changes.
const activeChangeCondition = `status_code NOT IN ('CLOSED','REJECTED','CANCELLED')`

// ChangeSummary aggregates PRD_Change_Management.md §14 metrics. Rates are
// nil when there is no data yet.
type ChangeSummary struct {
	ActiveTotal                int             `json:"active_total"`
	ByStatus                   map[string]int  `json:"by_status"`
	ByType                     map[string]int  `json:"by_type"`
	ByRisk                     map[string]int  `json:"by_risk"`
	Upcoming                   []*model.Change `json:"upcoming"`
	SuccessRate                *float64        `json:"success_rate"`
	FailureRate                *float64        `json:"failure_rate"`
	EmergencyRatio             *float64        `json:"emergency_ratio"`
	AvgHoursSubmitToApprove    *float64        `json:"avg_hours_submit_to_approve"`
	AvgHoursApproveToImplement *float64        `json:"avg_hours_approve_to_implement"`
	PIRCompletionRate          *float64        `json:"pir_completion_rate"`
}

func (r *Repository) changeCounts(ctx context.Context, column, where string) (map[string]int, error) {
	out := map[string]int{}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(
		`SELECT %s, COUNT(*) FROM trans_change WHERE %s GROUP BY %s`, column, where, column))
	if err != nil {
		return nil, fmt.Errorf("change summary %s: %w", column, err)
	}
	defer rows.Close()
	for rows.Next() {
		var code string
		var n int
		if err := rows.Scan(&code, &n); err != nil {
			return nil, fmt.Errorf("scan change summary %s: %w", column, err)
		}
		out[code] = n
	}
	return out, rows.Err()
}

// GetChangeSummary computes the change dashboard in a few aggregate queries.
func (r *Repository) GetChangeSummary(ctx context.Context) (*ChangeSummary, error) {
	sum := &ChangeSummary{}
	var err error
	if sum.ByStatus, err = r.changeCounts(ctx, "status_code", "TRUE"); err != nil {
		return nil, err
	}
	if sum.ByType, err = r.changeCounts(ctx, "type_code", activeChangeCondition); err != nil {
		return nil, err
	}
	if sum.ByRisk, err = r.changeCounts(ctx, "risk_code", activeChangeCondition); err != nil {
		return nil, err
	}
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM trans_change WHERE `+activeChangeCondition).
		Scan(&sum.ActiveTotal); err != nil {
		return nil, fmt.Errorf("change summary active: %w", err)
	}

	rows, err := r.pool.Query(ctx, `SELECT `+changeColumns+` FROM trans_change
		WHERE status_code = 'SCHEDULED' AND planned_start >= now()
			AND planned_start < now() + INTERVAL '7 days'
		ORDER BY planned_start LIMIT 10`)
	if err != nil {
		return nil, fmt.Errorf("change summary upcoming: %w", err)
	}
	sum.Upcoming = []*model.Change{}
	for rows.Next() {
		c, err := scanChange(rows)
		if err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan upcoming: %w", err)
		}
		sum.Upcoming = append(sum.Upcoming, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate upcoming: %w", err)
	}

	metrics := []struct {
		dest  **float64
		query string
	}{
		{&sum.SuccessRate, `
			SELECT CASE WHEN COUNT(*) = 0 THEN NULL
				ELSE SUM(CASE WHEN outcome = 'SUCCESS' THEN 1 ELSE 0 END)::float / COUNT(*) END
			FROM trans_change WHERE status_code = 'CLOSED'`},
		// Gagal = outcome FAILED/ROLLED_BACK atau punya incident CAUSED_BY (§14).
		{&sum.FailureRate, `
			SELECT CASE WHEN COUNT(*) = 0 THEN NULL
				ELSE SUM(CASE WHEN outcome IN ('FAILED','ROLLED_BACK') OR EXISTS (
					SELECT 1 FROM trans_change_incident l
					WHERE l.change_id = c.id AND l.relation = 'CAUSED_BY') THEN 1 ELSE 0 END)::float / COUNT(*) END
			FROM trans_change c WHERE status_code = 'CLOSED'`},
		{&sum.EmergencyRatio, `
			SELECT CASE WHEN COUNT(*) = 0 THEN NULL
				ELSE SUM(CASE WHEN type_code = 'EMERGENCY' THEN 1 ELSE 0 END)::float / COUNT(*) END
			FROM trans_change WHERE status_code <> 'DRAFT'`},
		{&sum.AvgHoursSubmitToApprove, `
			SELECT AVG(EXTRACT(EPOCH FROM (ap.at - sb.at)) / 3600) FROM
				(SELECT change_id, MIN(created_at) AS at FROM trans_change_activity
					WHERE to_status = 'SUBMITTED' GROUP BY change_id) sb
				JOIN (SELECT change_id, MIN(created_at) AS at FROM trans_change_activity
					WHERE to_status = 'APPROVED' GROUP BY change_id) ap ON ap.change_id = sb.change_id`},
		{&sum.AvgHoursApproveToImplement, `
			SELECT AVG(EXTRACT(EPOCH FROM (c.actual_start - ap.at)) / 3600)
			FROM trans_change c JOIN (SELECT change_id, MIN(created_at) AS at FROM trans_change_activity
				WHERE to_status = 'APPROVED' GROUP BY change_id) ap ON ap.change_id = c.id
			WHERE c.actual_start IS NOT NULL`},
		{&sum.PIRCompletionRate, `
			SELECT CASE WHEN COUNT(*) = 0 THEN NULL
				ELSE SUM(CASE WHEN review_notes <> '' THEN 1 ELSE 0 END)::float / COUNT(*) END
			FROM trans_change WHERE status_code = 'CLOSED'
				AND (type_code = 'EMERGENCY' OR outcome IS DISTINCT FROM 'SUCCESS')`},
	}
	for _, m := range metrics {
		if *m.dest, err = avgHours(ctx, r.pool, m.query); err != nil {
			return nil, err
		}
	}
	return sum, nil
}

// ListRecentChanges returns changes implemented on the same application +
// environment within the last window hours (triage context, §8.3).
func (r *Repository) ListRecentChanges(ctx context.Context, applicationID, environment string, hours int) ([]*model.Change, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+changeColumns+` FROM trans_change
		WHERE application_id = $1 AND environment_code = $2
			AND status_code IN ('IMPLEMENTING','REVIEWING','CLOSED')
			AND actual_start >= now() - make_interval(hours => $3)
		ORDER BY actual_start DESC LIMIT 20`, applicationID, environment, hours)
	if err != nil {
		return nil, fmt.Errorf("list recent changes: %w", err)
	}
	defer rows.Close()
	items := []*model.Change{}
	for rows.Next() {
		c, err := scanChange(rows)
		if err != nil {
			return nil, fmt.Errorf("scan recent change: %w", err)
		}
		items = append(items, c)
	}
	return items, rows.Err()
}
