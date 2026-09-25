// Package handler wires HTTP routes to handlers. Handlers stay thin:
// parse request, resolve actor, call service, write JSON response.
package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/khairulazzam0001/incident-management/backend/internal/apierror"
	"github.com/khairulazzam0001/incident-management/backend/internal/middleware"
	"github.com/khairulazzam0001/incident-management/backend/internal/model"
	"github.com/khairulazzam0001/incident-management/backend/internal/service"
)

// Deps bundles services for route constructors.
type Deps struct {
	Log            *slog.Logger
	Auth           *service.AuthService
	Incidents      *service.IncidentService
	AllowedOrigins []string
}

// NewRouter builds the chi router with global middleware and routes.
func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.CORS(d.AllowedOrigins))
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger(d.Log))
	r.Use(middleware.Recovery)

	r.Get("/health", Health)
	r.Post("/api/auth/login", HandleLogin(d))

	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireAuth(d.Auth.Parse))
		r.Get("/api/meta", HandleMeta(d))
		r.Get("/api/users", HandleUsers(d))
		r.Post("/api/incidents", HandleCreateIncident(d))
		r.Get("/api/incidents", HandleListIncidents(d))
		r.Get("/api/incidents/{id}", HandleGetIncident(d))
		r.Patch("/api/incidents/{id}/status", HandleChangeStatus(d))
		r.Patch("/api/incidents/{id}/assignment", HandleAssign(d))
		r.Post("/api/incidents/{id}/comments", HandleAddComment(d))
		r.Get("/api/incidents/{id}/comments", HandleComments(d))
		r.Post("/api/incidents/{id}/investigations", HandleAddInvestigation(d))
		r.Get("/api/incidents/{id}/investigations", HandleInvestigations(d))
		r.Post("/api/incidents/{id}/fixes", HandleAddFix(d))
		r.Get("/api/incidents/{id}/fixes", HandleFixes(d))
		r.Post("/api/incidents/{id}/verification", HandleVerify(d))
		r.Get("/api/incidents/{id}/verifications", HandleVerifications(d))
		r.Post("/api/incidents/{id}/close", HandleClose(d))
		r.Post("/api/incidents/{id}/reopen", HandleReopen(d))
		r.Get("/api/incidents/{id}/timeline", HandleTimeline(d))
	})

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		apierror.Write(w, r, http.StatusNotFound, "NOT_FOUND", "Endpoint tidak ditemukan.", nil)
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		apierror.Write(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method tidak diizinkan.", nil)
	})
	return r
}

// Health reports liveness. A DB-aware check follows in hardening.
func Health(w http.ResponseWriter, _ *http.Request) {
	apierror.WriteJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "incident-management-api",
	})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		apierror.Write(w, r, http.StatusBadRequest, "INVALID_JSON", "Body request bukan JSON valid.", nil)
		return false
	}
	return true
}

func writeServiceError(d Deps, w http.ResponseWriter, r *http.Request, err error) {
	var svcErr *service.Error
	if errors.As(err, &svcErr) {
		apierror.Write(w, r, svcErr.Status, svcErr.Code, svcErr.Message, svcErr.Details)
		return
	}
	d.Log.Error("internal error", "request_id", apierror.RequestIDFrom(r.Context()), "error", err)
	apierror.Write(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Terjadi kesalahan internal.", nil)
}

func actorOf(d Deps, w http.ResponseWriter, r *http.Request) (*model.AuthUser, bool) {
	u, ok := middleware.AuthUserFrom(r.Context())
	if !ok || u == nil {
		apierror.Write(w, r, http.StatusUnauthorized, "MISSING_TOKEN", "Token autentikasi wajib diisi.", nil)
		return nil, false
	}
	return u, true
}
