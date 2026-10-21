package simulations

import (
	"context"
	"time"

	"go.uber.org/fx"
	"go.uber.org/zap"

	datasims "intelli.sim.core/data/simulations"
	"intelli.sim.core/modules/llm"
)

// Module is the FX module for the simulations package: config, event hub,
// generator, create/edit service, and the run-phase engine (run config, connect
// ticket store, WebSocket stream hub, run manager, run service) plus the HTTP API.
// The Generator and the run engine both depend on the llm.Service (as their narrow
// llmInvoker). Registered in cmd/intelli-sim/app.go.
var Module = fx.Module(
	"simulations",
	fx.Provide(
		NewModuleConfig,
		NewEventsHub,
		provideGenerator,
		NewService,
		// Run-phase engine wiring.
		NewRunConfig,
		provideTicketStore,
		NewStreamHub,
		provideEngineLLM,
		provideRunManager,
		provideRunService,
		NewAPI,
		NewAnalysisConfig,
		NewAnalysisService,
	),
	fx.Invoke(registerRunReconcile),
	fx.Invoke(registerAnalysisReconcile),
)

// provideGenerator is the FX adapter for NewGenerator: it takes the concrete
// llm.Service FX provides and passes it as the generator's narrow llmInvoker
// (which llm.Service satisfies). NewGenerator's own signature stays
// test-friendly (a narrow interface) and is not FX-resolvable directly.
func provideGenerator(llmService llm.Service, cfg ModuleConfig) *Generator {
	return NewGenerator(llmService, cfg)
}

// provideEngineLLM is the FX adapter handing the concrete llm.Service to the run
// engine as its narrow llmInvoker (mirrors provideGenerator). It is exposed as the
// llmInvoker interface so both the RunManager and the RunService resolve the same
// engine LLM dependency.
func provideEngineLLM(llmService llm.Service) llmInvoker {
	return llmService
}

// provideTicketStore builds the connect-ticket store with the TTL from RunConfig.
func provideTicketStore(cfg RunConfig) *TicketStore {
	return NewTicketStore(time.Duration(cfg.StreamTicketTTLSec) * time.Second)
}

// provideRunManager wires the run manager over its concrete deps, handing the
// WebSocket stream hub in as the engine's stepBroadcaster port (*StreamHub
// satisfies stepBroadcaster). NewRunManager's signature stays narrow (interfaces)
// so it is unit-testable without the FX graph or the live transport.
func provideRunManager(
	cfg RunConfig,
	repo datasims.SimulationsRepository,
	steps datasims.SimulationStepsRepository,
	inv llmInvoker,
	hub *EventsHub,
	stream *StreamHub,
	logger *zap.Logger,
) *RunManager {
	return NewRunManager(cfg, repo, steps, inv, hub, stream, logger)
}

// provideRunService wires the run service over its narrow ports: *RunManager
// satisfies runManagerPort and *TicketStore satisfies ticketIssuer.
func provideRunService(
	repo datasims.SimulationsRepository,
	mgr *RunManager,
	tickets *TicketStore,
	steps datasims.SimulationStepsRepository,
	inv llmInvoker,
	cfg RunConfig,
	logger *zap.Logger,
) RunService {
	return NewRunService(repo, mgr, tickets, steps, inv, cfg, logger)
}

// registerRunReconcile schedules the crash-recovery reconciliation: on app start,
// any simulation left persisted as running/paused (with no live run goroutine —
// every goroutine died with the previous process) is reconciled to failed. It runs
// once, synchronously, before the server begins serving.
func registerRunReconcile(lc fx.Lifecycle, mgr *RunManager, logger *zap.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := mgr.Reconcile(ctx); err != nil {
				logger.Named("simulations").Warn("run reconciliation failed at startup", zap.Error(err))
			}
			return nil
		},
	})
}

// registerAnalysisReconcile schedules the analysis crash-recovery reconciliation:
// on app start, any analysis left persisted as pending/running (its driving
// goroutine died with the previous process) is marked failed. Mirrors
// registerRunReconcile; a failure is logged, not fatal.
func registerAnalysisReconcile(lc fx.Lifecycle, svc AnalysisService, logger *zap.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := svc.Reconcile(ctx); err != nil {
				logger.Named("simulations").Warn("analysis reconciliation failed at startup", zap.Error(err))
			}
			return nil
		},
	})
}
