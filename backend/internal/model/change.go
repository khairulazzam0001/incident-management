package model

import "time"

// Change Request statuses (PRD_Change_Management.md §7).
const (
	ChangeDraft        = "DRAFT"
	ChangeSubmitted    = "SUBMITTED"
	ChangeApproved     = "APPROVED"
	ChangeScheduled    = "SCHEDULED"
	ChangeImplementing = "IMPLEMENTING"
	ChangeReviewing    = "REVIEWING"
	ChangeClosed       = "CLOSED"
	ChangeRejected     = "REJECTED"
	ChangeCancelled    = "CANCELLED"
)

// Change types (§6.1) and risk levels (§6.2).
const (
	ChangeTypeStandard  = "STANDARD"
	ChangeTypeNormal    = "NORMAL"
	ChangeTypeEmergency = "EMERGENCY"

	RiskLow    = "LOW"
	RiskMedium = "MEDIUM"
	RiskHigh   = "HIGH"
)

// AllowedChangeTransitions maps each status to the statuses it may move to.
// SUBMITTED → DRAFT is "request changes"; SCHEDULED → SCHEDULED is a
// reschedule. APPROVED → IMPLEMENTING is EMERGENCY-only (CanChangeTransition).
var AllowedChangeTransitions = map[string][]string{
	ChangeDraft:        {ChangeSubmitted, ChangeCancelled},
	ChangeSubmitted:    {ChangeApproved, ChangeRejected, ChangeDraft, ChangeCancelled},
	ChangeApproved:     {ChangeScheduled, ChangeImplementing, ChangeCancelled},
	ChangeScheduled:    {ChangeScheduled, ChangeImplementing, ChangeCancelled},
	ChangeImplementing: {ChangeReviewing},
	ChangeReviewing:    {ChangeClosed},
	ChangeClosed:       {},
	ChangeRejected:     {},
	ChangeCancelled:    {},
}

// CanChangeTransition reports whether a change of the given type may move
// from one status to another.
func CanChangeTransition(changeType, from, to string) bool {
	if from == ChangeApproved && to == ChangeImplementing && changeType != ChangeTypeEmergency {
		return false
	}
	for _, next := range AllowedChangeTransitions[from] {
		if next == to {
			return true
		}
	}
	return false
}

// CanViewChanges reports whether the role may see the change module (§5).
func CanViewChanges(role string) bool {
	return role != "" && role != RoleUser
}

// CanCreateChange reports whether the role may create/edit change requests.
func CanCreateChange(role string) bool {
	switch role {
	case RoleSystemAnalyst, RoleDeveloper, RoleDevOps, RoleManagerLead:
		return true
	default:
		return false
	}
}

// ApprovalDenial returns an error code when the actor may not decide on a
// change, or "" when allowed. Single approver ManagerLead; the requester never
// approves their own change; SystemAnalyst may approve LOW/MEDIUM changes
// requested by a ManagerLead (PRD §5, Q1–Q2).
func ApprovalDenial(actorID, actorRole, requesterID, requesterRole, risk string) string {
	if actorID == requesterID {
		return "SELF_APPROVAL_FORBIDDEN"
	}
	if actorRole == RoleManagerLead {
		return ""
	}
	if actorRole == RoleSystemAnalyst && requesterRole == RoleManagerLead &&
		(risk == RiskLow || risk == RiskMedium) {
		return ""
	}
	return "FORBIDDEN_APPROVAL"
}

