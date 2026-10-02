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

// Change Management CM-1 (PRD_Change_Management.md §5–§7, §10).

func requireChangeID(id string) *Error {
	if !validUUID(id) {
		return BadRequest("INVALID_ID", "ID change tidak valid.", nil)
	}
	return nil
}

func requireChangeViewer(actor *model.AuthUser) *Error {
	if !model.CanViewChanges(actor.Role) {
		return Forbidden("FORBIDDEN_READ", "Anda tidak berhak mengakses Change Management.")
	}
	return nil
}

// loadChange returns the change or 404 after the view-permission check.
func (s *IncidentService) loadChange(ctx context.Context, actor *model.AuthUser, id string) (*model.Change, error) {
	if err := requireChangeViewer(actor); err != nil {
		return nil, err
	}
	if err := requireChangeID(id); err != nil {
		return nil, err
	}
	c, err := s.repo.GetChange(ctx, id)
	if err != nil {
		if isNotFound(err) {
			return nil, NotFound("Change")
		}
		return nil, fmt.Errorf("get change: %w", err)
	}
	return c, nil
}

func isRequesterOrManager(actor *model.AuthUser, c *model.Change) bool {
	return actor.ID == c.RequesterID || actor.Role == model.RoleManagerLead
}

func invalidChangeTransition(c *model.Change, to string) *Error {
	return Conflict("INVALID_TRANSITION",
		fmt.Sprintf("Transisi %s → %s tidak diizinkan.", c.Status, to),
		map[string]string{"from": c.Status, "to": to})
}

func staleChange(err error) error {
	if errors.Is(err, repository.ErrStaleStatus) {
		return Conflict("STALE_STATUS", "Status change sudah berubah. Muat ulang lalu coba lagi.", nil)
	}
	return err
}

func blankToNil(p *string) *string {
	if p == nil || strings.TrimSpace(*p) == "" {
		return nil
	}
	v := strings.TrimSpace(*p)
	return &v
}

// validateChangeInput normalizes and checks the fields required to store a
// draft. Plan completeness is checked at submit (validateSubmittable).
func (s *IncidentService) validateChangeInput(ctx context.Context, in *model.ChangeInput) error {
	in.Title = strings.TrimSpace(in.Title)
	in.Environment = strings.TrimSpace(in.Environment)
	in.ApplicationID = strings.TrimSpace(in.ApplicationID)
	in.ImplementerID = blankToNil(in.ImplementerID)
	in.TeamID = blankToNil(in.TeamID)
	in.IncidentID = blankToNil(in.IncidentID)

	if len(in.Title) < 5 {
		return BadRequest("VALIDATION_ERROR", "Title minimal 5 karakter.", map[string]string{"field": "title"})
	}
	meta, err := s.repo.ListMeta(ctx)
	if err != nil {
		return fmt.Errorf("load masters: %w", err)
	}
	if !codesIn(in.Type, meta.ChangeTypes) {
		return BadRequest("VALIDATION_ERROR", "Type tidak dikenal.", map[string]string{"field": "type"})
	}
	if !codesIn(in.Risk, meta.ChangeRisks) {
		return BadRequest("VALIDATION_ERROR", "Risk tidak dikenal.", map[string]string{"field": "risk"})
	}
	if !codesIn(in.Environment, meta.Environments) {
		return BadRequest("VALIDATION_ERROR", "Environment wajib dipilih.", map[string]string{"field": "environment"})
	}
	appFound := false
	for _, a := range meta.Applications {
		if a.ID == in.ApplicationID {
			appFound = true
			break
		}
	}
	if !appFound {
		return BadRequest("VALIDATION_ERROR", "Application wajib dipilih.", map[string]string{"field": "application_id"})
	}
	if in.ImplementerID != nil {
		if !validUUID(*in.ImplementerID) {
			return BadRequest("VALIDATION_ERROR", "Implementer tidak valid.", map[string]string{"field": "implementer_id"})
		}
		u, err := s.repo.GetUserByID(ctx, *in.ImplementerID)
		if err != nil {
			if isNotFound(err) {
				return BadRequest("VALIDATION_ERROR", "Implementer tidak ditemukan.", map[string]string{"field": "implementer_id"})
			}
			return fmt.Errorf("get implementer: %w", err)
		}
		if !u.IsActive || !model.CanCreateChange(u.Role) {
			return BadRequest("VALIDATION_ERROR", "Implementer harus user aktif dengan role teknis.", map[string]string{"field": "implementer_id"})
		}
	}
	if in.TeamID != nil {
		if !validUUID(*in.TeamID) {
			return BadRequest("VALIDATION_ERROR", "Team tidak valid.", map[string]string{"field": "team_id"})
		}
		ok, err := s.repo.TeamExists(ctx, in.TeamID)
		if err != nil {
			return fmt.Errorf("check team: %w", err)
		}
		if !ok {
			return BadRequest("VALIDATION_ERROR", "Team tidak ditemukan.", map[string]string{"field": "team_id"})
		}
	}
	return nil
}

