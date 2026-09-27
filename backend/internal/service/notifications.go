package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/khairulazzam0001/incident-management/backend/internal/model"
	"github.com/khairulazzam0001/incident-management/backend/internal/repository"
)

// fanout stores one in-app notification per recipient and enqueues email
// delivery. It never fails the triggering action: errors are only logged.
func (s *IncidentService) fanout(ctx context.Context, in *model.Incident, actor *model.AuthUser, ntype string) {
	recipients, err := s.resolveRecipients(ctx, in, actor, ntype)
	if err != nil {
		s.log.Warn("notifikasi: gagal resolve penerima", "error", err, "type", ntype)
		return
	}
	if len(recipients) == 0 {
		return
	}
	emailStatus := model.EmailSkipped
	if s.mailer.Enabled() {
		emailStatus = model.EmailPending
	}
	rows := make([]repository.NotificationInput, 0, len(recipients))
	for _, rid := range recipients {
		rows = append(rows, repository.NotificationInput{
			IncidentID:  in.ID,
			Type:        ntype,
			RecipientID: rid,
			ActorID:     &actor.ID,
			EmailStatus: emailStatus,
			Payload:     fmt.Sprintf(`{"incident_no":%q,"title":%q,"status":%q}`, in.IncidentNo, in.Title, in.Status),
		})
	}
	ids, err := s.repo.CreateNotifications(ctx, rows)
	if err != nil {
		s.log.Warn("notifikasi: gagal simpan", "error", err, "type", ntype)
		return
	}
	if !s.mailer.Enabled() {
		return
	}
	emails, err := s.repo.EmailsByIDs(ctx, recipients)
	if err != nil {
		s.log.Warn("notifikasi: gagal ambil email", "error", err)
		return
	}
	subject, body := mailContent(ntype, in)
	for i, rid := range recipients {
		to, ok := emails[rid]
		if !ok || to == "" || i >= len(ids) {
			continue
		}
		s.mailer.Enqueue(ids[i], to, subject, body)
	}
}

func (s *IncidentService) resolveRecipients(ctx context.Context, in *model.Incident, actor *model.AuthUser, ntype string) ([]string, error) {
	var ids []string
	add := func(id *string) {
		if id != nil && *id != "" {
			ids = append(ids, *id)
		}
	}
	switch ntype {
	case model.NotifCreated:
		coords, err := s.repo.CoordinatorIDs(ctx)
		if err != nil {
			return nil, err
		}
		ids = append(ids, coords...)
	case model.NotifAssigned:
		add(in.PICID)
	case model.NotifStatusChanged, model.NotifComment:
		add(in.PICID)
		add(in.ReporterID)
	case model.NotifVerificationFailed:
		add(in.PICID)
	case model.NotifResolved, model.NotifClosed:
		add(in.ReporterID)
	default:
		return nil, fmt.Errorf("tipe notifikasi tak dikenal: %s", ntype)
	}
	seen := map[string]bool{}
	out := []string{}
	for _, id := range ids {
		if id == actor.ID || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out, nil
}

func mailContent(ntype string, in *model.Incident) (subject, body string) {
	subject = fmt.Sprintf("[%s] %s: %s", strings.ToUpper(ntype), in.IncidentNo, in.Title)
	body = fmt.Sprintf("Incident %s\n%s\nStatus: %s\nSeverity: %s, Priority: %s\nWaktu: %s",
		in.IncidentNo, in.Title, in.Status, in.Severity, in.Priority,
		time.Now().Format("2006-01-02 15:04:05"))
	return subject, body
}

// Dashboard returns PRD §14 aggregates for the actor.
func (s *IncidentService) Dashboard(ctx context.Context, actor *model.AuthUser) (*repository.Dashboard, error) {
	d, err := s.repo.GetDashboard(ctx, actor.ID)
	if err != nil {
		return nil, fmt.Errorf("dashboard: %w", err)
	}
	return d, nil
}

// Notifications returns the actor's items plus unread count (FR-11).
func (s *IncidentService) Notifications(ctx context.Context, actor *model.AuthUser, unreadOnly bool, limit int) ([]*model.Notification, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		return nil, 0, BadRequest("VALIDATION_ERROR", "Limit maksimal 100.", map[string]string{"field": "limit"})
	}
	items, err := s.repo.ListNotifications(ctx, actor.ID, unreadOnly, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("list notifications: %w", err)
	}
	unread, err := s.repo.UnreadCount(ctx, actor.ID)
	if err != nil {
		return nil, 0, fmt.Errorf("unread count: %w", err)
	}
	return items, unread, nil
}

