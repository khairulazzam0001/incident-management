package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/khairulazzam0001/incident-management/backend/internal/repository"
	"github.com/khairulazzam0001/incident-management/backend/internal/service"
)

// testDB connects using TEST_DATABASE_URL (falls back to DATABASE_URL).
func testDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = os.Getenv("DATABASE_URL")
	}
	if url == "" {
		t.Skip("TEST_DATABASE_URL/DATABASE_URL tidak diset — lewati integration test")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func testRouter(t *testing.T, pool *pgxpool.Pool) http.Handler {
	t.Helper()
	repo := repository.New(pool)
	authSvc, err := service.NewAuthService(repo, "test-secret")
	if err != nil {
		t.Fatalf("auth service: %v", err)
	}
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	return NewRouter(Deps{Log: log, Auth: authSvc, Incidents: service.NewIncidentService(repo)})
}

// reset truncates transaction tables; masters and seeds stay intact.
func reset(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `TRUNCATE
		trans_incident_attachment, trans_incident_verification, trans_incident_fix,
		trans_incident_investigation, trans_incident_activity, trans_incident_comment,
		trans_incident_assignment, trans_incident`)
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

func userID(t *testing.T, pool *pgxpool.Pool, email string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`SELECT id FROM master_user WHERE email = $1`, email).Scan(&id); err != nil {
		t.Fatalf("seed user %s: %v", email, err)
	}
	return id
}

func teamID(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(), `
		INSERT INTO master_team (code, name) VALUES ('backend-test','Backend Test')
		ON CONFLICT (code) DO UPDATE SET name = EXCLUDED.name
		RETURNING id`).Scan(&id)
	if err != nil {
		t.Fatalf("seed team: %v", err)
	}
	return id
}

func doRequest(t *testing.T, router http.Handler, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return v
}

func login(t *testing.T, router http.Handler, email string) string {
	t.Helper()
	rec := doRequest(t, router, http.MethodPost, "/api/auth/login", "",
		map[string]string{"email": email, "password": "Password123!"})
	if rec.Code != http.StatusOK {
		t.Fatalf("login %s: status %d body %s", email, rec.Code, rec.Body.String())
	}
	token, _ := decodeBody(t, rec)["token"].(string)
	if token == "" {
		t.Fatal("login: token kosong")
	}
	return token
}

