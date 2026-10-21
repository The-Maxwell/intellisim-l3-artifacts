// Package data wires the storage-agnostic repository interfaces of
// data/{chats,models,simulations} to their MongoDB implementations in
// data/mongodb. Include data.Module in the fx application instead of
// providing repository constructors directly.
package data

import (
	"go.uber.org/fx"

	mongochats "intelli.sim.core/data/mongodb/chats"
	mongomodels "intelli.sim.core/data/mongodb/models"
	mongosimulations "intelli.sim.core/data/mongodb/simulations"
)

// Module provides every repository implementation, typed as its interface:
//
//	chats.ChatsRepository
//	models.ModelsRepository
//	simulations.SimulationsRepository
//	simulations.SimulationStepsRepository
var Module = fx.Options(
	fx.Provide(
		mongochats.NewChatsRepository,
		mongomodels.NewModelsRepository,
		mongosimulations.NewSimulationsRepository,
		mongosimulations.NewSimulationStepsRepository,
		mongosimulations.NewAnalysesRepository,
		mongosimulations.NewAnalysisFileStore,
	),
)
