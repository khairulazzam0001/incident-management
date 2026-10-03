package service

import (
	"context"
	"fmt"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/khairulazzam0001/incident-management/backend/internal/model"
	"github.com/khairulazzam0001/incident-management/backend/internal/repository"
	"github.com/khairulazzam0001/incident-management/backend/internal/sla"
)

// SLA & Escalation (PRD_SLA_Escalation.md §6–§12).

// slaBatchSize bounds instances handled per worker tick.
const slaBatchSize = 200

// slaLastTick holds the unix time of the last completed worker tick.
var slaLastTick atomic.Int64

// planSLA computes instances for the given metrics from start using the
// priority's current policy (snapshot). A missing policy yields no SLA.
func (s *IncidentService) planSLA(ctx context.Context, priority string, start time.Time, metrics ...string) ([]repository.SLAInstanceInput, error) {
	p, err := s.repo.GetSLAPolicy(ctx, priority)
	if err != nil {
		if isNotFound(err) {
			s.log.Warn("sla: policy tidak ada, incident tanpa SLA", "priority", priority)
			return nil, nil
		}
		return nil, fmt.Errorf("get sla policy: %w", err)
	}
	cal, err := s.repo.LoadCalendar(ctx, p.CalendarCode)
	if err != nil {
		return nil, err
	}
	rows := make([]repository.SLAInstanceInput, 0, len(metrics))
	for _, m := range metrics {
		minutes := p.ResolutionMinutes
		if m == sla.MetricResponse {
			minutes = p.ResponseMinutes
		}
		target, err := cal.AddWorkingMinutes(start, minutes)
		if err != nil {
			return nil, fmt.Errorf("sla target: %w", err)
		}
		warn, err := cal.AddWorkingMinutes(start, sla.WarnMinutes(minutes, p.WarnPercent))
		if err != nil {
			return nil, fmt.Errorf("sla warn: %w", err)
		}
		rows = append(rows, repository.SLAInstanceInput{
			Metric: m, PriorityCode: priority, TargetMinutes: minutes, CalendarCode: p.CalendarCode,
			WarnPercent: p.WarnPercent, ReminderMinutes: p.BreachReminderMinutes,
			StartedAt: start, WarnAt: warn, TargetAt: target,
		})
	}
	return rows, nil
}

// attachSLA fills Incident.SLA for list/detail responses in one query.
func (s *IncidentService) attachSLA(ctx context.Context, items ...*model.Incident) error {
	ids := make([]string, 0, len(items))
	for _, in := range items {
		ids = append(ids, in.ID)
	}
	sums, err := s.repo.SLASummaries(ctx, ids)
	if err != nil {
		return err
	}
	for _, in := range items {
		in.SLA = sums[in.ID]
	}
	return nil
}

// ProcessSLA runs one worker tick at now and notifies new escalations.
// Exported so tests can drive time deterministically.
func (s *IncidentService) ProcessSLA(ctx context.Context, now time.Time) (int, error) {
	events, err := s.repo.ProcessDueSLA(ctx, now.UTC(), slaBatchSize, sla.Decide)
	if err != nil {
		return 0, fmt.Errorf("process sla: %w", err)
	}
	for _, ev := range events {
		in, err := s.repo.GetIncident(ctx, ev.IncidentID)
		if err != nil {
			s.log.Warn("sla: incident tidak ditemukan untuk notifikasi", "incident_id", ev.IncidentID, "error", err)
			continue
		}
		s.slaFanout(ctx, in, ev)
	}
	slaLastTick.Store(now.Unix())
	return len(events), nil
}