// Change is the core trans_change record.
type Change struct {
	ID                 string     `json:"id"`
	ChangeNo           string     `json:"change_no"`
	Title              string     `json:"title"`
	Description        string     `json:"description"`
	Justification      string     `json:"justification"`
	Type               string     `json:"type"`
	Risk               string     `json:"risk"`
	Status             string     `json:"status"`
	ApplicationID      string     `json:"application_id"`
	Environment        string     `json:"environment"`
	ImplementationPlan string     `json:"implementation_plan"`
	RollbackPlan       string     `json:"rollback_plan"`
	TestPlan           string     `json:"test_plan"`
	RequesterID        string     `json:"requester_id"`
	ImplementerID      *string    `json:"implementer_id"`
	TeamID             *string    `json:"team_id"`
	Revision           int        `json:"revision"`
	PlannedStart       *time.Time `json:"planned_start"`
	PlannedEnd         *time.Time `json:"planned_end"`
	ActualStart        *time.Time `json:"actual_start"`
	ActualEnd          *time.Time `json:"actual_end"`
	Outcome            *string    `json:"outcome"`
	OutcomeNotes       string     `json:"outcome_notes"`
	ReviewNotes        string     `json:"review_notes"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	ClosedAt           *time.Time `json:"closed_at"`
	ClosedBy           *string    `json:"closed_by"`
}

// ChangeInput is the payload for POST /api/changes and PATCH /api/changes/:id.
// IncidentID (create only) links the new change as FIX_FOR that incident.
type ChangeInput struct {
	Title              string  `json:"title"`
	Description        string  `json:"description"`
	Justification      string  `json:"justification"`
	Type               string  `json:"type"`
	Risk               string  `json:"risk"`
	ApplicationID      string  `json:"application_id"`
	Environment        string  `json:"environment"`
	ImplementationPlan string  `json:"implementation_plan"`
	RollbackPlan       string  `json:"rollback_plan"`
	TestPlan           string  `json:"test_plan"`
	ImplementerID      *string `json:"implementer_id"`
	TeamID             *string `json:"team_id"`
	IncidentID         *string `json:"incident_id"`
}

// Approval decisions stored in trans_change_approval.
const (
	DecisionApproved         = "APPROVED"
	DecisionRejected         = "REJECTED"
	DecisionChangesRequested = "CHANGES_REQUESTED"
	DecisionAutoApproved     = "AUTO_APPROVED"
)

// ChangeApproval is a trans_change_approval row.
type ChangeApproval struct {
	ID         string    `json:"id"`
	ChangeID   string    `json:"change_id"`
	ApproverID *string   `json:"approver_id"`
	Decision   string    `json:"decision"`
	Reason     string    `json:"reason"`
	Revision   int       `json:"revision"`
	CreatedAt  time.Time `json:"created_at"`
}

// ChangeActivity is a trans_change_activity row (change audit trail).
type ChangeActivity struct {
	ID         string    `json:"id"`
	ChangeID   string    `json:"change_id"`
	Type       string    `json:"type"`
	ActorID    *string   `json:"actor_id"`
	FromStatus *string   `json:"from_status"`
	ToStatus   *string   `json:"to_status"`
	Payload    any       `json:"payload"`
	CreatedAt  time.Time `json:"created_at"`
}

// Change activity types.
const (
	ChangeActivityCreated      = "created"
	ChangeActivityUpdated      = "updated"
	ChangeActivityStatusChange = "status_change"
	ChangeActivityApproval     = "approval"
	ChangeActivityComment      = "comment"
	ChangeActivityIncidentLink = "incident_link"

	// ActivityChangeLink is written to trans_incident_activity.
	ActivityChangeLink = "change_link"
)

// ChangeComment is a trans_change_comment row.
type ChangeComment struct {
	ID        string    `json:"id"`
	ChangeID  string    `json:"change_id"`
	AuthorID  *string   `json:"author_id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

// Change ↔ incident relations (§8.1).
const (
	RelationFixFor   = "FIX_FOR"
	RelationCausedBy = "CAUSED_BY"
)

// ChangeIncidentLink is a trans_change_incident row joined with the summary
// fields both detail pages need.
type ChangeIncidentLink struct {
	ID             string    `json:"id"`
	ChangeID       string    `json:"change_id"`
	ChangeNo       string    `json:"change_no"`
	ChangeTitle    string    `json:"change_title"`
	ChangeStatus   string    `json:"change_status"`
	IncidentID     string    `json:"incident_id"`
	IncidentNo     string    `json:"incident_no"`
	IncidentTitle  string    `json:"incident_title"`
	IncidentStatus string    `json:"incident_status"`
	Relation       string    `json:"relation"`
	CreatedBy      *string   `json:"created_by"`
	CreatedAt      time.Time `json:"created_at"`
}

// Implementation outcomes (CM-05).
const (
	OutcomeSuccess    = "SUCCESS"
	OutcomeFailed     = "FAILED"
	OutcomeRolledBack = "ROLLED_BACK"
)

// RequiresPIR reports whether closing needs post-implementation review notes
// (EMERGENCY or a non-successful outcome, CM-06).
func RequiresPIR(c *Change) bool {
	return c.Type == ChangeTypeEmergency || c.Outcome == nil || *c.Outcome != OutcomeSuccess
}

// Change activity types added in CM-2.
const (
	ChangeActivitySchedule = "schedule"
	ChangeActivityStart    = "start"
	ChangeActivityComplete = "complete"
	ChangeActivityClose    = "close"
)

// Change notification types (§13).
const (
	NotifChangeSubmitted        = "change_submitted"
	NotifChangeApproved         = "change_approved"
	NotifChangeRejected         = "change_rejected"
	NotifChangeChangesRequested = "change_changes_requested"
	NotifChangeScheduled        = "change_scheduled"
	NotifChangeStarted          = "change_started"
	NotifChangeFailed           = "change_failed"
	NotifChangeClosed           = "change_closed"
	NotifChangeCancelled        = "change_cancelled"
	NotifChangeCausedIncident   = "change_caused_incident"
)

// ChangeActivityAttachment records an uploaded file on the change timeline.
const ChangeActivityAttachment = "attachment"

// ChangeAttachment is a trans_change_attachment row (CM-FR-13). The storage
// key never leaves the server.
type ChangeAttachment struct {
	ID         string    `json:"id"`
	ChangeID   string    `json:"change_id"`
	FileName   string    `json:"file_name"`
	MimeType   string    `json:"mime_type"`
	SizeBytes  int64     `json:"size_bytes"`
	UploadedBy *string   `json:"uploaded_by"`
	CreatedAt  time.Time `json:"created_at"`
}
