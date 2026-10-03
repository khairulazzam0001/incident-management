package sla

import (
	"testing"
	"time"
)

var jakarta = time.FixedZone("WIB", 7*3600)

func businessCalendar() Calendar {
	w := Window{Start: 8 * 60, End: 17 * 60}
	return Calendar{
		Code: "BUSINESS_HOURS", Location: jakarta,
		Hours: map[time.Weekday]Window{
			time.Monday: w, time.Tuesday: w, time.Wednesday: w, time.Thursday: w, time.Friday: w,
		},
		Holidays: map[string]bool{"2026-10-07": true}, // Rabu libur
	}
}

func at(y int, m time.Month, d, hh, mm int) time.Time {
	return time.Date(y, m, d, hh, mm, 0, 0, jakarta)
}

// TestAddWorkingMinutes pins §6.2 / SLA-05 edge cases. 2026-10-02 is a Friday.
func TestAddWorkingMinutes(t *testing.T) {
	cal := businessCalendar()
	tests := []struct {
		name    string
		start   time.Time
		minutes int
		want    time.Time
	}{
		{"dalam jam kerja", at(2026, 10, 5, 9, 0), 60, at(2026, 10, 5, 10, 0)},
		{"lintas akhir pekan (Jumat 16:00 + 4 jam)", at(2026, 10, 2, 16, 0), 240, at(2026, 10, 5, 11, 0)},
		{"mulai sebelum jam buka", at(2026, 10, 5, 6, 30), 30, at(2026, 10, 5, 8, 30)},
		{"mulai setelah jam tutup", at(2026, 10, 5, 18, 0), 60, at(2026, 10, 6, 9, 0)},
		{"mulai Sabtu", at(2026, 10, 3, 10, 0), 60, at(2026, 10, 5, 9, 0)},
		{"tepat habis di jam tutup", at(2026, 10, 5, 16, 0), 60, at(2026, 10, 5, 17, 0)},
		{"lewati hari libur (Selasa 16:00 + 2 jam → Kamis)", at(2026, 10, 6, 16, 0), 120, at(2026, 10, 8, 9, 0)},
		{"5 hari kerja (P4) dari Senin 08:00", at(2026, 10, 5, 8, 0), 2700, at(2026, 10, 12, 17, 0)},
		{"nol menit", at(2026, 10, 3, 10, 0), 0, at(2026, 10, 3, 10, 0)},
	}
	for _, tt := range tests {
		got, err := cal.AddWorkingMinutes(tt.start, tt.minutes)
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		if !got.Equal(tt.want) {
			t.Errorf("%s: got %s, want %s", tt.name, got.In(jakarta), tt.want)
		}
	}

	always := Calendar{Code: "24x7", Always: true}
	if got, _ := always.AddWorkingMinutes(at(2026, 10, 3, 23, 50), 15); !got.Equal(at(2026, 10, 4, 0, 5)) {
		t.Errorf("24x7: got %s", got.In(jakarta))
	}
	if _, err := (Calendar{Location: jakarta}).AddWorkingMinutes(at(2026, 10, 5, 9, 0), 10); err != ErrNoWorkingTime {
		t.Errorf("kalender tanpa jam kerja: err = %v", err)
	}
}

func TestDecide(t *testing.T) {
	start := at(2026, 10, 5, 9, 0)
	warn := start.Add(45 * time.Minute)
	target := start.Add(60 * time.Minute)
	reminder := 30
	base := Instance{Status: StatusRunning, WarnAt: warn, TargetAt: target, ReminderMinutes: &reminder}

	d := Decide(base, start.Add(10*time.Minute))
	if len(d.Levels) != 0 || d.NextCheckAt == nil || !d.NextCheckAt.Equal(warn) {
		t.Fatalf("sebelum warn: %+v", d)
	}
	d = Decide(base, warn)
	if len(d.Levels) != 1 || d.Levels[0] != LevelWarning || d.NewStatus != StatusRunning || !d.NextCheckAt.Equal(target) {
		t.Fatalf("warn: %+v", d)
	}
	warned := warn
	afterWarn := base
	afterWarn.WarnedAt = &warned
	if d = Decide(afterWarn, warn.Add(time.Minute)); len(d.Levels) != 0 {
		t.Fatalf("warning tidak boleh berulang: %+v", d)
	}
	d = Decide(afterWarn, target)
	if len(d.Levels) != 1 || d.Levels[0] != LevelBreach || d.NewStatus != StatusBreached ||
		d.NextCheckAt == nil || !d.NextCheckAt.Equal(target.Add(30*time.Minute)) {
		t.Fatalf("breach: %+v", d)
	}
	// Tick pertama sudah lewat target + reminder: breach dan reminder sekaligus, tanpa warning.
	d = Decide(base, target.Add(2*time.Hour))
	if len(d.Levels) != 2 || d.Levels[0] != LevelBreach || d.Levels[1] != LevelReminder || d.NextCheckAt != nil {
		t.Fatalf("breach+reminder: %+v", d)
	}
	breached := target
	reminded := target.Add(30 * time.Minute)
	done := Instance{Status: StatusBreached, TargetAt: target, BreachedAt: &breached, RemindedAt: &reminded, ReminderMinutes: &reminder}
	if d = Decide(done, target.Add(3*time.Hour)); len(d.Levels) != 0 || d.NextCheckAt != nil {
		t.Fatalf("reminder sekali saja: %+v", d)
	}
	stopped := start
	if d = Decide(Instance{Status: StatusRunning, TargetAt: target, WarnAt: warn, StoppedAt: &stopped}, target); len(d.Levels) != 0 {
		t.Fatalf("instance berhenti: %+v", d)
	}
	if got := WarnMinutes(15, 75); got != 12 {
		t.Errorf("WarnMinutes(15,75) = %d, want 12", got)
	}
}
