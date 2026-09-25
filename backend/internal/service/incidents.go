package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/khairulazzam0001/incident-management/backend/internal/model"
	"github.com/khairulazzam0001/incident-management/backend/internal/repository"
)

// IncidentService implements US-01–US-06 workflow rules (PRD §8) on top of queries.
type IncidentService struct {
	repo *repository.Repository
}

// NewIncidentService creates the service.
func NewIncidentService(repo *repository.Repository) *IncidentService {
	return &IncidentService{repo: repo}
}

func validUUID(id string) bool {
	_, err := uuid.Parse(id)
	return err == nil
}

func requireIncidentID(id string) *Error {
	if !validUUID(id) {
		return BadRequest("INVALID_ID", "ID incident tidak valid.", nil)
	}
	return nil
}

func isNotFound(err error) bool {
	return err == pgx.ErrNoRows
}

// codesIn reports whether code is one of the allowed values.
func codesIn(code string, items []model.MasterItem) bool {
	for _, it := range items {
		if it.Code == code {
			return true
		}
	}
	return false
}

// Create validates input (US-01) and stores the incident with status NEW.
func (s *IncidentService) Create(ctx context.Context, actor *model.AuthUser, in model.CreateIncidentInput) (*model.Incident, error) {
	in.Title = strings.TrimSpace(in.Title)
	if len(in.Title) < 5 {
		return nil, BadRequest("VALIDATION_ERROR", "Title minimal 5 karakter.", map[string]string{"field": "title"})
	}
	meta, err := s.repo.ListMeta(ctx)
	if err != nil {
		return nil, fmt.Errorf("load masters: %w", err)
	}
	if !codesIn(in.Severity, meta.Severities) {
		return nil, BadRequest("VALIDATION_ERROR", "Severity tidak dikenal.", map[string]string{"field": "severity"})
	}
	if !codesIn(in.Priority, meta.Priorities) {
		return nil, BadRequest("VALIDATION_ERROR", "Priority tidak dikenal.", map[string]string{"field": "priority"})
	}
	if !codesIn(in.Source, meta.Sources) {
		return nil, BadRequest("VALIDATION_ERROR", "Source tidak dikenal.", map[string]string{"field": "source"})
	}
	if in.Environment != nil && *in.Environment != "" && !codesIn(*in.Environment, meta.Environments) {
		return nil, BadRequest("VALIDATION_ERROR", "Environment tidak dikenal.", map[string]string{"field": "environment"})
	}
	if in.ApplicationID != nil && *in.ApplicationID != "" {
		found := false
		for _, a := range meta.Applications {
			if a.ID == *in.ApplicationID {
				found = true
				break
			}
		}
		if !found {
			return nil, BadRequest("VALIDATION_ERROR", "Application tidak dikenal.", map[string]string{"field": "application_id"})
		}
	}
	no, err := s.repo.NextIncidentNo(ctx)
	if err != nil {
		return nil, fmt.Errorf("generate incident_no: %w", err)
	}
	created, err := s.repo.CreateIncident(ctx, in, no, actor.ID)
	if err != nil {
		return nil, fmt.Errorf("create incident: %w", err)
	}
	return created, nil
}

// List validates paging params and returns a page (FR-02).
func (s *IncidentService) List(ctx context.Context, f repository.IncidentFilter) ([]*model.Incident, int, error) {
	if f.Page <= 0 {
		f.Page = 1
	}
	if f.Limit <= 0 {
		f.Limit = 20
	}
	if f.Limit > 100 {
		return nil, 0, BadRequest("VALIDATION_ERROR", "Limit maksimal 100.", map[string]string{"field": "limit"})
	}
	items, total, err := s.repo.ListIncidents(ctx, f)
	if err != nil {
		return nil, 0, fmt.Errorf("list incidents: %w", err)
	}
	return items, total, nil
}

// Get returns one incident or 404.
func (s *IncidentService) Get(ctx context.Context, id string) (*model.Incident, error) {
	if err := requireIncidentID(id); err != nil {
		return nil, err
	}
	in, err := s.repo.GetIncident(ctx, id)
	if err != nil {
		if isNotFound(err) {
			return nil, NotFound("Incident")
		}
		return nil, fmt.Errorf("get incident: %w", err)
	}
	return in, nil
}

// canActOn reports whether the actor may change status/comment: the current
// PIC or a coordinator role. (Read scope tightening follows in hardening.)
func canActOn(actor *model.AuthUser, in *model.Incident) bool {
	if model.IsCoordinator(actor.Role) {
		return true
	}
	return in.PICID != nil && *in.PICID == actor.ID
}

// ChangeStatus enforces the controlled transition map server-side (FR-04).
func (s *IncidentService) ChangeStatus(ctx context.Context, actor *model.AuthUser, id, to string) (*model.Incident, error) {
	if err := requireIncidentID(id); err != nil {
		return nil, err
	}
	if _, ok := model.AllowedTransitions[to]; !ok {
		return nil, BadRequest("VALIDATION_ERROR", "Status tidak dikenal.", map[string]string{"field": "status"})
	}
	in, err := s.repo.GetIncident(ctx, id)
	if err != nil {
		if isNotFound(err) {
			return nil, NotFound("Incident")
		}
		return nil, fmt.Errorf("get incident: %w", err)
	}
	if !model.CanTransition(in.Status, to) {
		return nil, Conflict("INVALID_TRANSITION",
			fmt.Sprintf("Transisi %s → %s tidak diizinkan.", in.Status, to),
			map[string]string{"from": in.Status, "to": to})
	}
	if !canActOn(actor, in) {
		return nil, Forbidden("FORBIDDEN_TRANSITION", "Hanya PIC atau koordinator yang boleh mengubah status.")
	}
	updated, err := s.repo.UpdateStatus(ctx, id, in.Status, to, actor.ID)
	if err != nil {
		return nil, fmt.Errorf("update status: %w", err)
	}
	return updated, nil
}

