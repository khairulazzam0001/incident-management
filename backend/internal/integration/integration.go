// Package integration normalizes monitoring webhooks into alerts and maps
// them to incident fields (PRD_Monitoring_Integration.md §6.2–§6.3). Pure
// functions only, so every vendor format is unit-tested with fixtures.
package integration

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Integration types.
const (
	TypeAlertmanager = "ALERTMANAGER"
	TypeGrafana      = "GRAFANA"
	TypeGenericJSON  = "GENERIC_JSON"

	StatusFiring   = "firing"
	StatusResolved = "resolved"
)

// Alert is one normalized alert.
type Alert struct {
	Fingerprint string            `json:"fingerprint"`
	Status      string            `json:"status"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Labels      map[string]string `json:"labels"`
	StartsAt    *time.Time        `json:"starts_at,omitempty"`
	SourceURL   string            `json:"source_url"`
	Raw         json.RawMessage   `json:"-"`
}

// Mapping is the per-integration configuration stored in
// master_integration.mapping. Zero values fall back to defaults.
type Mapping struct {
	SeverityLabel string            `json:"severity_label,omitempty"`
	SeverityMap   map[string]string `json:"severity_map,omitempty"`
	PriorityLabel string            `json:"priority_label,omitempty"`
	AppLabels     []string          `json:"app_labels,omitempty"`
	EnvLabels     []string          `json:"env_labels,omitempty"`
	// GENERIC_JSON only: dotted paths into each alert object.
	AlertsPath      string   `json:"alerts_path,omitempty"`
	FingerprintPath string   `json:"fingerprint_path,omitempty"`
	StatusPath      string   `json:"status_path,omitempty"`
	TitlePath       string   `json:"title_path,omitempty"`
	DescriptionPath string   `json:"description_path,omitempty"`
	SeverityPath    string   `json:"severity_path,omitempty"`
	LabelsPath      string   `json:"labels_path,omitempty"`
	SourceURLPath   string   `json:"source_url_path,omitempty"`
	ResolvedValues  []string `json:"resolved_values,omitempty"`
}

// DefaultSeverityMap is Q2: critical→S1, high/error→S2, warning→S3, info→S4.
var DefaultSeverityMap = map[string]string{
	"critical": "S1", "page": "S1",
	"high": "S2", "error": "S2", "major": "S2",
	"warning": "S3", "warn": "S3", "minor": "S3",
	"info": "S4", "informational": "S4", "none": "S4",
}

// ErrInvalidPayload means the body could not be parsed for the type.
var ErrInvalidPayload = errors.New("payload tidak sesuai format integrasi")

// Parse normalizes a webhook body for the integration type.
func Parse(kind string, body []byte, m Mapping) ([]Alert, error) {
	switch kind {
	case TypeAlertmanager, TypeGrafana:
		return parseAlertmanager(body)
	case TypeGenericJSON:
		return parseGeneric(body, m)
	default:
		return nil, fmt.Errorf("tipe integrasi tidak dikenal: %s", kind)
	}
}

// amAlert covers Prometheus Alertmanager and Grafana unified alerting
// webhooks (Grafana adds dashboardURL/panelURL).
type amAlert struct {
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     *time.Time        `json:"startsAt"`
	GeneratorURL string            `json:"generatorURL"`
	DashboardURL string            `json:"dashboardURL"`
	PanelURL     string            `json:"panelURL"`
	Fingerprint  string            `json:"fingerprint"`
}

func parseAlertmanager(body []byte) ([]Alert, error) {
	var payload struct {
		Alerts []json.RawMessage `json:"alerts"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || payload.Alerts == nil {
		return nil, ErrInvalidPayload
	}
	out := make([]Alert, 0, len(payload.Alerts))
	for _, raw := range payload.Alerts {
		var a amAlert
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, ErrInvalidPayload
		}
		title := firstNonEmpty(a.Annotations["summary"], a.Annotations["title"], a.Labels["alertname"], "Alert tanpa judul")
		desc := firstNonEmpty(a.Annotations["description"], a.Annotations["message"])
		fp := a.Fingerprint
		if fp == "" {
			fp = LabelFingerprint(a.Labels)
		}
		out = append(out, Alert{
			Fingerprint: fp, Status: normStatus(a.Status, nil), Title: title, Description: desc,
			Labels: nonNilLabels(a.Labels), StartsAt: a.StartsAt,
			SourceURL: firstNonEmpty(a.PanelURL, a.DashboardURL, a.GeneratorURL), Raw: raw,
		})
	}
	return out, nil
}

func parseGeneric(body []byte, m Mapping) ([]Alert, error) {
	var root any
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, ErrInvalidPayload
	}
	items := []any{root}
	if m.AlertsPath != "" {
		v, ok := lookup(root, m.AlertsPath)
		if !ok {
			return nil, ErrInvalidPayload
		}
		items = toSlice(v)
	} else if arr, ok := root.([]any); ok {
		items = arr
	}
	path := func(p, def string) string {
		if p == "" {
			return def
		}
		return p
	}
	out := make([]Alert, 0, len(items))
	for _, it := range items {
		obj, ok := it.(map[string]any)
		if !ok {
			return nil, ErrInvalidPayload
		}
		raw, _ := json.Marshal(obj)
		labels := map[string]string{}
		if v, ok := lookup(obj, path(m.LabelsPath, "labels")); ok {
			if lm, ok := v.(map[string]any); ok {
				for k, val := range lm {
					labels[k] = fmt.Sprint(val)
				}
			}
		}
		if sev := str(obj, path(m.SeverityPath, "severity")); sev != "" {
			labels[severityLabel(m)] = sev
		}
		fp := str(obj, path(m.FingerprintPath, "id"))
		if fp == "" {
			fp = LabelFingerprint(labels) + ":" + str(obj, path(m.TitlePath, "title"))
		}
		out = append(out, Alert{
			Fingerprint: fp,
			Status:      normStatus(str(obj, path(m.StatusPath, "status")), m.ResolvedValues),
			Title:       firstNonEmpty(str(obj, path(m.TitlePath, "title")), "Alert tanpa judul"),
			Description: str(obj, path(m.DescriptionPath, "description")),
			Labels:      labels,
			SourceURL:   str(obj, path(m.SourceURLPath, "url")),
			Raw:         raw,
		})
	}
	return out, nil
}

