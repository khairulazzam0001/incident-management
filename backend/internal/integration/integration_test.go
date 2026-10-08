package integration

import (
	"strings"
	"testing"
)

const alertmanagerFixture = `{
  "receiver": "incident-management", "status": "firing",
  "alerts": [
    {"status": "firing",
     "labels": {"alertname": "HighErrorRate", "severity": "critical", "service": "checkout", "env": "Production"},
     "annotations": {"summary": "Error rate checkout > 5%", "description": "5xx naik sejak deploy"},
     "startsAt": "2026-10-04T01:00:00Z", "generatorURL": "http://prom/graph?g0", "fingerprint": "abc123"},
    {"status": "resolved",
     "labels": {"alertname": "DiskFull", "severity": "warning"},
     "annotations": {}, "startsAt": "2026-10-04T00:00:00Z"}
  ]}`

const grafanaFixture = `{
  "receiver": "im", "status": "firing", "orgId": 1,
  "alerts": [{"status": "firing", "labels": {"alertname": "Latency", "severity": "high", "app": "payment"},
    "annotations": {"summary": "p95 latency 2s"}, "fingerprint": "g1",
    "dashboardURL": "http://grafana/d/x", "panelURL": "http://grafana/d/x?viewPanel=2"}]}`

func TestParseAlertmanager(t *testing.T) {
	alerts, err := Parse(TypeAlertmanager, []byte(alertmanagerFixture), Mapping{})
	if err != nil || len(alerts) != 2 {
		t.Fatalf("parse: %v, n=%d", err, len(alerts))
	}
	a := alerts[0]
	if a.Fingerprint != "abc123" || a.Status != StatusFiring || a.Title != "Error rate checkout > 5%" ||
		a.SourceURL != "http://prom/graph?g0" || a.StartsAt == nil {
		t.Fatalf("alert 0 = %+v", a)
	}
	b := alerts[1]
	if b.Status != StatusResolved || b.Title != "DiskFull" || len(b.Fingerprint) != 16 {
		t.Fatalf("alert 1 (fallback title & fingerprint) = %+v", b)
	}
	m := Map(a, Mapping{})
	if m.Severity != "S1" || m.Priority != "P1" || m.ApplicationCode != "checkout" || m.Environment != "production" {
		t.Fatalf("map = %+v", m)
	}
	if !strings.Contains(m.Description, "5xx naik sejak deploy") || !strings.Contains(m.Description, "service=checkout") {
		t.Fatalf("description = %q", m.Description)
	}
	if _, err := Parse(TypeAlertmanager, []byte(`{"foo":1}`), Mapping{}); err != ErrInvalidPayload {
		t.Fatalf("payload tanpa alerts: %v", err)
	}
	if _, err := Parse(TypeAlertmanager, []byte(`bukan json`), Mapping{}); err != ErrInvalidPayload {
		t.Fatalf("bukan json: %v", err)
	}
}

func TestParseGrafana(t *testing.T) {
	alerts, err := Parse(TypeGrafana, []byte(grafanaFixture), Mapping{})
	if err != nil || len(alerts) != 1 {
		t.Fatalf("parse: %v", err)
	}
	if alerts[0].SourceURL != "http://grafana/d/x?viewPanel=2" {
		t.Fatalf("source url = %q", alerts[0].SourceURL)
	}
	if m := Map(alerts[0], Mapping{}); m.Severity != "S2" || m.ApplicationCode != "payment" {
		t.Fatalf("map = %+v", m)
	}
}

func TestParseGeneric(t *testing.T) {
	body := `{"data": {"events": [
		{"id": "e1", "state": "triggered", "name": "DB down", "level": "critical", "tags": {"service": "billing"}},
		{"id": "e2", "state": "RECOVERED", "name": "DB down", "level": "critical"}]}}`
	m := Mapping{AlertsPath: "data.events", StatusPath: "state", TitlePath: "name", SeverityPath: "level",
		LabelsPath: "tags", ResolvedValues: []string{"recovered"}}
	alerts, err := Parse(TypeGenericJSON, []byte(body), m)
	if err != nil || len(alerts) != 2 {
		t.Fatalf("parse: %v", err)
	}
	if alerts[0].Fingerprint != "e1" || alerts[0].Status != StatusFiring || alerts[1].Status != StatusResolved {
		t.Fatalf("alerts = %+v", alerts)
	}
	if mp := Map(alerts[0], m); mp.Severity != "S1" || mp.ApplicationCode != "billing" || mp.Title != "DB down" {
		t.Fatalf("map = %+v", mp)
	}
	if _, err := Parse(TypeGenericJSON, []byte(`{"data":{}}`), m); err != ErrInvalidPayload {
		t.Fatalf("alerts_path hilang: %v", err)
	}
}

func TestMapRules(t *testing.T) {
	cases := []struct {
		labels   map[string]string
		mapping  Mapping
		sev, pri string
	}{
		{map[string]string{"severity": "warning"}, Mapping{}, "S3", "P3"},
		{map[string]string{"severity": "info"}, Mapping{}, "S4", "P4"},
		{map[string]string{"severity": "aneh"}, Mapping{}, "S3", "P3"},
		{map[string]string{"severity": "critical", "priority": "p2"}, Mapping{}, "S1", "P2"},
		{map[string]string{"level": "sev1"}, Mapping{SeverityLabel: "level", SeverityMap: map[string]string{"sev1": "S1"}}, "S1", "P1"},
	}
	for _, c := range cases {
		m := Map(Alert{Title: "x", Labels: c.labels}, c.mapping)
		if m.Severity != c.sev || m.Priority != c.pri {
			t.Errorf("labels %v → %s/%s, want %s/%s", c.labels, m.Severity, m.Priority, c.sev, c.pri)
		}
	}
	if got := IncidentTitle("up"); got != "Alert: up" {
		t.Errorf("judul pendek = %q", got)
	}
	if got := IncidentTitle(strings.Repeat("a", 300)); len([]rune(got)) != 200 {
		t.Errorf("judul panjang = %d rune", len([]rune(got)))
	}
	if LabelFingerprint(map[string]string{"a": "1", "b": "2"}) != LabelFingerprint(map[string]string{"b": "2", "a": "1"}) {
		t.Error("fingerprint harus stabil terhadap urutan label")
	}
	if SeverityRank("S2") != 2 || SeverityRank("x") != 4 {
		t.Error("SeverityRank")
	}
}
