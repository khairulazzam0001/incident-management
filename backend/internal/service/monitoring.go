package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/khairulazzam0001/incident-management/backend/internal/integration"
	"github.com/khairulazzam0001/incident-management/backend/internal/model"
	"github.com/khairulazzam0001/incident-management/backend/internal/repository"
	"github.com/khairulazzam0001/incident-management/backend/internal/sla"
)

// Monitoring Integration (PRD_Monitoring_Integration.md §6–§12).

const (
	duplicateWindow    = 60 * time.Second // retry identik diabaikan (§6.5)
	refireActivityGap  = 15 * time.Minute // activity alert_refired maks 1 per 15 mnt (§6.4)
	reoccurWindow      = 24 * time.Hour   // Q4
	stormWindow        = 5 * time.Minute  // §6.4
	stormThreshold     = 10               // incident baru per integrasi per 5 mnt
	alertEventMaxBytes = 64 << 10         // payload event dipangkas (§9)
	alertRetention     = 90 * 24 * time.Hour
)

// alertLocks serializes alert processing per integration (single replica,
// Q5) so concurrent webhooks cannot create duplicate incidents.
var alertLocks sync.Map

// rateWindow is a fixed one-minute window per integration (in-process, Q5).
type rateWindow struct {
	mu    sync.Mutex
	start time.Time
	count int
}

var rateWindows sync.Map

func allowRate(integrationID string, limit int, now time.Time) bool {
	v, _ := rateWindows.LoadOrStore(integrationID, &rateWindow{})
	w := v.(*rateWindow)
	w.mu.Lock()
	defer w.mu.Unlock()
	if now.Sub(w.start) >= time.Minute {
		w.start, w.count = now, 0
	}
	if w.count >= limit {
		return false
	}
	w.count++
	return true
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func newIntegrationToken() (token, hash, last4 string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", "", fmt.Errorf("generate token: %w", err)
	}
	token = "imk_" + base64.RawURLEncoding.EncodeToString(b)
	return token, hashToken(token), token[len(token)-4:], nil
}

func requireIntegrationManager(actor *model.AuthUser) *Error {
	if !model.CanManageIntegrations(actor.Role) {
		return Forbidden("FORBIDDEN_ACTION", "Hanya Manager/Lead atau DevOps yang boleh mengelola integrasi.")
	}
	return nil
}

// validateIntegration normalizes input into a storable row.
func (s *IncidentService) validateIntegration(ctx context.Context, in model.IntegrationInput, kind string) (repository.IntegrationRow, error) {
	row := repository.IntegrationRow{Type: kind, Name: strings.TrimSpace(in.Name), IsActive: true, RateLimitPerMin: in.RateLimitPerMin}
	if in.IsActive != nil {
		row.IsActive = *in.IsActive
	}
	bad := func(field, msg string) (repository.IntegrationRow, error) {
		return row, BadRequest("VALIDATION_ERROR", msg, map[string]string{"field": field})
	}
	if len(row.Name) < 3 {
		return bad("name", "Nama integrasi minimal 3 karakter.")
	}
	if row.RateLimitPerMin == 0 {
		row.RateLimitPerMin = 60
	}
	if row.RateLimitPerMin < 1 || row.RateLimitPerMin > 6000 {
		return bad("rate_limit_per_min", "Rate limit harus 1–6000 per menit.")
	}
	meta, err := s.repo.ListMeta(ctx)
	if err != nil {
		return row, fmt.Errorf("load masters: %w", err)
	}
	row.MinSeverity = strings.TrimSpace(in.MinSeverity)
	if row.MinSeverity == "" {
		row.MinSeverity = "S3"
	}
	if !codesIn(row.MinSeverity, meta.Severities) {
		return bad("min_severity", "Severity minimum tidak dikenal.")
	}
	if app := blankToNil(in.DefaultApplicationID); app != nil {
		found := false
		for _, a := range meta.Applications {
			found = found || a.ID == *app
		}
		if !found {
			return bad("default_application_id", "Application default tidak dikenal.")
		}
		row.DefaultApplicationID = app
	}
	if env := blankToNil(in.DefaultEnvironment); env != nil {
		if !codesIn(*env, meta.Environments) {
			return bad("default_environment", "Environment default tidak dikenal.")
		}
		row.DefaultEnvironment = env
	}
	var mapping integration.Mapping
	if len(in.Mapping) > 0 && string(in.Mapping) != "null" {
		dec := json.NewDecoder(strings.NewReader(string(in.Mapping)))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&mapping); err != nil {
			return bad("mapping", "Mapping tidak valid: "+err.Error())
		}
	}
	for k, v := range mapping.SeverityMap {
		if integration.SeverityRank(v) == 4 && v != "S4" {
			return bad("mapping", fmt.Sprintf("severity_map[%s] harus S1–S4.", k))
		}
	}
	row.Mapping, _ = json.Marshal(mapping)
	row.IPAllowlist = []string{}
	for _, raw := range in.IPAllowlist {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if !strings.Contains(raw, "/") {
			addr, err := netip.ParseAddr(raw)
			if err != nil {
				return bad("ip_allowlist", "IP/CIDR tidak valid: "+raw)
			}
			raw = netip.PrefixFrom(addr, addr.BitLen()).String()
		}
		p, err := netip.ParsePrefix(raw)
		if err != nil {
			return bad("ip_allowlist", "IP/CIDR tidak valid: "+raw)
		}
		row.IPAllowlist = append(row.IPAllowlist, p.Masked().String())
	}
	return row, nil
}

