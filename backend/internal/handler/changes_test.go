package handler

import (
	"context"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func applicationID(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(), `
		INSERT INTO master_application (code, name) VALUES ('change-test','Change Test App')
		ON CONFLICT (code) DO UPDATE SET name = EXCLUDED.name
		RETURNING id`).Scan(&id)
	if err != nil {
		t.Fatalf("seed application: %v", err)
	}
	return id
}

func expectStatus(t *testing.T, rec interface {
	Result() *http.Response
}, want int, step string) {
	t.Helper()
	if got := rec.Result().StatusCode; got != want {
		t.Fatalf("%s: status %d, want %d", step, got, want)
	}
}

// TestChangeCM1Flow covers CM-01–CM-03, CM-07–CM-09 (PRD_Change_Management.md §19).
func TestChangeCM1Flow(t *testing.T) {
	pool := testDB(t)
	router := testRouter(t, pool)
	reset(t, pool)

	dev := login(t, router, "developer@example.com")
	manager := login(t, router, "manager@example.com")
	analyst := login(t, router, "analyst@example.com")
	qa := login(t, router, "qa@example.com")
	customer := login(t, router, "user@example.com")
	helpdesk := login(t, router, "helpdesk@example.com")
	app := applicationID(t, pool)
	devID := userID(t, pool, "developer@example.com")

	base := func() map[string]any {
		return map[string]any{
			"title": "Deploy hotfix checkout", "description": "Naikkan timeout gateway.",
			"type": "NORMAL", "risk": "MEDIUM", "application_id": app, "environment": "production",
			"implementation_plan": "Deploy v1.9.0", "rollback_plan": "Redeploy v1.8.2",
		}
	}

	// Incident sumber untuk link FIX_FOR.
	rec := doRequest(t, router, http.MethodPost, "/api/incidents", helpdesk, map[string]any{
		"title": "Checkout gagal 500", "severity": "S1", "priority": "P1", "source": "helpdesk",
		"application_id": app, "environment": "production",
	})
	expectStatus(t, rec, http.StatusCreated, "create incident")
	incidentID, _ := decodeBody(t, rec)["id"].(string)

	// Role tanpa hak create → 403; User tidak boleh list.
	expectStatus(t, doRequest(t, router, http.MethodPost, "/api/changes", qa, base()), http.StatusForbidden, "qa create")
	expectStatus(t, doRequest(t, router, http.MethodGet, "/api/changes", customer, nil), http.StatusForbidden, "user list")

	// Validasi.
	bad := base()
	bad["title"] = "abc"
	expectStatus(t, doRequest(t, router, http.MethodPost, "/api/changes", dev, bad), http.StatusBadRequest, "short title")
	bad = base()
	bad["application_id"] = ""
	expectStatus(t, doRequest(t, router, http.MethodPost, "/api/changes", dev, bad), http.StatusBadRequest, "no app")

	// CM-01: create dari incident → DRAFT + nomor CHG + link FIX_FOR.
	in := base()
	in["incident_id"] = incidentID
	in["implementer_id"] = devID
	rec = doRequest(t, router, http.MethodPost, "/api/changes", dev, in)
	expectStatus(t, rec, http.StatusCreated, "create change")
	chg := decodeBody(t, rec)
	chgID, _ := chg["id"].(string)
	if chg["status"] != "DRAFT" {
		t.Fatalf("status = %v, want DRAFT", chg["status"])
	}
	if no, _ := chg["change_no"].(string); len(no) != len("CHG-2026-000001") || no[:4] != "CHG-" {
		t.Fatalf("change_no = %q", no)
	}
	rec = doRequest(t, router, http.MethodGet, "/api/incidents/"+incidentID+"/changes", helpdesk, nil)
	expectStatus(t, rec, http.StatusOK, "incident changes")
	links, _ := decodeBody(t, rec)["data"].([]any)
	if len(links) != 1 || links[0].(map[string]any)["relation"] != "FIX_FOR" {
		t.Fatalf("incident links = %v", links)
	}

	// Edit DRAFT oleh non-requester → 403; requester → 200.
	upd := base()
	upd["title"] = "Deploy hotfix checkout v2"
	upd["implementer_id"] = devID
	expectStatus(t, doRequest(t, router, http.MethodPatch, "/api/changes/"+chgID, analyst, upd), http.StatusForbidden, "analyst edit")
	expectStatus(t, doRequest(t, router, http.MethodPatch, "/api/changes/"+chgID, dev, upd), http.StatusOK, "dev edit")

	// CM-02: submit → SUBMITTED; double submit → 409.
	expectStatus(t, doRequest(t, router, http.MethodPost, "/api/changes/"+chgID+"/submit", dev, nil), http.StatusOK, "submit")
	expectStatus(t, doRequest(t, router, http.MethodPost, "/api/changes/"+chgID+"/submit", dev, nil), http.StatusConflict, "double submit")
	expectStatus(t, doRequest(t, router, http.MethodPatch, "/api/changes/"+chgID, dev, upd), http.StatusConflict, "edit submitted")

	// CM-03: approve oleh non-manager / requester → 403; reject tanpa reason → 400.
	approve := map[string]string{"decision": "APPROVE"}
	expectStatus(t, doRequest(t, router, http.MethodPost, "/api/changes/"+chgID+"/approval", dev, approve), http.StatusForbidden, "self approve")
	expectStatus(t, doRequest(t, router, http.MethodPost, "/api/changes/"+chgID+"/approval", analyst, approve), http.StatusForbidden, "analyst approve dev")
	expectStatus(t, doRequest(t, router, http.MethodPost, "/api/changes/"+chgID+"/approval", manager,
		map[string]string{"decision": "REJECT"}), http.StatusBadRequest, "reject no reason")

	// Request changes → DRAFT + revision 2, lalu submit ulang & approve.
	rec = doRequest(t, router, http.MethodPost, "/api/changes/"+chgID+"/approval", manager,
		map[string]string{"decision": "REQUEST_CHANGES", "reason": "Tambah langkah smoke test."})
	expectStatus(t, rec, http.StatusOK, "request changes")
	if body := decodeBody(t, rec); body["status"] != "DRAFT" || body["revision"] != float64(2) {
		t.Fatalf("request changes → %v rev %v", body["status"], body["revision"])
	}
	expectStatus(t, doRequest(t, router, http.MethodPost, "/api/changes/"+chgID+"/submit", dev, nil), http.StatusOK, "resubmit")
	rec = doRequest(t, router, http.MethodPost, "/api/changes/"+chgID+"/approval", manager, approve)
	expectStatus(t, rec, http.StatusOK, "approve")
	if decodeBody(t, rec)["status"] != "APPROVED" {
		t.Fatal("approve: status bukan APPROVED")
	}
	expectStatus(t, doRequest(t, router, http.MethodPost, "/api/changes/"+chgID+"/approval", manager, approve), http.StatusConflict, "double approve")

	rec = doRequest(t, router, http.MethodGet, "/api/changes/"+chgID+"/approvals", qa, nil)
	expectStatus(t, rec, http.StatusOK, "approvals")
	approvals, _ := decodeBody(t, rec)["data"].([]any)
	if len(approvals) != 2 {
		t.Fatalf("approvals = %d, want 2", len(approvals))
	}
	if a := approvals[0].(map[string]any); a["decision"] != "CHANGES_REQUESTED" || a["revision"] != float64(1) {
		t.Fatalf("approval[0] = %v", a)
	}
	if a := approvals[1].(map[string]any); a["decision"] != "APPROVED" || a["revision"] != float64(2) {
		t.Fatalf("approval[1] = %v", a)
	}

	// CM-02 STANDARD: submit langsung APPROVED; risk ≠ LOW ditolak saat submit.
	std := base()
	std["type"] = "STANDARD"
	rec = doRequest(t, router, http.MethodPost, "/api/changes", dev, std)
	expectStatus(t, rec, http.StatusCreated, "create standard")
	stdID, _ := decodeBody(t, rec)["id"].(string)
	expectStatus(t, doRequest(t, router, http.MethodPost, "/api/changes/"+stdID+"/submit", dev, nil), http.StatusBadRequest, "standard medium")
	std["risk"] = "LOW"
	expectStatus(t, doRequest(t, router, http.MethodPatch, "/api/changes/"+stdID, dev, std), http.StatusOK, "standard → low")
	rec = doRequest(t, router, http.MethodPost, "/api/changes/"+stdID+"/submit", dev, nil)
	expectStatus(t, rec, http.StatusOK, "submit standard")
	if decodeBody(t, rec)["status"] != "APPROVED" {
		t.Fatal("standard: tidak auto-approve")
	}

	// Risk HIGH tanpa test plan → 400 saat submit.
	high := base()
	high["risk"] = "HIGH"
	rec = doRequest(t, router, http.MethodPost, "/api/changes", dev, high)
	expectStatus(t, rec, http.StatusCreated, "create high")
	highID, _ := decodeBody(t, rec)["id"].(string)
	expectStatus(t, doRequest(t, router, http.MethodPost, "/api/changes/"+highID+"/submit", dev, nil), http.StatusBadRequest, "high no test plan")

	// Q2: Analyst approve CR LOW milik Manager → 200; Manager self → 403.
	mgr := base()
	mgr["risk"] = "LOW"
	rec = doRequest(t, router, http.MethodPost, "/api/changes", manager, mgr)
	expectStatus(t, rec, http.StatusCreated, "manager create")
	mgrID, _ := decodeBody(t, rec)["id"].(string)
	expectStatus(t, doRequest(t, router, http.MethodPost, "/api/changes/"+mgrID+"/submit", manager, nil), http.StatusOK, "manager submit")
	expectStatus(t, doRequest(t, router, http.MethodPost, "/api/changes/"+mgrID+"/approval", manager, approve), http.StatusForbidden, "manager self approve")
	expectStatus(t, doRequest(t, router, http.MethodPost, "/api/changes/"+mgrID+"/approval", analyst, approve), http.StatusOK, "analyst approve manager low")

	// CM-07: cancel tanpa reason → 400; oleh QA → 403; requester → CANCELLED; lagi → 409.
	cancel := map[string]string{"reason": "Diganti CR lain."}
	expectStatus(t, doRequest(t, router, http.MethodPost, "/api/changes/"+highID+"/cancel", dev, map[string]string{}), http.StatusBadRequest, "cancel no reason")
	expectStatus(t, doRequest(t, router, http.MethodPost, "/api/changes/"+highID+"/cancel", qa, cancel), http.StatusForbidden, "qa cancel")
	expectStatus(t, doRequest(t, router, http.MethodPost, "/api/changes/"+highID+"/cancel", dev, cancel), http.StatusOK, "cancel")
	expectStatus(t, doRequest(t, router, http.MethodPost, "/api/changes/"+highID+"/cancel", dev, cancel), http.StatusConflict, "cancel again")

	// CM-09: komentar + timeline.
	expectStatus(t, doRequest(t, router, http.MethodPost, "/api/changes/"+chgID+"/comments", qa,
		map[string]string{"body": "Regression test OK di staging."}), http.StatusCreated, "comment")
	rec = doRequest(t, router, http.MethodGet, "/api/changes/"+chgID+"/timeline", helpdesk, nil)
	expectStatus(t, rec, http.StatusOK, "timeline")
	types := []string{}
	for _, a := range decodeBody(t, rec)["data"].([]any) {
		types = append(types, a.(map[string]any)["type"].(string))
	}
	want := []string{"created", "incident_link", "updated", "status_change", "approval", "status_change", "approval", "comment"}
	if len(types) != len(want) {
		t.Fatalf("timeline types = %v, want %v", types, want)
	}
	for i := range want {
		if types[i] != want[i] {
			t.Fatalf("timeline types = %v, want %v", types, want)
		}
	}

	// Incident timeline ikut mencatat link.
	rec = doRequest(t, router, http.MethodGet, "/api/incidents/"+incidentID+"/timeline", helpdesk, nil)
	expectStatus(t, rec, http.StatusOK, "incident timeline")
	found := false
	for _, a := range decodeBody(t, rec)["data"].([]any) {
		if a.(map[string]any)["type"] == "change_link" {
			found = true
		}
	}
	if !found {
		t.Fatal("incident timeline tanpa change_link")
	}

	// List + filter + 404.
	rec = doRequest(t, router, http.MethodGet, "/api/changes?status=APPROVED&requester=me", dev, nil)
	expectStatus(t, rec, http.StatusOK, "list")
	if total := decodeBody(t, rec)["meta"].(map[string]any)["total"]; total != float64(2) {
		t.Fatalf("list approved mine total = %v, want 2", total)
	}
	expectStatus(t, doRequest(t, router, http.MethodGet, "/api/changes/00000000-0000-4000-8000-000000000000", dev, nil), http.StatusNotFound, "404")
	expectStatus(t, doRequest(t, router, http.MethodGet, "/api/changes/abc", dev, nil), http.StatusBadRequest, "bad id")

	// Meta memuat master change.
	rec = doRequest(t, router, http.MethodGet, "/api/meta", dev, nil)
	expectStatus(t, rec, http.StatusOK, "meta")
	if ct, _ := decodeBody(t, rec)["change_types"].([]any); len(ct) != 3 {
		t.Fatalf("meta change_types = %v", ct)
	}
}
