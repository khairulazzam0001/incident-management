package model

// Roles (PRD §5).
const (
	RoleUser          = "User"
	RoleHelpDesk      = "HelpDesk"
	RoleSystemAnalyst = "SystemAnalyst"
	RoleDeveloper     = "Developer"
	RoleQA            = "QA"
	RoleDevOps        = "DevOps"
	RoleManagerLead   = "ManagerLead"
)

// IsCoordinator reports whether the role may assign incidents and act on
// incidents they don't own (HelpDesk triage, Analyst coordination,
// ManagerLead oversight).
func IsCoordinator(role string) bool {
	switch role {
	case RoleHelpDesk, RoleSystemAnalyst, RoleManagerLead:
		return true
	default:
		return false
	}
}

// User is a master_user row (password hash never leaves the repository).
type User struct {
	ID       string  `json:"id"`
	Email    string  `json:"email"`
	Name     string  `json:"name"`
	Role     string  `json:"role"`
	TeamID   *string `json:"team_id"`
	IsActive bool    `json:"is_active"`
}

// AuthUser is the authenticated principal carried in the request context.
type AuthUser struct {
	ID    string
	Email string
	Name  string
	Role  string
}

// MasterItem is a code-based master row (status, severity, priority,
// environment, source).
type MasterItem struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// MasterIDItem is an id-based master row (application, team).
type MasterIDItem struct {
	ID   string `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

// Meta bundles all master data needed by forms and filters.
type Meta struct {
	Statuses     []MasterItem   `json:"statuses"`
	Severities   []MasterItem   `json:"severities"`
	Priorities   []MasterItem   `json:"priorities"`
	Environments []MasterItem   `json:"environments"`
	Sources      []MasterItem   `json:"sources"`
	Applications []MasterIDItem `json:"applications"`
	Teams        []MasterIDItem `json:"teams"`
}
