package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/khairulazzam0001/incident-management/backend/internal/model"
	"github.com/khairulazzam0001/incident-management/backend/internal/repository"
)

// Change Management CM-2 (PRD_Change_Management.md §7–§9, CM-04–CM-06, CM-08).

// scheduleSkew tolerates small client/server clock differences (§9).
const scheduleSkew = 5 * time.Minute

// canImplement: the implementer (or the requester when none is set) or a ManagerLead.
func canImplement(actor *model.AuthUser, c *model.Change) bool {
	if actor.Role == model.RoleManagerLead {
		return true
	}
	if c.ImplementerID != nil {
		return *c.ImplementerID == actor.ID
	}
	return c.RequesterID == actor.ID
}

func canSchedule(actor *model.AuthUser, c *model.Change) bool {
	return isRequesterOrManager(actor, c) || (c.ImplementerID != nil && *c.ImplementerID == actor.ID)
}

func canLinkChange(role string) bool {
	switch role {
	case model.RoleHelpDesk, model.RoleSystemAnalyst, model.RoleDeveloper, model.RoleDevOps, model.RoleManagerLead:
		return true
	default:
		return false
	}
}

func parseTimeField(field, v string) (time.Time, *Error) {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(v))
	if err != nil {
		return time.Time{}, BadRequest("VALIDATION_ERROR", "Format waktu harus RFC3339 (cth 2026-10-02T21:00:00+07:00).", map[string]string{"field": field})
	}
	return t.UTC(), nil
}

// ScheduleInput is the payload for POST /api/changes/:id/schedule.
type ScheduleInput struct {
	PlannedStart string `json:"planned_start"`
	PlannedEnd   string `json:"planned_end"`
}

// ScheduleResult carries the scheduled change plus overlapping changes
// (warning only, §9).
type ScheduleResult struct {
	Change    *model.Change   `json:"change"`
	Conflicts []*model.Change `json:"conflicts"`
}