// MarkNotificationRead marks one item read; 404 when not owned or already read.
func (s *IncidentService) MarkNotificationRead(ctx context.Context, actor *model.AuthUser, id string) error {
	if !validUUID(id) {
		return BadRequest("INVALID_ID", "ID notifikasi tidak valid.", nil)
	}
	updated, err := s.repo.MarkNotificationRead(ctx, id, actor.ID)
	if err != nil {
		return fmt.Errorf("mark read: %w", err)
	}
	if !updated {
		return NotFound("Notifikasi")
	}
	return nil
}

// MasterInput is the payload for creating applications/teams.
type MasterInput struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

func validateMasterInput(in MasterInput) *Error {
	if strings.TrimSpace(in.Code) == "" || strings.TrimSpace(in.Name) == "" {
		return BadRequest("VALIDATION_ERROR", "Code dan name wajib diisi.", nil)
	}
	return nil
}

// CreateApplication adds a master application (coordinators only).
func (s *IncidentService) CreateApplication(ctx context.Context, actor *model.AuthUser, in MasterInput) (*model.MasterIDItem, error) {
	if !model.IsCoordinator(actor.Role) {
		return nil, Forbidden("FORBIDDEN_MASTER", "Hanya koordinator yang boleh kelola master data.")
	}
	if err := validateMasterInput(in); err != nil {
		return nil, err
	}
	it, err := s.repo.CreateApplication(ctx, strings.TrimSpace(in.Code), strings.TrimSpace(in.Name))
	if err != nil {
		if repository.IsUniqueViolation(err) {
			return nil, Conflict("DUPLICATE_CODE", "Code aplikasi sudah dipakai.", map[string]string{"field": "code"})
		}
		return nil, fmt.Errorf("create application: %w", err)
	}
	return it, nil
}

// CreateTeam adds a master team (coordinators only).
func (s *IncidentService) CreateTeam(ctx context.Context, actor *model.AuthUser, in MasterInput) (*model.MasterIDItem, error) {
	if !model.IsCoordinator(actor.Role) {
		return nil, Forbidden("FORBIDDEN_MASTER", "Hanya koordinator yang boleh kelola master data.")
	}
	if err := validateMasterInput(in); err != nil {
		return nil, err
	}
	it, err := s.repo.CreateTeam(ctx, strings.TrimSpace(in.Code), strings.TrimSpace(in.Name))
	if err != nil {
		if repository.IsUniqueViolation(err) {
			return nil, Conflict("DUPLICATE_CODE", "Code team sudah dipakai.", map[string]string{"field": "code"})
		}
		return nil, fmt.Errorf("create team: %w", err)
	}
	return it, nil
}

// DeleteApplication removes an unused application (ManagerLead only).
func (s *IncidentService) DeleteApplication(ctx context.Context, actor *model.AuthUser, id string) error {
	if actor.Role != model.RoleManagerLead {
		return Forbidden("FORBIDDEN_MASTER", "Hanya ManagerLead yang boleh hapus master data.")
	}
	if !validUUID(id) {
		return BadRequest("INVALID_ID", "ID tidak valid.", nil)
	}
	used, found, err := s.repo.DeleteApplication(ctx, id)
	if err != nil {
		return fmt.Errorf("delete application: %w", err)
	}
	if !found {
		return NotFound("Application")
	}
	if used {
		return Conflict("IN_USE", "Application masih dipakai incident.", nil)
	}
	return nil
}

// DeleteTeam removes an unused team (ManagerLead only).
func (s *IncidentService) DeleteTeam(ctx context.Context, actor *model.AuthUser, id string) error {
	if actor.Role != model.RoleManagerLead {
		return Forbidden("FORBIDDEN_MASTER", "Hanya ManagerLead yang boleh hapus master data.")
	}
	if !validUUID(id) {
		return BadRequest("INVALID_ID", "ID tidak valid.", nil)
	}
	used, found, err := s.repo.DeleteTeam(ctx, id)
	if err != nil {
		return fmt.Errorf("delete team: %w", err)
	}
	if !found {
		return NotFound("Team")
	}
	if used {
		return Conflict("IN_USE", "Team masih dipakai.", nil)
	}
	return nil
}
