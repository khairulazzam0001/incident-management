package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"

	"github.com/khairulazzam0001/incident-management/backend/internal/model"
)

// Change attachments (PRD_Change_Management.md CM-FR-13) reuse the incident
// file rules (FR-09): png/jpg/gif/webp/pdf/txt/zip, max 10 MiB, sniffed MIME.

// canAttachToChange: requester, implementer, SystemAnalyst, or ManagerLead —
// the change counterpart of "PIC or coordinator" for incidents.
func canAttachToChange(actor *model.AuthUser, c *model.Change) bool {
	switch actor.Role {
	case model.RoleManagerLead, model.RoleSystemAnalyst:
		return true
	}
	return actor.ID == c.RequesterID || (c.ImplementerID != nil && *c.ImplementerID == actor.ID)
}

// UploadChangeAttachment validates and stores a file on a change.
func (s *IncidentService) UploadChangeAttachment(ctx context.Context, actor *model.AuthUser, changeID, fileName string, content []byte) (*model.ChangeAttachment, error) {
	c, err := s.loadChange(ctx, actor, changeID)
	if err != nil {
		return nil, err
	}
	if !canAttachToChange(actor, c) {
		return nil, Forbidden("FORBIDDEN_ACTION", "Hanya requester, implementer, Analyst, atau Manager yang boleh mengunggah file.")
	}
	if c.Status == model.ChangeRejected || c.Status == model.ChangeCancelled {
		return nil, Conflict("NOT_EDITABLE", "Change yang rejected/cancelled tidak menerima file.", map[string]string{"status": c.Status})
	}
	name, ext, mime, verr := s.validateUpload(fileName, content)
	if verr != nil {
		return nil, verr
	}
	rel := filepath.Join("changes", changeID, uuid.NewString()+ext)
	abs, err := s.storeUpload(rel, content)
	if err != nil {
		return nil, err
	}
	a, err := s.repo.CreateChangeAttachment(ctx, changeID, name, rel, mime, int64(len(content)), actor.ID)
	if err != nil {
		_ = os.Remove(abs)
		return nil, fmt.Errorf("create change attachment: %w", err)
	}
	return a, nil
}

// ChangeAttachments returns the file list for change viewers.
func (s *IncidentService) ChangeAttachments(ctx context.Context, actor *model.AuthUser, changeID string) ([]*model.ChangeAttachment, error) {
	if _, err := s.loadChange(ctx, actor, changeID); err != nil {
		return nil, err
	}
	items, err := s.repo.ListChangeAttachments(ctx, changeID)
	if err != nil {
		return nil, fmt.Errorf("list change attachments: %w", err)
	}
	return items, nil
}

// ChangeAttachmentFile resolves the on-disk path with view-permission check.
func (s *IncidentService) ChangeAttachmentFile(ctx context.Context, actor *model.AuthUser, changeID, id string) (string, *model.ChangeAttachment, error) {
	if _, err := s.loadChange(ctx, actor, changeID); err != nil {
		return "", nil, err
	}
	if !validUUID(id) {
		return "", nil, BadRequest("INVALID_ID", "ID attachment tidak valid.", nil)
	}
	a, key, err := s.repo.GetChangeAttachment(ctx, changeID, id)
	if err != nil {
		if isNotFound(err) {
			return "", nil, NotFound("Attachment")
		}
		return "", nil, fmt.Errorf("get change attachment: %w", err)
	}
	abs := filepath.Join(s.uploadCfg.Dir, key)
	if st, err := os.Stat(abs); err != nil || st.IsDir() {
		return "", nil, NotFound("File attachment")
	}
	return abs, a, nil
}
