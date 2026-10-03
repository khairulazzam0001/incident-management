package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/khairulazzam0001/incident-management/backend/internal/apierror"
	"github.com/khairulazzam0001/incident-management/backend/internal/model"
	"github.com/khairulazzam0001/incident-management/backend/internal/service"
)

// SLA & Escalation handlers (PRD_SLA_Escalation.md §10).

// HandleIncidentSLA returns all SLA instances of an incident.
func HandleIncidentSLA(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		items, err := d.Incidents.IncidentSLA(r.Context(), actor, chi.URLParam(r, "id"))
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, map[string]any{"data": items})
	}
}

// HandleSLADashboard returns compliance per priority/metric + open breaches.
func HandleSLADashboard(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		q := r.URL.Query()
		board, err := d.Incidents.SLADashboard(r.Context(), actor, q.Get("from"), q.Get("to"))
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, board)
	}
}

// HandleSLASettings returns policies + calendars.
func HandleSLASettings(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		settings, err := d.Incidents.SLASettings(r.Context(), actor)
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, settings)
	}
}

// HandleUpdateSLAPolicy changes one priority's policy (Manager).
func HandleUpdateSLAPolicy(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		var in service.SLAPolicyInput
		if !decodeJSON(w, r, &in) {
			return
		}
		p, err := d.Incidents.UpdateSLAPolicy(r.Context(), actor, chi.URLParam(r, "priority"), in)
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, p)
	}
}

// HandleUpdateBusinessHours replaces a calendar's working hours (Manager).
func HandleUpdateBusinessHours(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		var body struct {
			Hours []model.BusinessHours `json:"hours"`
		}
		if !decodeJSON(w, r, &body) {
			return
		}
		if err := d.Incidents.UpdateBusinessHours(r.Context(), actor, chi.URLParam(r, "calendar"), body.Hours); err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

// HandleHolidays lists holidays (?year=).
func HandleHolidays(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		year, err := service.ParseYear(r.URL.Query().Get("year"))
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		items, err := d.Incidents.Holidays(r.Context(), actor, year)
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, map[string]any{"data": items})
	}
}

// HandleCreateHoliday adds a holiday (Manager, 201).
func HandleCreateHoliday(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		var in model.Holiday
		if !decodeJSON(w, r, &in) {
			return
		}
		h, err := d.Incidents.CreateHoliday(r.Context(), actor, in)
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusCreated, h)
	}
}

// HandleDeleteHoliday removes a holiday (Manager).
func HandleDeleteHoliday(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		if err := d.Incidents.DeleteHoliday(r.Context(), actor, chi.URLParam(r, "id")); err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}
