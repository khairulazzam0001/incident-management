package handler

import (
	"net/http"
	"testing"
	"time"
)

// TestChangeCM3Flow covers the change summary (§14), recent changes for
// triage (§8.3) and calendar ordering (CM-FR-10–12).
func TestChangeCM3Flow(t *testing.T) {
	pool := testDB(t)
	router := testRouter(t, pool)
	reset(t, pool)

	dev := login(t, router, "developer@example.com")
	manager := login(t, router, "manager@example.com")
	helpdesk := login(t, router, "helpdesk@example.com")
	customer := login(t, router, "user@example.com")
	app := applicationID(t, pool)
	devID := userID(t, pool, "developer@example.com")
	future := func(h int) string { return time.Now().Add(time.Duration(h) * time.Hour).Format(time.RFC3339) }
	post := func(path, token string, body any) map[string]any {
		t.Helper()
		rec := doRequest(t, router, http.MethodPost, path, token, body)
		if rec.Code >= 300 {
			t.Fatalf("POST %s: status %d body %s", path, rec.Code, rec.Body.String())
		}
		return decodeBody(t, rec)
	}
	get := func(path, token string) map[string]any {
		t.Helper()
		rec := doRequest(t, router, http.MethodGet, path, token, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s: status %d body %s", path, rec.Code, rec.Body.String())
		}
		return decodeBody(t, rec)
	}
	approved := func(typ, title string) string {
		t.Helper()
		id, _ := post("/api/changes", dev, map[string]any{
			"title": title, "description": "desc", "type": typ, "risk": "LOW",
			"application_id": app, "environment": "production",
			"implementation_plan": "deploy", "rollback_plan": "rollback", "implementer_id": devID,
		})["id"].(string)
		post("/api/changes/"+id+"/submit", dev, nil)
		if typ != "STANDARD" {
			post("/api/changes/"+id+"/approval", manager, map[string]string{"decision": "APPROVE"})
		}
		return id
	}

	// Kosong: rate nil, bukan 0.
	sum := get("/api/changes/summary", manager)
	if sum["success_rate"] != nil || sum["active_total"] != float64(0) {
		t.Fatalf("summary kosong = %v", sum)
	}
	expectStatus(t, doRequest(t, router, http.MethodGet, "/api/changes/summary", customer, nil), http.StatusForbidden, "user summary")

	// A: NORMAL sukses → CLOSED. E: EMERGENCY rollback → CLOSED + PIR.
	a := approved("NORMAL", "Deploy release 1.9")
	post("/api/changes/"+a+"/schedule", dev, map[string]string{"planned_start": future(1), "planned_end": future(2)})
	post("/api/changes/"+a+"/start", dev, nil)
	post("/api/changes/"+a+"/complete", dev, map[string]string{"outcome": "SUCCESS"})
	post("/api/changes/"+a+"/close", manager, map[string]string{})
	e := approved("EMERGENCY", "Hotfix pool DB")
	post("/api/changes/"+e+"/start", dev, nil)
	post("/api/changes/"+e+"/complete", dev, map[string]string{"outcome": "ROLLED_BACK", "outcome_notes": "rollback"})
	post("/api/changes/"+e+"/close", manager, map[string]string{"review_notes": "PIR"})
	// S: STANDARD terjadwal dalam 7 hari → upcoming.
	s := approved("STANDARD", "Rotate log server")
	post("/api/changes/"+s+"/schedule", dev, map[string]string{"planned_start": future(24), "planned_end": future(25)})
	// L: terjadwal lebih awal dari S tapi > 7 hari → tidak upcoming.
	l := approved("NORMAL", "Migrasi DB kuartal depan")
	post("/api/changes/"+l+"/schedule", dev, map[string]string{"planned_start": future(24 * 10), "planned_end": future(24*10 + 1)})

	sum = get("/api/changes/summary", manager)
	if sum["active_total"] != float64(2) {
		t.Fatalf("active_total = %v, want 2", sum["active_total"])
	}
	if sum["success_rate"] != 0.5 || sum["failure_rate"] != 0.5 {
		t.Fatalf("success/failure = %v/%v, want 0.5/0.5", sum["success_rate"], sum["failure_rate"])
	}
	if sum["emergency_ratio"] != 0.25 || sum["pir_completion_rate"] != float64(1) {
		t.Fatalf("emergency/pir = %v/%v", sum["emergency_ratio"], sum["pir_completion_rate"])
	}
	if up := sum["upcoming"].([]any); len(up) != 1 || up[0].(map[string]any)["id"] != s {
		t.Fatalf("upcoming = %v", up)
	}
	if sum["avg_hours_submit_to_approve"] == nil || sum["avg_hours_approve_to_implement"] == nil {
		t.Fatalf("lead time nil: %v", sum)
	}

	// CAUSED_BY ke change sukses → failure rate naik ke 1.
	incID, _ := post("/api/incidents", helpdesk, map[string]any{
		"title": "Payment error setelah release", "severity": "S2", "priority": "P1", "source": "monitoring",
		"application_id": app, "environment": "production",
	})["id"].(string)
	post("/api/changes/"+a+"/incidents", helpdesk, map[string]string{"incident_id": incID, "relation": "CAUSED_BY"})
	if sum = get("/api/changes/summary", manager); sum["failure_rate"] != float64(1) {
		t.Fatalf("failure_rate setelah CAUSED_BY = %v, want 1", sum["failure_rate"])
	}

	// Recent changes: A dan E (actual_start < 72 jam, app+env sama); S/L belum mulai.
	recent := get("/api/incidents/"+incID+"/recent-changes", helpdesk)["data"].([]any)
	if len(recent) != 2 {
		t.Fatalf("recent changes = %d, want 2", len(recent))
	}
	other, _ := post("/api/incidents", helpdesk, map[string]any{
		"title": "Typo halaman login", "severity": "S4", "priority": "P3", "source": "user",
	})["id"].(string)
	if r := get("/api/incidents/"+other+"/recent-changes", helpdesk)["data"].([]any); len(r) != 0 {
		t.Fatalf("recent tanpa app/env = %v", r)
	}

	// Kalender: sort=planned_start → S sebelum L.
	cal := get("/api/changes?status=SCHEDULED&sort=planned_start", dev)["data"].([]any)
	if len(cal) != 2 || cal[0].(map[string]any)["id"] != s || cal[1].(map[string]any)["id"] != l {
		t.Fatalf("calendar order = %v", cal)
	}
}
