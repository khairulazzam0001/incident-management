package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/khairulazzam0001/incident-management/backend/internal/model"
)

// InvestigationInput is the payload for POST /api/incidents/:id/investigations.
type InvestigationInput struct {
	Notes    string `json:"notes"`
	Findings string `json:"findings"`
}

// AddInvestigation records notes/findings (US-03/FR-05). Allowed from ASSIGNED
// (moves to INVESTIGATING) or INVESTIGATING (appends). PIC or coordinator only.
func (s *IncidentService) AddInvestigation(ctx context.Context, actor *model.AuthUser, id string, in InvestigationInput) (*model.Investigation, error) {
	if err := requireIncidentID(id); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Notes) == "" && strings.TrimSpace(in.Findings) == "" {
		return nil, BadRequest("VALIDATION_ERROR", "Notes atau findings wajib diisi.", nil)
	}
	cur, err := s.repo.GetIncident(ctx, id)
	if err != nil {
		if isNotFound(err) {
			return nil, NotFound("Incident")
		}
		return nil, fmt.Errorf("get incident: %w", err)
	}
	if cur.Status != model.StatusAssigned && cur.Status != model.StatusInvestigating {
		return nil, Conflict("INVALID_TRANSITION",
			fmt.Sprintf("Investigation hanya dari status ASSIGNED/INVESTIGATING (saat ini %s).", cur.Status),
			map[string]string{"status": cur.Status})
	}
	if !canActOn(actor, cur) {
		return nil, Forbidden("FORBIDDEN_ACTION", "Hanya PIC atau koordinator yang boleh mencatat investigation.")
	}
	inv, err := s.repo.AddInvestigation(ctx, id, actor.ID, in.Notes, in.Findings, cur.Status, model.StatusInvestigating)
	if err != nil {
		return nil, fmt.Errorf("add investigation: %w", err)
	}
	return inv, nil
}

// Investigations returns the investigation records or 404.
func (s *IncidentService) Investigations(ctx context.Context, id string) ([]*model.Investigation, error) {
	if err := requireIncidentID(id); err != nil {
		return nil, err
	}
	if _, err := s.repo.GetIncident(ctx, id); err != nil {
		if isNotFound(err) {
			return nil, NotFound("Incident")
		}
		return nil, fmt.Errorf("get incident: %w", err)
	}
	items, err := s.repo.ListInvestigations(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list investigations: %w", err)
	}
	return items, nil
}

// FixInput is the payload for POST /api/incidents/:id/fixes.
type FixInput struct {
	Description string `json:"description"`
	Reference   string `json:"reference"`
}

// AddFix records the fix (US-04/FR-06). Allowed from INVESTIGATING (moves to
// FIXING) or FIXING (appends). PIC or coordinator only.
func (s *IncidentService) AddFix(ctx context.Context, actor *model.AuthUser, id string, in FixInput) (*model.Fix, error) {
	if err := requireIncidentID(id); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Description) == "" {
		return nil, BadRequest("VALIDATION_ERROR", "Fix description wajib diisi.", map[string]string{"field": "description"})
	}
	cur, err := s.repo.GetIncident(ctx, id)
	if err != nil {
		if isNotFound(err) {
			return nil, NotFound("Incident")
		}
		return nil, fmt.Errorf("get incident: %w", err)
	}
	if cur.Status != model.StatusInvestigating && cur.Status != model.StatusFixing {
		return nil, Conflict("INVALID_TRANSITION",
			fmt.Sprintf("Fix hanya dari status INVESTIGATING/FIXING (saat ini %s).", cur.Status),
			map[string]string{"status": cur.Status})
	}
	if !canActOn(actor, cur) {
		return nil, Forbidden("FORBIDDEN_ACTION", "Hanya PIC atau koordinator yang boleh mencatat fix.")
	}
	f, err := s.repo.AddFix(ctx, id, actor.ID, in.Description, in.Reference, cur.Status, model.StatusFixing)
	if err != nil {
		return nil, fmt.Errorf("add fix: %w", err)
	}
	return f, nil
}

// Fixes returns the fix records or 404.
func (s *IncidentService) Fixes(ctx context.Context, id string) ([]*model.Fix, error) {
	if err := requireIncidentID(id); err != nil {
		return nil, err
	}
	if _, err := s.repo.GetIncident(ctx, id); err != nil {
		if isNotFound(err) {
			return nil, NotFound("Incident")
		}
		return nil, fmt.Errorf("get incident: %w", err)
	}
	items, err := s.repo.ListFixes(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list fixes: %w", err)
	}
	return items, nil
}

// VerificationInput is the payload for POST /api/incidents/:id/verification.
type VerificationInput struct {
	Result string `json:"result"`
	Reason string `json:"reason"`
}