// validateSubmittable enforces the content rules before approval (§6, CM-01).
func validateSubmittable(c *model.Change) *Error {
	required := []struct{ field, value, label string }{
		{"description", c.Description, "Deskripsi"},
		{"implementation_plan", c.ImplementationPlan, "Implementation plan"},
		{"rollback_plan", c.RollbackPlan, "Rollback plan"},
	}
	for _, r := range required {
		if strings.TrimSpace(r.value) == "" {
			return BadRequest("VALIDATION_ERROR", r.label+" wajib diisi sebelum submit.", map[string]string{"field": r.field})
		}
	}
	if c.Risk == model.RiskHigh && strings.TrimSpace(c.TestPlan) == "" {
		return BadRequest("VALIDATION_ERROR", "Test plan wajib untuk risk HIGH.", map[string]string{"field": "test_plan"})
	}
	if c.Type == model.ChangeTypeStandard && c.Risk != model.RiskLow {
		return BadRequest("VALIDATION_ERROR", "Standard change harus berisiko LOW.", map[string]string{"field": "risk"})
	}
	return nil
}

// CreateChange stores a DRAFT change (CM-01). With incident_id the change is
// linked as FIX_FOR that incident.
func (s *IncidentService) CreateChange(ctx context.Context, actor *model.AuthUser, in model.ChangeInput) (*model.Change, error) {
	if !model.CanCreateChange(actor.Role) {
		return nil, Forbidden("FORBIDDEN_ACTION", "Role Anda tidak boleh membuat change request.")
	}
	if err := s.validateChangeInput(ctx, &in); err != nil {
		return nil, err
	}
	var incident *model.Incident
	if in.IncidentID != nil {
		if !validUUID(*in.IncidentID) {
			return nil, BadRequest("VALIDATION_ERROR", "Incident tidak valid.", map[string]string{"field": "incident_id"})
		}
		inc, err := s.repo.GetIncident(ctx, *in.IncidentID)
		if err != nil {
			if isNotFound(err) {
				return nil, BadRequest("VALIDATION_ERROR", "Incident tidak ditemukan.", map[string]string{"field": "incident_id"})
			}
			return nil, fmt.Errorf("get incident: %w", err)
		}
		incident = inc
	}
	no, err := s.repo.NextChangeNo(ctx)
	if err != nil {
		return nil, fmt.Errorf("generate change_no: %w", err)
	}
	created, err := s.repo.CreateChange(ctx, in, no, actor.ID, incident)
	if err != nil {
		return nil, fmt.Errorf("create change: %w", err)
	}
	return created, nil
}

