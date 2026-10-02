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

// Change Management handlers (PRD_Change_Management.md §15).

// HandleCreateChange creates a DRAFT change request (201).
func HandleCreateChange(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		var in model.ChangeInput
		if !decodeJSON(w, r, &in) {
			return
		}
		created, err := d.Incidents.CreateChange(r.Context(), actor, in)
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusCreated, created)
	}
}

// HandleListChanges lists/filters change requests with pagination.
func HandleListChanges(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
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
		f := repository.ChangeFilter{
			Status:        q.Get("status"),
			Type:          q.Get("type"),
			Risk:          q.Get("risk"),
			ApplicationID: q.Get("application_id"),
			Environment:   q.Get("environment"),
			RequesterID:   q.Get("requester"),
			ImplementerID: q.Get("implementer"),
			ScheduledFrom: q.Get("scheduled_from"),
			ScheduledTo:   q.Get("scheduled_to"),
			Q:             q.Get("q"),
			Sort:          q.Get("sort"),
			Page:          page,
			Limit:         limit,
		}
		items, total, err := d.Incidents.ListChanges(r.Context(), actor, f)
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		if f.Page <= 0 {
			f.Page = 1
		}
		if f.Limit <= 0 {
			f.Limit = 20
		}
		apierror.WriteJSON(w, http.StatusOK, map[string]any{
			"data": items,
			"meta": pageMeta{Page: f.Page, Limit: f.Limit, Total: total},
		})
	}
}

// HandleGetChange returns one change.
func HandleGetChange(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		c, err := d.Incidents.GetChange(r.Context(), actor, chi.URLParam(r, "id"))
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, c)
	}
}

// HandleUpdateChange edits a DRAFT change.
func HandleUpdateChange(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		var in model.ChangeInput
		if !decodeJSON(w, r, &in) {
			return
		}
		c, err := d.Incidents.UpdateChange(r.Context(), actor, chi.URLParam(r, "id"), in)
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, c)
	}
}

// HandleSubmitChange moves DRAFT → SUBMITTED (STANDARD → APPROVED).
func HandleSubmitChange(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		c, err := d.Incidents.SubmitChange(r.Context(), actor, chi.URLParam(r, "id"))
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, c)
	}
}

// HandleDecideChange records an approval decision.
func HandleDecideChange(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		var in service.ApprovalInput
		if !decodeJSON(w, r, &in) {
			return
		}
		c, err := d.Incidents.DecideChange(r.Context(), actor, chi.URLParam(r, "id"), in)
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, c)
	}
}

// HandleCancelChange cancels a change before implementation.
func HandleCancelChange(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		var body struct {
			Reason string `json:"reason"`
		}
		if !decodeJSON(w, r, &body) {
			return
		}
		c, err := d.Incidents.CancelChange(r.Context(), actor, chi.URLParam(r, "id"), body.Reason)
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, c)
	}
}

// HandleChangeApprovals returns the approval history.
func HandleChangeApprovals(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		items, err := d.Incidents.ChangeApprovals(r.Context(), actor, chi.URLParam(r, "id"))
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, map[string]any{"data": items})
	}
}

// HandleChangeTimeline returns the change audit trail.
func HandleChangeTimeline(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		items, err := d.Incidents.ChangeTimeline(r.Context(), actor, chi.URLParam(r, "id"))
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, map[string]any{"data": items})
	}
}

// HandleAddChangeComment adds a comment to a change (201).
func HandleAddChangeComment(d Deps) http.HandlerFunc {
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
		c, err := d.Incidents.AddChangeComment(r.Context(), actor, chi.URLParam(r, "id"), body.Body)
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusCreated, c)
	}
}

// HandleChangeComments returns change comments.
func HandleChangeComments(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		items, err := d.Incidents.ChangeComments(r.Context(), actor, chi.URLParam(r, "id"))
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, map[string]any{"data": items})
	}
}

// HandleChangeIncidents returns incidents linked to a change.
func HandleChangeIncidents(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		items, err := d.Incidents.ChangeIncidents(r.Context(), actor, chi.URLParam(r, "id"))
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, map[string]any{"data": items})
	}
}

// HandleIncidentChanges returns changes linked to an incident.
func HandleIncidentChanges(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		items, err := d.Incidents.IncidentChanges(r.Context(), actor, chi.URLParam(r, "id"))
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, map[string]any{"data": items})
	}
}

// HandleScheduleChange sets the planned window; returns change + conflicts.
func HandleScheduleChange(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		var in service.ScheduleInput
		if !decodeJSON(w, r, &in) {
			return
		}
		res, err := d.Incidents.ScheduleChange(r.Context(), actor, chi.URLParam(r, "id"), in)
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, res)
	}
}

// HandleStartChange moves the change to IMPLEMENTING.
func HandleStartChange(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		c, err := d.Incidents.StartChange(r.Context(), actor, chi.URLParam(r, "id"))
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, c)
	}
}

// HandleCompleteChange records the implementation outcome.
func HandleCompleteChange(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		var in service.CompleteInput
		if !decodeJSON(w, r, &in) {
			return
		}
		c, err := d.Incidents.CompleteChange(r.Context(), actor, chi.URLParam(r, "id"), in)
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, c)
	}
}

// HandleCloseChange closes a reviewed change (PIR).
func HandleCloseChange(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		var body struct {
			ReviewNotes string `json:"review_notes"`
		}
		if !decodeJSON(w, r, &body) {
			return
		}
		c, err := d.Incidents.CloseChange(r.Context(), actor, chi.URLParam(r, "id"), body.ReviewNotes)
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, c)
	}
}

// HandleLinkIncident links an incident to a change (201, returns all links).
func HandleLinkIncident(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		var in service.LinkInput
		if !decodeJSON(w, r, &in) {
			return
		}
		items, err := d.Incidents.LinkIncident(r.Context(), actor, chi.URLParam(r, "id"), in)
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusCreated, map[string]any{"data": items})
	}
}

// HandleUnlinkIncident removes a link; relation comes from ?relation=.
func HandleUnlinkIncident(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		err := d.Incidents.UnlinkIncident(r.Context(), actor, chi.URLParam(r, "id"),
			chi.URLParam(r, "incidentId"), r.URL.Query().Get("relation"))
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

// HandleChangeSummary returns the change dashboard aggregates.
func HandleChangeSummary(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		sum, err := d.Incidents.ChangeSummary(r.Context(), actor)
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, sum)
	}
}

// HandleRecentChanges returns recent changes on the incident's app/env.
func HandleRecentChanges(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		items, err := d.Incidents.RecentChanges(r.Context(), actor, chi.URLParam(r, "id"))
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, map[string]any{"data": items})
	}
}
