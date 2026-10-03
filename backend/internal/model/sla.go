package model

import "time"

// SLA & Escalation (PRD_SLA_Escalation.md). Status/metric/level constants
// live in package sla next to the rules that use them.

// SLAPolicy is a master_sla_policy row (one per priority).
type SLAPolicy struct {
	PriorityCode          string    `json:"priority"`
	ResponseMinutes       int       `json:"response_minutes"`
	ResolutionMinutes     int       `json:"resolution_minutes"`
	CalendarCode          string    `json:"calendar_code"`
	WarnPercent           int       `json:"warn_percent"`
	BreachReminderMinutes *int      `json:"breach_reminder_minutes"`
	UpdatedBy             *string   `json:"updated_by"`
	UpdatedAt             time.Time `json:"updated_at"`
}

// BusinessHours is one working window of a calendar (weekday 0 = Sunday,
// minutes since midnight in the calendar time zone).
type BusinessHours struct {
	Weekday     int `json:"weekday"`
	StartMinute int `json:"start_minute"`
	EndMinute   int `json:"end_minute"`
}

// BusinessCalendar is a master_business_calendar row with its hours.
type BusinessCalendar struct {
	Code     string          `json:"code"`
	Name     string          `json:"name"`
	Timezone string          `json:"timezone"`
	Is24x7   bool            `json:"is_24x7"`
	Hours    []BusinessHours `json:"hours"`
}

// Holiday is a master_holiday row.
type Holiday struct {
	ID           string `json:"id"`
	CalendarCode string `json:"calendar_code"`
	Date         string `json:"date"` // YYYY-MM-DD
	Name         string `json:"name"`
}

// IncidentSLA is a trans_incident_sla row.
type IncidentSLA struct {
	ID              string     `json:"id"`
	IncidentID      string     `json:"incident_id"`
	Metric          string     `json:"metric"`
	Cycle           int        `json:"cycle"`
	Status          string     `json:"status"`
	PriorityCode    string     `json:"priority"`
	TargetMinutes   int        `json:"target_minutes"`
	CalendarCode    string     `json:"calendar_code"`
	WarnPercent     int        `json:"warn_percent"`
	ReminderMinutes *int       `json:"reminder_minutes"`
	StartedAt       time.Time  `json:"started_at"`
	WarnAt          time.Time  `json:"warn_at"`
	TargetAt        time.Time  `json:"target_at"`
	WarnedAt        *time.Time `json:"warned_at"`
	BreachedAt      *time.Time `json:"breached_at"`
	RemindedAt      *time.Time `json:"reminded_at"`
	StoppedAt       *time.Time `json:"stopped_at"`
}

// SLAState is the compact per-metric state shown in incident lists.
type SLAState struct {
	Status    string     `json:"status"`
	Cycle     int        `json:"cycle"`
	WarnAt    time.Time  `json:"warn_at"`
	TargetAt  time.Time  `json:"target_at"`
	StoppedAt *time.Time `json:"stopped_at"`
}

// SLASummary is the latest cycle per metric of one incident. Nil when the
// incident predates SLA (no instances).
type SLASummary struct {
	Response   *SLAState `json:"response"`
	Resolution *SLAState `json:"resolution"`
}

// SLAEvent is one escalation recorded by the worker, to be notified.
type SLAEvent struct {
	IncidentID string
	SLAID      string
	Metric     string
	Level      string
}

// Incident activity & notification types for SLA (§6.4, §12).
const (
	ActivitySLAWarning  = "sla_warning"
	ActivitySLABreached = "sla_breached"
	ActivitySLAReminder = "sla_reminder"

	NotifSLAWarning  = "sla_warning"
	NotifSLABreached = "sla_breached"
	NotifSLAReminder = "sla_reminder"
)
