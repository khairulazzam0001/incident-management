package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/khairulazzam0001/incident-management/backend/internal/apierror"
	"github.com/khairulazzam0001/incident-management/backend/internal/model"
	"github.com/khairulazzam0001/incident-management/backend/internal/repository"
	"github.com/khairulazzam0001/incident-management/backend/internal/service"
)

// HandleLogin issues a JWT for valid credentials.
func HandleLogin(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if !decodeJSON(w, r, &body) {
			return
		}
		token, user, err := d.Auth.Login(r.Context(), body.Email, body.Password)
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, map[string]any{"token": token, "user": user})
	}
}

// HandleUsers returns active users for assignment lookup.
func HandleUsers(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		users, err := d.Incidents.Users(r.Context())
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		if users == nil {
			users = []*model.User{}
		}
		apierror.WriteJSON(w, http.StatusOK, map[string]any{"data": users})
	}
}

// HandleMeta returns master data for forms and filters.
func HandleMeta(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		meta, err := d.Incidents.Meta(r.Context())
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, meta)
	}
}

// HandleCreateIncident creates an incident with status NEW (201).
func HandleCreateIncident(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		var in model.CreateIncidentInput
		if !decodeJSON(w, r, &in) {
			return
		}
		created, err := d.Incidents.Create(r.Context(), actor, in)
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusCreated, created)
	}
}

// listResponse is the paginated envelope for GET /api/incidents.
type listResponse struct {
	Data []*model.Incident `json:"data"`
	Meta pageMeta          `json:"meta"`
}

type pageMeta struct {
	Page  int `json:"page"`
	Limit int `json:"limit"`
	Total int `json:"total"`
}

// HandleListIncidents lists/filters/searches incidents with pagination.
func HandleListIncidents(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		page, err := strconv.Atoi(q.Get("page"))
		if q.Get("page") != "" && err != nil {
			writeServiceError(d, w, r, service.BadRequest("VALIDATION_ERROR", "Page harus angka.", map[string]string{"field": "page"}))
			return
		}
		limit, err := strconv.Atoi(q.Get("limit"))
		if q.Get("limit") != "" && err != nil {
			writeServiceError(d, w, r, service.BadRequest("VALIDATION_ERROR", "Limit harus angka.", map[string]string{"field": "limit"}))
			return
		}
		f := repository.IncidentFilter{
			Status:   q.Get("status"),
			Severity: q.Get("severity"),
			Priority: q.Get("priority"),
			Q:        q.Get("q"),
			Sort:     q.Get("sort"),
			Order:    q.Get("order"),
			Page:     page,
			Limit:    limit,
		}
		items, total, err := d.Incidents.List(r.Context(), f)
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		if items == nil {
			items = []*model.Incident{}
		}
		if f.Page <= 0 {
			f.Page = 1
		}
		if f.Limit <= 0 {
			f.Limit = 20
		}
		apierror.WriteJSON(w, http.StatusOK, listResponse{
			Data: items,
			Meta: pageMeta{Page: f.Page, Limit: f.Limit, Total: total},
		})
	}
}

// HandleGetIncident returns one incident or 404.
func HandleGetIncident(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		in, err := d.Incidents.Get(r.Context(), chi.URLParam(r, "id"))
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, in)
	}
}

// HandleChangeStatus runs a controlled status transition.
func HandleChangeStatus(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		var body struct {
			Status string `json:"status"`
		}
		if !decodeJSON(w, r, &body) {
			return
		}
		updated, err := d.Incidents.ChangeStatus(r.Context(), actor, chi.URLParam(r, "id"), body.Status)
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, updated)
	}
}

// HandleAssign assigns team/PIC (coordinators only).
func HandleAssign(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		var in service.AssignInput
		if !decodeJSON(w, r, &in) {
			return
		}
		updated, err := d.Incidents.Assign(r.Context(), actor, chi.URLParam(r, "id"), in)
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, updated)
	}
}

// HandleAddComment adds a comment (201).
func HandleAddComment(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		var body struct {
			Body string `json:"body"`
		}
		if !decodeJSON(w, r, &body) {
			return
		}
		c, err := d.Incidents.AddComment(r.Context(), actor, chi.URLParam(r, "id"), body.Body)
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusCreated, c)
	}
}

// HandleComments returns the comment list.
func HandleComments(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := d.Incidents.Comments(r.Context(), chi.URLParam(r, "id"))
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		if items == nil {
			items = []*model.Comment{}
		}
		apierror.WriteJSON(w, http.StatusOK, map[string]any{"data": items})
	}
}

// HandleTimeline returns the audit trail oldest-first.
func HandleTimeline(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := d.Incidents.Timeline(r.Context(), chi.URLParam(r, "id"))
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		if items == nil {
			items = []*model.Activity{}
		}
		apierror.WriteJSON(w, http.StatusOK, map[string]any{"data": items})
	}
}
