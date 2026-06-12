package main

import (
	"testing"
	"time"
)

func TestDayBoundsUsesInclusiveEndOfDay(t *testing.T) {
	date := time.Date(2026, 6, 12, 13, 14, 15, 0, time.Local)

	start, end := dayBounds(date)
	wantStart := time.Date(2026, 6, 12, 0, 0, 0, 0, time.Local)
	wantEnd := time.Date(2026, 6, 13, 0, 0, 0, 0, time.Local).Add(-time.Nanosecond)

	if !start.Equal(wantStart) {
		t.Fatalf("start = %s, want %s", start, wantStart)
	}
	if !end.Equal(wantEnd) {
		t.Fatalf("end = %s, want %s", end, wantEnd)
	}
	if !end.Before(wantStart.AddDate(0, 0, 1)) {
		t.Fatalf("end must be before the next day, got %s", end)
	}
}
