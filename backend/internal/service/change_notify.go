package service

import (
	"context"
	"fmt"
	"time"

	"github.com/khairulazzam0001/incident-management/backend/internal/model"
)

// changeFanout stores change notifications (PRD_Change_Management.md §13)
// via the shared trans_notification table and enqueues email. Like fanout,
// it never fails the triggering action.
func (s *IncidentService) changeFanout(ctx context.Context, c *model.Change, actor *model.AuthUser, ntype string) {
	recipients, err := s.changeRecipients(ctx, c, actor, ntype)
	if err != nil {
		s.log.Warn("notifikasi change: gagal resolve penerima", "error", err, "type", ntype)
		return
	}
	s.deliver(ctx, delivery{
		changeID: &c.ID, ntype: ntype, actorID: &actor.ID, recipients: recipients,
		payload: fmt.Sprintf(`{"change_no":%q,"title":%q,"status":%q}`, c.ChangeNo, c.Title, c.Status),
		subject: fmt.Sprintf("[%s] %s: %s", ntype, c.ChangeNo, c.Title),
		body: fmt.Sprintf("Change %s\n%s\nStatus: %s\nType: %s, Risk: %s\nWaktu: %s",
			c.ChangeNo, c.Title, c.Status, c.Type, c.Risk, time.Now().Format("2006-01-02 15:04:05")),
	})
}

func (s *IncidentService) changeRecipients(ctx context.Context, c *model.Change, actor *model.AuthUser, ntype string) ([]string, error) {
	ids := []string{}
	add := func(id *string) {
		if id != nil && *id != "" {
			ids = append(ids, *id)
		}
	}
	addRoles := func(roles ...string) error {
		found, err := s.repo.ActiveUserIDsByRole(ctx, roles...)
		if err != nil {
			return err
		}
		ids = append(ids, found...)
		return nil
	}
	requester := &c.RequesterID
	switch ntype {
	case model.NotifChangeSubmitted:
		roles := []string{model.RoleManagerLead}
		if c.Risk != model.RiskHigh {
			if u, err := s.repo.GetUserByID(ctx, c.RequesterID); err == nil && u.Role == model.RoleManagerLead {
				roles = append(roles, model.RoleSystemAnalyst)
			}
		}
		if err := addRoles(roles...); err != nil {
			return nil, err
		}
	case model.NotifChangeApproved, model.NotifChangeRejected, model.NotifChangeChangesRequested,
		model.NotifChangeScheduled, model.NotifChangeClosed, model.NotifChangeCancelled,
		model.NotifChangeCausedIncident:
		add(requester)
		add(c.ImplementerID)
	case model.NotifChangeStarted:
		add(requester)
		if err := addRoles(model.RoleManagerLead); err != nil {
			return nil, err
		}
	case model.NotifChangeFailed:
		add(requester)
		add(c.ImplementerID)
		if err := addRoles(model.RoleManagerLead); err != nil {
			return nil, err
		}
		pics, err := s.repo.FixForPICs(ctx, c.ID)
		if err != nil {
			return nil, err
		}
		ids = append(ids, pics...)
	default:
		return nil, fmt.Errorf("tipe notifikasi change tak dikenal: %s", ntype)
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
