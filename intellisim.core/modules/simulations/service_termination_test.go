package simulations

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	datamodels "intelli.sim.core/data/models"
)

func popZeroTermination() *datamodels.Condition {
	return &datamodels.Condition{Type: datamodels.ConditionTypePopulation, Operator: datamodels.ConditionOpEquals, Value: 0.0}
}

// U2: creating a simulation stamps the model's default termination condition
// onto the simulation.
func TestService_Create_StampsModelTermination(t *testing.T) {
	model := readyModel("m1", "alice")
	model.SimulationConfig.Termination = popZeroTermination()
	simsRepo := newFakeSimsRepo()
	modelsRepo := newFakeModelsRepo()
	modelsRepo.put(model)

	svc, _ := newTestService(t, simsRepo, modelsRepo, &fakeSvcLLM{})
	resp, err := svc.Create(ctxAs("alice", false), CreateSimulationRequest{ModelID: "m1"})
	require.NoError(t, err)

	persisted := simsRepo.get(resp.ID)
	require.NotNil(t, persisted.Termination, "model default termination must be stamped at create")
	require.Equal(t, *popZeroTermination(), *persisted.Termination)
	require.NotSame(t, model.SimulationConfig.Termination, persisted.Termination, "stamp must deep-copy, not alias the model's condition")
}

// U2: duplicating a simulation carries its termination condition to the copy.
func TestService_Duplicate_CopiesTermination(t *testing.T) {
	model := readyModel("m1", "alice")
	sim := readySim("s1", "alice", model)
	sim.Termination = popZeroTermination()
	simsRepo := newFakeSimsRepo()
	simsRepo.store["s1"] = sim
	modelsRepo := newFakeModelsRepo()
	modelsRepo.put(model)

	svc, _ := newTestService(t, simsRepo, modelsRepo, &fakeSvcLLM{})
	resp, err := svc.Duplicate(ctxAs("alice", false), "s1", DuplicateRequest{})
	require.NoError(t, err)

	dup := simsRepo.get(resp.ID)
	require.NotNil(t, dup.Termination, "duplicate must carry the termination condition")
	require.Equal(t, *popZeroTermination(), *dup.Termination)
	require.NotSame(t, sim.Termination, dup.Termination, "duplicate must deep-copy, not alias")
}

// U2: PUT applies (and can clear) the termination condition; the response
// exposes it.
func TestService_Update_AppliesTermination(t *testing.T) {
	model := readyModel("m1", "alice")
	sim := readySim("s1", "alice", model)
	simsRepo := newFakeSimsRepo()
	simsRepo.store["s1"] = sim
	modelsRepo := newFakeModelsRepo()
	modelsRepo.put(model)

	svc, _ := newTestService(t, simsRepo, modelsRepo, &fakeSvcLLM{})
	req := UpdateSimulationRequest{
		Name:  "Sim",
		Steps: 50,
		Agents: []SimulationAgentDTO{
			{ID: 1, AgentUID: "prey", Name: "Prey #1", State: "alive"},
		},
		Termination: &TerminationConditionDTO{Type: "population", Operator: "eq", Value: 0.0},
	}
	resp, err := svc.Update(ctxAs("alice", false), "s1", req)
	require.NoError(t, err)
	require.NotNil(t, resp.Termination, "response must expose the termination condition")
	require.Equal(t, "population", resp.Termination.Type)

	persisted := simsRepo.get("s1")
	require.NotNil(t, persisted.Termination)
	require.Equal(t, datamodels.ConditionTypePopulation, persisted.Termination.Type)

	// Full-replace semantics: omitting the field clears it.
	req.Termination = nil
	_, err = svc.Update(ctxAs("alice", false), "s1", req)
	require.NoError(t, err)
	require.Nil(t, simsRepo.get("s1").Termination)
}

// U2: the service rejects termination conditions outside the restricted subset
// (defense in depth behind the DTO's oneof — nested sub-conditions included).
func TestService_Update_RejectsInvalidTermination(t *testing.T) {
	model := readyModel("m1", "alice")
	sim := readySim("s1", "alice", model)
	simsRepo := newFakeSimsRepo()
	simsRepo.store["s1"] = sim
	modelsRepo := newFakeModelsRepo()
	modelsRepo.put(model)

	svc, _ := newTestService(t, simsRepo, modelsRepo, &fakeSvcLLM{})
	req := UpdateSimulationRequest{
		Name:  "Sim",
		Steps: 50,
		Agents: []SimulationAgentDTO{
			{ID: 1, AgentUID: "prey", Name: "Prey #1", State: "alive"},
		},
		// population missing its operator: unenforceable, must 422.
		Termination: &TerminationConditionDTO{Type: "population", Value: 0.0},
	}
	_, err := svc.Update(ctxAs("alice", false), "s1", req)
	var ve *ValidationError
	require.True(t, errors.As(err, &ve), "expected *ValidationError, got %T", err)
	require.NotEmpty(t, ve.Result.Errors)
}

// U2: ValidateInstance enforces the termination condition contract.
func TestValidateInstance_Termination(t *testing.T) {
	model := readyModel("m1", "alice")
	base := readySim("s1", "alice", model)

	termErrs := func(c *datamodels.Condition) []ValidationIssue {
		sim := base
		sim.Termination = c
		res := ValidateInstance(sim)
		var out []ValidationIssue
		for _, e := range res.Errors {
			if len(e.Path) >= len("termination") && e.Path[:len("termination")] == "termination" {
				out = append(out, e)
			}
		}
		return out
	}

	require.Empty(t, termErrs(popZeroTermination()), "valid population condition must pass")
	require.Empty(t, termErrs(nil), "nil termination must pass")

	require.NotEmpty(t, termErrs(&datamodels.Condition{Type: datamodels.ConditionTypeRandom, Operator: datamodels.ConditionOpLessOrEq, Value: 0.5}),
		"unsupported type must be rejected")
	require.NotEmpty(t, termErrs(&datamodels.Condition{Type: datamodels.ConditionTypePopulation, Value: 0.0}),
		"population without operator must be rejected")
	require.NotEmpty(t, termErrs(&datamodels.Condition{Type: datamodels.ConditionTypePopulation, Operator: datamodels.ConditionOpEquals}),
		"population without value must be rejected")
	require.NotEmpty(t, termErrs(&datamodels.Condition{Type: datamodels.ConditionTypeGlobalProperty, Operator: datamodels.ConditionOpEquals, Value: true}),
		"global_property without property_uid must be rejected")
	require.NotEmpty(t, termErrs(&datamodels.Condition{Type: datamodels.ConditionTypeComposite, LogicOperator: datamodels.LogicOpAND}),
		"composite without sub_conditions must be rejected")
	require.NotEmpty(t, termErrs(&datamodels.Condition{
		Type: datamodels.ConditionTypeComposite, LogicOperator: datamodels.LogicOpAND,
		SubConditions: []datamodels.Condition{{Type: datamodels.ConditionTypeProximity, Operator: datamodels.ConditionOpEquals, Value: 1.0}},
	}), "unsupported nested type must be rejected")
}
