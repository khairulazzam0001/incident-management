package service

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"github.com/khairulazzam0001/incident-management/backend/internal/model"
)

// UploadConfig bounds attachment storage (FR-09).
type UploadConfig struct {
	Dir      string
	MaxBytes int64
}

// DefaultUploadConfig caps files at 10 MiB under dir.
func DefaultUploadConfig(dir string) UploadConfig {
	if dir == "" {
		dir = "./uploads"
	}
	return UploadConfig{Dir: dir, MaxBytes: 10 << 20}
}

var allowedExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true,
	".pdf": true, ".txt": true, ".zip": true,
}

var allowedMimes = []string{
	"image/png", "image/jpeg", "image/gif", "image/webp",
	"application/pdf", "text/plain", "application/zip",
}

func mimeAllowed(mime string) bool {
	for _, m := range allowedMimes {
		if strings.HasPrefix(mime, m) {
			return true
		}
	}
	return false
}

// canRead enforces read scope (PRD §15, MVP): coordinators and internal roles
// read everything; User/Customer only incidents they reported or own as PIC.
func canRead(actor *model.AuthUser, in *model.Incident) bool {
	if actor.Role != model.RoleUser {
		return true
	}
	if in.ReporterID != nil && *in.ReporterID == actor.ID {
		return true
	}
	return in.PICID != nil && *in.PICID == actor.ID
}

func (s *IncidentService) checkRead(actor *model.AuthUser, in *model.Incident) *Error {
	if !canRead(actor, in) {
		return Forbidden("FORBIDDEN_READ", "Anda tidak berhak melihat incident ini.")
	}
	return nil
}

// UploadAttachment validates and stores a file (FR-09). PIC or coordinator only.
func (s *IncidentService) UploadAttachment(ctx context.Context, actor *model.AuthUser, incidentID, fileName string, content []byte) (*model.Attachment, error) {
	if err := requireIncidentID(incidentID); err != nil {
		return nil, err
	}
	fileName = strings.TrimSpace(fileName)
	if fileName == "" {
		return nil, BadRequest("VALIDATION_ERROR", "Nama file wajib diisi.", nil)
	}
	if int64(len(content)) > s.uploadCfg.MaxBytes {
		return nil, BadRequest("FILE_TOO_LARGE",
			fmt.Sprintf("Ukuran maksimal %d MiB.", s.uploadCfg.MaxBytes>>20), nil)
	}
	ext := strings.ToLower(filepath.Ext(fileName))
	if !allowedExtensions[ext] {
		return nil, BadRequest("INVALID_FILE_TYPE",
			"Tipe file tidak diizinkan (png/jpg/gif/webp/pdf/txt/zip).", nil)
	}
	sniffLen := min(len(content), 512)
	if !mimeAllowed(http.DetectContentType(content[:sniffLen])) {
		return nil, BadRequest("INVALID_FILE_TYPE", "Isi file tidak sesuai tipenya.", nil)
	}
	cur, err := s.repo.GetIncident(ctx, incidentID)
	if err != nil {
		if isNotFound(err) {
			return nil, NotFound("Incident")
		}
		return nil, fmt.Errorf("get incident: %w", err)
	}
	if !canActOn(actor, cur) {
		return nil, Forbidden("FORBIDDEN_ACTION", "Hanya PIC atau koordinator yang boleh mengunggah file.")
	}
	key := uuid.NewString() + ext
	rel := filepath.Join(incidentID, key)
	abs := filepath.Join(s.uploadCfg.Dir, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o750); err != nil {
		return nil, fmt.Errorf("mkdir upload: %w", err)
	}
	if err := os.WriteFile(abs, content, 0o600); err != nil {
		return nil, fmt.Errorf("write upload: %w", err)
	}
	a, err := s.repo.CreateAttachment(ctx, incidentID, filepath.Base(fileName), rel, http.DetectContentType(content[:sniffLen]), int64(len(content)), actor.ID)
	if err != nil {
		_ = os.Remove(abs)
		return nil, fmt.Errorf("create attachment: %w", err)
	}
	return a, nil
}

// Attachments returns the file list with read-scope check.
func (s *IncidentService) Attachments(ctx context.Context, actor *model.AuthUser, incidentID string) ([]*model.Attachment, error) {
	if err := requireIncidentID(incidentID); err != nil {
		return nil, err
	}
	cur, err := s.repo.GetIncident(ctx, incidentID)
	if err != nil {
		if isNotFound(err) {
			return nil, NotFound("Incident")
		}
		return nil, fmt.Errorf("get incident: %w", err)
	}
	if err := s.checkRead(actor, cur); err != nil {
		return nil, err
	}
	items, err := s.repo.ListAttachments(ctx, incidentID)
	if err != nil {
		return nil, fmt.Errorf("list attachments: %w", err)
	}
	return items, nil
}

// AttachmentFile resolves the on-disk path with read-scope check.
func (s *IncidentService) AttachmentFile(ctx context.Context, actor *model.AuthUser, incidentID, id string) (string, *model.Attachment, error) {
	if err := requireIncidentID(incidentID); err != nil {
		return "", nil, err
	}
	if !validUUID(id) {
		return "", nil, BadRequest("INVALID_ID", "ID attachment tidak valid.", nil)
	}
	cur, err := s.repo.GetIncident(ctx, incidentID)
	if err != nil {
		if isNotFound(err) {
			return "", nil, NotFound("Incident")
		}
		return "", nil, fmt.Errorf("get incident: %w", err)
	}
	if err := s.checkRead(actor, cur); err != nil {
		return "", nil, err
	}
	a, err := s.repo.GetAttachment(ctx, incidentID, id)
	if err != nil {
		if isNotFound(err) {
			return "", nil, NotFound("Attachment")
		}
		return "", nil, fmt.Errorf("get attachment: %w", err)
	}
	key, err := s.repo.StorageKey(ctx, incidentID, id)
	if err != nil {
		if isNotFound(err) {
			return "", nil, NotFound("Attachment")
		}
		return "", nil, fmt.Errorf("storage key: %w", err)
	}
	abs := filepath.Join(s.uploadCfg.Dir, key)
	if st, err := os.Stat(abs); err != nil || st.IsDir() {
		return "", nil, NotFound("File attachment")
	}
	return abs, a, nil
}
