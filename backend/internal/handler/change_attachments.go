package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/khairulazzam0001/incident-management/backend/internal/apierror"
)

// HandleUploadChangeAttachment stores a multipart file on a change (201).
func HandleUploadChangeAttachment(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		name, content, ok := readUploadFile(d, w, r)
		if !ok {
			return
		}
		a, err := d.Incidents.UploadChangeAttachment(r.Context(), actor, chi.URLParam(r, "id"), name, content)
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusCreated, a)
	}
}

// HandleChangeAttachments returns the change file list.
func HandleChangeAttachments(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		items, err := d.Incidents.ChangeAttachments(r.Context(), actor, chi.URLParam(r, "id"))
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, map[string]any{"data": items})
	}
}

// HandleDownloadChangeAttachment serves a change file with access control.
func HandleDownloadChangeAttachment(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		path, a, err := d.Incidents.ChangeAttachmentFile(r.Context(), actor, chi.URLParam(r, "id"), chi.URLParam(r, "aid"))
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		w.Header().Set("Content-Disposition", "attachment; filename=\""+a.FileName+"\"")
		http.ServeFile(w, r, path)
	}
}
