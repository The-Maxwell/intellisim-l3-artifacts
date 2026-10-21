package simulations

import (
	"testing"

	"github.com/stretchr/testify/require"

	datasims "intelli.sim.core/data/simulations"
)

func TestAnalyzableStatus(t *testing.T) {
	require.True(t, analyzableStatus(datasims.SimulationStatusFinished))
	require.True(t, analyzableStatus(datasims.SimulationStatusPaused))
	require.False(t, analyzableStatus(datasims.SimulationStatusRunning))
	require.False(t, analyzableStatus(datasims.SimulationStatusReady))
}

func TestValidateAnalysisSelection(t *testing.T) {
	model := testModel()
	recorded := []datasims.PropertyInfo{{UID: "energy", DataType: "number"}}
	const recMin, recMax int64 = 0, 100

	// valid
	require.Nil(t, validateAnalysisSelection(datasims.AnalysisSelection{AgentUIDs: []string{"prey"}, PropertyUIDs: []string{"energy"}}, model, recorded, recMin, recMax))

	// unknown agent
	err := validateAnalysisSelection(datasims.AnalysisSelection{AgentUIDs: []string{"ghost"}}, model, recorded, recMin, recMax)
	require.NotNil(t, err)
	require.Len(t, err.Result.Errors, 1)

	// unknown property + bad range
	from, to := uint64(50), uint64(10)
	err = validateAnalysisSelection(datasims.AnalysisSelection{PropertyUIDs: []string{"nope"}, StepFrom: &from, StepTo: &to}, model, recorded, recMin, recMax)
	require.NotNil(t, err)
	require.Len(t, err.Result.Errors, 2)

	// step_from beyond the recorded range
	beyond := uint64(999999)
	err = validateAnalysisSelection(datasims.AnalysisSelection{StepFrom: &beyond}, model, recorded, recMin, recMax)
	require.NotNil(t, err)
	require.Len(t, err.Result.Errors, 1)
	require.Equal(t, "selection.step_from", err.Result.Errors[0].Path)

	// step_to before the recorded range (min=10, max=100, step_to=5)
	before := uint64(5)
	err = validateAnalysisSelection(datasims.AnalysisSelection{StepTo: &before}, model, recorded, 10, recMax)
	require.NotNil(t, err)
	require.Len(t, err.Result.Errors, 1)
	require.Equal(t, "selection.step_to", err.Result.Errors[0].Path)
}
