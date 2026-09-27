package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/khairulazzam0001/incident-management/backend/internal/apierror"
	"github.com/khairulazzam0001/incident-management/backend/internal/model"
	"github.com/khairulazzam0001/incident-management/backend/internal/service"
)

// HandleDashboard returns PRD §14 aggregates.
func HandleDashboard(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		board, err := d.Incidents.Dashboard(r.Context(), actor)
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, board)
	}
}

// HandleNotifications returns the actor's notifications plus unread count.
func HandleNotifications(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		q := r.URL.Query()
		unreadOnly := q.Get("unread") == "true" || q.Get("unread") == "1"
		limit := 20
		if q.Get("limit") != "" {
			var err error
			if limit, err = atoiQuery(q.Get("limit")); err != nil {
				writeServiceError(d, w, r, service.BadRequest("VALIDATION_ERROR", "Limit harus angka.", map[string]string{"field": "limit"}))
				return
			}
		}
		items, unread, err := d.Incidents.Notifications(r.Context(), actor, unreadOnly, limit)
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		if items == nil {
			items = []*model.Notification{}
		}
		apierror.WriteJSON(w, http.StatusOK, map[string]any{"data": items, "unread": unread})
	}
}

// HandleMarkNotificationRead marks one notification read.
func HandleMarkNotificationRead(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		if err := d.Incidents.MarkNotificationRead(r.Context(), actor, chi.URLParam(r, "id")); err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}

// HandleCreateApplication adds a master application (201).
func HandleCreateApplication(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		var in service.MasterInput
		if !decodeJSON(w, r, &in) {
			return
		}
		it, err := d.Incidents.CreateApplication(r.Context(), actor, in)
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusCreated, it)
	}
}

// HandleCreateTeam adds a master team (201).
func HandleCreateTeam(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		var in service.MasterInput
		if !decodeJSON(w, r, &in) {
			return
		}
		it, err := d.Incidents.CreateTeam(r.Context(), actor, in)
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusCreated, it)
	}
}

// HandleDeleteApplication removes an unused application (204).
func HandleDeleteApplication(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		if err := d.Incidents.DeleteApplication(r.Context(), actor, chi.URLParam(r, "id")); err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// HandleDeleteTeam removes an unused team (204).
func HandleDeleteTeam(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		if err := d.Incidents.DeleteTeam(r.Context(), actor, chi.URLParam(r, "id")); err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
