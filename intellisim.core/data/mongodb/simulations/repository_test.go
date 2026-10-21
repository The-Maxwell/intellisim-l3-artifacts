package simulations

import (
	"reflect"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"intelli.sim.core/data/simulations"
)

func TestListMatch_EmptyFilter(t *testing.T) {
	match, err := listMatch(simulations.ListFilter{})
	if err != nil {
		t.Fatalf("listMatch: %v", err)
	}
	if len(match) != 0 {
		t.Fatalf("empty filter must produce an empty match, got %+v", match)
	}
}

func TestListMatch_SingleStatus(t *testing.T) {
	match, err := listMatch(simulations.ListFilter{
		Statuses: []simulations.SimulationStatus{simulations.SimulationStatusRunning},
	})
	if err != nil {
		t.Fatalf("listMatch: %v", err)
	}
	want := bson.D{{Key: "status", Value: string(simulations.SimulationStatusRunning)}}
	if !reflect.DeepEqual(match, want) {
		t.Fatalf("single status match = %+v, want %+v", match, want)
	}
}

func TestListMatch_MultipleStatuses(t *testing.T) {
	match, err := listMatch(simulations.ListFilter{
		Statuses: []simulations.SimulationStatus{
			simulations.SimulationStatusRunning,
			simulations.SimulationStatusPaused,
		},
	})
	if err != nil {
		t.Fatalf("listMatch: %v", err)
	}
	want := bson.D{{Key: "status", Value: bson.D{{Key: "$in", Value: bson.A{
		string(simulations.SimulationStatusRunning),
		string(simulations.SimulationStatusPaused),
	}}}}}
	if !reflect.DeepEqual(match, want) {
		t.Fatalf("multi status match = %+v, want %+v", match, want)
	}
}
