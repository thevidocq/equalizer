package store

import "time"

// DiscoveryTime is when continuous discovery runs: at Hour (0-23) every
// day, or on Weekday (0 Sunday … 6 Saturday) for weekly, in Zone.
type DiscoveryTime struct {
	Hour    int        `json:"hour"`
	Weekday int        `json:"weekday"`
	Zone    string     `json:"zone"`
	Next    *time.Time `json:"next"` // nil when off
	Last    *time.Time `json:"last"` // the latest discovery of the site, any
	setAt   time.Time
}

func (t DiscoveryTime) location() *time.Location {
	if loc, err := time.LoadLocation(t.Zone); err == nil && t.Zone != "" {
		return loc
	}
	return time.UTC
}

// LastSlot is the latest moment at or before now the schedule called for.
func (t DiscoveryTime) LastSlot(schedule string, now time.Time) time.Time {
	loc := t.location()
	n := now.In(loc)
	slot := time.Date(n.Year(), n.Month(), n.Day(), t.Hour, 0, 0, 0, loc)
	if slot.After(n) {
		slot = slot.AddDate(0, 0, -1)
	}
	if schedule == "weekly" {
		back := (int(slot.Weekday()) - t.Weekday + 7) % 7
		slot = slot.AddDate(0, 0, -back)
	}
	return slot
}

// NextSlot is the first moment after now the schedule calls for.
func (t DiscoveryTime) NextSlot(schedule string, now time.Time) time.Time {
	last := t.LastSlot(schedule, now)
	if schedule == "weekly" {
		return last.AddDate(0, 0, 7)
	}
	return last.AddDate(0, 0, 1)
}

// due tells whether a slot passed since the schedule was set and since the
// latest discovery started.
func (t DiscoveryTime) due(schedule string, now time.Time) bool {
	slot := t.LastSlot(schedule, now)
	if !slot.After(t.setAt) {
		return false
	}
	return t.Last == nil || t.Last.Before(slot)
}
