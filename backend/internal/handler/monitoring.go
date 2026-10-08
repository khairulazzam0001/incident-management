package handler

import (
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/khairulazzam0001/incident-management/backend/internal/apierror"
	"github.com/khairulazzam0001/incident-management/backend/internal/model"
)

// Monitoring Integration handlers (PRD_Monitoring_Integration.md §10).

// alertBodyLimit is the webhook body cap (§6.1).
const alertBodyLimit = 1 << 20

// HandleAlertWebhook receives monitoring alerts. It sits outside the JWT
// group: the integration token (Bearer) authenticates it.
func HandleAlertWebhook(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, alertBodyLimit)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			var tooBig *http.MaxBytesError
			if errors.As(err, &tooBig) {
				apierror.Write(w, r, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "Body maksimal 1 MiB.", nil)
				return
			}
			apierror.Write(w, r, http.StatusBadRequest, "INVALID_PAYLOAD", "Body tidak bisa dibaca.", nil)
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		res, err := d.Incidents.ReceiveAlerts(r.Context(), chi.URLParam(r, "id"), token, host, body)
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusAccepted, res)
	}
}

// HandleIntegrations lists integrations (Manager/DevOps).
func HandleIntegrations(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		items, err := d.Incidents.Integrations(r.Context(), actor)
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, map[string]any{"data": items})
	}
}

// HandleCreateIntegration creates an integration; the token is returned once (201).
func HandleCreateIntegration(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		var in model.IntegrationInput
		if !decodeJSON(w, r, &in) {
			return
		}
		res, err := d.Incidents.CreateIntegration(r.Context(), actor, in)
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusCreated, res)
	}
}

// HandleUpdateIntegration edits an integration.
func HandleUpdateIntegration(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		var in model.IntegrationInput
		if !decodeJSON(w, r, &in) {
			return
		}
		it, err := d.Incidents.UpdateIntegration(r.Context(), actor, chi.URLParam(r, "id"), in)
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, it)
	}
}

// HandleRotateIntegrationToken issues a new token (returned once).
func HandleRotateIntegrationToken(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		token, err := d.Incidents.RotateIntegrationToken(r.Context(), actor, chi.URLParam(r, "id"))
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, map[string]string{"token": token})
	}
}

// HandleTestIntegration dry-runs a sample payload (no incident is created).
func HandleTestIntegration(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, alertBodyLimit)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			apierror.Write(w, r, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "Body maksimal 1 MiB.", nil)
			return
		}
		items, err := d.Incidents.TestIntegration(r.Context(), actor, chi.URLParam(r, "id"), body)
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, map[string]any{"data": items})
	}
}

// HandleAlertEvents returns an integration's event log page (?result=&page=&limit=).
func HandleAlertEvents(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		q := r.URL.Query()
		page, _ := strconv.Atoi(q.Get("page"))
		limit, _ := strconv.Atoi(q.Get("limit"))
		items, total, err := d.Incidents.AlertEvents(r.Context(), actor, chi.URLParam(r, "id"), q.Get("result"), page, limit)
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		if page <= 0 {
			page = 1
		}
		if limit <= 0 || limit > 100 {
			limit = 20
		}
		apierror.WriteJSON(w, http.StatusOK, map[string]any{
			"data": items, "meta": pageMeta{Page: page, Limit: limit, Total: total},
		})
	}
}

// HandleIncidentAlerts returns alerts linked to an incident.
func HandleIncidentAlerts(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorOf(d, w, r)
		if !ok {
			return
		}
		items, err := d.Incidents.IncidentAlerts(r.Context(), actor, chi.URLParam(r, "id"))
		if err != nil {
			writeServiceError(d, w, r, err)
			return
		}
		apierror.WriteJSON(w, http.StatusOK, map[string]any{"data": items})
	}
}
