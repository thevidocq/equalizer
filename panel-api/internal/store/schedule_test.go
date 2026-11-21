package store

import (
	"testing"
	"time"
)

func TestDiscoverySlots(t *testing.T) {
	rome, _ := time.LoadLocation("Europe/Rome")
	now := time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC) // Saturday, 11:00 in Rome
	d := DiscoveryTime{Hour: 3, Weekday: 1, Zone: "Europe/Rome"}
	if got, want := d.LastSlot("daily", now), time.Date(2026, 11, 21, 3, 0, 0, 0, rome); !got.Equal(want) {
		t.Errorf("daily last: %v, want %v", got, want)
	}
	if got, want := d.NextSlot("daily", now), time.Date(2026, 11, 22, 3, 0, 0, 0, rome); !got.Equal(want) {
		t.Errorf("daily next: %v, want %v", got, want)
	}
	if got, want := d.LastSlot("weekly", now), time.Date(2026, 11, 16, 3, 0, 0, 0, rome); !got.Equal(want) {
		t.Errorf("weekly last (Monday): %v, want %v", got, want)
	}
	if got, want := d.NextSlot("weekly", now), time.Date(2026, 11, 23, 3, 0, 0, 0, rome); !got.Equal(want) {
		t.Errorf("weekly next: %v, want %v", got, want)
	}
	d.setAt = now.Add(-time.Hour) // set after today's 03:00: not due until tomorrow
	if d.due("daily", now) {
		t.Error("due for a slot before the schedule was set")
	}
	d.setAt = now.Add(-48 * time.Hour)
	last := time.Date(2026, 11, 20, 2, 0, 0, 0, time.UTC)
	d.Last = &last
	if !d.due("daily", now) {
		t.Error("not due although no discovery ran since today's slot")
	}
	ran := time.Date(2026, 11, 21, 2, 30, 0, 0, time.UTC)
	d.Last = &ran
	if d.due("daily", now) {
		t.Error("due again after it ran")
	}
}