// AssignInput is the payload for PATCH /api/incidents/:id/assignment.
type AssignInput struct {
	TeamID *string `json:"team_id"`
	PICID  *string `json:"pic_id"`
	Note   string  `json:"note"`
}

// Assign stores team/PIC (US-02/FR-03). Coordinators only. Assigning a NEW
// incident moves it to ASSIGNED.
func (s *IncidentService) Assign(ctx context.Context, actor *model.AuthUser, id string, in AssignInput) (*model.Incident, error) {
	if err := requireIncidentID(id); err != nil {
		return nil, err
	}
	if !model.IsCoordinator(actor.Role) {
		return nil, Forbidden("FORBIDDEN_ASSIGN", "Hanya HelpDesk, SystemAnalyst, atau ManagerLead yang boleh assign.")
	}
	if (in.PICID == nil || *in.PICID == "") && (in.TeamID == nil || *in.TeamID == "") {
		return nil, BadRequest("VALIDATION_ERROR", "team_id atau pic_id wajib diisi.", nil)
	}
	if in.PICID != nil && *in.PICID != "" {
		if !validUUID(*in.PICID) {
			return nil, BadRequest("VALIDATION_ERROR", "pic_id tidak valid.", map[string]string{"field": "pic_id"})
		}
		pic, err := s.repo.GetUserByID(ctx, *in.PICID)
		if err != nil {
			if isNotFound(err) {
				return nil, BadRequest("VALIDATION_ERROR", "PIC tidak ditemukan.", map[string]string{"field": "pic_id"})
			}
			return nil, fmt.Errorf("get pic: %w", err)
		}
		if !pic.IsActive {
			return nil, BadRequest("VALIDATION_ERROR", "PIC tidak aktif.", map[string]string{"field": "pic_id"})
		}
	}
	if ok, err := s.repo.TeamExists(ctx, in.TeamID); err != nil {
		return nil, fmt.Errorf("check team: %w", err)
	} else if !ok {
		return nil, BadRequest("VALIDATION_ERROR", "Team tidak ditemukan.", map[string]string{"field": "team_id"})
	}
	cur, err := s.repo.GetIncident(ctx, id)
	if err != nil {
		if isNotFound(err) {
			return nil, NotFound("Incident")
		}
		return nil, fmt.Errorf("get incident: %w", err)
	}
	to := cur.Status
	if cur.Status == model.StatusNew {
		to = model.StatusAssigned
	}
	updated, err := s.repo.AssignIncident(ctx, id, in.TeamID, in.PICID, actor.ID, cur.Status, to, in.Note)
	if err != nil {
		return nil, fmt.Errorf("assign incident: %w", err)
	}
	return updated, nil
}

// AddComment stores a comment plus its timeline entry (FR-08).
func (s *IncidentService) AddComment(ctx context.Context, actor *model.AuthUser, id, body string) (*model.Comment, error) {
	if err := requireIncidentID(id); err != nil {
		return nil, err
	}
	if strings.TrimSpace(body) == "" {
		return nil, BadRequest("VALIDATION_ERROR", "Komentar tidak boleh kosong.", map[string]string{"field": "body"})
	}
	cur, err := s.repo.GetIncident(ctx, id)
	if err != nil {
		if isNotFound(err) {
			return nil, NotFound("Incident")
		}
		return nil, fmt.Errorf("get incident: %w", err)
	}
	_ = cur
	c, err := s.repo.AddComment(ctx, id, actor.ID, body)
	if err != nil {
		return nil, fmt.Errorf("add comment: %w", err)
	}
	return c, nil
}

// Comments returns the comment list (FR-08) or 404.
func (s *IncidentService) Comments(ctx context.Context, id string) ([]*model.Comment, error) {
	if err := requireIncidentID(id); err != nil {
		return nil, err
	}
	if _, err := s.repo.GetIncident(ctx, id); err != nil {
		if isNotFound(err) {
			return nil, NotFound("Incident")
		}
		return nil, fmt.Errorf("get incident: %w", err)
	}
	items, err := s.repo.ListComments(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}
	return items, nil
}

// Timeline returns the audit trail oldest-first (FR-08).
func (s *IncidentService) Timeline(ctx context.Context, id string) ([]*model.Activity, error) {
	if err := requireIncidentID(id); err != nil {
		return nil, err
	}
	if _, err := s.repo.GetIncident(ctx, id); err != nil {
		if isNotFound(err) {
			return nil, NotFound("Incident")
		}
		return nil, fmt.Errorf("get incident: %w", err)
	}
	items, err := s.repo.ListActivities(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list timeline: %w", err)
	}
	return items, nil
}

// Users returns active users for assignment lookup.
func (s *IncidentService) Users(ctx context.Context) ([]*model.User, error) {
	users, err := s.repo.ListUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	return users, nil
}

// Meta returns master data for forms and filters.
func (s *IncidentService) Meta(ctx context.Context) (*model.Meta, error) {
	meta, err := s.repo.ListMeta(ctx)
	if err != nil {
		return nil, fmt.Errorf("list meta: %w", err)
	}
	return meta, nil
}
