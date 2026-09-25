package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/khairulazzam0001/incident-management/backend/internal/apierror"
	"github.com/khairulazzam0001/incident-management/backend/internal/model"
	"github.com/khairulazzam0001/incident-management/backend/internal/service"
)

// HandleAddInvestigation records investigation notes/findings (201).
func HandleAddInvestigation(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		var in service.InvestigationInput
		if !decodeJSON(w, r, &in) {
			return
		}
		inv, err := d.Incidents.AddInvestigation(r.Context(), actor, chi.URLParam(r, "id"), in)
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusCreated, inv)
	}
}

// HandleInvestigations returns the investigation records.
func HandleInvestigations(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := d.Incidents.Investigations(r.Context(), chi.URLParam(r, "id"))
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		if items == nil {
			items = []*model.Investigation{}
		}
		apierror.WriteJSON(w, http.StatusOK, map[string]any{"data": items})
	}
}

// HandleAddFix records a fix (201).
func HandleAddFix(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		var in service.FixInput
		if !decodeJSON(w, r, &in) {
			return
		}
		f, err := d.Incidents.AddFix(r.Context(), actor, chi.URLParam(r, "id"), in)
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusCreated, f)
	}
}

// HandleFixes returns the fix records.
func HandleFixes(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := d.Incidents.Fixes(r.Context(), chi.URLParam(r, "id"))
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		if items == nil {
			items = []*model.Fix{}
		}
		apierror.WriteJSON(w, http.StatusOK, map[string]any{"data": items})
	}
}

// HandleVerify records PASS/FAIL verification (200, returns the record;
// fetch the incident for its new status).
func HandleVerify(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		var in service.VerificationInput
		if !decodeJSON(w, r, &in) {
			return
		}
		v, err := d.Incidents.Verify(r.Context(), actor, chi.URLParam(r, "id"), in)
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, v)
	}
}

// HandleVerifications returns the verification records.
func HandleVerifications(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := d.Incidents.Verifications(r.Context(), chi.URLParam(r, "id"))
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		if items == nil {
			items = []*model.Verification{}
		}
		apierror.WriteJSON(w, http.StatusOK, map[string]any{"data": items})
	}
}

// HandleClose closes a RESOLVED incident (200).
func HandleClose(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		closed, err := d.Incidents.Close(r.Context(), actor, chi.URLParam(r, "id"))
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, closed)
	}
}

// HandleReopen reopens RESOLVED/CLOSED back to INVESTIGATING (200).
func HandleReopen(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		var in service.ReopenInput
		if !decodeJSON(w, r, &in) {
			return
		}
		reopened, err := d.Incidents.Reopen(r.Context(), actor, chi.URLParam(r, "id"), in)
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, reopened)
	}
}
