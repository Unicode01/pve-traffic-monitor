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

func TestShouldApplyRateLimitOnlyTightens(t *testing.T) {
	tests := []struct {
		name    string
		current float64
		desired float64
		want    bool
	}{
		{name: "unlimited applies", current: 0, desired: 10, want: true},
		{name: "wider current applies", current: 20, desired: 10, want: true},
		{name: "same current skips", current: 10, desired: 10, want: false},
		{name: "stricter current skips", current: 5, desired: 10, want: false},
		{name: "invalid desired skips", current: 10, desired: 0, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldApplyRateLimit(tt.current, tt.desired); got != tt.want {
				t.Fatalf("shouldApplyRateLimit(%v, %v) = %v, want %v", tt.current, tt.desired, got, tt.want)
			}
		})
	}
}

func TestCalculatePeriodStartUsesCreationTime(t *testing.T) {
	monitor := &Monitor{}
	creation := time.Date(2026, 1, 15, 10, 30, 0, 0, time.Local)

	tests := []struct {
		name   string
		period string
		now    time.Time
		want   time.Time
	}{
		{
			name:   "hour",
			period: "hour",
			now:    time.Date(2026, 1, 15, 12, 45, 0, 0, time.Local),
			want:   time.Date(2026, 1, 15, 12, 30, 0, 0, time.Local),
		},
		{
			name:   "day",
			period: "day",
			now:    time.Date(2026, 1, 17, 9, 0, 0, 0, time.Local),
			want:   time.Date(2026, 1, 16, 10, 30, 0, 0, time.Local),
		},
		{
			name:   "month",
			period: "month",
			now:    time.Date(2026, 3, 14, 9, 0, 0, 0, time.Local),
			want:   time.Date(2026, 2, 15, 10, 30, 0, 0, time.Local),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := monitor.calculatePeriodStart(tt.period, creation, tt.now); !got.Equal(tt.want) {
				t.Fatalf("calculatePeriodStart(%s) = %s, want %s", tt.period, got, tt.want)
			}
		})
	}
}