// ScheduleChange sets the planned window: APPROVED → SCHEDULED, or a
// reschedule SCHEDULED → SCHEDULED (CM-04).
func (s *IncidentService) ScheduleChange(ctx context.Context, actor *model.AuthUser, id string, in ScheduleInput) (*ScheduleResult, error) {
	start, verr := parseTimeField("planned_start", in.PlannedStart)
	if verr != nil {
		return nil, verr
	}
	end, verr := parseTimeField("planned_end", in.PlannedEnd)
	if verr != nil {
		return nil, verr
	}
	if !end.After(start) {
		return nil, BadRequest("VALIDATION_ERROR", "planned_end harus setelah planned_start.", map[string]string{"field": "planned_end"})
	}
	if start.Before(time.Now().UTC().Add(-scheduleSkew)) {
		return nil, BadRequest("VALIDATION_ERROR", "planned_start tidak boleh di masa lalu.", map[string]string{"field": "planned_start"})
	}
	c, err := s.loadChange(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	if !canSchedule(actor, c) {
		return nil, Forbidden("FORBIDDEN_ACTION", "Hanya requester, implementer, atau Manager yang boleh menjadwalkan change.")
	}
	if !model.CanChangeTransition(c.Type, c.Status, model.ChangeScheduled) {
		return nil, invalidChangeTransition(c, model.ChangeScheduled)
	}
	payload := map[string]any{"planned_start": start, "planned_end": end}
	if c.PlannedStart != nil {
		payload["previous_start"] = *c.PlannedStart
		payload["previous_end"] = c.PlannedEnd
	}
	updated, err := s.repo.ApplyChangeTransitions(ctx, id, repository.ChangeTransition{
		From: c.Status, To: model.ChangeScheduled, ActorID: &actor.ID,
		ActivityType: model.ChangeActivitySchedule, Payload: payload,
		Fields: map[string]any{"planned_start": start, "planned_end": end},
	})
	if err != nil {
		return nil, staleChange(err)
	}
	conflicts, err := s.repo.FindScheduleConflicts(ctx, updated, start, end)
	if err != nil {
		return nil, fmt.Errorf("schedule conflicts: %w", err)
	}
	s.changeFanout(ctx, updated, actor, model.NotifChangeScheduled)
	return &ScheduleResult{Change: updated, Conflicts: conflicts}, nil
}

// StartChange moves SCHEDULED (or EMERGENCY APPROVED) → IMPLEMENTING and
// records actual_start (CM-05).
func (s *IncidentService) StartChange(ctx context.Context, actor *model.AuthUser, id string) (*model.Change, error) {
	c, err := s.loadChange(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	if !canImplement(actor, c) {
		return nil, Forbidden("FORBIDDEN_ACTION", "Hanya implementer atau Manager yang boleh memulai implementasi.")
	}
	if !model.CanChangeTransition(c.Type, c.Status, model.ChangeImplementing) {
		return nil, invalidChangeTransition(c, model.ChangeImplementing)
	}
	updated, err := s.repo.ApplyChangeTransitions(ctx, id, repository.ChangeTransition{
		From: c.Status, To: model.ChangeImplementing, ActorID: &actor.ID,
		ActivityType: model.ChangeActivityStart,
		Fields:       map[string]any{"actual_start": time.Now().UTC()},
	})
	if err != nil {
		return nil, staleChange(err)
	}
	s.changeFanout(ctx, updated, actor, model.NotifChangeStarted)
	return updated, nil
}

// CompleteInput is the payload for POST /api/changes/:id/complete.
type CompleteInput struct {
	Outcome      string `json:"outcome"`
	OutcomeNotes string `json:"outcome_notes"`
}

// CompleteChange moves IMPLEMENTING → REVIEWING with an outcome (CM-05).
func (s *IncidentService) CompleteChange(ctx context.Context, actor *model.AuthUser, id string, in CompleteInput) (*model.Change, error) {
	in.OutcomeNotes = strings.TrimSpace(in.OutcomeNotes)
	switch in.Outcome {
	case model.OutcomeSuccess:
	case model.OutcomeFailed, model.OutcomeRolledBack:
		if in.OutcomeNotes == "" {
			return nil, BadRequest("VALIDATION_ERROR", "Outcome notes wajib untuk FAILED/ROLLED_BACK.", map[string]string{"field": "outcome_notes"})
		}
	default:
		return nil, BadRequest("VALIDATION_ERROR", "Outcome harus SUCCESS, FAILED, atau ROLLED_BACK.", map[string]string{"field": "outcome"})
	}
	c, err := s.loadChange(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	if !canImplement(actor, c) {
		return nil, Forbidden("FORBIDDEN_ACTION", "Hanya implementer atau Manager yang boleh menyelesaikan implementasi.")
	}
	if !model.CanChangeTransition(c.Type, c.Status, model.ChangeReviewing) {
		return nil, invalidChangeTransition(c, model.ChangeReviewing)
	}
	updated, err := s.repo.ApplyChangeTransitions(ctx, id, repository.ChangeTransition{
		From: c.Status, To: model.ChangeReviewing, ActorID: &actor.ID,
		ActivityType: model.ChangeActivityComplete,
		Payload:      map[string]any{"outcome": in.Outcome, "notes": in.OutcomeNotes},
		Fields: map[string]any{
			"actual_end": time.Now().UTC(), "outcome": in.Outcome, "outcome_notes": in.OutcomeNotes,
		},
	})
	if err != nil {
		return nil, staleChange(err)
	}
	if in.Outcome != model.OutcomeSuccess {
		s.changeFanout(ctx, updated, actor, model.NotifChangeFailed)
	}
	return updated, nil
}

// CloseChange moves REVIEWING → CLOSED; PIR notes are mandatory for
// EMERGENCY changes or non-successful outcomes (CM-06).
func (s *IncidentService) CloseChange(ctx context.Context, actor *model.AuthUser, id, reviewNotes string) (*model.Change, error) {
	reviewNotes = strings.TrimSpace(reviewNotes)
	c, err := s.loadChange(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	if actor.Role != model.RoleSystemAnalyst && actor.Role != model.RoleManagerLead {
		return nil, Forbidden("FORBIDDEN_ACTION", "Hanya System Analyst atau Manager yang boleh close change.")
	}
	if !model.CanChangeTransition(c.Type, c.Status, model.ChangeClosed) {
		return nil, invalidChangeTransition(c, model.ChangeClosed)
	}
	if reviewNotes == "" && model.RequiresPIR(c) {
		return nil, BadRequest("VALIDATION_ERROR", "Post-implementation review wajib untuk emergency atau outcome tidak SUCCESS.", map[string]string{"field": "review_notes"})
	}
	updated, err := s.repo.ApplyChangeTransitions(ctx, id, repository.ChangeTransition{
		From: c.Status, To: model.ChangeClosed, ActorID: &actor.ID,
		ActivityType: model.ChangeActivityClose,
		Payload:      map[string]any{"review_notes": reviewNotes},
		Fields: map[string]any{
			"review_notes": reviewNotes, "closed_at": time.Now().UTC(), "closed_by": actor.ID,
		},
	})
	if err != nil {
		return nil, staleChange(err)
	}
	s.changeFanout(ctx, updated, actor, model.NotifChangeClosed)
	return updated, nil
}

// LinkInput is the payload for POST /api/changes/:id/incidents.
type LinkInput struct {
	IncidentID string `json:"incident_id"`
	Relation   string `json:"relation"`
}

// resolveLink validates role, relation, and loads both sides of a link.
func (s *IncidentService) resolveLink(ctx context.Context, actor *model.AuthUser, changeID, incidentID, relation string) (*model.Change, *model.Incident, error) {
	if !canLinkChange(actor.Role) {
		return nil, nil, Forbidden("FORBIDDEN_ACTION", "Role Anda tidak boleh mengelola link change–incident.")
	}
	if relation != model.RelationFixFor && relation != model.RelationCausedBy {
		return nil, nil, BadRequest("VALIDATION_ERROR", "Relation harus FIX_FOR atau CAUSED_BY.", map[string]string{"field": "relation"})
	}
	c, err := s.loadChange(ctx, actor, changeID)
	if err != nil {
		return nil, nil, err
	}
	if !validUUID(incidentID) {
		return nil, nil, BadRequest("VALIDATION_ERROR", "Incident tidak valid.", map[string]string{"field": "incident_id"})
	}
	in, err := s.repo.GetIncident(ctx, incidentID)
	if err != nil {
		if isNotFound(err) {
			return nil, nil, NotFound("Incident")
		}
		return nil, nil, fmt.Errorf("get incident: %w", err)
	}
	return c, in, nil
}

// LinkIncident links an incident to a change (CM-08). CAUSED_BY is only
// allowed once the change has been implemented.
func (s *IncidentService) LinkIncident(ctx context.Context, actor *model.AuthUser, changeID string, in LinkInput) ([]*model.ChangeIncidentLink, error) {
	c, inc, err := s.resolveLink(ctx, actor, changeID, strings.TrimSpace(in.IncidentID), in.Relation)
	if err != nil {
		return nil, err
	}
	switch c.Status {
	case model.ChangeRejected, model.ChangeCancelled:
		return nil, Conflict("INVALID_LINK", "Change yang rejected/cancelled tidak bisa ditautkan.", nil)
	}
	if in.Relation == model.RelationCausedBy {
		switch c.Status {
		case model.ChangeImplementing, model.ChangeReviewing, model.ChangeClosed:
		default:
			return nil, Conflict("INVALID_LINK", "CAUSED_BY hanya untuk change yang sudah diimplementasikan.", map[string]string{"status": c.Status})
		}
	}
	if err := s.repo.LinkChangeIncident(ctx, c, inc, in.Relation, actor.ID); err != nil {
		if errors.Is(err, repository.ErrDuplicateLink) {
			return nil, Conflict("DUPLICATE_LINK", "Incident sudah ditautkan dengan relasi ini.", nil)
		}
		return nil, fmt.Errorf("link incident: %w", err)
	}
	if in.Relation == model.RelationCausedBy {
		s.changeFanout(ctx, c, actor, model.NotifChangeCausedIncident)
	}
	return s.ChangeIncidents(ctx, actor, changeID)
}

// UnlinkIncident removes a change ↔ incident link (CM-08).
func (s *IncidentService) UnlinkIncident(ctx context.Context, actor *model.AuthUser, changeID, incidentID, relation string) error {
	c, inc, err := s.resolveLink(ctx, actor, changeID, incidentID, relation)
	if err != nil {
		return err
	}
	if err := s.repo.UnlinkChangeIncident(ctx, c, inc, relation, actor.ID); err != nil {
		if isNotFound(err) {
			return NotFound("Link")
		}
		return fmt.Errorf("unlink incident: %w", err)
	}
	return nil
}

// requireProductionFix enforces the production gate (§8.2, Q3): a production
// incident may only enter VERIFYING once a FIX_FOR change is implemented.
func (s *IncidentService) requireProductionFix(ctx context.Context, in *model.Incident, to string) error {
	if in.Status != model.StatusFixing || to != model.StatusVerifying ||
		in.Environment == nil || *in.Environment != "production" {
		return nil
	}
	ok, err := s.repo.HasImplementedFix(ctx, in.ID)
	if err != nil {
		return err
	}
	if !ok {
		return Conflict("CHANGE_REQUIRED",
			"Incident production butuh change request FIX_FOR yang sudah diimplementasikan sebelum VERIFYING.", nil)
	}
	return nil
}

// recentChangeHours is the triage look-back window for recent changes (§8.3).
const recentChangeHours = 72

// ChangeSummary returns the change dashboard (§14).
func (s *IncidentService) ChangeSummary(ctx context.Context, actor *model.AuthUser) (*repository.ChangeSummary, error) {
	if err := requireChangeViewer(actor); err != nil {
		return nil, err
	}
	sum, err := s.repo.GetChangeSummary(ctx)
	if err != nil {
		return nil, fmt.Errorf("change summary: %w", err)
	}
	return sum, nil
}

// RecentChanges returns changes implemented on the incident's application +
// environment in the last 72 hours (§8.3). Empty when either is unset.
func (s *IncidentService) RecentChanges(ctx context.Context, actor *model.AuthUser, incidentID string) ([]*model.Change, error) {
	if err := requireChangeViewer(actor); err != nil {
		return nil, err
	}
	in, err := s.Get(ctx, actor, incidentID)
	if err != nil {
		return nil, err
	}
	if in.ApplicationID == nil || in.Environment == nil {
		return []*model.Change{}, nil
	}
	items, err := s.repo.ListRecentChanges(ctx, *in.ApplicationID, *in.Environment, recentChangeHours)
	if err != nil {
		return nil, fmt.Errorf("recent changes: %w", err)
	}
	return items, nil
}
