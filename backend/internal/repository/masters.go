package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/khairulazzam0001/incident-management/backend/internal/model"
)

// CreateApplication inserts a master application; duplicate code → 409 upstream.
func (r *Repository) CreateApplication(ctx context.Context, code, name string) (*model.MasterIDItem, error) {
	it := &model.MasterIDItem{}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO master_application (code, name) VALUES ($1,$2)
		RETURNING id, code, name`, code, name).Scan(&it.ID, &it.Code, &it.Name)
	if err != nil {
		return nil, err
	}
	return it, nil
}

// CreateTeam inserts a master team.
func (r *Repository) CreateTeam(ctx context.Context, code, name string) (*model.MasterIDItem, error) {
	it := &model.MasterIDItem{}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO master_team (code, name) VALUES ($1,$2)
		RETURNING id, code, name`, code, name).Scan(&it.ID, &it.Code, &it.Name)
	if err != nil {
		return nil, err
	}
	return it, nil
}

// DeleteApplication removes an application only when unused.
// Returns (used, found).
func (r *Repository) DeleteApplication(ctx context.Context, id string) (bool, bool, error) {
	var n int
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM trans_incident WHERE application_id = $1`, id).Scan(&n); err != nil {
		return false, false, fmt.Errorf("check application usage: %w", err)
	}
	if n > 0 {
		return true, true, nil
	}
	tag, err := r.pool.Exec(ctx, `DELETE FROM master_application WHERE id = $1`, id)
	if err != nil {
		return false, false, fmt.Errorf("delete application: %w", err)
	}
	return false, tag.RowsAffected() == 1, nil
}

// DeleteTeam removes a team only when unused. Returns (used, found).
func (r *Repository) DeleteTeam(ctx context.Context, id string) (bool, bool, error) {
	var n int
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM trans_incident WHERE current_team_id = $1
			OR EXISTS (SELECT 1 FROM trans_incident_assignment WHERE team_id = $1 AND incident_id = trans_incident.id)`,
		id).Scan(&n); err != nil {
		return false, false, fmt.Errorf("check team usage: %w", err)
	}
	if n > 0 {
		return true, true, nil
	}
	var member int
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM master_user WHERE team_id = $1`, id).Scan(&member); err != nil {
		return false, false, fmt.Errorf("check team members: %w", err)
	}
	if member > 0 {
		return true, true, nil
	}
	tag, err := r.pool.Exec(ctx, `DELETE FROM master_team WHERE id = $1`, id)
	if err != nil {
		return false, false, fmt.Errorf("delete team: %w", err)
	}
	return false, tag.RowsAffected() == 1, nil
}

// IsUniqueViolation reports Postgres unique violations (SQLSTATE 23505).
func IsUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "23505")
}
