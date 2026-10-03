package handler

import (
	"net/http"
	"strings"
	"testing"
)

// TestChangeAttachments covers CM-FR-13: upload rules shared with incident
// attachments, permission, list, download, and terminal-status guard.
func TestChangeAttachments(t *testing.T) {
	pool := testDB(t)
	router := testRouter(t, pool)
	reset(t, pool)

	dev := login(t, router, "developer@example.com")
	qa := login(t, router, "qa@example.com")
	analyst := login(t, router, "analyst@example.com")
	customer := login(t, router, "user@example.com")
	app := applicationID(t, pool)

	newChange := func() string {
		t.Helper()
		rec := doRequest(t, router, http.MethodPost, "/api/changes", dev, map[string]any{
			"title": "Deploy runbook baru", "type": "NORMAL", "risk": "LOW",
			"application_id": app, "environment": "staging",
		})
		expectStatus(t, rec, http.StatusCreated, "create change")
		id, _ := decodeBody(t, rec)["id"].(string)
		return id
	}
	id := newChange()
	base := "/api/changes/" + id + "/attachments"

	// Aturan file sama dengan incident: tipe & isi divalidasi.
	expectStatus(t, uploadRequest(t, router, base, dev, "script.exe", []byte("MZ")), http.StatusBadRequest, "ext ditolak")
	expectStatus(t, uploadRequest(t, router, base, dev, "fake.png", []byte{'M', 'Z', 0x90, 0x00, 0x03, 0x00, 0x00, 0x00, 0x04, 0x00}),
		http.StatusBadRequest, "isi biner tak dikenal")
	// Hak upload: QA (bukan requester/implementer) → 403; User → 403.
	expectStatus(t, uploadRequest(t, router, base, qa, "plan.png", tinyPNG), http.StatusForbidden, "qa upload")
	expectStatus(t, uploadRequest(t, router, base, customer, "plan.png", tinyPNG), http.StatusForbidden, "user upload")

	rec := uploadRequest(t, router, base, dev, "runbook.png", tinyPNG)
	expectStatus(t, rec, http.StatusCreated, "dev upload")
	att := decodeBody(t, rec)
	aid, _ := att["id"].(string)
	if att["file_name"] != "runbook.png" || att["mime_type"] != "image/png" || att["storage_key"] != nil {
		t.Fatalf("attachment = %v", att)
	}
	expectStatus(t, uploadRequest(t, router, base, analyst, "notes.txt", []byte("catatan PIR")), http.StatusCreated, "analyst upload")

	// List (QA boleh baca) + timeline mencatat attachment.
	rec = doRequest(t, router, http.MethodGet, base, qa, nil)
	expectStatus(t, rec, http.StatusOK, "list")
	if items := decodeBody(t, rec)["data"].([]any); len(items) != 2 {
		t.Fatalf("attachments = %d, want 2", len(items))
	}
	rec = doRequest(t, router, http.MethodGet, "/api/changes/"+id+"/timeline", qa, nil)
	found := 0
	for _, a := range decodeBody(t, rec)["data"].([]any) {
		if a.(map[string]any)["type"] == "attachment" {
			found++
		}
	}
	if found != 2 {
		t.Fatalf("timeline attachment = %d, want 2", found)
	}

	// Download: isi utuh + header; ID asing → 404; User → 403.
	rec = doRequest(t, router, http.MethodGet, base+"/"+aid+"/download", qa, nil)
	expectStatus(t, rec, http.StatusOK, "download")
	if rec.Body.Len() != len(tinyPNG) || !strings.Contains(rec.Header().Get("Content-Disposition"), "runbook.png") {
		t.Fatalf("download len=%d disposition=%q", rec.Body.Len(), rec.Header().Get("Content-Disposition"))
	}
	other := newChange()
	expectStatus(t, doRequest(t, router, http.MethodGet, "/api/changes/"+other+"/attachments/"+aid+"/download", qa, nil),
		http.StatusNotFound, "attachment change lain")
	expectStatus(t, doRequest(t, router, http.MethodGet, base+"/"+aid+"/download", customer, nil), http.StatusForbidden, "user download")

	// Change CANCELLED tidak menerima file.
	expectStatus(t, doRequest(t, router, http.MethodPost, "/api/changes/"+other+"/cancel", dev,
		map[string]string{"reason": "batal"}), http.StatusOK, "cancel")
	expectStatus(t, uploadRequest(t, router, "/api/changes/"+other+"/attachments", dev, "x.png", tinyPNG),
		http.StatusConflict, "upload ke cancelled")
}