// IntegrationCreated is returned once: the token is never retrievable again.
type IntegrationCreated struct {
	Integration *model.Integration `json:"integration"`
	WebhookPath string             `json:"webhook_path"`
	Token       string             `json:"token"`
}

func webhookPath(id string) string { return "/api/integrations/alerts/" + id }

// CreateIntegration stores a new integration and returns its token once.
func (s *IncidentService) CreateIntegration(ctx context.Context, actor *model.AuthUser, in model.IntegrationInput) (*IntegrationCreated, error) {
	if err := requireIntegrationManager(actor); err != nil {
		return nil, err
	}
	switch in.Type {
	case integration.TypeAlertmanager, integration.TypeGrafana, integration.TypeGenericJSON:
	default:
		return nil, BadRequest("VALIDATION_ERROR", "Tipe harus ALERTMANAGER, GRAFANA, atau GENERIC_JSON.", map[string]string{"field": "type"})
	}
	row, err := s.validateIntegration(ctx, in, in.Type)
	if err != nil {
		return nil, err
	}
	token, hash, last4, err := newIntegrationToken()
	if err != nil {
		return nil, err
	}
	it, err := s.repo.CreateIntegration(ctx, row, hash, last4, actor.ID)
	if err != nil {
		return nil, fmt.Errorf("create integration: %w", err)
	}
	return &IntegrationCreated{Integration: it, WebhookPath: webhookPath(it.ID), Token: token}, nil
}

// UpdateIntegration edits an integration (type is immutable).
func (s *IncidentService) UpdateIntegration(ctx context.Context, actor *model.AuthUser, id string, in model.IntegrationInput) (*model.Integration, error) {
	if err := requireIntegrationManager(actor); err != nil {
		return nil, err
	}
	cur, err := s.loadIntegration(ctx, id)
	if err != nil {
		return nil, err
	}
	row, err := s.validateIntegration(ctx, in, cur.Type)
	if err != nil {
		return nil, err
	}
	it, err := s.repo.UpdateIntegration(ctx, id, row)
	if err != nil {
		return nil, fmt.Errorf("update integration: %w", err)
	}
	return it, nil
}

// RotateIntegrationToken issues a new token; the old one stops working at once.
func (s *IncidentService) RotateIntegrationToken(ctx context.Context, actor *model.AuthUser, id string) (string, error) {
	if err := requireIntegrationManager(actor); err != nil {
		return "", err
	}
	if _, err := s.loadIntegration(ctx, id); err != nil {
		return "", err
	}
	token, hash, last4, err := newIntegrationToken()
	if err != nil {
		return "", err
	}
	if err := s.repo.RotateIntegrationToken(ctx, id, hash, last4); err != nil {
		return "", fmt.Errorf("rotate token: %w", err)
	}
	return token, nil
}

// Integrations lists integrations (Manager/DevOps).
func (s *IncidentService) Integrations(ctx context.Context, actor *model.AuthUser) ([]*model.Integration, error) {
	if err := requireIntegrationManager(actor); err != nil {
		return nil, err
	}
	return s.repo.ListIntegrations(ctx)
}

