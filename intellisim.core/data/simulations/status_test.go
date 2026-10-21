package simulations_test

import (
	"testing"

	"intelli.sim.core/data/simulations"
)

// TestSimulationStatus_Streamable pins the streamable set to exactly the live-run
// statuses (running, paused). A stream room exists only for the duration of a live
// run, so any other status must report not-streamable to keep the WS join from
// orphaning a room.
func TestSimulationStatus_Streamable(t *testing.T) {
	cases := map[simulations.SimulationStatus]bool{
		simulations.SimulationStatusRunning:          true,
		simulations.SimulationStatusPaused:           true,
		simulations.SimulationStatusReady:            false,
		simulations.SimulationStatusGenerating:       false,
		simulations.SimulationStatusGenerationFailed: false,
		simulations.SimulationStatusQueued:           false,
		simulations.SimulationStatusFinished:         false,
		simulations.SimulationStatusFailed:           false,
		simulations.SimulationStatusStopped:          false,
	}
	for status, want := range cases {
		if got := status.Streamable(); got != want {
			t.Errorf("status %q Streamable()=%v want %v", status, got, want)
		}
	}
}
