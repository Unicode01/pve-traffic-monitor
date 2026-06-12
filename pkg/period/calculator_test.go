package period

import (
	"testing"
	"time"
)

func TestCalculateCreationBasedPeriodStart(t *testing.T) {
	creation := time.Date(2026, 1, 15, 10, 30, 0, 0, time.Local)

	tests := []struct {
		name       string
		periodType string
		now        time.Time
		want       time.Time
	}{
		{
			name:       "hour keeps minute",
			periodType: "hour",
			now:        time.Date(2026, 1, 15, 12, 45, 0, 0, time.Local),
			want:       time.Date(2026, 1, 15, 12, 30, 0, 0, time.Local),
		},
		{
			name:       "day keeps creation clock",
			periodType: "day",
			now:        time.Date(2026, 1, 17, 9, 0, 0, 0, time.Local),
			want:       time.Date(2026, 1, 16, 10, 30, 0, 0, time.Local),
		},
		{
			name:       "month before anniversary uses previous month",
			periodType: "month",
			now:        time.Date(2026, 3, 14, 9, 0, 0, 0, time.Local),
			want:       time.Date(2026, 2, 15, 10, 30, 0, 0, time.Local),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateCreationBasedPeriodStart(tt.periodType, creation, tt.now)
			if !got.Equal(tt.want) {
				t.Fatalf("period start = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestCalculateCreationBasedPeriodStartClampsMonthEnd(t *testing.T) {
	creation := time.Date(2026, 1, 31, 10, 30, 0, 0, time.Local)
	now := time.Date(2026, 2, 28, 11, 0, 0, 0, time.Local)
	want := time.Date(2026, 2, 28, 10, 30, 0, 0, time.Local)

	got := CalculateCreationBasedPeriodStart("month", creation, now)
	if !got.Equal(want) {
		t.Fatalf("period start = %s, want %s", got, want)
	}
}

func TestCalculateNextCreationBasedPeriodStartClampsMonthEnd(t *testing.T) {
	creation := time.Date(2026, 1, 31, 10, 30, 0, 0, time.Local)
	now := time.Date(2026, 2, 28, 11, 0, 0, 0, time.Local)
	want := time.Date(2026, 3, 31, 10, 30, 0, 0, time.Local)

	got := CalculateNextCreationBasedPeriodStart("month", creation, now)
	if !got.Equal(want) {
		t.Fatalf("next period start = %s, want %s", got, want)
	}
}