// Mapped is the incident-facing result of mapping one alert.
type Mapped struct {
	Severity        string `json:"severity"`
	Priority        string `json:"priority"`
	ApplicationCode string `json:"application_code"`
	Environment     string `json:"environment"`
	Title           string `json:"title"`
	Description     string `json:"description"`
}

// Map derives severity/priority/application/environment + incident text.
// Unknown severity values map to S3 (warning) so they are never dropped silently.
func Map(a Alert, m Mapping) Mapped {
	sevMap := DefaultSeverityMap
	if len(m.SeverityMap) > 0 {
		sevMap = m.SeverityMap
	}
	sev := sevMap[strings.ToLower(strings.TrimSpace(a.Labels[severityLabel(m)]))]
	if sev == "" {
		sev = "S3"
	}
	prio := "P" + strings.TrimPrefix(sev, "S")
	pl := m.PriorityLabel
	if pl == "" {
		pl = "priority"
	}
	if p := strings.ToUpper(strings.TrimSpace(a.Labels[pl])); len(p) == 2 && p[0] == 'P' && p[1] >= '1' && p[1] <= '4' {
		prio = p
	}
	appLabels := m.AppLabels
	if len(appLabels) == 0 {
		appLabels = []string{"service", "app", "application"}
	}
	envLabels := m.EnvLabels
	if len(envLabels) == 0 {
		envLabels = []string{"env", "environment"}
	}
	return Mapped{
		Severity: sev, Priority: prio,
		ApplicationCode: firstLabel(a.Labels, appLabels),
		Environment:     strings.ToLower(firstLabel(a.Labels, envLabels)),
		Title:           IncidentTitle(a.Title),
		Description:     IncidentDescription(a),
	}
}

// IncidentTitle keeps the DB constraint (5..200 characters).
func IncidentTitle(t string) string {
	t = strings.TrimSpace(t)
	if len([]rune(t)) < 5 {
		t = "Alert: " + t
	}
	if r := []rune(t); len(r) > 200 {
		t = string(r[:197]) + "..."
	}
	return t
}

// IncidentDescription is the alert text plus a sorted label listing.
func IncidentDescription(a Alert) string {
	var b strings.Builder
	if a.Description != "" {
		b.WriteString(a.Description)
		b.WriteString("\n\n")
	}
	b.WriteString("Labels:\n")
	for _, k := range sortedKeys(a.Labels) {
		fmt.Fprintf(&b, "  %s=%s\n", k, a.Labels[k])
	}
	if a.SourceURL != "" {
		b.WriteString("\nSumber: " + a.SourceURL)
	}
	return strings.TrimSpace(b.String())
}

// LabelFingerprint is a stable hash of sorted labels (when the sender has
// no fingerprint).
func LabelFingerprint(labels map[string]string) string {
	h := sha256.New()
	for _, k := range sortedKeys(labels) {
		fmt.Fprintf(h, "%s=%s\n", k, labels[k])
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// PayloadHash identifies an identical retry of one alert.
func PayloadHash(a Alert) string {
	sum := sha256.Sum256(append([]byte(a.Status+"|"), a.Raw...))
	return hex.EncodeToString(sum[:])
}

// SeverityRank turns S1..S4 into 1..4 (unknown → 4).
func SeverityRank(s string) int {
	if len(s) == 2 && s[0] == 'S' && s[1] >= '1' && s[1] <= '4' {
		return int(s[1] - '0')
	}
	return 4
}

// ---------- helpers ----------

func severityLabel(m Mapping) string {
	if m.SeverityLabel != "" {
		return m.SeverityLabel
	}
	return "severity"
}

func normStatus(s string, resolvedValues []string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == StatusResolved || s == "ok" || s == "closed" || s == "normal" {
		return StatusResolved
	}
	for _, v := range resolvedValues {
		if strings.EqualFold(v, s) {
			return StatusResolved
		}
	}
	return StatusFiring
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func firstLabel(labels map[string]string, keys []string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(labels[k]); v != "" {
			return v
		}
	}
	return ""
}

func nonNilLabels(l map[string]string) map[string]string {
	if l == nil {
		return map[string]string{}
	}
	return l
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func lookup(v any, path string) (any, bool) {
	cur := v
	for _, part := range strings.Split(path, ".") {
		obj, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		if cur, ok = obj[part]; !ok {
			return nil, false
		}
	}
	return cur, true
}

func str(v any, path string) string {
	x, ok := lookup(v, path)
	if !ok || x == nil {
		return ""
	}
	if s, ok := x.(string); ok {
		return s
	}
	return fmt.Sprint(x)
}

func toSlice(v any) []any {
	if arr, ok := v.([]any); ok {
		return arr
	}
	return []any{v}
}