// UpdateChange edits a DRAFT change. Requester or ManagerLead only.
func (s *IncidentService) UpdateChange(ctx context.Context, actor *model.AuthUser, id string, in model.ChangeInput) (*model.Change, error) {
	c, err := s.loadChange(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	if !isRequesterOrManager(actor, c) {
		return nil, Forbidden("FORBIDDEN_ACTION", "Hanya requester atau Manager yang boleh mengubah change.")
	}
	if c.Status != model.ChangeDraft {
		return nil, Conflict("NOT_EDITABLE", "Change hanya bisa diedit saat DRAFT.", map[string]string{"status": c.Status})
	}
	if err := s.validateChangeInput(ctx, &in); err != nil {
		return nil, err
	}
	updated, err := s.repo.UpdateDraftChange(ctx, id, in, actor.ID)
	if err != nil {
		return nil, staleChange(err)
	}
	return updated, nil
}

// GetChange returns one change or 404/403.
func (s *IncidentService) GetChange(ctx context.Context, actor *model.AuthUser, id string) (*model.Change, error) {
	return s.loadChange(ctx, actor, id)
}

// ListChanges validates filters and returns a page (CM-FR-02).
func (s *IncidentService) ListChanges(ctx context.Context, actor *model.AuthUser, f repository.ChangeFilter) ([]*model.Change, int, error) {
	if err := requireChangeViewer(actor); err != nil {
		return nil, 0, err
	}
	if f.Page <= 0 {
		f.Page = 1
	}
	if f.Limit <= 0 {
		f.Limit = 20
	}
	if f.Limit > 100 {
		return nil, 0, BadRequest("VALIDATION_ERROR", "Limit maksimal 100.", map[string]string{"field": "limit"})
	}
	if f.RequesterID == "me" {
		f.RequesterID = actor.ID
	}
	if f.ImplementerID == "me" {
		f.ImplementerID = actor.ID
	}
	for field, id := range map[string]string{
		"application_id": f.ApplicationID, "requester": f.RequesterID, "implementer": f.ImplementerID,
	} {
		if id != "" && !validUUID(id) {
			return nil, 0, BadRequest("VALIDATION_ERROR", "Filter tidak valid.", map[string]string{"field": field})
		}
	}
	for field, d := range map[string]string{"scheduled_from": f.ScheduledFrom, "scheduled_to": f.ScheduledTo} {
		if d == "" {
			continue
		}
		if _, err := time.Parse("2006-01-02", d); err != nil {
			return nil, 0, BadRequest("VALIDATION_ERROR", "Format tanggal YYYY-MM-DD.", map[string]string{"field": field})
		}
	}
	items, total, err := s.repo.ListChanges(ctx, f)
	if err != nil {
		return nil, 0, fmt.Errorf("list changes: %w", err)
	}
	return items, total, nil
}

// SubmitChange moves DRAFT → SUBMITTED (CM-02). STANDARD changes are
// pre-approved: SUBMITTED → APPROVED in the same transaction, actor = system.
func (s *IncidentService) SubmitChange(ctx context.Context, actor *model.AuthUser, id string) (*model.Change, error) {
	c, err := s.loadChange(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	if !isRequesterOrManager(actor, c) {
		return nil, Forbidden("FORBIDDEN_ACTION", "Hanya requester atau Manager yang boleh submit change.")
	}
	if !model.CanChangeTransition(c.Type, c.Status, model.ChangeSubmitted) {
		return nil, invalidChangeTransition(c, model.ChangeSubmitted)
	}
	if err := validateSubmittable(c); err != nil {
		return nil, err
	}
	steps := []repository.ChangeTransition{{
		From: model.ChangeDraft, To: model.ChangeSubmitted, ActorID: &actor.ID,
		ActivityType: model.ChangeActivityStatusChange,
	}}
	if c.Type == model.ChangeTypeStandard {
		steps = append(steps, repository.ChangeTransition{
			From: model.ChangeSubmitted, To: model.ChangeApproved,
			ActivityType: model.ChangeActivityApproval,
			Payload:      map[string]any{"decision": model.DecisionAutoApproved, "auto_approved": true},
			Approval:     &repository.ChangeApprovalRow{Decision: model.DecisionAutoApproved, Reason: "Standard change (pre-approved)."},
		})
	}
	updated, err := s.repo.ApplyChangeTransitions(ctx, id, steps...)
	if err != nil {
		return nil, staleChange(err)
	}
	return updated, nil
}

// ApprovalInput is the payload for POST /api/changes/:id/approval.
type ApprovalInput struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
}

// DecideChange records APPROVE / REJECT / REQUEST_CHANGES on a SUBMITTED
// change (CM-03). Single approver with segregation of duties.
func (s *IncidentService) DecideChange(ctx context.Context, actor *model.AuthUser, id string, in ApprovalInput) (*model.Change, error) {
	in.Reason = strings.TrimSpace(in.Reason)
	targets := map[string]struct{ to, decision string }{
		"APPROVE":         {model.ChangeApproved, model.DecisionApproved},
		"REJECT":          {model.ChangeRejected, model.DecisionRejected},
		"REQUEST_CHANGES": {model.ChangeDraft, model.DecisionChangesRequested},
	}
	t, ok := targets[in.Decision]
	if !ok {
		return nil, BadRequest("VALIDATION_ERROR", "Decision harus APPROVE, REJECT, atau REQUEST_CHANGES.", map[string]string{"field": "decision"})
	}
	if in.Decision != "APPROVE" && in.Reason == "" {
		return nil, BadRequest("VALIDATION_ERROR", "Reason wajib diisi untuk reject/request changes.", map[string]string{"field": "reason"})
	}
	c, err := s.loadChange(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	requester, err := s.repo.GetUserByID(ctx, c.RequesterID)
	if err != nil {
		return nil, fmt.Errorf("get requester: %w", err)
	}
	switch model.ApprovalDenial(actor.ID, actor.Role, requester.ID, requester.Role, c.Risk) {
	case "SELF_APPROVAL_FORBIDDEN":
		return nil, Forbidden("SELF_APPROVAL_FORBIDDEN", "Requester tidak boleh memutuskan change miliknya sendiri.")
	case "FORBIDDEN_APPROVAL":
		return nil, Forbidden("FORBIDDEN_APPROVAL", "Anda tidak berhak memutuskan change ini.")
	}
	if c.Status != model.ChangeSubmitted || !model.CanChangeTransition(c.Type, c.Status, t.to) {
		return nil, invalidChangeTransition(c, t.to)
	}
	updated, err := s.repo.ApplyChangeTransitions(ctx, id, repository.ChangeTransition{
		From: model.ChangeSubmitted, To: t.to, ActorID: &actor.ID,
		ActivityType: model.ChangeActivityApproval,
		Payload:      map[string]any{"decision": t.decision, "reason": in.Reason},
		Approval:     &repository.ChangeApprovalRow{ApproverID: &actor.ID, Decision: t.decision, Reason: in.Reason},
		BumpRevision: t.to == model.ChangeDraft,
	})
	if err != nil {
		return nil, staleChange(err)
	}
	return updated, nil
}

// CancelChange moves a not-yet-implemented change to CANCELLED (CM-07).
func (s *IncidentService) CancelChange(ctx context.Context, actor *model.AuthUser, id, reason string) (*model.Change, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, BadRequest("VALIDATION_ERROR", "Reason wajib diisi untuk cancel.", map[string]string{"field": "reason"})
	}
	c, err := s.loadChange(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	if !isRequesterOrManager(actor, c) {
		return nil, Forbidden("FORBIDDEN_ACTION", "Hanya requester atau Manager yang boleh cancel change.")
	}
	if !model.CanChangeTransition(c.Type, c.Status, model.ChangeCancelled) {
		return nil, invalidChangeTransition(c, model.ChangeCancelled)
	}
	updated, err := s.repo.ApplyChangeTransitions(ctx, id, repository.ChangeTransition{
		From: c.Status, To: model.ChangeCancelled, ActorID: &actor.ID,
		ActivityType: model.ChangeActivityStatusChange,
		Payload:      map[string]any{"reason": reason},
	})
	if err != nil {
		return nil, staleChange(err)
	}
	return updated, nil
}

// ChangeApprovals returns the approval history.
func (s *IncidentService) ChangeApprovals(ctx context.Context, actor *model.AuthUser, id string) ([]*model.ChangeApproval, error) {
	if _, err := s.loadChange(ctx, actor, id); err != nil {
		return nil, err
	}
	items, err := s.repo.ListChangeApprovals(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list approvals: %w", err)
	}
	return items, nil
}

// ChangeTimeline returns the change audit trail (CM-09).
func (s *IncidentService) ChangeTimeline(ctx context.Context, actor *model.AuthUser, id string) ([]*model.ChangeActivity, error) {
	if _, err := s.loadChange(ctx, actor, id); err != nil {
		return nil, err
	}
	items, err := s.repo.ListChangeActivities(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list change timeline: %w", err)
	}
	return items, nil
}

// AddChangeComment stores a comment on a change.
func (s *IncidentService) AddChangeComment(ctx context.Context, actor *model.AuthUser, id, body string) (*model.ChangeComment, error) {
	if strings.TrimSpace(body) == "" {
		return nil, BadRequest("VALIDATION_ERROR", "Komentar tidak boleh kosong.", map[string]string{"field": "body"})
	}
	if _, err := s.loadChange(ctx, actor, id); err != nil {
		return nil, err
	}
	c, err := s.repo.AddChangeComment(ctx, id, actor.ID, body)
	if err != nil {
		return nil, fmt.Errorf("add change comment: %w", err)
	}
	return c, nil
}

// ChangeComments returns the comment list.
func (s *IncidentService) ChangeComments(ctx context.Context, actor *model.AuthUser, id string) ([]*model.ChangeComment, error) {
	if _, err := s.loadChange(ctx, actor, id); err != nil {
		return nil, err
	}
	items, err := s.repo.ListChangeComments(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list change comments: %w", err)
	}
	return items, nil
}

// ChangeIncidents returns incidents linked to a change.
func (s *IncidentService) ChangeIncidents(ctx context.Context, actor *model.AuthUser, id string) ([]*model.ChangeIncidentLink, error) {
	if _, err := s.loadChange(ctx, actor, id); err != nil {
		return nil, err
	}
	items, err := s.repo.ListChangeLinks(ctx, id, "")
	if err != nil {
		return nil, fmt.Errorf("list change incidents: %w", err)
	}
	return items, nil
}

// IncidentChanges returns changes linked to an incident (Incident Detail panel).
func (s *IncidentService) IncidentChanges(ctx context.Context, actor *model.AuthUser, incidentID string) ([]*model.ChangeIncidentLink, error) {
	if err := requireChangeViewer(actor); err != nil {
		return nil, err
	}
	if _, err := s.Get(ctx, actor, incidentID); err != nil {
		return nil, err
	}
	items, err := s.repo.ListChangeLinks(ctx, "", incidentID)
	if err != nil {
		return nil, fmt.Errorf("list incident changes: %w", err)
	}
	return items, nil
}
