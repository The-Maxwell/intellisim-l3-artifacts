package simulations

import (
	"testing"
	"time"

	datasims "intelli.sim.core/data/simulations"
)

// TestActiveRunDuration exercises the pure active-duration computation: total
// wall-clock from run start to finish MINUS the time spent paused, derived from
// the status-history transitions within the run window. Deterministic (no clock),
// so it is the primary guard for the run-timing math.
func TestActiveRunDuration(t *testing.T) {
	base := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	at := func(sec int) time.Time { return base.Add(time.Duration(sec) * time.Second) }
	change := func(status datasims.SimulationStatus, sec int) datasims.SimulationStatusChange {
		return datasims.SimulationStatusChange{Status: status, At: at(sec), By: "system"}
	}

	cases := []struct {
		name    string
		started time.Time
		finished time.Time
		history []datasims.SimulationStatusChange
		want    time.Duration
	}{
		{
			name:     "no pauses — full wall clock",
			started:  at(0),
			finished: at(100),
			history: []datasims.SimulationStatusChange{
				change(datasims.SimulationStatusRunning, 0),
				change(datasims.SimulationStatusFinished, 100),
			},
			want: 100 * time.Second,
		},
		{
			name:     "one pause/resume cycle — paused gap excluded",
			started:  at(0),
			finished: at(100),
			history: []datasims.SimulationStatusChange{
				change(datasims.SimulationStatusRunning, 0),
				change(datasims.SimulationStatusPaused, 30), // paused 30..50 = 20s excluded
				change(datasims.SimulationStatusRunning, 50),
				change(datasims.SimulationStatusFinished, 100),
			},
			want: 80 * time.Second,
		},
		{
			name:     "finalized while paused — trailing gap extends to finish",
			started:  at(0),
			finished: at(100),
			history: []datasims.SimulationStatusChange{
				change(datasims.SimulationStatusRunning, 0),
				change(datasims.SimulationStatusPaused, 40),   // paused from 40 to finish (100) = 60s
				change(datasims.SimulationStatusStopped, 100), // terminal entry AT finish
			},
			want: 40 * time.Second,
		},
		{
			name:     "multiple pause/resume cycles",
			started:  at(0),
			finished: at(200),
			history: []datasims.SimulationStatusChange{
				change(datasims.SimulationStatusRunning, 0),
				change(datasims.SimulationStatusPaused, 20), // 20..40 = 20s
				change(datasims.SimulationStatusRunning, 40),
				change(datasims.SimulationStatusPaused, 100), // 100..150 = 50s
				change(datasims.SimulationStatusRunning, 150),
				change(datasims.SimulationStatusFinished, 200),
			},
			want: 130 * time.Second, // 200 - 70 paused
		},
		{
			name:     "prior-run entries before start are ignored",
			started:  at(100),
			finished: at(160),
			history: []datasims.SimulationStatusChange{
				// A previous run's transitions — all before this run's start.
				change(datasims.SimulationStatusRunning, 0),
				change(datasims.SimulationStatusPaused, 10),
				change(datasims.SimulationStatusFinished, 40),
				// This run.
				change(datasims.SimulationStatusRunning, 100),
				change(datasims.SimulationStatusPaused, 120), // 120..140 = 20s
				change(datasims.SimulationStatusRunning, 140),
				change(datasims.SimulationStatusFinished, 160),
			},
			want: 40 * time.Second, // 60 window - 20 paused
		},
		{
			name:     "no history entries — falls back to wall clock",
			started:  at(0),
			finished: at(50),
			history:  nil,
			want:     50 * time.Second,
		},
		{
			name:     "finished before started (clock skew) — clamped to zero",
			started:  at(100),
			finished: at(50),
			history:  nil,
			want:     0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := activeRunDuration(tc.started, tc.finished, tc.history)
			if got != tc.want {
				t.Errorf("activeRunDuration = %v, want %v", got, tc.want)
			}
		})
	}
}
