package simulations

import (
	"testing"

	"intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

func termAgent(id uint64, uid string, removed bool) *RuntimeAgent {
	ra := &RuntimeAgent{A: &datasims.SimulationAgent{ID: id, AgentUID: uid}}
	ra.removed = removed
	return ra
}

// U2: the population primitive counts LIVE agents (removed excluded), optionally
// filtered to archetype UIDs via TargetUIDs (or the single TargetUID), and
// compares via the shared operator semantics.
func TestEvalTermination_Population(t *testing.T) {
	rs := newRS(&datasims.SimulationEnvironment{Width: 10, Height: 10},
		termAgent(1, "walker", false),
		termAgent(2, "walker", true), // removed: not counted
		termAgent(3, "guide", false),
	)
	cases := []struct {
		name string
		c    models.Condition
		want bool
	}{
		{"total live eq 2", models.Condition{Type: models.ConditionTypePopulation, Operator: models.ConditionOpEquals, Value: 2.0}, true},
		{"total live eq 0", models.Condition{Type: models.ConditionTypePopulation, Operator: models.ConditionOpEquals, Value: 0.0}, false},
		{"total lte 1", models.Condition{Type: models.ConditionTypePopulation, Operator: models.ConditionOpLessOrEq, Value: 1.0}, false},
		{"walker eq 1 via TargetUIDs", models.Condition{Type: models.ConditionTypePopulation, TargetUIDs: []string{"walker"}, Operator: models.ConditionOpEquals, Value: 1.0}, true},
		{"guide eq 1 via TargetUID", models.Condition{Type: models.ConditionTypePopulation, TargetUID: "guide", Operator: models.ConditionOpEquals, Value: 1.0}, true},
		{"missing archetype eq 0", models.Condition{Type: models.ConditionTypePopulation, TargetUIDs: []string{"ghost"}, Operator: models.ConditionOpEquals, Value: 0.0}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := evalTermination(rs, tc.c); got != tc.want {
				t.Fatalf("evalTermination = %v, want %v", got, tc.want)
			}
		})
	}
}

// U2: step and global_property delegate to the shared rule helpers; composite
// recurses over the termination evaluator itself.
func TestEvalTermination_StepGlobalAndComposite(t *testing.T) {
	rs := newRS(&datasims.SimulationEnvironment{
		Width: 10, Height: 10,
		GlobalProperties: map[string]interface{}{"alarm": true},
	}, termAgent(1, "walker", false))
	rs.Step = 40

	step := models.Condition{Type: models.ConditionTypeStep, Operator: models.ConditionOpGreaterOrEq, Value: 40.0}
	if !evalTermination(rs, step) {
		t.Fatal("step >= 40 at step 40 must be true")
	}
	global := models.Condition{Type: models.ConditionTypeGlobalProperty, PropertyUID: "alarm", Operator: models.ConditionOpEquals, Value: true}
	if !evalTermination(rs, global) {
		t.Fatal("global_property alarm == true must be true")
	}
	pop1 := models.Condition{Type: models.ConditionTypePopulation, Operator: models.ConditionOpEquals, Value: 1.0}
	and := models.Condition{Type: models.ConditionTypeComposite, LogicOperator: models.LogicOpAND, SubConditions: []models.Condition{step, pop1}}
	if !evalTermination(rs, and) {
		t.Fatal("AND(step, population) must be true")
	}
	not := models.Condition{Type: models.ConditionTypeComposite, LogicOperator: models.LogicOpNOT, SubConditions: []models.Condition{pop1}}
	if evalTermination(rs, not) {
		t.Fatal("NOT(population==1) must be false")
	}
}

// U2: only population / step / global_property / composite are legal termination
// primitives — anything else is never-true even if it slips past validation
// (agent-positional conditions are meaningless and random/event would make the
// stop nondeterministic / order-coupled).
func TestEvalTermination_UnsupportedTypesFalse(t *testing.T) {
	rs := newRS(&datasims.SimulationEnvironment{Width: 10, Height: 10}, termAgent(1, "walker", false))
	for _, ct := range []models.ConditionType{
		models.ConditionTypeRandom,
		models.ConditionTypeEvent,
		models.ConditionTypeProperty,
		models.ConditionTypeProximity,
		models.ConditionTypeState,
	} {
		c := models.Condition{Type: ct, Operator: models.ConditionOpEquals, Value: 1.0}
		if evalTermination(rs, c) {
			t.Fatalf("condition type %q must never terminate a run", ct)
		}
	}
}
