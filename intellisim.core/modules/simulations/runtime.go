package simulations

import (
	"math/rand"
	"sort"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// RuntimeAgent wraps a persistent agent with per-run transient state. It is the
// in-memory unit the step pipeline operates on; only A's fields are persisted.
type RuntimeAgent struct {
	A            *datasims.SimulationAgent // persistent fields (Position/Properties/State/Memory)
	Archetype    *models.Agent             // resolved snapshot archetype (UID == A.AgentUID)
	Strategy     ExecutionStrategy         // selected per agent (hybrid → per-archetype)
	Velocity     Vector
	// rem is the sub-cell movement remainder (U4): the fractional part of the
	// agent's true position that integer grid rounding discarded this step,
	// carried into the next integration so fractional speeds converge to the
	// commanded speed (|rem| ≤ 0.5 per axis by construction). Runtime-only,
	// never persisted; zero at spawn/teleport; zeroed per axis when collision
	// response corrects the integrated position; cloned and committed with the
	// synchronous working copy like Velocity.
	rem Vec2
	Cooldowns    map[string]uint64 // ruleID -> step last fired
	OnceFired    map[string]bool   // ExecuteOnce rules already fired
	LastDecision *DecisionIntent   // for FallbackLastDecision
	memCap       int               // effective memory capacity for this agent (resolved at spawn)
	removed      bool              // marked for removal at tick end

	// llmCallPending marks an on-demand LLM decision requested by a fired
	// call_llm rule action. It is consumed (cleared) by the NEXT step's LLM
	// phase, which includes the agent in the decision-purpose call even when
	// DecisionFrequency is not due. Runtime-only, never persisted; cloned and
	// committed with the synchronous working copy like LastDecision.
	llmCallPending bool

	// stepDecision is the compact summary of the LLM decision applied to this
	// agent THIS step (decision label(s), or the LLMNote action summary for
	// full-control strategies). Transient and this-step-only: stamped by
	// recordStepDecisions right after the LLM phase, emitted by buildStepRows
	// (SimulationStep.Decision → WS frame + replay DTO), then cleared by
	// clearStepDecisions so between-step keyframes never carry a stale decision.
	stepDecision string
}

// spawnRequest is a queued agent-creation request, applied at tick end so that
// spawned agents only participate from the next tick (legacy ordering).
type spawnRequest struct {
	archetypeUID string                 // SpawnAgentUID
	count        int                    // resolved spawn count (>=1)
	properties   map[string]interface{} // explicit overrides
	inheritFrom  []string               // parent property UIDs to copy
	transform    bool                   // true when produced by ActionTypeTransform: the spawn REPLACES the source (applySpawns queues parentID for removal)
	position     *models.SpawnPosition  // optional placement directive
	parentID     uint64                 // emitting agent instance id (for inherit/placement)
}

// forcedInteraction is one queued force-fire request produced by an interact
// rule action or LLM interact intent. It is drained by ProcessInteractions in
// the same tick (rules/intents run before the interactions phase in both update
// modes). The interaction's Conditions and UseLLM mediation are SKIPPED on a
// forced fire (the rule/LLM already made the decision they gate); geometric
// eligibility (radius / shape overlap / InitiatorUIDs membership) is kept.
type forcedInteraction struct {
	interactionID string // models.Interaction.ID (model-local id namespace)
	initiatorID   uint64 // instance id of the agent that fired the action
}

// runtimeEvent is one emitted event live this step (and for TTLSteps after).
// EmitStep is the step it was emitted on; Source is the emitting agent id.
type runtimeEvent struct {
	EventUID string
	Source   uint64
	Position *common.Point
	Data     interface{}
	EmitStep uint64
}

// statsAccumulator holds run-history counters that are accumulated across ticks
// and cannot be derived from a single snapshot. It is allocated once in
// NewRuntimeState and shared by reference when the engine does a shallow copy of
// RuntimeState (e.g. workView := *rs), so the pointer must not be replaced.
type statsAccumulator struct {
	AgentsSpawned, AgentsRemoved, Interactions, Events, RuleExecutions uint64
	MovementDistance                                                   float64
	ByRule, ByEvent, ByInteraction                                     map[string]uint64
	PopSpawned, PopRemoved, PopPeak, PopMin                            map[string]uint64
	popMinSeen                                                         map[string]bool
}

func newStatsAccumulator() *statsAccumulator {
	return &statsAccumulator{
		ByRule: map[string]uint64{}, ByEvent: map[string]uint64{}, ByInteraction: map[string]uint64{},
		PopSpawned: map[string]uint64{}, PopRemoved: map[string]uint64{},
		PopPeak: map[string]uint64{}, PopMin: map[string]uint64{}, popMinSeen: map[string]bool{},
	}
}

// RuntimeState is the full in-memory state of an active run. It is NOT persisted
// directly; per-step snapshots flow to the timeseries via the step pipeline.
type RuntimeState struct {
	Sim    *datasims.Simulation
	Model  *models.Model // == Sim.ModelSnapshot (authoritative for rules/interactions/events)
	Env    *datasims.SimulationEnvironment
	Agents []*RuntimeAgent
	ByID   map[uint64]*RuntimeAgent
	Index  *SpatialIndex
	RNG    *rand.Rand // seeded from Sim.Seed (+ run/step for jitter)
	Step   uint64
	nextID uint64        // next instance id for spawns
	rules  []models.Rule // indexed/sorted once at run start
	runIdx int           // ExecutionRun.Seq-1 for token attribution

	// stats accumulates run-history counters across ticks (pointer so shallow
	// copies of RuntimeState share the same accumulator).
	stats *statsAccumulator
	// warnings collects non-fatal diagnostic messages emitted during a run.
	warnings []string

	// spawns/removals are queued during rule/interaction application and applied
	// at tick end (spawn/remove phase). events holds the emit-event queue visible
	// to ConditionTypeEvent within the step (and for TTLSteps thereafter).
	// forced holds force-fire interaction requests drained at the end of
	// ProcessInteractions the same tick. NOTE (synchronous mode): workView copies
	// the forced slice header and must NOT nil it — the copied header is how the
	// drain sees the rules-phase entries; drain-time appends extend only the
	// view's slice and are consumed by its index loop, so nothing folds back.
	spawns   []spawnRequest
	removals []uint64
	events   []runtimeEvent
	forced   []forcedInteraction

	// Read-only lookup maps built once in NewRuntimeState (never mutated after
	// build, so they are safe to read from the parallel LLM gather goroutines).
	// The consumers keep a linear-scan fallback for states built without
	// NewRuntimeState (test literals).
	eventDefs        map[string]*models.EventDefinition   // Model.Events by UID
	shapesByUID      map[string]*datasims.SimulationShape // rs.Env.Shapes, first instance per ShapeUID
	modelShapeDefs   map[string]*models.Shape             // Model.Environment.Shapes by UID
	recordableShapes map[string]bool                      // ShapeUID → emit per-step timeseries rows (built once; nil ⇒ none)
	interactionsByID map[string]*models.Interaction       // Model.Interactions by ID
	envPropDefs      map[string]*models.AgentProperty     // Model.Environment.GlobalProperties by UID

	// warned dedups warnOnce keys for the whole run. It is a reference type, so
	// the synchronous workView (a shallow copy of RuntimeState) shares it.
	warned map[string]struct{}

	// flow is the U10 flow-limiter budget state (per gated shape UID). A pointer
	// so the synchronous workView shares it like stats/warned; accrual happens
	// exactly once per tick because applySpatial runs once per tick in both
	// update modes. Runtime-only, never persisted (same acceptance as ra.rem).
	// Allocated by NewRuntimeState; hand-built synchronous test states must
	// pre-allocate (a lazy alloc on the workView copy is lost at commit).
	flow *flowRuntime

	// Global-property write deferral (synchronous mode). When deferGlobalWrites is
	// set, set/modify actions targeting the environment queue on globalWrites
	// instead of mutating Env.GlobalProperties, preserving frozen reads; the queue
	// drains serially in insertion order after the commit phase (set =
	// last-write-wins, modify = accumulate — matching sequential semantics).
	deferGlobalWrites bool
	globalWrites      []models.Action

	// envCooldowns / envOnceFired track CooldownSteps / ExecuteOnce for
	// environment-scoped rules (the synthetic env agent is rebuilt every step, so
	// this state must live on the run). Lazily allocated by runEnvironmentRules.
	envCooldowns map[string]uint64
	envOnceFired map[string]bool
}

// warnOnce appends a warning the first time the exact message is seen in this
// run; repeats are dropped (rs.warnings otherwise grows unboundedly for per-step
// conditions). Nil-safe for test-built states without a warned set.
func warnOnce(rs *RuntimeState, msg string) {
	if rs == nil {
		return
	}
	if rs.warned != nil {
		if _, seen := rs.warned[msg]; seen {
			return
		}
		rs.warned[msg] = struct{}{}
	}
	rs.warnings = append(rs.warnings, msg)
}

// NewRuntimeState builds the in-memory state from a ready Simulation. It:
//
//   - wraps every SimulationAgent in a RuntimeAgent, resolving its archetype from
//     the model snapshot (by UID) and its ExecutionStrategy via selectStrategy on
//     the resolved per-agent strategy (the archetype's own LLMConfig.Strategy when
//     set, else the model DefaultStrategy);
//   - indexes the agents by instance id and computes nextID as max(existing id)+1
//     so spawns never collide with an existing instance;
//   - seeds the run RNG deterministically from sim.Seed;
//   - pre-sorts the model rules by descending Priority once (the rule engine reads
//     rs.rules directly, avoiding a per-agent sort);
//   - builds the SpatialIndex from the initial positions for spatial models (cell
//     size from Model.SpatialCellSize), leaving it nil for non-spatial models.
//
// It returns an error only when the simulation has no model snapshot (the snapshot
// is the authoritative source for archetypes/rules/interactions).
func NewRuntimeState(sim *datasims.Simulation) (*RuntimeState, error) {
	if sim == nil {
		return nil, ErrNotRunnable
	}
	model := sim.ModelSnapshot
	if model == nil {
		return nil, ErrNotRunnable
	}

	// Index the model archetypes by UID for O(1) resolution.
	archByUID := make(map[string]*models.Agent, len(model.Agents))
	for i := range model.Agents {
		archByUID[model.Agents[i].UID] = &model.Agents[i]
	}

	rs := &RuntimeState{
		Sim:   sim,
		Model: model,
		// Run-owned deep copy of the environment: the run mutates global + shape
		// properties, and Sim.Environment must stay the pristine initial config
		// (so Duplicate/GET reflect the initial world, not the final run state).
		Env:    cloneEnvironment(sim.Environment),
		Agents: make([]*RuntimeAgent, 0, len(sim.Agents)),
		ByID:   make(map[uint64]*RuntimeAgent, len(sim.Agents)),
		RNG:    rand.New(rand.NewSource(int64(sim.Seed))),
		Step:   sim.CurrentStep,
		stats:  newStatsAccumulator(),
		warned: make(map[string]struct{}),
		flow:   &flowRuntime{budgets: make(map[string]float64)},
	}

	// Read-only lookup maps (see the field docs). Pointer targets are stable:
	// nothing appends to Model.* or Env.Shapes during a run (the shape phase
	// mutates shape Properties only).
	rs.eventDefs = make(map[string]*models.EventDefinition, len(model.Events))
	for i := range model.Events {
		rs.eventDefs[model.Events[i].UID] = &model.Events[i]
	}
	rs.interactionsByID = make(map[string]*models.Interaction, len(model.Interactions))
	for i := range model.Interactions {
		rs.interactionsByID[model.Interactions[i].ID] = &model.Interactions[i]
	}
	if model.Environment != nil {
		rs.modelShapeDefs = make(map[string]*models.Shape, len(model.Environment.Shapes))
		for i := range model.Environment.Shapes {
			rs.modelShapeDefs[model.Environment.Shapes[i].UID] = &model.Environment.Shapes[i]
		}
		rs.envPropDefs = make(map[string]*models.AgentProperty, len(model.Environment.GlobalProperties))
		for i := range model.Environment.GlobalProperties {
			rs.envPropDefs[model.Environment.GlobalProperties[i].UID] = &model.Environment.GlobalProperties[i]
		}
		rs.recordableShapes = make(map[string]bool, len(rs.modelShapeDefs))
		for uid, def := range rs.modelShapeDefs {
			rs.recordableShapes[uid] = shapeRecordable(def)
		}
	}
	// shapesByUID indexes the run-owned clone (rs.Env), not sim.Environment — the
	// shape phase / interactions mutate these, and the stored env must stay
	// pristine.
	if rs.Env != nil {
		rs.shapesByUID = make(map[string]*datasims.SimulationShape, len(rs.Env.Shapes))
		for i := range rs.Env.Shapes {
			sh := &rs.Env.Shapes[i]
			if _, seen := rs.shapesByUID[sh.ShapeUID]; !seen {
				rs.shapesByUID[sh.ShapeUID] = sh
			}
		}
	}

	var maxID uint64
	for i := range sim.Agents {
		// Run-owned deep copy: the step pipeline mutates Position/Properties/
		// State/Memory, and Sim.Agents must stay the pristine initial config so a
		// Duplicate copies the initial world (final state lives in FinalAgents /
		// the step timeseries).
		a := copySimulationAgent(&sim.Agents[i])
		arch := archByUID[a.AgentUID]
		strat := resolveArchetypeStrategy(model, arch)
		var agentType models.AgentType
		if arch != nil {
			agentType = arch.Type
		}
		ra := &RuntimeAgent{
			A:         a,
			Archetype: arch,
			Strategy:  selectStrategy(strat, agentType),
			Cooldowns: make(map[string]uint64),
			OnceFired: make(map[string]bool),
			memCap:    resolveMemoryCap(model, arch),
		}
		rs.Agents = append(rs.Agents, ra)
		rs.ByID[a.ID] = ra
		if a.ID > maxID {
			maxID = a.ID
		}
	}
	rs.nextID = maxID + 1

	// Pre-sort the model rules by descending priority (stable for equal priority).
	rs.rules = append([]models.Rule(nil), model.Rules...)
	sort.SliceStable(rs.rules, func(i, j int) bool { return rs.rules[i].Priority > rs.rules[j].Priority })

	// Build the spatial index for spatial models from the initial positions.
	if model.IsSpatial && rs.Env != nil {
		rs.Index = NewSpatialIndex(rs.Env.Width, rs.Env.Height, model.SpatialCellSize, rs.Env.Torus)
		rs.Index.Rebuild(rs.Agents)
	}

	return rs, nil
}
