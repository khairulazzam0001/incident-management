package model

import (
	"encoding/json"
	"time"
)

// Monitoring Integration (PRD_Monitoring_Integration.md).

// CanManageIntegrations: ManagerLead and DevOps own monitoring integrations (§5).
func CanManageIntegrations(role string) bool {
	return role == RoleManagerLead || role == RoleDevOps
}

// Integration is a master_integration row. The token hash never leaves the
// repository; only the last four characters are exposed.
type Integration struct {
	ID                     string          `json:"id"`
	Type                   string          `json:"type"`
	Name                   string          `json:"name"`
	TokenLast4             string          `json:"token_last4"`
	IsActive               bool            `json:"is_active"`
	DefaultApplicationID   *string         `json:"default_application_id"`
	DefaultEnvironmentCode *string         `json:"default_environment"`
	MinSeverity            string          `json:"min_severity"`
	Mapping                json.RawMessage `json:"mapping"`
	IPAllowlist            []string        `json:"ip_allowlist"`
	RateLimitPerMin        int             `json:"rate_limit_per_min"`
	CreatedBy              *string         `json:"created_by"`
	CreatedAt              time.Time       `json:"created_at"`
	RotatedAt              *time.Time      `json:"rotated_at"`
	LastEventAt            *time.Time      `json:"last_event_at"`
	Events24h              int             `json:"events_24h"`
}

// IntegrationInput is the payload for creating/updating an integration.
type IntegrationInput struct {
	Type                 string          `json:"type"`
	Name                 string          `json:"name"`
	IsActive             *bool           `json:"is_active"`
	DefaultApplicationID *string         `json:"default_application_id"`
	DefaultEnvironment   *string         `json:"default_environment"`
	MinSeverity          string          `json:"min_severity"`
	Mapping              json.RawMessage `json:"mapping"`
	IPAllowlist          []string        `json:"ip_allowlist"`
	RateLimitPerMin      int             `json:"rate_limit_per_min"`
}

// AlertEvent is a trans_alert_event row (raw audit of every received alert).
type AlertEvent struct {
	ID          string          `json:"id"`
	ReceivedAt  time.Time       `json:"received_at"`
	Fingerprint string          `json:"fingerprint"`
	Status      string          `json:"status"`
	Result      string          `json:"result"`
	Warning     string          `json:"warning"`
	Error       string          `json:"error"`
	IncidentID  *string         `json:"incident_id"`
	IncidentNo  *string         `json:"incident_no"`
	Payload     json.RawMessage `json:"payload"`
}

// IncidentAlert is a trans_incident_alert row joined with the integration name.
type IncidentAlert struct {
	ID              string            `json:"id"`
	IncidentID      string            `json:"incident_id"`
	IntegrationID   string            `json:"integration_id"`
	IntegrationName string            `json:"integration_name"`
	Fingerprint     string            `json:"fingerprint"`
	Title           string            `json:"title"`
	Labels          map[string]string `json:"labels"`
	SourceURL       string            `json:"source_url"`
	FirstSeen       time.Time         `json:"first_seen"`
	LastSeen        time.Time         `json:"last_seen"`
	FireCount       int               `json:"fire_count"`
	LastStatus      string            `json:"last_status"`
}

// Incident activity & notification types for monitoring alerts (§9, §11).
const (
	ActivityAlertCreated    = "alert_created"
	ActivityAlertRefired    = "alert_refired"
	ActivityAlertResolved   = "alert_resolved"
	ActivityAlertReoccurred = "alert_reoccurred"
	ActivityAlertGrouped    = "alert_grouped"

	NotifAlertStorm      = "alert_storm"
	NotifAlertReoccurred = "alert_reoccurred"
)