// Verify records PASS/FAIL (US-05/FR-07). Only from VERIFYING:
// PASS → RESOLVED, FAIL → FIXING (reason mandatory). QA or coordinator only,
// and the PIC notifies on FAIL via activity + notification hook (Fase 3).
func (s *IncidentService) Verify(ctx context.Context, actor *model.AuthUser, id string, in VerificationInput) (*model.Verification, error) {
	if err := requireIncidentID(id); err != nil {
		return nil, err
	}
	if in.Result != model.VerificationPass && in.Result != model.VerificationFail {
		return nil, BadRequest("VALIDATION_ERROR", "Result harus PASS atau FAIL.", map[string]string{"field": "result"})
	}
	if in.Result == model.VerificationFail && strings.TrimSpace(in.Reason) == "" {
		return nil, BadRequest("VALIDATION_ERROR", "Reason wajib diisi untuk FAIL.", map[string]string{"field": "reason"})
	}
	if actor.Role != model.RoleQA && !model.IsCoordinator(actor.Role) {
		return nil, Forbidden("FORBIDDEN_VERIFY", "Hanya QA atau koordinator yang boleh verifikasi.")
	}
	cur, err := s.repo.GetIncident(ctx, id)
	if err != nil {
		if isNotFound(err) {
			return nil, NotFound("Incident")
		}
		return nil, fmt.Errorf("get incident: %w", err)
	}
	if cur.Status != model.StatusVerifying {
		return nil, Conflict("INVALID_TRANSITION",
			fmt.Sprintf("Verification hanya dari status VERIFYING (saat ini %s).", cur.Status),
			map[string]string{"status": cur.Status})
	}
	to := model.StatusResolved
	if in.Result == model.VerificationFail {
		to = model.StatusFixing
	}
	v, err := s.repo.AddVerification(ctx, id, actor.ID, in.Result, in.Reason, cur.Status, to)
	if err != nil {
		return nil, fmt.Errorf("add verification: %w", err)
	}
	return v, nil
}

// Verifications returns the verification records or 404.
func (s *IncidentService) Verifications(ctx context.Context, id string) ([]*model.Verification, error) {
	if err := requireIncidentID(id); err != nil {
		return nil, err
	}
	if _, err := s.repo.GetIncident(ctx, id); err != nil {
		if isNotFound(err) {
			return nil, NotFound("Incident")
		}
		return nil, fmt.Errorf("get incident: %w", err)
	}
	items, err := s.repo.ListVerifications(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list verifications: %w", err)
	}
	return items, nil
}

// Close moves RESOLVED → CLOSED (US-06). Coordinators only. closed_at and
// closed_by are stamped by the repository.
func (s *IncidentService) Close(ctx context.Context, actor *model.AuthUser, id string) (*model.Incident, error) {
	if err := requireIncidentID(id); err != nil {
		return nil, err
	}
	if !model.IsCoordinator(actor.Role) {
		return nil, Forbidden("FORBIDDEN_CLOSE", "Hanya koordinator yang boleh close incident.")
	}
	cur, err := s.repo.GetIncident(ctx, id)
	if err != nil {
		if isNotFound(err) {
			return nil, NotFound("Incident")
		}
		return nil, fmt.Errorf("get incident: %w", err)
	}
	if cur.Status != model.StatusResolved {
		return nil, Conflict("INVALID_TRANSITION",
			fmt.Sprintf("Close hanya dari status RESOLVED (saat ini %s).", cur.Status),
			map[string]string{"status": cur.Status})
	}
	updated, err := s.repo.UpdateStatus(ctx, id, cur.Status, model.StatusClosed, actor.ID)
	if err != nil {
		return nil, fmt.Errorf("close incident: %w", err)
	}
	return updated, nil
}

// ReopenInput is the payload for POST /api/incidents/:id/reopen.
type ReopenInput struct {
	Reason string `json:"reason"`
}

// Reopen moves RESOLVED/CLOSED back to INVESTIGATING (FR-12, permission
// khusus: koordinator saja, reason wajib).
func (s *IncidentService) Reopen(ctx context.Context, actor *model.AuthUser, id string, in ReopenInput) (*model.Incident, error) {
	if err := requireIncidentID(id); err != nil {
		return nil, err
	}
	if !model.IsCoordinator(actor.Role) {
		return nil, Forbidden("FORBIDDEN_REOPEN", "Hanya koordinator yang boleh reopen incident.")
	}
	if strings.TrimSpace(in.Reason) == "" {
		return nil, BadRequest("VALIDATION_ERROR", "Reason wajib diisi untuk reopen.", map[string]string{"field": "reason"})
	}
	cur, err := s.repo.GetIncident(ctx, id)
	if err != nil {
		if isNotFound(err) {
			return nil, NotFound("Incident")
		}
		return nil, fmt.Errorf("get incident: %w", err)
	}
	if cur.Status != model.StatusResolved && cur.Status != model.StatusClosed {
		return nil, Conflict("INVALID_TRANSITION",
			fmt.Sprintf("Reopen hanya dari status RESOLVED/CLOSED (saat ini %s).", cur.Status),
			map[string]string{"status": cur.Status})
	}
	updated, err := s.repo.Reopen(ctx, id, cur.Status, model.StatusInvestigating, actor.ID, in.Reason)
	if err != nil {
		return nil, fmt.Errorf("reopen incident: %w", err)
	}
	return updated, nil
}