// TestPhase1Flow covers the Fase 1 acceptance path end-to-end.
func TestPhase1Flow(t *testing.T) {
	pool := testDB(t)
	router := testRouter(t, pool)
	reset(t, pool)

	helpdesk := login(t, router, "helpdesk@example.com")
	customer := login(t, router, "user@example.com")
	team := teamID(t, pool)
	devID := userID(t, pool, "developer@example.com")

	// 1. Login salah → 401.
	rec := doRequest(t, router, http.MethodPost, "/api/auth/login", "",
		map[string]string{"email": "helpdesk@example.com", "password": "salah"})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("login salah: status %d, want 401", rec.Code)
	}

	// 2. Tanpa token → 401.
	rec = doRequest(t, router, http.MethodGet, "/api/incidents", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("tanpa token: status %d, want 401", rec.Code)
	}

	// 3. Create invalid (title pendek) → 400, input tidak hilang di server.
	rec = doRequest(t, router, http.MethodPost, "/api/incidents", customer,
		map[string]string{"title": "x", "description": "d", "severity": "S1", "priority": "P1", "source": "user"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("create invalid: status %d, want 400", rec.Code)
	}

	// 4. Create valid → 201 + NEW + incident_no unik.
	rec = doRequest(t, router, http.MethodPost, "/api/incidents", customer,
		map[string]string{"title": "Checkout gagal 500", "description": "POST /checkout 500", "severity": "S1", "priority": "P1", "source": "user"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: status %d body %s", rec.Code, rec.Body.String())
	}
	created := decodeBody(t, rec)
	incidentID, _ := created["id"].(string)
	incidentNo, _ := created["incident_no"].(string)
	if created["status"] != "NEW" {
		t.Fatalf("create: status %v, want NEW", created["status"])
	}
	if !strings.HasPrefix(incidentNo, "INC-") {
		t.Fatalf("create: incident_no %q tidak berformat INC-", incidentNo)
	}

	// 5. Assign oleh User biasa → 403.
	rec = doRequest(t, router, http.MethodPatch, "/api/incidents/"+incidentID+"/assignment", customer,
		map[string]string{"team_id": team, "pic_id": devID})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("assign unauthorized: status %d, want 403", rec.Code)
	}

	// 6. Assign oleh HelpDesk → 200 + ASSIGNED.
	rec = doRequest(t, router, http.MethodPatch, "/api/incidents/"+incidentID+"/assignment", helpdesk,
		map[string]string{"team_id": team, "pic_id": devID})
	if rec.Code != http.StatusOK {
		t.Fatalf("assign: status %d body %s", rec.Code, rec.Body.String())
	}
	if got := decodeBody(t, rec)["status"]; got != "ASSIGNED" {
		t.Fatalf("assign: status %v, want ASSIGNED", got)
	}

	// 7. Transisi ilegal ASSIGNED → RESOLVED → 409, status tetap.
	rec = doRequest(t, router, http.MethodPatch, "/api/incidents/"+incidentID+"/status", helpdesk,
		map[string]string{"status": "RESOLVED"})
	if rec.Code != http.StatusConflict {
		t.Fatalf("transisi ilegal: status %d, want 409", rec.Code)
	}
	rec = doRequest(t, router, http.MethodGet, "/api/incidents/"+incidentID, helpdesk, nil)
	if got := decodeBody(t, rec)["status"]; got != "ASSIGNED" {
		t.Fatalf("setelah 409: status %v, want ASSIGNED", got)
	}

	// 8. Transisi legal oleh PIC → INVESTIGATING.
	dev := login(t, router, "developer@example.com")
	rec = doRequest(t, router, http.MethodPatch, "/api/incidents/"+incidentID+"/status", dev,
		map[string]string{"status": "INVESTIGATING"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status change: status %d body %s", rec.Code, rec.Body.String())
	}

	// 9. Komentar kosong → 400; valid → 201.
	rec = doRequest(t, router, http.MethodPost, "/api/incidents/"+incidentID+"/comments", dev,
		map[string]string{"body": "   "})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("comment kosong: status %d, want 400", rec.Code)
	}
	rec = doRequest(t, router, http.MethodPost, "/api/incidents/"+incidentID+"/comments", dev,
		map[string]string{"body": "Log menunjukkan NPE di PaymentService."})
	if rec.Code != http.StatusCreated {
		t.Fatalf("comment: status %d body %s", rec.Code, rec.Body.String())
	}

	// 10b. Comments list memuat komentar tadi.
	rec = doRequest(t, router, http.MethodGet, "/api/incidents/"+incidentID+"/comments", helpdesk, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("comments: status %d", rec.Code)
	}
	comments, _ := decodeBody(t, rec)["data"].([]any)
	if len(comments) != 1 {
		t.Fatalf("comments: count %d, want 1", len(comments))
	}
	rec = doRequest(t, router, http.MethodGet, "/api/incidents/"+incidentID+"/timeline", helpdesk, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("timeline: status %d", rec.Code)
	}
	items, _ := decodeBody(t, rec)["data"].([]any)
	types := map[string]bool{}
	for _, it := range items {
		if m, ok := it.(map[string]any); ok {
			if ty, ok := m["type"].(string); ok {
				types[ty] = true
			}
		}
	}
	for _, want := range []string{"created", "assignment", "status_change", "comment"} {
		if !types[want] {
			t.Fatalf("timeline: tipe %q hilang (ada: %v)", want, types)
		}
	}

	// 11. List filter + pagination meta.
	rec = doRequest(t, router, http.MethodGet, "/api/incidents?status=INVESTIGATING&page=1&limit=20", helpdesk, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: status %d", rec.Code)
	}
	list := decodeBody(t, rec)
	meta, _ := list["meta"].(map[string]any)
	if fmt.Sprintf("%v", meta["total"]) != "1" {
		t.Fatalf("list: total %v, want 1", meta["total"])
	}

	// 12. GET unknown id → 404.
	rec = doRequest(t, router, http.MethodGet, "/api/incidents/00000000-0000-0000-0000-000000000000", helpdesk, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("get unknown: status %d, want 404", rec.Code)
	}

	// 14. Users untuk assignment lookup.
	rec = doRequest(t, router, http.MethodGet, "/api/users", helpdesk, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("users: status %d", rec.Code)
	}
	users, _ := decodeBody(t, rec)["data"].([]any)
	if len(users) < 6 {
		t.Fatalf("users: count %d, want >= 6 seed users", len(users))
	}

	// 15. Query invalid → 400.
	rec = doRequest(t, router, http.MethodGet, "/api/incidents?limit=1000", helpdesk, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("limit invalid: status %d, want 400", rec.Code)
	}
}

