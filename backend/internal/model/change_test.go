package model

import "testing"

// TestCanChangeTransition pins the PRD_Change_Management.md §7 lifecycle.
func TestCanChangeTransition(t *testing.T) {
	tests := []struct {
		typ, from, to string
		want          bool
	}{
		{ChangeTypeNormal, ChangeDraft, ChangeSubmitted, true},
		{ChangeTypeNormal, ChangeDraft, ChangeCancelled, true},
		{ChangeTypeNormal, ChangeDraft, ChangeApproved, false},
		{ChangeTypeNormal, ChangeSubmitted, ChangeApproved, true},
		{ChangeTypeNormal, ChangeSubmitted, ChangeRejected, true},
		{ChangeTypeNormal, ChangeSubmitted, ChangeDraft, true},
		{ChangeTypeNormal, ChangeApproved, ChangeScheduled, true},
		{ChangeTypeNormal, ChangeApproved, ChangeImplementing, false},
		{ChangeTypeEmergency, ChangeApproved, ChangeImplementing, true},
		{ChangeTypeNormal, ChangeScheduled, ChangeScheduled, true},
		{ChangeTypeNormal, ChangeScheduled, ChangeImplementing, true},
		{ChangeTypeNormal, ChangeImplementing, ChangeCancelled, false},
		{ChangeTypeNormal, ChangeImplementing, ChangeReviewing, true},
		{ChangeTypeNormal, ChangeReviewing, ChangeClosed, true},
		{ChangeTypeNormal, ChangeClosed, ChangeDraft, false},
		{ChangeTypeNormal, ChangeRejected, ChangeSubmitted, false},
		{ChangeTypeNormal, ChangeCancelled, ChangeDraft, false},
	}
	for _, tt := range tests {
		if got := CanChangeTransition(tt.typ, tt.from, tt.to); got != tt.want {
			t.Errorf("CanChangeTransition(%s, %s → %s) = %v, want %v", tt.typ, tt.from, tt.to, got, tt.want)
		}
	}
}

// TestApprovalDenial pins single approver + segregation of duties (Q1–Q2).
func TestApprovalDenial(t *testing.T) {
	tests := []struct {
		name                               string
		actor, actorRole, req, reqRole, rk string
		want                               string
	}{
		{"manager approves developer", "m", RoleManagerLead, "d", RoleDeveloper, RiskHigh, ""},
		{"requester self-approve", "m", RoleManagerLead, "m", RoleManagerLead, RiskLow, "SELF_APPROVAL_FORBIDDEN"},
		{"analyst approves manager LOW", "a", RoleSystemAnalyst, "m", RoleManagerLead, RiskLow, ""},
		{"analyst approves manager MEDIUM", "a", RoleSystemAnalyst, "m", RoleManagerLead, RiskMedium, ""},
		{"analyst approves manager HIGH", "a", RoleSystemAnalyst, "m", RoleManagerLead, RiskHigh, "FORBIDDEN_APPROVAL"},
		{"analyst approves developer", "a", RoleSystemAnalyst, "d", RoleDeveloper, RiskLow, "FORBIDDEN_APPROVAL"},
		{"developer approves", "x", RoleDeveloper, "d", RoleDeveloper, RiskLow, "FORBIDDEN_APPROVAL"},
		{"qa approves", "q", RoleQA, "m", RoleManagerLead, RiskLow, "FORBIDDEN_APPROVAL"},
	}
	for _, tt := range tests {
		if got := ApprovalDenial(tt.actor, tt.actorRole, tt.req, tt.reqRole, tt.rk); got != tt.want {
			t.Errorf("%s: ApprovalDenial = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestChangeRoles(t *testing.T) {
	if CanViewChanges(RoleUser) || !CanViewChanges(RoleHelpDesk) || !CanViewChanges(RoleQA) {
		t.Error("CanViewChanges: User dilarang, HelpDesk/QA boleh")
	}
	for _, r := range []string{RoleSystemAnalyst, RoleDeveloper, RoleDevOps, RoleManagerLead} {
		if !CanCreateChange(r) {
			t.Errorf("CanCreateChange(%s) = false", r)
		}
	}
	for _, r := range []string{RoleUser, RoleHelpDesk, RoleQA} {
		if CanCreateChange(r) {
			t.Errorf("CanCreateChange(%s) = true", r)
		}
	}
}
