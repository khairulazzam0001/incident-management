package handler

import (
	"net/http"
	"testing"
)

// TestMyIncidentsInvolved: My Incidents (PRD §9) = incident yang saya
// laporkan ATAU yang ditugaskan kepada saya (filter involved=me).
func TestMyIncidentsInvolved(t *testing.T) {
	pool := testDB(t)
	router := testRouter(t, pool)
	reset(t, pool)

	customer := login(t, router, "user@example.com")
	helpdesk := login(t, router, "helpdesk@example.com")
	dev := login(t, router, "developer@example.com")
	devID := userID(t, pool, "developer@example.com")
	team := teamID(t, pool)

	total := func(token, query string) float64 {
		t.Helper()
		rec := doRequest(t, router, http.MethodGet, "/api/incidents"+query, token, nil)
		expectStatus(t, rec, http.StatusOK, "list "+query)
		return decodeBody(t, rec)["meta"].(map[string]any)["total"].(float64)
	}

	rec := doRequest(t, router, http.MethodPost, "/api/incidents", customer, map[string]any{
		"title": "Checkout Klob error 500", "severity": "S2", "priority": "P2", "source": "user",
	})
	expectStatus(t, rec, http.StatusCreated, "customer create")
	id, _ := decodeBody(t, rec)["id"].(string)

	// Pelapor: muncul di involved=me, tidak di assignee=me (perilaku lama yang menjadi bug).
	if n := total(customer, "?involved=me"); n != 1 {
		t.Fatalf("customer involved=me = %v, want 1", n)
	}
	if n := total(customer, "?assignee=me"); n != 0 {
		t.Fatalf("customer assignee=me = %v, want 0", n)
	}

	// PIC: muncul setelah di-assign; Help Desk (bukan pelapor/PIC) tidak.
	if n := total(dev, "?involved=me"); n != 0 {
		t.Fatalf("dev sebelum assign = %v, want 0", n)
	}
	expectStatus(t, doRequest(t, router, http.MethodPatch, "/api/incidents/"+id+"/assignment", helpdesk,
		map[string]string{"team_id": team, "pic_id": devID}), http.StatusOK, "assign")
	if n := total(dev, "?involved=me"); n != 1 {
		t.Fatalf("dev setelah assign = %v, want 1", n)
	}
	if n := total(helpdesk, "?involved=me"); n != 0 {
		t.Fatalf("helpdesk involved=me = %v, want 0", n)
	}
	expectStatus(t, doRequest(t, router, http.MethodGet, "/api/incidents?involved=orang-lain", helpdesk, nil),
		http.StatusBadRequest, "involved selain me")
}
