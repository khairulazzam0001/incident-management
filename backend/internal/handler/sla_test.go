package handler

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/khairulazzam0001/incident-management/backend/internal/repository"
	"github.com/khairulazzam0001/incident-management/backend/internal/service"
)

// testRouterService is testRouter plus the service, so tests can drive the
// SLA worker clock directly.
func testRouterService(t *testing.T, pool *pgxpool.Pool) (http.Handler, *service.IncidentService) {
	t.Helper()
	repo := repository.New(pool)
	authSvc, err := service.NewAuthService(repo, "test-secret")
	if err != nil {
		t.Fatalf("auth service: %v", err)
	}
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	svc := service.NewIncidentService(repo, nil, log, service.DefaultUploadConfig(t.TempDir()))
	return NewRouter(Deps{Log: log, Auth: authSvc, Incidents: svc}), svc
}

// TestSLAFlow covers SLA-01–SLA-04, SLA-07–SLA-09 (PRD_SLA_Escalation.md §15).
func TestSLAFlow(t *testing.T) {
	pool := testDB(t)
	router, svc := testRouterService(t, pool)
	reset(t, pool)
	ctx := context.Background()

	helpdesk := login(t, router, "helpdesk@example.com")
	manager := login(t, router, "manager@example.com")
	dev := login(t, router, "developer@example.com")
	qa := login(t, router, "qa@example.com")
	customer := login(t, router, "user@example.com")
	team := teamID(t, pool)
	devID := userID(t, pool, "developer@example.com")

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `UPDATE master_sla_policy SET response_minutes = 240, resolution_minutes = 1620,
			calendar_code = 'BUSINESS_HOURS', warn_percent = 75, breach_reminder_minutes = NULL WHERE priority_code = 'P3'`)
		_, _ = pool.Exec(ctx, `DELETE FROM master_business_hours WHERE calendar_code = 'BUSINESS_HOURS'`)
		_, _ = pool.Exec(ctx, `INSERT INTO master_business_hours (calendar_code, weekday, start_minute, end_minute)
			SELECT 'BUSINESS_HOURS', d, 480, 1020 FROM generate_series(1, 5) AS d`)
		_, _ = pool.Exec(ctx, `DELETE FROM master_holiday`)
	})

	post := func(path, token string, body any, want int) map[string]any {
		t.Helper()
		rec := doRequest(t, router, http.MethodPost, path, token, body)
		if rec.Code != want {
			t.Fatalf("POST %s: status %d, want %d, body %s", path, rec.Code, want, rec.Body.String())
		}
		if rec.Body.Len() == 0 {
			return nil
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
	slaOf := func(id string) []map[string]any {
		t.Helper()
		out := []map[string]any{}
		for _, it := range get("/api/incidents/"+id+"/sla", helpdesk)["data"].([]any) {
			out = append(out, it.(map[string]any))
		}
		return out
	}
	find := func(items []map[string]any, metric string, cycle float64) map[string]any {
		t.Helper()
		for _, it := range items {
			if it["metric"] == metric && it["cycle"] == cycle {
				return it
			}
		}
		t.Fatalf("instance %s cycle %v tidak ada: %v", metric, cycle, items)
		return nil
	}
	parse := func(v any) time.Time {
		t.Helper()
		ts, err := time.Parse(time.RFC3339Nano, v.(string))
		if err != nil {
			t.Fatalf("parse waktu %v: %v", v, err)
		}
		return ts
	}
	start := time.Now()
	tick := func(after time.Duration) int {
		t.Helper()
		n, err := svc.ProcessSLA(ctx, start.Add(after))
		if err != nil {
			t.Fatalf("ProcessSLA: %v", err)
		}
		return n
	}

	// SLA-01: P1 → RESPONSE 15 mnt & RESOLUTION 240 mnt (24x7).
	incA, _ := post("/api/incidents", helpdesk, map[string]any{
		"title": "Checkout down total", "severity": "S1", "priority": "P1", "source": "helpdesk",
	}, http.StatusCreated)["id"].(string)
	inst := slaOf(incA)
	if len(inst) != 2 {
		t.Fatalf("instance = %d, want 2", len(inst))
	}
	resp := find(inst, "RESPONSE", 1)
	if d := parse(resp["target_at"]).Sub(parse(resp["started_at"])); d != 15*time.Minute || resp["status"] != "RUNNING" {
		t.Fatalf("response target %v status %v", d, resp["status"])
	}
	if res := find(inst, "RESOLUTION", 1); parse(res["target_at"]).Sub(parse(res["started_at"])) != 240*time.Minute {
		t.Fatalf("resolution target salah: %v", res)
	}
	if sla := get("/api/incidents/"+incA, helpdesk)["sla"].(map[string]any); sla["response"].(map[string]any)["status"] != "RUNNING" {
		t.Fatalf("ringkasan sla detail = %v", sla)
	}

	// SLA-03: warning sekali (koordinator karena belum ada PIC); tick ulang tidak menggandakan.
	if n := tick(13 * time.Minute); n != 1 {
		t.Fatalf("warning events = %d, want 1", n)
	}
	if n := tick(13 * time.Minute); n != 0 {
		t.Fatalf("warning ganda: %d", n)
	}
	if types, _ := notifTypes(t, router, helpdesk); !types["sla_warning"] {
		t.Fatalf("helpdesk tanpa sla_warning: %v", types)
	}
	if items := get("/api/incidents?sla=at_risk", helpdesk)["data"].([]any); len(items) != 1 {
		t.Fatalf("filter at_risk = %d, want 1", len(items))
	}

	// SLA-04: breach → Manager; filter breached; reminder 60 mnt kemudian.
	if n := tick(16 * time.Minute); n != 1 {
		t.Fatalf("breach events = %d, want 1", n)
	}
	if types, _ := notifTypes(t, router, manager); !types["sla_breached"] {
		t.Fatalf("manager tanpa sla_breached: %v", types)
	}
	if items := get("/api/incidents?sla=breached", helpdesk)["data"].([]any); len(items) != 1 {
		t.Fatalf("filter breached = %d, want 1", len(items))
	}
	if n := tick(16*time.Minute + 61*time.Minute); n != 1 {
		t.Fatalf("reminder events = %d, want 1", n)
	}
	if types, _ := notifTypes(t, router, manager); !types["sla_reminder"] {
		t.Fatalf("manager tanpa sla_reminder: %v", types)
	}
	expectStatus(t, doRequest(t, router, http.MethodGet, "/api/incidents?sla=late", helpdesk, nil), http.StatusBadRequest, "filter sla invalid")

	// Assign setelah breach → RESPONSE tetap BREACHED tapi berhenti; keluar dari filter breached.
	expectStatus(t, doRequest(t, router, http.MethodPatch, "/api/incidents/"+incA+"/assignment", helpdesk,
		map[string]string{"team_id": team, "pic_id": devID}), http.StatusOK, "assign A")
	if r := find(slaOf(incA), "RESPONSE", 1); r["status"] != "BREACHED" || r["stopped_at"] == nil {
		t.Fatalf("response setelah assign = %v", r)
	}
	if items := get("/api/incidents?sla=breached", helpdesk)["data"].([]any); len(items) != 0 {
		t.Fatalf("filter breached setelah assign = %d, want 0", len(items))
	}

	// SLA-02 + SLA-07: P2 assign cepat → MET; resolve → MET; reopen → siklus 2.
	incB, _ := post("/api/incidents", helpdesk, map[string]any{
		"title": "Laporan bulanan lambat", "severity": "S3", "priority": "P2", "source": "user",
	}, http.StatusCreated)["id"].(string)
	expectStatus(t, doRequest(t, router, http.MethodPatch, "/api/incidents/"+incB+"/assignment", helpdesk,
		map[string]string{"team_id": team, "pic_id": devID}), http.StatusOK, "assign B")
	if r := find(slaOf(incB), "RESPONSE", 1); r["status"] != "MET" {
		t.Fatalf("response B = %v", r["status"])
	}
	post("/api/incidents/"+incB+"/investigations", dev, map[string]string{"notes": "cek query", "findings": "index hilang"}, http.StatusCreated)
	post("/api/incidents/"+incB+"/fixes", dev, map[string]string{"description": "tambah index", "reference": "PR #9"}, http.StatusCreated)
	expectStatus(t, doRequest(t, router, http.MethodPatch, "/api/incidents/"+incB+"/status", dev,
		map[string]string{"status": "VERIFYING"}), http.StatusOK, "verifying B")
	post("/api/incidents/"+incB+"/verification", qa, map[string]string{"result": "PASS"}, http.StatusOK)
	if r := find(slaOf(incB), "RESOLUTION", 1); r["status"] != "MET" || r["stopped_at"] == nil {
		t.Fatalf("resolution B = %v", r)
	}
	post("/api/incidents/"+incB+"/reopen", helpdesk, map[string]string{"reason": "muncul lagi"}, http.StatusOK)
	if r := find(slaOf(incB), "RESOLUTION", 2); r["status"] != "RUNNING" {
		t.Fatalf("resolution cycle 2 = %v", r)
	}

	// Akses: User/Customer bukan pelapor → 403.
	expectStatus(t, doRequest(t, router, http.MethodGet, "/api/incidents/"+incA+"/sla", customer, nil), http.StatusForbidden, "customer sla")

	// SLA-08: policy — baca semua role internal, ubah hanya Manager; snapshot tidak berubah.
	if pol := get("/api/master/sla-policies", qa)["policies"].([]any); len(pol) != 4 {
		t.Fatalf("policies = %d, want 4", len(pol))
	}
	expectStatus(t, doRequest(t, router, http.MethodGet, "/api/master/sla-policies", customer, nil), http.StatusForbidden, "customer policies")
	newP3 := map[string]any{"response_minutes": 120, "resolution_minutes": 600, "calendar_code": "24x7", "warn_percent": 80}
	expectStatus(t, doRequest(t, router, http.MethodPut, "/api/master/sla-policies/P3", helpdesk, newP3), http.StatusForbidden, "helpdesk put")
	expectStatus(t, doRequest(t, router, http.MethodPut, "/api/master/sla-policies/P3", manager,
		map[string]any{"response_minutes": 600, "resolution_minutes": 60, "calendar_code": "24x7", "warn_percent": 80}),
		http.StatusBadRequest, "resolution < response")
	expectStatus(t, doRequest(t, router, http.MethodPut, "/api/master/sla-policies/P3", manager,
		map[string]any{"response_minutes": 60, "resolution_minutes": 600, "calendar_code": "NOPE", "warn_percent": 80}),
		http.StatusBadRequest, "kalender tak dikenal")
	expectStatus(t, doRequest(t, router, http.MethodPut, "/api/master/sla-policies/P9", manager, newP3), http.StatusNotFound, "priority tak dikenal")
	expectStatus(t, doRequest(t, router, http.MethodPut, "/api/master/sla-policies/P3", manager, newP3), http.StatusOK, "manager put")
	incC, _ := post("/api/incidents", helpdesk, map[string]any{
		"title": "Typo di halaman profil", "severity": "S4", "priority": "P3", "source": "user",
	}, http.StatusCreated)["id"].(string)
	if r := find(slaOf(incC), "RESPONSE", 1); r["target_minutes"] != float64(120) || r["calendar_code"] != "24x7" {
		t.Fatalf("incident baru tidak memakai policy baru: %v", r)
	}
	if r := find(slaOf(incA), "RESOLUTION", 1); r["target_minutes"] != float64(240) {
		t.Fatalf("snapshot A berubah: %v", r["target_minutes"])
	}

	// Jam kerja & hari libur.
	expectStatus(t, doRequest(t, router, http.MethodPut, "/api/master/business-hours/24x7", manager,
		map[string]any{"hours": []map[string]int{{"weekday": 1, "start_minute": 480, "end_minute": 1020}}}),
		http.StatusBadRequest, "jam kerja 24x7")
	expectStatus(t, doRequest(t, router, http.MethodPut, "/api/master/business-hours/BUSINESS_HOURS", manager,
		map[string]any{"hours": []map[string]int{{"weekday": 1, "start_minute": 1020, "end_minute": 480}}}),
		http.StatusBadRequest, "jam terbalik")
	expectStatus(t, doRequest(t, router, http.MethodPut, "/api/master/business-hours/BUSINESS_HOURS", manager,
		map[string]any{"hours": []map[string]int{{"weekday": 1, "start_minute": 540, "end_minute": 1080}}}),
		http.StatusOK, "ubah jam kerja")
	hol := map[string]string{"date": "2026-12-25", "name": "Natal"}
	expectStatus(t, doRequest(t, router, http.MethodPost, "/api/master/holidays", helpdesk, hol), http.StatusForbidden, "helpdesk holiday")
	holID, _ := post("/api/master/holidays", manager, hol, http.StatusCreated)["id"].(string)
	post("/api/master/holidays", manager, hol, http.StatusConflict)
	post("/api/master/holidays", manager, map[string]string{"date": "25-12-2026", "name": "Natal"}, http.StatusBadRequest)
	if items := get("/api/master/holidays?year=2026", qa)["data"].([]any); len(items) != 1 {
		t.Fatalf("holidays = %d, want 1", len(items))
	}
	expectStatus(t, doRequest(t, router, http.MethodDelete, "/api/master/holidays/"+holID, manager, nil), http.StatusOK, "hapus libur")
	expectStatus(t, doRequest(t, router, http.MethodDelete, "/api/master/holidays/"+holID, manager, nil), http.StatusNotFound, "hapus lagi")

	// SLA-09: dashboard.
	board := get("/api/dashboard/sla", manager)
	if buckets := board["buckets"].([]any); len(buckets) < 4 {
		t.Fatalf("buckets = %v", buckets)
	}
	if board["worker_last_tick_at"] == nil {
		t.Fatal("worker_last_tick_at kosong setelah tick")
	}
	expectStatus(t, doRequest(t, router, http.MethodGet, "/api/dashboard/sla", customer, nil), http.StatusForbidden, "customer dashboard sla")
	expectStatus(t, doRequest(t, router, http.MethodGet, "/api/dashboard/sla?from=kemarin", manager, nil), http.StatusBadRequest, "tanggal invalid")
}