// TestPhase2Flow covers investigation → fix → verification → close → reopen.
func TestPhase2Flow(t *testing.T) {
	pool := testDB(t)
	router := testRouter(t, pool)
	reset(t, pool)

	helpdesk := login(t, router, "helpdesk@example.com")
	customer := login(t, router, "user@example.com")
	dev := login(t, router, "developer@example.com")
	qa := login(t, router, "qa@example.com")
	manager := login(t, router, "manager@example.com")
	team := teamID(t, pool)
	devID := userID(t, pool, "developer@example.com")

	// Setup: create → assign → INVESTIGATING-ready.
	rec := doRequest(t, router, http.MethodPost, "/api/incidents", customer,
		map[string]string{"title": "Worker macet saat retry", "description": "queue menumpuk", "severity": "S2", "priority": "P2", "source": "monitoring"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: status %d", rec.Code)
	}
	id, _ := decodeBody(t, rec)["id"].(string)
	rec = doRequest(t, router, http.MethodPatch, "/api/incidents/"+id+"/assignment", helpdesk,
		map[string]string{"team_id": team, "pic_id": devID})
	if rec.Code != http.StatusOK {
		t.Fatalf("assign: status %d", rec.Code)
	}

	// 1. Investigation kosong → 400.
	rec = doRequest(t, router, http.MethodPost, "/api/incidents/"+id+"/investigations", dev,
		map[string]string{"notes": "  ", "findings": ""})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("investigation kosong: status %d, want 400", rec.Code)
	}

	// 2. Investigation oleh PIC → 201 + status INVESTIGATING.
	rec = doRequest(t, router, http.MethodPost, "/api/incidents/"+id+"/investigations", dev,
		map[string]string{"notes": "cek log worker", "findings": "deadlock di job X"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("investigation: status %d body %s", rec.Code, rec.Body.String())
	}
	rec = doRequest(t, router, http.MethodGet, "/api/incidents/"+id, helpdesk, nil)
	if got := decodeBody(t, rec)["status"]; got != "INVESTIGATING" {
		t.Fatalf("setelah investigation: status %v, want INVESTIGATING", got)
	}

	// 3. Fix tanpa description → 400; valid → 201 + FIXING.
	rec = doRequest(t, router, http.MethodPost, "/api/incidents/"+id+"/fixes", dev,
		map[string]string{"description": "", "reference": ""})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("fix kosong: status %d, want 400", rec.Code)
	}
	rec = doRequest(t, router, http.MethodPost, "/api/incidents/"+id+"/fixes", dev,
		map[string]string{"description": "tambah timeout + backoff", "reference": "PR #42"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("fix: status %d body %s", rec.Code, rec.Body.String())
	}
	rec = doRequest(t, router, http.MethodGet, "/api/incidents/"+id, helpdesk, nil)
	if got := decodeBody(t, rec)["status"]; got != "FIXING" {
		t.Fatalf("setelah fix: status %v, want FIXING", got)
	}

	// 4. Verification di luar VERIFYING → 409.
	rec = doRequest(t, router, http.MethodPost, "/api/incidents/"+id+"/verification", qa,
		map[string]string{"result": "PASS"})
	if rec.Code != http.StatusConflict {
		t.Fatalf("verify di FIXING: status %d, want 409", rec.Code)
	}

	// 5. Berangkat ke VERIFYING via status endpoint.
	rec = doRequest(t, router, http.MethodPatch, "/api/incidents/"+id+"/status", dev,
		map[string]string{"status": "VERIFYING"})
	if rec.Code != http.StatusOK {
		t.Fatalf("ke VERIFYING: status %d", rec.Code)
	}

	// 6. FAIL tanpa reason → 400.
	rec = doRequest(t, router, http.MethodPost, "/api/incidents/"+id+"/verification", qa,
		map[string]string{"result": "FAIL", "reason": ""})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("FAIL tanpa reason: status %d, want 400", rec.Code)
	}

	// 7. Verifikasi oleh PIC (bukan QA/koordinator) → 403.
	rec = doRequest(t, router, http.MethodPost, "/api/incidents/"+id+"/verification", dev,
		map[string]string{"result": "PASS"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("verify oleh PIC: status %d, want 403", rec.Code)
	}

	// 8. FAIL oleh QA → 200 + kembali FIXING.
	rec = doRequest(t, router, http.MethodPost, "/api/incidents/"+id+"/verification", qa,
		map[string]string{"result": "FAIL", "reason": "masih timeout di skenario Y"})
	if rec.Code != http.StatusOK {
		t.Fatalf("verify FAIL: status %d body %s", rec.Code, rec.Body.String())
	}
	rec = doRequest(t, router, http.MethodGet, "/api/incidents/"+id, helpdesk, nil)
	if got := decodeBody(t, rec)["status"]; got != "FIXING" {
		t.Fatalf("setelah FAIL: status %v, want FIXING", got)
	}

	// 9. Putaran kedua: fix tambahan → VERIFYING → PASS → RESOLVED.
	rec = doRequest(t, router, http.MethodPost, "/api/incidents/"+id+"/fixes", dev,
		map[string]string{"description": "naikkan timeout skenario Y"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("fix kedua: status %d", rec.Code)
	}
	rec = doRequest(t, router, http.MethodPatch, "/api/incidents/"+id+"/status", dev,
		map[string]string{"status": "VERIFYING"})
	if rec.Code != http.StatusOK {
		t.Fatalf("ke VERIFYING lagi: status %d", rec.Code)
	}
	rec = doRequest(t, router, http.MethodPost, "/api/incidents/"+id+"/verification", qa,
		map[string]string{"result": "PASS", "reason": "skenario Y lolos"})
	if rec.Code != http.StatusOK {
		t.Fatalf("verify PASS: status %d", rec.Code)
	}
	rec = doRequest(t, router, http.MethodGet, "/api/incidents/"+id, helpdesk, nil)
	body := decodeBody(t, rec)
	if body["status"] != "RESOLVED" {
		t.Fatalf("setelah PASS: status %v, want RESOLVED", body["status"])
	}
	if body["resolved_at"] == nil {
		t.Fatal("setelah PASS: resolved_at kosong")
	}

	// 10. Close oleh PIC → 403; oleh koordinator → CLOSED + closed_by.
	rec = doRequest(t, router, http.MethodPost, "/api/incidents/"+id+"/close", dev, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("close oleh PIC: status %d, want 403", rec.Code)
	}
	rec = doRequest(t, router, http.MethodPost, "/api/incidents/"+id+"/close", helpdesk, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("close: status %d body %s", rec.Code, rec.Body.String())
	}
	closed := decodeBody(t, rec)
	if closed["status"] != "CLOSED" || closed["closed_at"] == nil || closed["closed_by"] == nil {
		t.Fatalf("close: tidak lengkap: %v", closed)
	}

	// 11. Status change setelah CLOSED → 409 (terminal).
	rec = doRequest(t, router, http.MethodPatch, "/api/incidents/"+id+"/status", helpdesk,
		map[string]string{"status": "INVESTIGATING"})
	if rec.Code != http.StatusConflict {
		t.Fatalf("status setelah CLOSED: status %d, want 409", rec.Code)
	}

	// 12. Reopen oleh PIC → 403; tanpa reason → 400; manager → INVESTIGATING.
	rec = doRequest(t, router, http.MethodPost, "/api/incidents/"+id+"/reopen", dev,
		map[string]string{"reason": "kambuh"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("reopen oleh PIC: status %d, want 403", rec.Code)
	}
	rec = doRequest(t, router, http.MethodPost, "/api/incidents/"+id+"/reopen", manager,
		map[string]string{"reason": ""})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("reopen tanpa reason: status %d, want 400", rec.Code)
	}
	rec = doRequest(t, router, http.MethodPost, "/api/incidents/"+id+"/reopen", manager,
		map[string]string{"reason": "kambuh di skenario Z"})
	if rec.Code != http.StatusOK {
		t.Fatalf("reopen: status %d body %s", rec.Code, rec.Body.String())
	}
	if got := decodeBody(t, rec)["status"]; got != "INVESTIGATING" {
		t.Fatalf("setelah reopen: status %v, want INVESTIGATING", got)
	}

	// 13. List records: 1 investigation, 2 fixes, 2 verifications.
	for path, want := range map[string]int{
		"/investigations": 1, "/fixes": 2, "/verifications": 2,
	} {
		rec = doRequest(t, router, http.MethodGet, "/api/incidents/"+id+path, helpdesk, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s: status %d", path, rec.Code)
		}
		items, _ := decodeBody(t, rec)["data"].([]any)
		if len(items) != want {
			t.Fatalf("GET %s: count %d, want %d", path, len(items), want)
		}
	}

	// 14. Timeline memuat verification + reopen.
	rec = doRequest(t, router, http.MethodGet, "/api/incidents/"+id+"/timeline", helpdesk, nil)
	types := map[string]bool{}
	for _, it := range decodeBody(t, rec)["data"].([]any) {
		if m, ok := it.(map[string]any); ok {
			if ty, ok := m["type"].(string); ok {
				types[ty] = true
			}
		}
	}
	for _, want := range []string{"investigation", "fix", "verification", "reopen"} {
		if !types[want] {
			t.Fatalf("timeline: tipe %q hilang", want)
		}
	}
}
