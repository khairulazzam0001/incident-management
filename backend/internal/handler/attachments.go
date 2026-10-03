package handler

import (
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/khairulazzam0001/incident-management/backend/internal/apierror"
	"github.com/khairulazzam0001/incident-management/backend/internal/model"
	"github.com/khairulazzam0001/incident-management/backend/internal/service"
)

// readUploadFile reads the multipart field "file" (body ≤ 12 MiB; the service
// enforces the 10 MiB file limit). It writes the error response itself.
func readUploadFile(d Deps, w http.ResponseWriter, r *http.Request) (string, []byte, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, 12<<20)
	if err := r.ParseMultipartForm(12 << 20); err != nil {
		writeServiceError(d, w, r, service.BadRequest("INVALID_UPLOAD", "Body harus multipart dengan field file.", nil))
		return "", nil, false
	}
	files := r.MultipartForm.File["file"]
	if len(files) == 0 {
		writeServiceError(d, w, r, service.BadRequest("VALIDATION_ERROR", "Field file wajib diisi.", nil))
		return "", nil, false
	}
	fh := files[0]
	f, err := fh.Open()
	if err != nil {
		writeServiceError(d, w, r, service.BadRequest("INVALID_UPLOAD", "File tidak bisa dibaca.", nil))
		return "", nil, false
	}
	defer func() { _ = f.Close() }()
	content, err := io.ReadAll(f)
	if err != nil {
		writeServiceError(d, w, r, service.BadRequest("INVALID_UPLOAD", "File tidak bisa dibaca.", nil))
		return "", nil, false
	}
	return fh.Filename, content, true
}

// HandleUploadAttachment stores a multipart file (201). Field name: "file".
func HandleUploadAttachment(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		name, content, ok := readUploadFile(d, w, r)
		if !ok {
			return
		}
		a, err := d.Incidents.UploadAttachment(r.Context(), actor, chi.URLParam(r, "id"), name, content)
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusCreated, a)
	}
}

// HandleAttachments returns the file list.
func HandleAttachments(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		items, err := d.Incidents.Attachments(r.Context(), actor, chi.URLParam(r, "id"))
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		if items == nil {
			items = []*model.Attachment{}
		}
		apierror.WriteJSON(w, http.StatusOK, map[string]any{"data": items})
	}
}

// HandleDownloadAttachment serves the file with access control.
func HandleDownloadAttachment(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		path, a, err := d.Incidents.AttachmentFile(r.Context(), actor, chi.URLParam(r, "id"), chi.URLParam(r, "aid"))
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		w.Header().Set("Content-Disposition", "attachment; filename=\""+a.FileName+"\"")
		http.ServeFile(w, r, path)
	}
}