func (s *IncidentService) loadIntegration(ctx context.Context, id string) (*model.Integration, error) {
	if !validUUID(id) {
		return nil, BadRequest("INVALID_ID", "ID integrasi tidak valid.", nil)
	}
	it, _, err := s.repo.GetIntegration(ctx, id)
	if err != nil {
		if isNotFound(err) {
			return nil, NotFound("Integrasi")
		}
		return nil, fmt.Errorf("get integration: %w", err)
	}
	return it, nil
}

// AlertEvents returns a page of an integration's event log.
func (s *IncidentService) AlertEvents(ctx context.Context, actor *model.AuthUser, id, result string, page, limit int) ([]*model.AlertEvent, int, error) {
	if err := requireIntegrationManager(actor); err != nil {
		return nil, 0, err
	}
	if _, err := s.loadIntegration(ctx, id); err != nil {
		return nil, 0, err
	}
	if page <= 0 {
		page = 1
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return s.repo.ListAlertEvents(ctx, id, result, page, limit)
}

// IncidentAlerts returns alerts linked to an incident (read scope).
func (s *IncidentService) IncidentAlerts(ctx context.Context, actor *model.AuthUser, incidentID string) ([]*model.IncidentAlert, error) {
	if _, err := s.Get(ctx, actor, incidentID); err != nil {
		return nil, err
	}
	return s.repo.ListIncidentAlerts(ctx, incidentID)
}

// ---------- webhook ----------

// AlertResult summarizes one webhook request (202 body).
type AlertResult struct {
	Received  int `json:"received"`
	Created   int `json:"created"`
	Updated   int `json:"updated"`
	Grouped   int `json:"grouped"`
	Ignored   int `json:"ignored"`
	Duplicate int `json:"duplicate"`
	Errors    int `json:"errors"`
}

// mapContext caches master data for one request.
type mapContext struct {
	apps map[string]string // code (lower) → id
	envs map[string]bool
}

func (s *IncidentService) loadMapContext(ctx context.Context) (*mapContext, error) {
	meta, err := s.repo.ListMeta(ctx)
	if err != nil {
		return nil, fmt.Errorf("load masters: %w", err)
	}
	mc := &mapContext{apps: map[string]string{}, envs: map[string]bool{}}
	for _, a := range meta.Applications {
		mc.apps[strings.ToLower(a.Code)] = a.ID
	}
	for _, e := range meta.Environments {
		mc.envs[e.Code] = true
	}
	return mc, nil
}

// resolveTarget turns mapped codes into ids, falling back to the integration
// defaults; warning explains any fallback (MON-06).
func (mc *mapContext) resolveTarget(it *model.Integration, m integration.Mapped) (appID, env *string, warning string) {
	var warns []string
	if id, ok := mc.apps[strings.ToLower(m.ApplicationCode)]; ok && m.ApplicationCode != "" {
		appID = &id
	} else {
		appID = it.DefaultApplicationID
		if m.ApplicationCode != "" {
			warns = append(warns, fmt.Sprintf("application %q tidak dikenal", m.ApplicationCode))
		}
		if appID == nil {
			warns = append(warns, "tanpa application")
		}
	}
	if m.Environment != "" && mc.envs[m.Environment] {
		e := m.Environment
		env = &e
	} else {
		env = it.DefaultEnvironmentCode
		if m.Environment != "" {
			warns = append(warns, fmt.Sprintf("environment %q tidak dikenal", m.Environment))
		}
	}
	if len(warns) > 0 {
		warning = "mapping_warning: " + strings.Join(warns, "; ")
	}
	return appID, env, warning
}

func clientIPAllowed(allowlist []string, remoteIP string) bool {
	if len(allowlist) == 0 {
		return true
	}
	addr, err := netip.ParseAddr(remoteIP)
	if err != nil {
		return false
	}
	addr = addr.Unmap()
	for _, raw := range allowlist {
		if p, err := netip.ParsePrefix(raw); err == nil && p.Contains(addr) {
			return true
		}
	}
	return false
}

// ReceiveAlerts authenticates and processes one webhook request (§6, MON-02).
// Unknown integration and wrong token both answer 401 (no enumeration).
func (s *IncidentService) ReceiveAlerts(ctx context.Context, integrationID, token, remoteIP string, body []byte) (*AlertResult, error) {
	unauthorized := Unauthorized("INVALID_TOKEN", "Token integrasi tidak valid.")
	if !validUUID(integrationID) || token == "" {
		return nil, unauthorized
	}
	it, hash, err := s.repo.GetIntegration(ctx, integrationID)
	if err != nil {
		if isNotFound(err) {
			return nil, unauthorized
		}
		return nil, fmt.Errorf("get integration: %w", err)
	}
	if subtle.ConstantTimeCompare([]byte(hashToken(token)), []byte(hash)) != 1 {
		return nil, unauthorized
	}
	if !it.IsActive {
		return nil, Forbidden("INTEGRATION_DISABLED", "Integrasi sedang nonaktif.")
	}
	if !clientIPAllowed(it.IPAllowlist, remoteIP) {
		return nil, Forbidden("IP_NOT_ALLOWED", "Alamat IP pengirim tidak diizinkan.")
	}
	if !allowRate(it.ID, it.RateLimitPerMin, time.Now()) {
		return nil, &Error{Status: 429, Code: "RATE_LIMITED", Message: "Terlalu banyak request; coba lagi sebentar."}
	}
	var mapping integration.Mapping
	_ = json.Unmarshal(it.Mapping, &mapping)
	alerts, err := integration.Parse(it.Type, body, mapping)
	if err != nil {
		raw := body
		if !json.Valid(raw) {
			raw, _ = json.Marshal(map[string]string{"raw": truncate(string(body), 2048)})
		}
		_ = s.repo.InsertAlertEvent(ctx, repository.AlertEventRow{
			IntegrationID: it.ID, PayloadHash: "invalid", Payload: trimPayload(raw), Status: "unknown",
			Result: "error", Error: err.Error(),
		})
		return nil, BadRequest("INVALID_PAYLOAD", "Payload tidak sesuai format "+it.Type+".", nil)
	}

	lock, _ := alertLocks.LoadOrStore(it.ID, &sync.Mutex{})
	lock.(*sync.Mutex).Lock()
	defer lock.(*sync.Mutex).Unlock()

	mc, err := s.loadMapContext(ctx)
	if err != nil {
		return nil, err
	}
	res := &AlertResult{Received: len(alerts)}
	for _, a := range alerts {
		ev := s.processAlert(ctx, it, mapping, mc, a)
		switch ev.Result {
		case "created":
			res.Created++
		case "updated":
			res.Updated++
		case "grouped":
			res.Grouped++
		case "ignored":
			res.Ignored++
		case "duplicate":
			res.Duplicate++
		default:
			res.Errors++
		}
		if err := s.repo.InsertAlertEvent(ctx, ev); err != nil {
			s.log.Error("simpan alert event", "integration_id", it.ID, "error", err)
		}
	}
	if err := s.repo.TouchIntegration(ctx, it.ID); err != nil {
		s.log.Warn("touch integration", "error", err)
	}
	return res, nil
}

// processAlert applies dedup → refire/resolve → threshold → storm → create.
// Errors become an "error" event; they never abort the whole request.
func (s *IncidentService) processAlert(ctx context.Context, it *model.Integration, mapping integration.Mapping,
	mc *mapContext, a integration.Alert) repository.AlertEventRow {
	ev := repository.AlertEventRow{
		IntegrationID: it.ID, PayloadHash: integration.PayloadHash(a), Payload: trimPayload(a.Raw),
		Fingerprint: a.Fingerprint, Status: a.Status,
	}
	fail := func(err error) repository.AlertEventRow {
		s.log.Error("proses alert", "integration_id", it.ID, "fingerprint", a.Fingerprint, "error", err)
		ev.Result, ev.Error = "error", err.Error()
		return ev
	}
	if dup, err := s.repo.IsRecentDuplicate(ctx, it.ID, ev.PayloadHash, duplicateWindow); err != nil {
		return fail(err)
	} else if dup {
		ev.Result = "duplicate"
		return ev
	}

	link, err := s.repo.LatestAlertLink(ctx, it.ID, a.Fingerprint)
	if err != nil && !isNotFound(err) {
		return fail(err)
	}
	if err != nil {
		link = nil
	}
	now := time.Now()

	if a.Status == integration.StatusResolved {
		if link == nil {
			ev.Result = "ignored"
			return ev
		}
		comment := fmt.Sprintf("Alert monitoring %q resolved pada %s.", a.Title, now.UTC().Format("2006-01-02 15:04 UTC"))
		if err := s.repo.TouchAlertLink(ctx, link.LinkID, link.IncidentID, integration.StatusResolved,
			model.ActivityAlertResolved, comment); err != nil {
			return fail(err)
		}
		ev.Result, ev.IncidentID = "updated", &link.IncidentID
		return ev
	}

	if link != nil {
		open := link.IncidentStatus != model.StatusResolved && link.IncidentStatus != model.StatusClosed
		if open {
			activity := ""
			if now.Sub(link.LastActivityAt) >= refireActivityGap {
				activity = model.ActivityAlertRefired
			}
			if err := s.repo.TouchAlertLink(ctx, link.LinkID, link.IncidentID, integration.StatusFiring, activity, ""); err != nil {
				return fail(err)
			}
			ev.Result, ev.IncidentID = "updated", &link.IncidentID
			return ev
		}
		ended := link.ClosedAt
		if ended == nil {
			ended = link.ResolvedAt
		}
		if ended != nil && now.Sub(*ended) < reoccurWindow {
			comment := fmt.Sprintf("Alert monitoring %q muncul lagi pada %s setelah incident %s.",
				a.Title, now.UTC().Format("2006-01-02 15:04 UTC"), strings.ToLower(link.IncidentStatus))
			if err := s.repo.TouchAlertLink(ctx, link.LinkID, link.IncidentID, integration.StatusFiring,
				model.ActivityAlertReoccurred, comment); err != nil {
				return fail(err)
			}
			if in, err := s.repo.GetIncident(ctx, link.IncidentID); err == nil {
				recipients := []string{}
				if in.PICID != nil {
					recipients = append(recipients, *in.PICID)
				} else if ids, err := s.repo.CoordinatorIDs(ctx); err == nil {
					recipients = ids
				}
				s.systemNotify(ctx, in, model.NotifAlertReoccurred, recipients)
			}
			ev.Result, ev.IncidentID = "updated", &link.IncidentID
			return ev
		}
		// > 24 jam setelah selesai: perlakukan sebagai alert baru.
	}

	mapped := integration.Map(a, mapping)
	if integration.SeverityRank(mapped.Severity) > integration.SeverityRank(it.MinSeverity) {
		ev.Result, ev.Warning = "ignored", fmt.Sprintf("severity %s di bawah ambang %s", mapped.Severity, it.MinSeverity)
		return ev
	}
	appID, env, warning := mc.resolveTarget(it, mapped)
	ev.Warning = warning
	linkIn := repository.AlertLinkInput{
		IntegrationID: it.ID, IntegrationName: it.Name, Fingerprint: a.Fingerprint,
		Title: mapped.Title, Labels: a.Labels, SourceURL: a.SourceURL,
	}

	recent, err := s.repo.CountRecentAlertIncidents(ctx, it.ID, now.Add(-stormWindow))
	if err != nil {
		return fail(err)
	}
	if recent >= stormThreshold {
		parentID, err := s.stormParent(ctx, it, mapped, appID, env, now)
		if err != nil {
			return fail(err)
		}
		if err := s.repo.GroupAlert(ctx, parentID, linkIn); err != nil {
			return fail(err)
		}
		ev.Result, ev.IncidentID = "grouped", &parentID
		return ev
	}

	created, err := s.createSystemIncident(ctx, model.CreateIncidentInput{
		Title: mapped.Title, Description: mapped.Description, Source: "monitoring",
		Severity: mapped.Severity, Priority: mapped.Priority, ApplicationID: appID, Environment: env,
	}, repository.AlertLinkStep(linkIn))
	if err != nil {
		return fail(err)
	}
	ev.Result, ev.IncidentID = "created", &created.ID
	return ev
}

// stormParent returns (or creates once) the incident that groups alerts of a
// storm per application, notifying ManagerLead when created (§6.4).
func (s *IncidentService) stormParent(ctx context.Context, it *model.Integration, m integration.Mapped,
	appID, env *string, now time.Time) (string, error) {
	key := "none"
	if appID != nil {
		key = *appID
	}
	id, err := s.repo.FindStormParent(ctx, it.ID, key, now.Add(-stormWindow))
	if err == nil {
		return id, nil
	}
	if !isNotFound(err) {
		return "", err
	}
	parent, err := s.createSystemIncident(ctx, model.CreateIncidentInput{
		Title:       integration.IncidentTitle(fmt.Sprintf("Alert storm dari %s", it.Name)),
		Description: fmt.Sprintf("Lebih dari %d alert baru dalam %s dari integrasi %s. Alert berikutnya dikelompokkan ke incident ini.", stormThreshold, stormWindow, it.Name),
		Source:      "monitoring", Severity: m.Severity, Priority: m.Priority, ApplicationID: appID, Environment: env,
	}, repository.AlertLinkStep(repository.AlertLinkInput{
		IntegrationID: it.ID, IntegrationName: it.Name, Fingerprint: "storm:" + key, Title: "Alert storm",
		Labels: map[string]string{"storm": "true"},
	}))
	if err != nil {
		return "", err
	}
	if ids, err := s.repo.ActiveUserIDsByRole(ctx, model.RoleManagerLead); err == nil {
		s.systemNotify(ctx, parent, model.NotifAlertStorm, ids)
	}
	return parent.ID, nil
}

// createSystemIncident creates an incident without a human reporter through
// the same path as Create (number, SLA, activity, notification).
func (s *IncidentService) createSystemIncident(ctx context.Context, in model.CreateIncidentInput, steps ...repository.IncidentStep) (*model.Incident, error) {
	no, err := s.repo.NextIncidentNo(ctx)
	if err != nil {
		return nil, fmt.Errorf("generate incident_no: %w", err)
	}
	slaRows, err := s.planSLA(ctx, in.Priority, time.Now().UTC(), sla.MetricResponse, sla.MetricResolution)
	if err != nil {
		return nil, err
	}
	created, err := s.repo.CreateIncident(ctx, in, no, nil, slaRows, steps...)
	if err != nil {
		return nil, fmt.Errorf("create incident: %w", err)
	}
	if ids, err := s.repo.CoordinatorIDs(ctx); err == nil {
		s.systemNotify(ctx, created, model.NotifCreated, ids)
	}
	return created, nil
}

// systemNotify sends an incident notification whose actor is the system.
func (s *IncidentService) systemNotify(ctx context.Context, in *model.Incident, ntype string, recipients []string) {
	subject, body := mailContent(ntype, in)
	s.deliver(ctx, delivery{
		incidentID: &in.ID, ntype: ntype, recipients: recipients,
		payload: fmt.Sprintf(`{"incident_no":%q,"title":%q,"status":%q}`, in.IncidentNo, in.Title, in.Status),
		subject: subject, body: body,
	})
}

// DryRunAlert is one alert as it would be processed (no writes, MON-FR-10).
type DryRunAlert struct {
	Alert       integration.Alert  `json:"alert"`
	Mapped      integration.Mapped `json:"mapped"`
	Application *string            `json:"application_id"`
	Environment *string            `json:"environment"`
	Action      string             `json:"action"` // create | ignore | update_resolved
	Warning     string             `json:"warning"`
}

// TestIntegration parses and maps a sample payload without side effects.
func (s *IncidentService) TestIntegration(ctx context.Context, actor *model.AuthUser, id string, body []byte) ([]DryRunAlert, error) {
	if err := requireIntegrationManager(actor); err != nil {
		return nil, err
	}
	it, err := s.loadIntegration(ctx, id)
	if err != nil {
		return nil, err
	}
	var mapping integration.Mapping
	_ = json.Unmarshal(it.Mapping, &mapping)
	alerts, err := integration.Parse(it.Type, body, mapping)
	if err != nil {
		if errors.Is(err, integration.ErrInvalidPayload) {
			return nil, BadRequest("INVALID_PAYLOAD", "Payload tidak sesuai format "+it.Type+".", nil)
		}
		return nil, err
	}
	mc, err := s.loadMapContext(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]DryRunAlert, 0, len(alerts))
	for _, a := range alerts {
		m := integration.Map(a, mapping)
		app, env, warn := mc.resolveTarget(it, m)
		action := "create"
		switch {
		case a.Status == integration.StatusResolved:
			action = "update_resolved"
		case integration.SeverityRank(m.Severity) > integration.SeverityRank(it.MinSeverity):
			action = "ignore"
		}
		out = append(out, DryRunAlert{Alert: a, Mapped: m, Application: app, Environment: env, Action: action, Warning: warn})
	}
	return out, nil
}

// RunMaintenance purges raw alert events past retention once a day (Q6).
func (s *IncidentService) RunMaintenance(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		if n, err := s.repo.PurgeAlertEvents(ctx, time.Now().Add(-alertRetention)); err != nil && ctx.Err() == nil {
			s.log.Error("purge alert events", "error", err)
		} else if n > 0 {
			s.log.Info("alert events dihapus (retensi)", "count", n)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func trimPayload(raw []byte) []byte {
	if len(raw) <= alertEventMaxBytes {
		return raw
	}
	b, _ := json.Marshal(map[string]any{"truncated": true, "size": len(raw), "head": truncate(string(raw), 4096)})
	return b
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
