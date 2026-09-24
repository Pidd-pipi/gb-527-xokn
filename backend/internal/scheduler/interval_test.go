package scheduler

import (
	"testing"
	"time"

	"satellite-contact-window-deconfliction/backend/internal/model"
)

func TestSweepCapacityUsesHalfOpenIntervalsAndBuffer(t *testing.T) {
	base := time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC)
	windows := []model.ContactWindow{
		{ID: 1, StartAt: base, EndAt: base.Add(10 * time.Minute)},
		{ID: 2, StartAt: base.Add(10 * time.Minute), EndAt: base.Add(20 * time.Minute)},
	}
	tests := []struct {
		name     string
		capacity int
		buffer   time.Duration
		want     int
	}{
		{name: "touching intervals do not overlap", capacity: 1, want: 0},
		{name: "slew buffer creates conflict", capacity: 1, buffer: time.Second, want: 1},
		{name: "second antenna absorbs overlap", capacity: 2, buffer: time.Minute, want: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := SweepCapacity(windows, test.capacity, test.buffer)
			if len(got) != test.want {
				t.Fatalf("got %d groups, want %d", len(got), test.want)
			}
		})
	}
}

func TestOverlapsIsSymmetric(t *testing.T) {
	base := time.Now().UTC()
	left := Interval{Start: base, End: base.Add(time.Minute)}
	right := Interval{Start: base.Add(30 * time.Second), End: base.Add(2 * time.Minute)}
	if !Overlaps(left, right) || !Overlaps(right, left) {
		t.Fatal("overlap must be symmetric")
	}
}
