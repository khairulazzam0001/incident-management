package handler

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func sendAlert(t *testing.T, router http.Handler, integrationID, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/integrations/alerts/"+integrationID, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func amPayload(alerts ...string) string {
	return `{"receiver":"im","status":"firing","alerts":[` + strings.Join(alerts, ",") + `]}`
}

func amAlert(fp, status, severity, service, env, summary string) string {
	return fmt.Sprintf(`{"status":%q,"fingerprint":%q,
		"labels":{"alertname":"HighErrorRate","severity":%q,"service":%q,"env":%q},
		"annotations":{"summary":%q,"description":"dari test"},
		"startsAt":"2026-10-04T01:00:00Z","generatorURL":"http://prom/graph"}`, status, fp, severity, service, env, summary)
}

// TestMonitoringFlow covers MON-01–MON-08 (PRD_Monitoring_Integration.md §15).
func TestMonitoringFlow(t *testing.T) {
	pool := testDB(t)
	router := testRouter(t, pool)
	reset(t, pool)
	ctx := context.Background()

	manager := login(t, router, "manager@example.com")
	dev := login(t, router, "developer@example.com")
	helpdesk := login(t, router, "helpdesk@example.com")
	app := applicationID(t, pool) // code "change-test"

	create := func(body map[string]any) (string, string) {
		t.Helper()
		rec := doRequest(t, router, http.MethodPost, "/api/master/integrations", manager, body)
		expectStatus(t, rec, http.StatusCreated, "create integration")
		res := decodeBody(t, rec)
		it := res["integration"].(map[string]any)
		token, _ := res["token"].(string)
		if !strings.HasPrefix(token, "imk_") || it["token_last4"] != token[len(token)-4:] {
			t.Fatalf("token = %q integration = %v", token, it)
		}
		if res["webhook_path"] != "/api/integrations/alerts/"+it["id"].(string) {
			t.Fatalf("webhook_path = %v", res["webhook_path"])
		}
		return it["id"].(string), token
	}

	// MON-01: hanya Manager/DevOps; validasi.
	expectStatus(t, doRequest(t, router, http.MethodPost, "/api/master/integrations", dev,
		map[string]any{"type": "ALERTMANAGER", "name": "Prom"}), http.StatusForbidden, "dev create")
	expectStatus(t, doRequest(t, router, http.MethodPost, "/api/master/integrations", manager,
		map[string]any{"type": "ZABBIX", "name": "Prom"}), http.StatusBadRequest, "tipe invalid")
	expectStatus(t, doRequest(t, router, http.MethodPost, "/api/master/integrations", manager,
		map[string]any{"type": "ALERTMANAGER", "name": "Prom", "ip_allowlist": []string{"bukan-ip"}}), http.StatusBadRequest, "cidr invalid")
	expectStatus(t, doRequest(t, router, http.MethodPost, "/api/master/integrations", manager,
		map[string]any{"type": "ALERTMANAGER", "name": "Prom", "mapping": map[string]any{"tidak_dikenal": 1}}), http.StatusBadRequest, "mapping invalid")
	intID, token := create(map[string]any{"type": "ALERTMANAGER", "name": "Prometheus Prod"})
	list := decodeBody(t, doRequest(t, router, http.MethodGet, "/api/master/integrations", manager, nil))["data"].([]any)
	for _, it := range list {
		if _, leaked := it.(map[string]any)["token_hash"]; leaked {
			t.Fatal("token_hash bocor di list")
		}
	}

	// MON-05: autentikasi & batas.
	firing := amPayload(amAlert("fp-1", "firing", "critical", "change-test", "production", "Error rate checkout > 5%"))
	expectStatus(t, sendAlert(t, router, intID, "", firing), http.StatusUnauthorized, "tanpa token")
	expectStatus(t, sendAlert(t, router, intID, "imk_salah", firing), http.StatusUnauthorized, "token salah")
	expectStatus(t, sendAlert(t, router, "00000000-0000-4000-8000-000000000000", token, firing), http.StatusUnauthorized, "integrasi tak dikenal")
	expectStatus(t, sendAlert(t, router, intID, token, `{"foo":1}`), http.StatusBadRequest, "payload invalid")
	big := `{"alerts":[],"pad":"` + strings.Repeat("x", 1<<20) + `"}`
	expectStatus(t, sendAlert(t, router, intID, token, big), http.StatusRequestEntityTooLarge, "body > 1 MiB")

	// MON-02: critical → incident S1/P1, app & env terpetakan, source monitoring, tanpa reporter, ber-SLA.
	rec := sendAlert(t, router, intID, token, firing)
	expectStatus(t, rec, http.StatusAccepted, "firing")
	if res := decodeBody(t, rec); res["created"] != float64(1) {
		t.Fatalf("result = %v", res)
	}
	var incID, sev, prio, source string
	var appID, env, reporter *string
	if err := pool.QueryRow(ctx, `SELECT i.id, severity_code, priority_code, source_code, application_id, environment_code, reporter_id
		FROM trans_incident i JOIN trans_incident_alert l ON l.incident_id = i.id WHERE l.fingerprint = 'fp-1'`).
		Scan(&incID, &sev, &prio, &source, &appID, &env, &reporter); err != nil {
		t.Fatalf("incident dari alert: %v", err)
	}
	if sev != "S1" || prio != "P1" || source != "monitoring" || appID == nil || *appID != app || env == nil || *env != "production" || reporter != nil {
		t.Fatalf("incident = %s/%s %s app=%v env=%v reporter=%v", sev, prio, source, appID, env, reporter)
	}
	if sla := decodeBody(t, doRequest(t, router, http.MethodGet, "/api/incidents/"+incID, helpdesk, nil))["sla"]; sla == nil {
		t.Fatal("incident dari alert tanpa SLA")
	}
	if types, _ := notifTypes(t, router, helpdesk); !types["created"] {
		t.Fatalf("koordinator tanpa notifikasi created: %v", types)
	}

	// MON-03: retry identik → duplicate; firing berikutnya → fire_count naik, tetap 1 incident.
	if res := decodeBody(t, sendAlert(t, router, intID, token, firing)); res["duplicate"] != float64(1) {
		t.Fatalf("retry identik = %v", res)
	}
	for i := 0; i < 4; i++ {
		body := amPayload(amAlert("fp-1", "firing", "critical", "change-test", "production", fmt.Sprintf("Error rate checkout > %d%%", 6+i)))
		if res := decodeBody(t, sendAlert(t, router, intID, token, body)); res["updated"] != float64(1) {
			t.Fatalf("refire %d = %v", i, res)
		}
	}
	alerts := decodeBody(t, doRequest(t, router, http.MethodGet, "/api/incidents/"+incID+"/alerts", helpdesk, nil))["data"].([]any)
	if len(alerts) != 1 || alerts[0].(map[string]any)["fire_count"] != float64(5) {
		t.Fatalf("alerts = %v", alerts)
	}
	var refired int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM trans_incident_activity WHERE incident_id = $1 AND type = 'alert_refired'`, incID).Scan(&refired)
	if refired != 0 {
		t.Fatalf("activity refire tidak dibatasi 15 mnt: %d", refired)
	}

	// MON-04: resolved → komentar sistem, status tetap NEW.
	resolved := amPayload(amAlert("fp-1", "resolved", "critical", "change-test", "production", "Error rate checkout > 5%"))
	if res := decodeBody(t, sendAlert(t, router, intID, token, resolved)); res["updated"] != float64(1) {
		t.Fatalf("resolved = %v", res)
	}
	inc := decodeBody(t, doRequest(t, router, http.MethodGet, "/api/incidents/"+incID, helpdesk, nil))
	if inc["status"] != "NEW" {
		t.Fatalf("status berubah oleh alert resolved: %v", inc["status"])
	}
	comments := decodeBody(t, doRequest(t, router, http.MethodGet, "/api/incidents/"+incID+"/comments", helpdesk, nil))["data"].([]any)
	if len(comments) != 1 || comments[0].(map[string]any)["author_id"] != nil {
		t.Fatalf("komentar sistem = %v", comments)
	}

	// Q2: warning → S3; info → hanya event.
	warn := amPayload(amAlert("fp-warn", "firing", "warning", "change-test", "staging", "Disk 80%"))
	if res := decodeBody(t, sendAlert(t, router, intID, token, warn)); res["created"] != float64(1) {
		t.Fatalf("warning = %v", res)
	}
	info := amPayload(amAlert("fp-info", "firing", "info", "change-test", "staging", "Deploy selesai"))
	if res := decodeBody(t, sendAlert(t, router, intID, token, info)); res["ignored"] != float64(1) {
		t.Fatalf("info = %v", res)
	}

	// MON-06: label aplikasi tak dikenal tanpa default → incident tanpa app + event mapping_warning.
	unknown := amPayload(amAlert("fp-unk", "firing", "high", "tidak-ada", "production", "Queue lag tinggi"))
	expectStatus(t, sendAlert(t, router, intID, token, unknown), http.StatusAccepted, "app tak dikenal")
	var warning string
	_ = pool.QueryRow(ctx, `SELECT warning FROM trans_alert_event WHERE fingerprint = 'fp-unk'`).Scan(&warning)
	if !strings.HasPrefix(warning, "mapping_warning") {
		t.Fatalf("warning = %q", warning)
	}

	// Q4: muncul lagi < 24 jam setelah CLOSED → komentar + notif; > 24 jam → incident baru.
	if _, err := pool.Exec(ctx, `UPDATE trans_incident SET status_code = 'CLOSED', closed_at = now() WHERE id = $1`, incID); err != nil {
		t.Fatal(err)
	}
	again := amPayload(amAlert("fp-1", "firing", "critical", "change-test", "production", "Error rate checkout kembali"))
	if res := decodeBody(t, sendAlert(t, router, intID, token, again)); res["updated"] != float64(1) {
		t.Fatalf("reoccur < 24 jam = %v", res)
	}
	if types, _ := notifTypes(t, router, helpdesk); !types["alert_reoccurred"] {
		t.Fatalf("tanpa notifikasi alert_reoccurred: %v", types)
	}
	if _, err := pool.Exec(ctx, `UPDATE trans_incident SET closed_at = now() - INTERVAL '25 hours' WHERE id = $1`, incID); err != nil {
		t.Fatal(err)
	}
	later := amPayload(amAlert("fp-1", "firing", "critical", "change-test", "production", "Error rate checkout minggu depan"))
	if res := decodeBody(t, sendAlert(t, router, intID, token, later)); res["created"] != float64(1) {
		t.Fatalf("reoccur > 24 jam = %v", res)
	}

	// Dry-run: tidak membuat incident.
	var before, after int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM trans_incident`).Scan(&before)
	req := httptest.NewRequest(http.MethodPost, "/api/master/integrations/"+intID+"/test",
		bytes.NewBufferString(amPayload(amAlert("fp-dry", "firing", "critical", "change-test", "production", "Uji"))))
	req.Header.Set("Authorization", "Bearer "+manager)
	dry := httptest.NewRecorder()
	router.ServeHTTP(dry, req)
	expectStatus(t, dry, http.StatusOK, "dry run")
	if items := decodeBody(t, dry)["data"].([]any); items[0].(map[string]any)["action"] != "create" {
		t.Fatalf("dry run = %v", items)
	}
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM trans_incident`).Scan(&after)
	if before != after {
		t.Fatal("dry run membuat incident")
	}

	// Event log.
	events := decodeBody(t, doRequest(t, router, http.MethodGet, "/api/master/integrations/"+intID+"/events?limit=100", manager, nil))
	if total := events["meta"].(map[string]any)["total"].(float64); total < 10 {
		t.Fatalf("event log total = %v", total)
	}
	expectStatus(t, doRequest(t, router, http.MethodGet, "/api/master/integrations/"+intID+"/events", dev, nil), http.StatusForbidden, "dev events")

	// MON-08: rotasi token → token lama ditolak.
	rot := decodeBody(t, doRequest(t, router, http.MethodPost, "/api/master/integrations/"+intID+"/rotate-token", manager, nil))
	newToken, _ := rot["token"].(string)
	expectStatus(t, sendAlert(t, router, intID, token, warn), http.StatusUnauthorized, "token lama")
	expectStatus(t, sendAlert(t, router, intID, newToken, warn), http.StatusAccepted, "token baru")

	// Nonaktif → 403; allowlist IP → 403 lalu 202.
	expectStatus(t, doRequest(t, router, http.MethodPatch, "/api/master/integrations/"+intID, manager,
		map[string]any{"name": "Prometheus Prod", "is_active": false}), http.StatusOK, "nonaktifkan")
	expectStatus(t, sendAlert(t, router, intID, newToken, warn), http.StatusForbidden, "integrasi nonaktif")
	expectStatus(t, doRequest(t, router, http.MethodPatch, "/api/master/integrations/"+intID, manager,
		map[string]any{"name": "Prometheus Prod", "is_active": true, "ip_allowlist": []string{"10.0.0.0/8"}}), http.StatusOK, "allowlist")
	expectStatus(t, sendAlert(t, router, intID, newToken, warn), http.StatusForbidden, "ip di luar allowlist")
	expectStatus(t, doRequest(t, router, http.MethodPatch, "/api/master/integrations/"+intID, manager,
		map[string]any{"name": "Prometheus Prod", "ip_allowlist": []string{"192.0.2.1"}}), http.StatusOK, "allowlist ip test")
	expectStatus(t, sendAlert(t, router, intID, newToken, warn), http.StatusAccepted, "ip dalam allowlist")

	// Rate limit per integrasi.
	rlID, rlToken := create(map[string]any{"type": "ALERTMANAGER", "name": "Rate limited", "rate_limit_per_min": 2})
	for i := 0; i < 2; i++ {
		expectStatus(t, sendAlert(t, router, rlID, rlToken, amPayload()), http.StatusAccepted, "dalam limit")
	}
	expectStatus(t, sendAlert(t, router, rlID, rlToken, amPayload()), http.StatusTooManyRequests, "lewat limit")

	// Storm: 12 fingerprint baru sekaligus → 10 incident + 1 induk menampung 2 sisanya.
	stID, stToken := create(map[string]any{"type": "GRAFANA", "name": "Grafana storm"})
	var batch []string
	for i := 0; i < 12; i++ {
		batch = append(batch, amAlert(fmt.Sprintf("st-%d", i), "firing", "high", "change-test", "production", fmt.Sprintf("Node %d down", i)))
	}
	res := decodeBody(t, sendAlert(t, router, stID, stToken, amPayload(batch...)))
	if res["created"] != float64(10) || res["grouped"] != float64(2) {
		t.Fatalf("storm = %v", res)
	}
	if types, _ := notifTypes(t, router, manager); !types["alert_storm"] {
		t.Fatalf("manager tanpa alert_storm: %v", types)
	}
	var parents int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM trans_incident_alert WHERE integration_id = $1 AND fingerprint LIKE 'storm:%'`, stID).Scan(&parents)
	if parents != 1 {
		t.Fatalf("storm parent = %d, want 1", parents)
	}
}
