// Package model defines domain types for the incident lifecycle (PRD §6–§8, §11).
package model

import "time"

// Incident statuses (PRD §6).
const (
	StatusNew           = "NEW"
	StatusAssigned      = "ASSIGNED"
	StatusInvestigating = "INVESTIGATING"
	StatusFixing        = "FIXING"
	StatusVerifying     = "VERIFYING"
	StatusResolved      = "RESOLVED"
	StatusClosed        = "CLOSED"
)

// AllowedTransitions maps each status to the statuses it may move to (PRD §6).
// VERIFYING returns to FIXING on verification FAIL; CLOSED is terminal
// except through the special reopen action (FR-12).
var AllowedTransitions = map[string][]string{
	StatusNew:           {StatusAssigned},
	StatusAssigned:      {StatusInvestigating},
	StatusInvestigating: {StatusFixing},
	StatusFixing:        {StatusVerifying},
	StatusVerifying:     {StatusResolved, StatusFixing},
	StatusResolved:      {StatusClosed},
	StatusClosed:        {},
}

// CanTransition reports whether moving from one status to another is allowed.
func CanTransition(from, to string) bool {
	for _, next := range AllowedTransitions[from] {
		if next == to {
			return true
		}
	}
	return false
}

// Incident is the core record (trans_incident).
type Incident struct {
	ID            string     `json:"id"`
	IncidentNo    string     `json:"incident_no"`
	Title         string     `json:"title"`
	Description   string     `json:"description"`
	Source        string     `json:"source"`
	Severity      string     `json:"severity"`
	Priority      string     `json:"priority"`
	Status        string     `json:"status"`
	ApplicationID *string    `json:"application_id"`
	Environment   *string    `json:"environment"`
	ReporterID    *string    `json:"reporter_id"`
	TeamID        *string    `json:"team_id"`
	PICID         *string    `json:"pic_id"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	ResolvedAt    *time.Time `json:"resolved_at"`
	ClosedAt      *time.Time `json:"closed_at"`
	ClosedBy      *string    `json:"closed_by"`
}

// CreateIncidentInput is the payload for POST /api/incidents.
type CreateIncidentInput struct {
	Title         string  `json:"title"`
	Description   string  `json:"description"`
	Source        string  `json:"source"`
	Severity      string  `json:"severity"`
	Priority      string  `json:"priority"`
	ApplicationID *string `json:"application_id"`
	Environment   *string `json:"environment"`
}

// Comment is a trans_incident_comment row.
type Comment struct {
	ID         string    `json:"id"`
	IncidentID string    `json:"incident_id"`
	AuthorID   *string   `json:"author_id"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"created_at"`
}

// Activity is a trans_incident_activity row (audit trail / timeline).
type Activity struct {
	ID         string    `json:"id"`
	IncidentID string    `json:"incident_id"`
	Type       string    `json:"type"`
	ActorID    *string   `json:"actor_id"`
	FromStatus *string   `json:"from_status"`
	ToStatus   *string   `json:"to_status"`
	Payload    any       `json:"payload"`
	CreatedAt  time.Time `json:"created_at"`
}

// Activity types.
const (
	ActivityCreated       = "created"
	ActivityStatusChange  = "status_change"
	ActivityAssignment    = "assignment"
	ActivityComment       = "comment"
	ActivityInvestigation = "investigation"
	ActivityFix           = "fix"
	ActivityVerification  = "verification"
	ActivityReopen        = "reopen"
)

// Investigation is a trans_incident_investigation row (US-03/FR-05).
type Investigation struct {
	ID         string    `json:"id"`
	IncidentID string    `json:"incident_id"`
	AuthorID   *string   `json:"author_id"`
	Notes      string    `json:"notes"`
	Findings   string    `json:"findings"`
	CreatedAt  time.Time `json:"created_at"`
}

// Fix is a trans_incident_fix row (US-04/FR-06).
type Fix struct {
	ID          string    `json:"id"`
	IncidentID  string    `json:"incident_id"`
	AuthorID    *string   `json:"author_id"`
	Description string    `json:"description"`
	Reference   string    `json:"reference"`
	CreatedAt   time.Time `json:"created_at"`
}

// Verification results (US-05/FR-07).
const (
	VerificationPass = "PASS"
	VerificationFail = "FAIL"
)

// Verification is a trans_incident_verification row.
type Verification struct {
	ID         string    `json:"id"`
	IncidentID string    `json:"incident_id"`
	VerifierID *string   `json:"verifier_id"`
	Result     string    `json:"result"`
	Reason     string    `json:"reason"`
	CreatedAt  time.Time `json:"created_at"`
}
