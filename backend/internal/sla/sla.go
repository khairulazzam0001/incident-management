// Package sla holds the pure SLA rules (PRD_SLA_Escalation.md §6): working
// calendar arithmetic and the warning/breach/reminder decision. No I/O, so
// everything here is unit-tested without a database.
package sla

import (
	"errors"
	"time"
)

// Metrics and statuses of an SLA instance (trans_incident_sla).
const (
	MetricResponse   = "RESPONSE"
	MetricResolution = "RESOLUTION"

	StatusRunning   = "RUNNING"
	StatusMet       = "MET"
	StatusBreached  = "BREACHED"
	StatusCancelled = "CANCELLED"

	LevelWarning  = "WARNING"
	LevelBreach   = "BREACH"
	LevelReminder = "REMINDER"
)

// Window is one working interval on a weekday, in minutes since midnight
// (local to the calendar's time zone). End is exclusive.
type Window struct {
	Start int
	End   int
}

// Calendar describes when SLA minutes elapse. Always=true means 24x7.
type Calendar struct {
	Code     string
	Location *time.Location
	Always   bool
	Hours    map[time.Weekday]Window
	Holidays map[string]bool // "2006-01-02" in Location
}

// maxSearchDays bounds the day walk so a calendar without working days can't
// loop forever.
const maxSearchDays = 3660

// ErrNoWorkingTime means the calendar has no working minutes in range.
var ErrNoWorkingTime = errors.New("sla: kalender tidak punya jam kerja")

func (c Calendar) loc() *time.Location {
	if c.Location == nil {
		return time.UTC
	}
	return c.Location
}

// AddWorkingMinutes returns the instant when `minutes` working minutes have
// elapsed after start. Starting outside working hours begins at the next
// working window. The result is in UTC.
func (c Calendar) AddWorkingMinutes(start time.Time, minutes int) (time.Time, error) {
	if minutes <= 0 {
		return start.UTC(), nil
	}
	if c.Always {
		return start.Add(time.Duration(minutes) * time.Minute).UTC(), nil
	}
	loc := c.loc()
	cur := start.In(loc)
	remaining := minutes
	for i := 0; i < maxSearchDays; i++ {
		day := time.Date(cur.Year(), cur.Month(), cur.Day(), 0, 0, 0, 0, loc)
		w, ok := c.Hours[day.Weekday()]
		if ok && w.End > w.Start && !c.Holidays[day.Format("2006-01-02")] {
			open := day.Add(time.Duration(w.Start) * time.Minute)
			closeAt := day.Add(time.Duration(w.End) * time.Minute)
			if cur.Before(open) {
				cur = open
			}
			if cur.Before(closeAt) {
				avail := int(closeAt.Sub(cur) / time.Minute)
				if remaining <= avail {
					return cur.Add(time.Duration(remaining) * time.Minute).UTC(), nil
				}
				remaining -= avail
			}
		}
		cur = day.AddDate(0, 0, 1)
	}
	return time.Time{}, ErrNoWorkingTime
}

// Instance is the SLA state the decision needs.
type Instance struct {
	Status          string
	TargetAt        time.Time
	WarnAt          time.Time
	WarnedAt        *time.Time
	BreachedAt      *time.Time
	RemindedAt      *time.Time
	StoppedAt       *time.Time
	ReminderMinutes *int
}

// Decision is what the worker must do for one instance at `now`.
type Decision struct {
	Levels      []string   // escalations to record, in order
	NewStatus   string     // status after this tick
	NextCheckAt *time.Time // nil = no further checks needed
}

// Decide applies §6.4: warning at warn_at, breach at target_at, optional
// reminder after breach. Stopped instances need nothing. A breach reached
// before any tick skips the warning (one notification, not two).
func Decide(in Instance, now time.Time) Decision {
	d := Decision{NewStatus: in.Status}
	if in.StoppedAt != nil || (in.Status != StatusRunning && in.Status != StatusBreached) {
		return d
	}
	breachedAt := in.BreachedAt
	if in.Status == StatusRunning {
		switch {
		case !now.Before(in.TargetAt):
			d.Levels = append(d.Levels, LevelBreach)
			d.NewStatus = StatusBreached
			t := in.TargetAt
			breachedAt = &t
		case in.WarnedAt == nil && !now.Before(in.WarnAt):
			d.Levels = append(d.Levels, LevelWarning)
			next := in.TargetAt
			d.NextCheckAt = &next
			return d
		case in.WarnedAt == nil:
			next := in.WarnAt
			d.NextCheckAt = &next
			return d
		default:
			next := in.TargetAt
			d.NextCheckAt = &next
			return d
		}
	}
	// BREACHED (now or earlier): optional single reminder.
	if in.ReminderMinutes != nil && *in.ReminderMinutes > 0 && in.RemindedAt == nil && breachedAt != nil {
		due := breachedAt.Add(time.Duration(*in.ReminderMinutes) * time.Minute)
		if !now.Before(due) {
			d.Levels = append(d.Levels, LevelReminder)
		} else {
			d.NextCheckAt = &due
		}
	}
	return d
}

// WarnMinutes is the warning threshold in working minutes (ceil of percent).
func WarnMinutes(targetMinutes, warnPercent int) int {
	if warnPercent <= 0 || warnPercent >= 100 {
		warnPercent = 75
	}
	return (targetMinutes*warnPercent + 99) / 100
}
