package model

import "testing"

// TestCanTransition pins the PRD §6 lifecycle map.
func TestCanTransition(t *testing.T) {
	legal := [][2]string{
		{"NEW", "ASSIGNED"},
		{"ASSIGNED", "INVESTIGATING"},
		{"INVESTIGATING", "FIXING"},
		{"FIXING", "VERIFYING"},
		{"VERIFYING", "RESOLVED"},
		{"VERIFYING", "FIXING"},
		{"RESOLVED", "CLOSED"},
	}
	for _, tt := range legal {
		if !CanTransition(tt[0], tt[1]) {
			t.Errorf("CanTransition(%q, %q) = false, want true", tt[0], tt[1])
		}
	}

	illegal := [][2]string{
		{"NEW", "INVESTIGATING"},
		{"NEW", "CLOSED"},
		{"ASSIGNED", "RESOLVED"},
		{"INVESTIGATING", "RESOLVED"},
		{"FIXING", "RESOLVED"},
		{"VERIFYING", "CLOSED"},
		{"RESOLVED", "FIXING"},
		{"CLOSED", "NEW"},
		{"CLOSED", "ASSIGNED"},
		{"NEW", "BOGUS"},
		{"BOGUS", "NEW"},
	}
	for _, tt := range illegal {
		if CanTransition(tt[0], tt[1]) {
			t.Errorf("CanTransition(%q, %q) = true, want false", tt[0], tt[1])
		}
	}
}