// RunSLAWorker ticks until ctx is cancelled (§6.5). Stop events are
// synchronous elsewhere, so a slow tick only delays notifications.
func (s *IncidentService) RunSLAWorker(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if _, err := s.ProcessSLA(ctx, time.Now()); err != nil && ctx.Err() == nil {
			s.log.Error("sla worker", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// slaFanout notifies an escalation (§12). The actor is the system (nil).
func (s *IncidentService) slaFanout(ctx context.Context, in *model.Incident, ev model.SLAEvent) {
	var ids []string
	add := func(found []string, err error) {
		if err != nil {
			s.log.Warn("sla: gagal resolve penerima", "error", err)
			return
		}
		ids = append(ids, found...)
	}
	if in.PICID != nil {
		ids = append(ids, *in.PICID)
	}
	var ntype string
	switch ev.Level {
	case sla.LevelWarning:
		ntype = model.NotifSLAWarning
		if in.PICID == nil {
			add(s.repo.CoordinatorIDs(ctx))
		}
	case sla.LevelBreach:
		ntype = model.NotifSLABreached
		add(s.repo.ActiveUserIDsByRole(ctx, model.RoleManagerLead))
		if in.PICID == nil {
			add(s.repo.CoordinatorIDs(ctx))
		}
	case sla.LevelReminder:
		ntype = model.NotifSLAReminder
		ids = nil
		add(s.repo.ActiveUserIDsByRole(ctx, model.RoleManagerLead))
	default:
		return
	}
	label := map[string]string{sla.MetricResponse: "response", sla.MetricResolution: "resolution"}[ev.Metric]
	s.deliver(ctx, delivery{
		incidentID: &in.ID, ntype: ntype, recipients: ids,
		payload: fmt.Sprintf(`{"incident_no":%q,"title":%q,"metric":%q}`, in.IncidentNo, in.Title, ev.Metric),
		subject: fmt.Sprintf("[SLA %s] %s: %s", ev.Level, in.IncidentNo, in.Title),
		body: fmt.Sprintf("Incident %s\n%s\nSLA %s: %s\nPriority: %s, Status: %s\nWaktu: %s",
			in.IncidentNo, in.Title, label, ev.Level, in.Priority, in.Status, time.Now().Format("2006-01-02 15:04:05")),
	})
}

// IncidentSLA returns all SLA instances of an incident (read scope applies).
func (s *IncidentService) IncidentSLA(ctx context.Context, actor *model.AuthUser, incidentID string) ([]*model.IncidentSLA, error) {
	if _, err := s.Get(ctx, actor, incidentID); err != nil {
		return nil, err
	}
	items, err := s.repo.ListIncidentSLA(ctx, incidentID)
	if err != nil {
		return nil, fmt.Errorf("list incident sla: %w", err)
	}
	return items, nil
}

func requireInternal(actor *model.AuthUser) *Error {
	if actor.Role == model.RoleUser {
		return Forbidden("FORBIDDEN_READ", "Anda tidak berhak mengakses data ini.")
	}
	return nil
}

func requireManager(actor *model.AuthUser) *Error {
	if actor.Role != model.RoleManagerLead {
		return Forbidden("FORBIDDEN_ACTION", "Hanya Manager/Lead yang boleh mengubah pengaturan SLA.")
	}
	return nil
}

// SLASettings bundles policies, calendars and holidays for Master Data.
type SLASettings struct {
	Policies  []*model.SLAPolicy        `json:"policies"`
	Calendars []*model.BusinessCalendar `json:"calendars"`
}

// SLASettings returns policies + calendars (internal roles).
func (s *IncidentService) SLASettings(ctx context.Context, actor *model.AuthUser) (*SLASettings, error) {
	if err := requireInternal(actor); err != nil {
		return nil, err
	}
	policies, err := s.repo.ListSLAPolicies(ctx)
	if err != nil {
		return nil, err
	}
	cals, err := s.repo.ListCalendars(ctx)
	if err != nil {
		return nil, err
	}
	return &SLASettings{Policies: policies, Calendars: cals}, nil
}

// SLAPolicyInput is the payload for PUT /api/master/sla-policies/:priority.
type SLAPolicyInput struct {
	ResponseMinutes       int    `json:"response_minutes"`
	ResolutionMinutes     int    `json:"resolution_minutes"`
	CalendarCode          string `json:"calendar_code"`
	WarnPercent           int    `json:"warn_percent"`
	BreachReminderMinutes *int   `json:"breach_reminder_minutes"`
}

// maxSLAMinutes caps targets at 90 calendar days to catch typos.
const maxSLAMinutes = 90 * 24 * 60

// UpdateSLAPolicy changes a policy. Running instances keep their snapshot
// (Q5): only incidents created afterwards use the new target.
func (s *IncidentService) UpdateSLAPolicy(ctx context.Context, actor *model.AuthUser, priority string, in SLAPolicyInput) (*model.SLAPolicy, error) {
	if err := requireManager(actor); err != nil {
		return nil, err
	}
	bad := func(field, msg string) (*model.SLAPolicy, error) {
		return nil, BadRequest("VALIDATION_ERROR", msg, map[string]string{"field": field})
	}
	if in.ResponseMinutes <= 0 || in.ResponseMinutes > maxSLAMinutes {
		return bad("response_minutes", "Target response harus 1 menit sampai 90 hari.")
	}
	if in.ResolutionMinutes <= 0 || in.ResolutionMinutes > maxSLAMinutes {
		return bad("resolution_minutes", "Target resolution harus 1 menit sampai 90 hari.")
	}
	if in.ResolutionMinutes < in.ResponseMinutes {
		return bad("resolution_minutes", "Target resolution tidak boleh lebih kecil dari response.")
	}
	if in.WarnPercent < 1 || in.WarnPercent > 99 {
		return bad("warn_percent", "Ambang peringatan harus 1–99%.")
	}
	if in.BreachReminderMinutes != nil && (*in.BreachReminderMinutes <= 0 || *in.BreachReminderMinutes > maxSLAMinutes) {
		return bad("breach_reminder_minutes", "Pengingat breach harus kosong atau 1 menit sampai 90 hari.")
	}
	exists, _, err := s.repo.CalendarExists(ctx, in.CalendarCode)
	if err != nil {
		return nil, fmt.Errorf("check calendar: %w", err)
	}
	if !exists {
		return bad("calendar_code", "Kalender tidak dikenal.")
	}
	p, err := s.repo.UpdateSLAPolicy(ctx, model.SLAPolicy{
		PriorityCode: priority, ResponseMinutes: in.ResponseMinutes, ResolutionMinutes: in.ResolutionMinutes,
		CalendarCode: in.CalendarCode, WarnPercent: in.WarnPercent, BreachReminderMinutes: in.BreachReminderMinutes,
	}, actor.ID)
	if err != nil {
		if isNotFound(err) {
			return nil, NotFound("SLA policy")
		}
		return nil, err
	}
	return p, nil
}

// UpdateBusinessHours replaces a non-24x7 calendar's working windows.
func (s *IncidentService) UpdateBusinessHours(ctx context.Context, actor *model.AuthUser, code string, hours []model.BusinessHours) error {
	if err := requireManager(actor); err != nil {
		return err
	}
	exists, is24x7, err := s.repo.CalendarExists(ctx, code)
	if err != nil {
		return fmt.Errorf("check calendar: %w", err)
	}
	if !exists {
		return NotFound("Kalender")
	}
	if is24x7 {
		return BadRequest("VALIDATION_ERROR", "Kalender 24x7 tidak punya jam kerja.", nil)
	}
	if len(hours) == 0 {
		return BadRequest("VALIDATION_ERROR", "Minimal satu hari kerja.", map[string]string{"field": "hours"})
	}
	seen := map[int]bool{}
	for _, h := range hours {
		if h.Weekday < 0 || h.Weekday > 6 || seen[h.Weekday] {
			return BadRequest("VALIDATION_ERROR", "Hari tidak valid atau ganda.", map[string]string{"field": "weekday"})
		}
		if h.StartMinute < 0 || h.EndMinute > 1440 || h.EndMinute <= h.StartMinute {
			return BadRequest("VALIDATION_ERROR", "Jam selesai harus setelah jam mulai.", map[string]string{"field": "hours"})
		}
		seen[h.Weekday] = true
	}
	return s.repo.ReplaceBusinessHours(ctx, code, hours)
}

// Holidays lists holidays of a year (0 = all), internal roles.
func (s *IncidentService) Holidays(ctx context.Context, actor *model.AuthUser, year int) ([]*model.Holiday, error) {
	if err := requireInternal(actor); err != nil {
		return nil, err
	}
	return s.repo.ListHolidays(ctx, year)
}

// CreateHoliday adds a holiday (Manager). Duplicate date → 409.
func (s *IncidentService) CreateHoliday(ctx context.Context, actor *model.AuthUser, h model.Holiday) (*model.Holiday, error) {
	if err := requireManager(actor); err != nil {
		return nil, err
	}
	if h.CalendarCode == "" {
		h.CalendarCode = "BUSINESS_HOURS"
	}
	if _, err := time.Parse("2006-01-02", h.Date); err != nil {
		return nil, BadRequest("VALIDATION_ERROR", "Format tanggal YYYY-MM-DD.", map[string]string{"field": "date"})
	}
	if len(h.Name) < 3 {
		return nil, BadRequest("VALIDATION_ERROR", "Nama hari libur minimal 3 karakter.", map[string]string{"field": "name"})
	}
	exists, _, err := s.repo.CalendarExists(ctx, h.CalendarCode)
	if err != nil {
		return nil, fmt.Errorf("check calendar: %w", err)
	}
	if !exists {
		return nil, BadRequest("VALIDATION_ERROR", "Kalender tidak dikenal.", map[string]string{"field": "calendar_code"})
	}
	out, err := s.repo.CreateHoliday(ctx, h, actor.ID)
	if err != nil {
		if repository.IsUniqueViolation(err) {
			return nil, Conflict("DUPLICATE_HOLIDAY", "Tanggal tersebut sudah terdaftar sebagai hari libur.", nil)
		}
		return nil, err
	}
	return out, nil
}

// DeleteHoliday removes a holiday (Manager).
func (s *IncidentService) DeleteHoliday(ctx context.Context, actor *model.AuthUser, id string) error {
	if err := requireManager(actor); err != nil {
		return err
	}
	if !validUUID(id) {
		return BadRequest("INVALID_ID", "ID tidak valid.", nil)
	}
	ok, err := s.repo.DeleteHoliday(ctx, id)
	if err != nil {
		return err
	}
	if !ok {
		return NotFound("Hari libur")
	}
	return nil
}

// SLADashboard returns compliance for [from, to] (default last 30 days).
func (s *IncidentService) SLADashboard(ctx context.Context, actor *model.AuthUser, from, to string) (*repository.SLADashboard, error) {
	if err := requireInternal(actor); err != nil {
		return nil, err
	}
	today := time.Now().UTC()
	if to == "" {
		to = today.Format("2006-01-02")
	}
	if from == "" {
		from = today.AddDate(0, 0, -30).Format("2006-01-02")
	}
	for field, d := range map[string]string{"from": from, "to": to} {
		if _, err := time.Parse("2006-01-02", d); err != nil {
			return nil, BadRequest("VALIDATION_ERROR", "Format tanggal YYYY-MM-DD.", map[string]string{"field": field})
		}
	}
	d, err := s.repo.GetSLADashboard(ctx, from, to)
	if err != nil {
		return nil, err
	}
	if t := slaLastTick.Load(); t > 0 {
		at := time.Unix(t, 0).UTC()
		d.WorkerLastTickAt = &at
	}
	return d, nil
}

// ParseYear parses an optional ?year= value (0 when empty).
func ParseYear(v string) (int, error) {
	if v == "" {
		return 0, nil
	}
	y, err := strconv.Atoi(v)
	if err != nil || y < 2000 || y > 2100 {
		return 0, BadRequest("VALIDATION_ERROR", "Tahun tidak valid.", map[string]string{"field": "year"})
	}
	return y, nil
}
