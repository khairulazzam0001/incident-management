package handler

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestChangeCM2Flow covers CM-04–CM-08, the production gate (Q3) and change
// notifications (PRD_Change_Management.md §8.2, §13, §19).
func TestChangeCM2Flow(t *testing.T) {
	pool := testDB(t)
	router := testRouter(t, pool)
	reset(t, pool)

	dev := login(t, router, "developer@example.com")
	manager := login(t, router, "manager@example.com")
	analyst := login(t, router, "analyst@example.com")
	qa := login(t, router, "qa@example.com")
	helpdesk := login(t, router, "helpdesk@example.com")
	app := applicationID(t, pool)
	team := teamID(t, pool)
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
	expect := func(method, path, token string, body any, want int, step string) {
		t.Helper()
		expectStatus(t, doRequest(t, router, method, path, token, body), want, step)
	}

	// Incident production sampai FIXING.
	incID, _ := post("/api/incidents", helpdesk, map[string]any{
		"title": "Checkout gagal 500", "severity": "S2", "priority": "P2", "source": "helpdesk",
		"application_id": app, "environment": "production",
	})["id"].(string)
	expect(http.MethodPatch, "/api/incidents/"+incID+"/assignment", helpdesk,
		map[string]string{"team_id": team, "pic_id": devID}, http.StatusOK, "assign")
	post("/api/incidents/"+incID+"/investigations", dev, map[string]string{"notes": "cek log", "findings": "timeout"})
	post("/api/incidents/"+incID+"/fixes", dev, map[string]string{"description": "naikkan timeout", "reference": "PR #45"})

	// Gate Q3: production FIXING → VERIFYING tanpa change → 409 CHANGE_REQUIRED.
	rec := doRequest(t, router, http.MethodPatch, "/api/incidents/"+incID+"/status", dev, map[string]string{"status": "VERIFYING"})
	expectStatus(t, rec, http.StatusConflict, "gate tanpa change")
	if code := decodeBody(t, rec)["code"]; code != "CHANGE_REQUIRED" {
		t.Fatalf("gate code = %v", code)
	}

	newApproved := func(typ, title, incident string) string {
		t.Helper()
		body := map[string]any{
			"title": title, "description": "desc", "type": typ, "risk": "MEDIUM",
			"application_id": app, "environment": "production",
			"implementation_plan": "deploy", "rollback_plan": "rollback", "implementer_id": devID,
		}
		if incident != "" {
			body["incident_id"] = incident
		}
		id, _ := post("/api/changes", dev, body)["id"].(string)
		post("/api/changes/"+id+"/submit", dev, nil)
		post("/api/changes/"+id+"/approval", manager, map[string]string{"decision": "APPROVE"})
		return id
	}
	chgA := newApproved("NORMAL", "Deploy fix checkout", incID)
	chgB := newApproved("NORMAL", "Upgrade library payment", "")

	// Notifikasi: manager dapat change_submitted, dev dapat change_approved.
	if types, _ := notifTypes(t, router, manager); !types["change_submitted"] {
		t.Fatalf("manager notif = %v", types)
	}
	if types, _ := notifTypes(t, router, dev); !types["change_approved"] {
		t.Fatalf("dev notif = %v", types)
	}

	// NORMAL APPROVED tidak boleh langsung start.
	expect(http.MethodPost, "/api/changes/"+chgA+"/start", dev, nil, http.StatusConflict, "start tanpa jadwal")

	// CM-04 schedule: validasi, lalu konflik terdeteksi untuk window overlap.
	sched := func(start, end string) map[string]string {
		return map[string]string{"planned_start": start, "planned_end": end}
	}
	expect(http.MethodPost, "/api/changes/"+chgA+"/schedule", dev, sched(future(3), future(2)), http.StatusBadRequest, "end <= start")
	expect(http.MethodPost, "/api/changes/"+chgA+"/schedule", dev, sched(future(-3), future(1)), http.StatusBadRequest, "start lampau")
	expect(http.MethodPost, "/api/changes/"+chgA+"/schedule", dev, sched("besok", future(1)), http.StatusBadRequest, "format waktu")
	expect(http.MethodPost, "/api/changes/"+chgA+"/schedule", qa, sched(future(1), future(2)), http.StatusForbidden, "qa schedule")
	res := post("/api/changes/"+chgA+"/schedule", dev, sched(future(1), future(3)))
	if res["change"].(map[string]any)["status"] != "SCHEDULED" || len(res["conflicts"].([]any)) != 0 {
		t.Fatalf("schedule A = %v", res)
	}
	res = post("/api/changes/"+chgB+"/schedule", dev, sched(future(2), future(4)))
	if c := res["conflicts"].([]any); len(c) != 1 || c[0].(map[string]any)["id"] != chgA {
		t.Fatalf("conflicts B = %v", c)
	}
	res = post("/api/changes/"+chgB+"/schedule", dev, sched(future(5), future(6)))
	if c := res["conflicts"].([]any); len(c) != 0 {
		t.Fatalf("conflicts setelah reschedule = %v", c)
	}

	// CM-05 start: QA → 403; implementer → IMPLEMENTING + actual_start; double → 409.
	expect(http.MethodPost, "/api/changes/"+chgA+"/start", qa, nil, http.StatusForbidden, "qa start")
	if body := post("/api/changes/"+chgA+"/start", dev, nil); body["status"] != "IMPLEMENTING" || body["actual_start"] == nil {
		t.Fatalf("start A = %v", body)
	}
	expect(http.MethodPost, "/api/changes/"+chgA+"/start", dev, nil, http.StatusConflict, "double start")
	expect(http.MethodPost, "/api/changes/"+chgA+"/cancel", dev, map[string]string{"reason": "batal"}, http.StatusConflict, "cancel implementing")

	// Gate lolos setelah change FIX_FOR IMPLEMENTING.
	expect(http.MethodPatch, "/api/incidents/"+incID+"/status", dev, map[string]string{"status": "VERIFYING"}, http.StatusOK, "gate dengan change")

	// CM-05 complete: outcome invalid / FAILED tanpa notes → 400; SUCCESS → REVIEWING.
	expect(http.MethodPost, "/api/changes/"+chgA+"/complete", dev, map[string]string{"outcome": "DONE"}, http.StatusBadRequest, "outcome invalid")
	expect(http.MethodPost, "/api/changes/"+chgA+"/complete", dev, map[string]string{"outcome": "FAILED"}, http.StatusBadRequest, "failed tanpa notes")
	if body := post("/api/changes/"+chgA+"/complete", dev, map[string]string{"outcome": "SUCCESS"}); body["status"] != "REVIEWING" || body["actual_end"] == nil {
		t.Fatalf("complete A = %v", body)
	}

	// CM-06 close: developer → 403; NORMAL SUCCESS tanpa PIR → 200 oleh analyst.
	expect(http.MethodPost, "/api/changes/"+chgA+"/close", dev, map[string]string{}, http.StatusForbidden, "dev close")
	if body := post("/api/changes/"+chgA+"/close", analyst, map[string]string{}); body["status"] != "CLOSED" || body["closed_by"] == nil {
		t.Fatalf("close A = %v", body)
	}

	// EMERGENCY: start langsung dari APPROVED; ROLLED_BACK → notif change_failed; PIR wajib.
	chgE := newApproved("EMERGENCY", "Hotfix darurat DB pool", "")
	post("/api/changes/"+chgE+"/start", dev, nil)
	post("/api/changes/"+chgE+"/complete", dev, map[string]string{"outcome": "ROLLED_BACK", "outcome_notes": "Error rate naik."})
	if types, _ := notifTypes(t, router, manager); !types["change_failed"] {
		t.Fatalf("manager notif tanpa change_failed: %v", types)
	}
	expect(http.MethodPost, "/api/changes/"+chgE+"/close", manager, map[string]string{"review_notes": " "}, http.StatusBadRequest, "emergency tanpa PIR")
	post("/api/changes/"+chgE+"/close", manager, map[string]string{"review_notes": "Root cause: pool size."})

	// CM-08 link: CAUSED_BY ke change belum implementasi → 409; ke CLOSED → 201;
	// duplikat → 409; QA → 403; unlink → 200, lagi → 404.
	inc2, _ := post("/api/incidents", helpdesk, map[string]any{
		"title": "Payment lambat setelah deploy", "severity": "S3", "priority": "P2", "source": "monitoring",
		"application_id": app, "environment": "production",
	})["id"].(string)
	caused := map[string]string{"incident_id": inc2, "relation": "CAUSED_BY"}
	expect(http.MethodPost, "/api/changes/"+chgB+"/incidents", helpdesk, caused, http.StatusConflict, "caused_by scheduled")
	expect(http.MethodPost, "/api/changes/"+chgA+"/incidents", qa, caused, http.StatusForbidden, "qa link")
	expect(http.MethodPost, "/api/changes/"+chgA+"/incidents", helpdesk,
		map[string]string{"incident_id": inc2, "relation": "BLAME"}, http.StatusBadRequest, "relation invalid")
	if links := post("/api/changes/"+chgA+"/incidents", helpdesk, caused)["data"].([]any); len(links) != 2 {
		t.Fatalf("links A = %d, want 2", len(links))
	}
	expect(http.MethodPost, "/api/changes/"+chgA+"/incidents", helpdesk, caused, http.StatusConflict, "duplicate link")
	if types, _ := notifTypes(t, router, dev); !types["change_caused_incident"] {
		t.Fatalf("dev notif tanpa change_caused_incident: %v", types)
	}
	unlink := "/api/changes/" + chgA + "/incidents/" + inc2 + "?relation=CAUSED_BY"
	expect(http.MethodDelete, unlink, helpdesk, nil, http.StatusOK, "unlink")
	expect(http.MethodDelete, unlink, helpdesk, nil, http.StatusNotFound, "unlink lagi")

	// Notifikasi change membawa change_id dan incident_id null.
	rec = doRequest(t, router, http.MethodGet, "/api/notifications?limit=100", dev, nil)
	expectStatus(t, rec, http.StatusOK, "notifications")
	for _, it := range decodeBody(t, rec)["data"].([]any) {
		n := it.(map[string]any)
		if strings.HasPrefix(n["type"].(string), "change_") && (n["change_id"] == nil || n["incident_id"] != nil) {
			t.Fatalf("notif change tanpa change_id: %v", n)
		}
	}
}
