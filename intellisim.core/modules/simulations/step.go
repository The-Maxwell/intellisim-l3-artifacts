package simulations

import (
	"context"
	"math"
	"sort"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// RunStep executes one simulation tick and returns the per-entity step rows for
// it (one SimulationStep per live agent + one per environment when global
// properties exist). It honors rs.Model.SimulationConfig.UpdateMode:
//
//   - synchronous: agents read a FROZEN snapshot of step t. The rules /
//     interactions / spatial phases mutate per-agent working copies while every
//     neighbour/condition read resolves against the frozen live agents; the
//     working copies are committed back atomically at tick end (order-independent,
//     deterministic).
//   - sequential: phases mutate live agents in place, so a later agent/rule sees
//     earlier writes within the same tick (priority/order dependent).
//
// Phases (article "Модель виконання симуляції"): perceive (rebuild the spatial
// index + neighbours) → LLM (group by archetype, strategy.Plan, gather intents) →
// rules (ApplyRules with the agent's LLM-owned purposes suppressed) → interactions
// (ProcessInteractions) → spatial (resolve every MovementIntent to a velocity via
// the shared resolver, then IntegrateMovement with torus/bounds) → spawn/remove
// (drain the rs queues) → memory (movement/decision/interaction/property events are
// appended by the rules/interactions phases via AppendMemory) → stats
// (ComputeStatistics) → build the SimulationStep documents. It increments rs.Step.
//
// A nil llmRT runs the rules-only path (no LLM): every strategy degrades to
// rule-driven behaviour. RunStep never aborts on an LLM failure — the LLM phase
// fails soft to rules per the fallback chain.
func RunStep(ctx context.Context, rs *RuntimeState, llmRT *llmRuntime) (datasims.SimulationStep, error) {
	steps, err := RunStepRows(ctx, rs, llmRT)
	if err != nil {
		return datasims.SimulationStep{}, err
	}
	if len(steps) == 0 {
		return datasims.SimulationStep{}, nil
	}
	return steps[0], nil
}

// RunStepRows is the full per-step pipeline returning every entity row for the
// completed tick (agents first in instance-id order, then the environment row when
// present). RunStep returns the first row for callers that only need a single
// document; the run loop persists the whole slice.
func RunStepRows(ctx context.Context, rs *RuntimeState, llmRT *llmRuntime) ([]datasims.SimulationStep, error) {
	if rs == nil || rs.Model == nil {
		return nil, ErrNotRunnable
	}

	// Reset the per-step spawn/removal/forced-interaction queues and drop events
	// whose TTL has elapsed; surviving events remain perceivable by
	// ConditionTypeEvent this tick.
	rs.spawns = rs.spawns[:0]
	rs.removals = rs.removals[:0]
	rs.forced = rs.forced[:0]
	rs.events = pruneEvents(rs, rs.events, rs.Step)

	// 1. Perceive — (re)build the spatial index from the current positions so the
	//    rules/interactions phases get correct neighbour queries this tick.
	if rs.Index != nil {
		rs.Index.Rebuild(rs.Agents)
	}

	// 1b. Shape-LLM phase — before the agent LLM phase, every runtime shape whose
	//     model shape carries a non-none LLMShapeConfig gets one LLM call (grouped
	//     per shape-UID in first-seen deterministic order) that returns property
	//     updates applied to rs.Env.Shapes[i].Properties. Running here — after the
	//     perceive/index rebuild and before the agent LLM phase — means agents
	//     perceive the updated shape state this tick. Because the phase runs once on
	//     the real rs before any agent clones are made, and groups are iterated in
	//     fixed order, writes are deterministic without touching the clone/commit
	//     cycle. A nil llmRT (rules-only path) skips the phase entirely; errors
	//     degrade only the affected group (fail-soft). Interaction-driven shape
	//     mutation keeps its existing semantics and is out of scope for this phase.
	runShapePhase(ctx, rs, llmRT)

	// 2. LLM phase — group by archetype and let each group's strategy plan its
	//    intents (no-op for the rules-only / nil-llm path). Intents are gathered
	//    per instance id for the rules + spatial phases.
	intents := runLLMPhase(ctx, rs, llmRT)

	// 2b. Stamp this step's LLM decision summaries onto the live agents (transient
	//     RuntimeAgent.stepDecision) BEFORE the mutation phases clone them, so the
	//     summary rides the synchronous clone/commit cycle untouched and lands on
	//     this step's rows (SimulationStep.Decision → WS frame + replay DTO).
	recordStepDecisions(rs, intents)

	// 3-5. Rules → interactions → spatial. In synchronous mode these run against
	//      per-agent working copies (frozen neighbour reads) and commit at the end;
	//      in sequential mode they mutate the live agents in place.
	synchronous := rs.Model.SimulationConfig.UpdateMode == models.UpdateModeSynchronous
	if synchronous {
		runMutationPhasesSynchronous(rs, llmRT, intents)
	} else {
		runMutationPhasesSequential(rs, llmRT, intents)
	}

	// 5b. Environment rules — once-per-step pass that evaluates env-scoped rules
	//     (AppliesTo == "environment"), emits events, and writes global properties
	//     via runEnvironmentRules. Runs AFTER the agent mutation phases (agents have
	//     already applied their rules/intents this tick) and BEFORE spawn/remove so
	//     emitted events are live in rs.events for the NEXT tick's prune/perceive
	//     cycle and global-property writes land in THIS tick's env step row. The
	//     environment is a singleton evaluated on the real rs (no working copy).
	runEnvironmentRules(rs)

	// 6. Spawn / remove — drain the queues populated during the mutation phases.
	//    Spawned agents only participate from the next tick (legacy ordering).
	//    applySpawns queues the source agent of every successful transform spawn
	//    onto rs.removals (ActionTypeTransform replaces, not duplicates), so a
	//    second applyRemovals drains those through the normal removal path.
	applyRemovals(rs)
	applySpawns(rs)
	applyRemovals(rs)
	// After spawn/remove the live set is final for this tick; sample per-archetype
	// population counts into the accumulator for Peak/Min tracking.
	accumulatePopulation(rs)

	// 7. Memory — movement/decision/interaction/property events were appended by
	//    the rules/interactions phases (ApplyAction → AppendMemory) as they fired,
	//    so the memory log already reflects this tick. Nothing further to do.

	// 8. Per-entity step documents for the completed tick. ComputeStatistics over
	//    the same post-tick state is the run loop's concern (the SimulationStep rows
	//    carry only per-entity snapshots); the loop reads rs after RunStep returns.
	stepNo := rs.Step
	rows := buildStepRows(rs, stepNo)

	// A decision is a per-step event, not state: clear the transient summaries
	// AFTER this step's rows are built so a between-step keyframe (BroadcastKeyframe
	// calls buildStepRows with the already-incremented step number) never re-stamps
	// a past decision onto the next step.
	clearStepDecisions(rs)

	rs.Step++
	return rows, nil
}

// runLLMPhase groups the live agents by archetype and runs each group's resolved
// ExecutionStrategy in parallel (bounded by resolveParallelWorkers), merging the
// per-group intents into one id-keyed map. A nil llmRT (rules-only path) skips
// the LLM entirely.
//
// Concurrency design (Task 13):
//   - Each group's strategy.Plan runs in its own eg.Go goroutine; the errgroup
//     SetLimit caps the number of concurrent goroutines.
//   - Loop variables are captured explicitly (i, g := i, g) before the closure.
//   - Each goroutine writes only to results[i] (pre-sized, disjoint by index) — no
//     shared-map writes inside goroutines.
//   - After eg.Wait() returns the intents map is populated SERIALLY — the only map
//     write is on the caller's goroutine, race-free.
//   - A strategy error degrades only its group (fail-soft, return nil from closure
//     so sibling goroutines are never cancelled), mirroring the sequential behaviour.
//   - token-fold races are prevented by llmRuntime.mu (wraps every AddRun call).
func runLLMPhase(ctx context.Context, rs *RuntimeState, llmRT *llmRuntime) map[uint64]*AgentIntent {
	intents := map[uint64]*AgentIntent{}
	if llmRT == nil {
		return intents
	}
	groups := groupByArchetype(rs)
	if len(groups) == 0 {
		return intents
	}

	workers := resolveParallelWorkers(rs, llmRT.cfg)
	sc := &StepContext{RS: rs, LLM: llmRT, Cfg: llmRT.cfg, Ctx: ctx}

	// Pre-size the results slice so each goroutine writes into its own slot (no
	// shared-map writes from inside goroutines).
	results := make([]PlanResult, len(groups))

	eg := new(errgroup.Group)
	eg.SetLimit(workers)

	for i, g := range groups {
		if g.Strategy.Type == "" || g.Strategy.Type == models.LLMStrategyNone {
			continue
		}
		// Capture loop variables so the closure doesn't close over the loop variable
		// that changes on the next iteration.
		i, g := i, g
		eg.Go(func() error {
			var agentType models.AgentType
			if g.Archetype != nil {
				agentType = g.Archetype.Type
			}
			strategy := selectStrategy(g.Strategy, agentType)
			res, err := strategy.Plan(sc, g)
			if err != nil {
				// Fail-soft: log a warning and leave results[i] as the zero value
				// (empty Intents map). This group degrades to rules-only for this
				// step; sibling groups are not affected (return nil, not err).
				llmRT.logger.Warn("strategy plan failed; degrading group to rules",
					zap.String("archetype", g.ArchetypeUID),
				)
				return nil
			}
			results[i] = res
			return nil
		})
	}

	// Wait for all goroutines. We always return nil from closures, so this never
	// propagates an error; the _ discard documents the intent explicitly.
	_ = eg.Wait()

	// Serial merge after Wait — the shared intents map is only ever written on the
	// calling goroutine (no concurrent writes, no race).
	for _, res := range results {
		for id, in := range res.Intents {
			if in == nil {
				continue
			}
			intents[id] = in
		}
	}
	return intents
}

// resolveParallelWorkers returns the effective LLM phase concurrency for this
// step: model SimulationConfig.LLMSimulationExecutionConfig.ParallelWorkers (when > 0),
// else RunConfig.DefaultParallelWorkers (env-driven, default 4), floored to 1.
//
// There is no per-sim config override (Task 11); the three-level resolution
// hierarchy stops at the model level for ParallelWorkers.
func resolveParallelWorkers(rs *RuntimeState, cfg RunConfig) int {
	if rs != nil && rs.Model != nil {
		if w := rs.Model.SimulationConfig.LLMSimulationExecutionConfig.ParallelWorkers; w > 0 {
			return int(w)
		}
	}
	if cfg.DefaultParallelWorkers > 1 {
		return cfg.DefaultParallelWorkers
	}
	return 1
}

// groupByArchetype builds one AgentGroup per archetype from rs.Agents, members
// sharing the archetype's resolved Strategy (its own LLMConfig.Strategy when set,
// else the model DefaultStrategy) and resolved LLM Target. Removed agents are
// excluded; groups are returned in first-seen archetype order so the LLM phase is
// deterministic.
func groupByArchetype(rs *RuntimeState) []*AgentGroup {
	if rs == nil {
		return nil
	}
	order := make([]string, 0)
	byUID := make(map[string]*AgentGroup)
	for _, ra := range rs.Agents {
		if ra == nil || ra.A == nil || ra.removed {
			continue
		}
		uid := ra.A.AgentUID
		g, ok := byUID[uid]
		if !ok {
			strat := resolveArchetypeStrategy(rs.Model, ra.Archetype)
			target, _ := resolveTarget(rs.Model, rs.Sim, ra.Archetype)
			g = &AgentGroup{
				ArchetypeUID: uid,
				Archetype:    ra.Archetype,
				Strategy:     strat,
				Target:       target,
			}
			byUID[uid] = g
			order = append(order, uid)
		}
		g.Members = append(g.Members, ra)
	}
	out := make([]*AgentGroup, 0, len(order))
	for _, uid := range order {
		out = append(out, byUID[uid])
	}
	return out
}

// ownedPurposesFor returns the LLM purposes the agent's strategy owns (so the rule
// engine suppresses conflicting rule actions for them). Only the per-purpose
// UseFor* flags of a non-none strategy contribute; a rules-only (none) agent owns
// nothing and the rules drive everything.
func ownedPurposesFor(ra *RuntimeAgent) purposeSet {
	if ra == nil || ra.Archetype == nil {
		return nil
	}
	strat := ra.Archetype.LLMConfig
	var s models.LLMStrategy
	if strat != nil {
		s = strat.Strategy.LLMStrategy
	}
	if s.Type == "" || s.Type == models.LLMStrategyNone {
		return nil
	}
	owned := purposeSet{}
	if s.UseForMovement {
		owned[purposeMovement] = true
	}
	if s.UseForProperties {
		owned[purposeProperties] = true
	}
	if s.UseForDecisions {
		owned[purposeDecision] = true
	}
	if s.UseForInteraction {
		owned[purposeInteraction] = true
	}
	if len(owned) == 0 {
		return nil
	}
	return owned
}

// --- sequential mutation phases -------------------------------------------------

// runMutationPhasesSequential runs rules → intent-actions → interactions → spatial
// directly on the live agents, so writes are visible to later agents/rules within
// the same tick.
func runMutationPhasesSequential(rs *RuntimeState, llmRT *llmRuntime, intents map[uint64]*AgentIntent) {
	for _, ra := range rs.Agents {
		if ra == nil || ra.A == nil || ra.removed {
			continue
		}
		ApplyRules(rs, ra, intents[ra.A.ID], ownedPurposesFor(ra))
		// Apply full-control LLM intent actions (Actions + Properties + LLMNote)
		// immediately after rules so their writes are visible to later phases within
		// the same tick (sequential semantics). ApplyIntentActions suppresses actions
		// for non-owned purposes so rules and intents remain complementary.
		ApplyIntentActions(rs, ra, intents[ra.A.ID])
	}
	ProcessInteractions(rs, llmRT, intents)
	applySpatial(rs, rs.Agents, intents)
}

// --- synchronous mutation phases ------------------------------------------------

// runMutationPhasesSynchronous runs rules → interactions → spatial against per-agent
// working copies while neighbour/condition reads resolve against the FROZEN live
// agents (rs.Agents / rs.ByID / rs.Index), then commits the working copies back
// atomically. This yields order-independent, reproducible updates: no agent ever
// observes another agent's mid-tick write.
func runMutationPhasesSynchronous(rs *RuntimeState, llmRT *llmRuntime, intents map[uint64]*AgentIntent) {
	// Defer global-property writes for the whole mutation window so every
	// condition/prompt read resolves against the frozen Env.GlobalProperties map.
	// The queued writes drain serially after the commit phase below.
	rs.deferGlobalWrites = true

	working := make([]*RuntimeAgent, 0, len(rs.Agents))
	workingByID := make(map[uint64]*RuntimeAgent, len(rs.Agents))
	for _, ra := range rs.Agents {
		if ra == nil || ra.A == nil || ra.removed {
			continue
		}
		w := cloneRuntimeAgent(ra)
		working = append(working, w)
		workingByID[ra.A.ID] = w
	}

	// Rules + intent-actions: mutate working copies while neighbour/condition reads
	// resolve against the frozen rs (rs.Agents / rs.ByID / rs.Index, none of which
	// change here). Spawn/remove/event side effects from both phases queue on the
	// real rs. ApplyIntentActions absorbs the Task 7 LLMNote flush loop and suppresses
	// intent actions for non-owned purposes so rules and intents remain complementary.
	for _, w := range working {
		ApplyRules(rs, w, intents[w.A.ID], ownedPurposesFor(w))
		ApplyIntentActions(rs, w, intents[w.A.ID])
	}

	// Interactions + spatial run over a working view of the state so their mutations
	// land on the working copies; membership/neighbour discovery reads the working
	// positions (still the unchanged t-positions — movement integration has not
	// happened yet, so reads remain frozen). The working view shares no queue slices
	// with rs, so its spawn/remove/event side effects are folded back below.
	workView := *rs
	workView.Agents = working
	workView.ByID = workingByID
	workView.spawns = nil
	workView.removals = nil
	workView.events = nil
	// warnings must be nil'd like the other queues: appends to the copied slice
	// header would be invisible to rs once the backing array reallocates, silently
	// dropping interactions/spatial-phase warnings. Folded back below. The warnOnce
	// dedup set (rs.warned) is a shared reference, so dedup spans both views.
	workView.warnings = nil
	// globalWrites queued by the interactions/spatial phases land on the view's own
	// queue (deferGlobalWrites was copied as true) and are folded back below.
	workView.globalWrites = nil
	if rs.Index != nil {
		idx := NewSpatialIndex(indexDims(rs))
		idx.Rebuild(working)
		workView.Index = idx
	}
	ProcessInteractions(&workView, llmRT, intents)
	applySpatial(&workView, working, intents)

	// Fold the working view's queued side effects back onto the real rs (the rules
	// phase already queued onto rs directly).
	rs.spawns = append(rs.spawns, workView.spawns...)
	rs.removals = append(rs.removals, workView.removals...)
	rs.events = append(rs.events, workView.events...)
	rs.warnings = append(rs.warnings, workView.warnings...)
	rs.globalWrites = append(rs.globalWrites, workView.globalWrites...)

	// Commit: copy each working copy's mutated state back onto its live agent.
	for _, w := range working {
		live := rs.ByID[w.A.ID]
		if live == nil {
			continue
		}
		commitRuntimeAgent(live, w)
	}

	// Drain the deferred global-property writes serially, in insertion order
	// (rules/intents queued on rs first, then the folded interactions/spatial
	// writes). set = last-write-wins, modify = accumulate — matching what
	// sequential mode would have produced for the same queue order.
	rs.deferGlobalWrites = false
	applyGlobalWrites(rs)
}

// indexDims returns the SpatialIndex constructor arguments matching rs's index
// (world dims + cell + torus). It is only called when rs.Index is non-nil.
func indexDims(rs *RuntimeState) (uint64, uint64, uint64, bool) {
	w, h, torus := envDims(rs)
	var cell uint64
	if rs.Model != nil {
		cell = rs.Model.SpatialCellSize
	}
	return w, h, cell, torus
}

// cloneRuntimeAgent makes a working copy of a runtime agent with a deep-copied
// persistent state (Position, Properties, State, Memory) and copied transient
// cooldown/once maps, sharing the immutable Archetype/Strategy. Mutations to the
// clone are invisible to the original until commitRuntimeAgent runs.
func cloneRuntimeAgent(ra *RuntimeAgent) *RuntimeAgent {
	a := *ra.A
	if ra.A.Position != nil {
		p := *ra.A.Position
		a.Position = &p
	}
	a.Properties = copyProps(ra.A.Properties)
	a.Memory = append([]datasims.AgentMemory(nil), ra.A.Memory...)

	w := &RuntimeAgent{
		A:              &a,
		Archetype:      ra.Archetype,
		Strategy:       ra.Strategy,
		Velocity:       ra.Velocity,
		rem:            ra.rem,
		Cooldowns:      copyU64Map(ra.Cooldowns),
		OnceFired:      copyBoolMap(ra.OnceFired),
		LastDecision:   ra.LastDecision,
		memCap:         ra.memCap,
		removed:        ra.removed,
		llmCallPending: ra.llmCallPending,
		stepDecision:   ra.stepDecision,
	}
	return w
}

// commitRuntimeAgent copies a working copy's mutated state back onto the live
// agent at tick end (synchronous commit).
func commitRuntimeAgent(live, w *RuntimeAgent) {
	live.A.Position = w.A.Position
	live.A.Properties = w.A.Properties
	live.A.State = w.A.State
	live.A.Memory = w.A.Memory
	live.Velocity = w.Velocity
	live.rem = w.rem
	live.Cooldowns = w.Cooldowns
	live.OnceFired = w.OnceFired
	live.LastDecision = w.LastDecision
	live.removed = w.removed
	live.llmCallPending = w.llmCallPending
	live.stepDecision = w.stepDecision
}

func copyProps(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return nil
	}
	out := make(map[string]interface{}, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func copyU64Map(src map[string]uint64) map[string]uint64 {
	out := make(map[string]uint64, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func copyBoolMap(src map[string]bool) map[string]bool {
	out := make(map[string]bool, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

// --- spatial phase --------------------------------------------------------------

// applySpatial resolves every agent's movement to a velocity and integrates it into
// the agent's position (torus wrap / bounds clamp). Agents driven by a MovementIntent
// (individual LLM intents or archetype directives) have their velocity resolved via
// the shared resolveMovement helper so individual and archetype movement behave
// identically; agents with no fresh intent keep their prior Velocity (natural reuse
// between DecisionFrequency calls). Static (immovable) agents never move. A movement
// memory event is recorded for any agent that actually changed position.
func applySpatial(rs *RuntimeState, agents []*RuntimeAgent, intents map[uint64]*AgentIntent) {
	w, h, torus := envDims(rs)
	// U10 congestion: resolve the (opt-in) configuration and, when speed–density
	// coupling is on, precompute per-agent factors from the FROZEN pre-integration
	// positions (post-rules — identical in sequential and synchronous modes).
	cong := congestionSettingsFor(rs)
	var factors map[uint64]float64
	if cong.density != nil {
		factors = densityFactors(rs, agents, cong.density)
	}
	// Capture each positioned agent's pre-phase position so movement-distance/memory
	// accounting reflects the FINAL position (after any block-mode collision resolution).
	// With flow limits active, the pre-phase sub-cell remainder is captured too, so a
	// held (denied) agent can be reverted without a stale remainder.
	before := make(map[uint64]common.Point, len(agents))
	var beforeRem map[uint64]Vec2
	if len(cong.limits) > 0 {
		beforeRem = make(map[uint64]Vec2, len(agents))
	}
	for _, ra := range agents {
		if ra == nil || ra.A == nil || ra.removed || ra.A.Position == nil {
			continue
		}
		before[ra.A.ID] = *ra.A.Position
		if beforeRem != nil {
			beforeRem[ra.A.ID] = ra.rem
		}
		if !movable(ra) {
			continue
		}
		if in := intents[ra.A.ID]; in != nil && in.Movement != nil {
			if in.Movement.Resolved != nil {
				// Archetype variation pre-resolved a per-agent jittered velocity (heading
				// + speed jitter): integrate it verbatim. (Re-resolving here would discard
				// the heading jitter — the bug this seam fixes.)
				ra.Velocity = *in.Movement.Resolved
			} else {
				// Individual strategies leave Resolved nil: resolve the intent as before.
				ra.Velocity = resolveMovement(rs, ra, *in.Movement)
			}
		}
		if ra.Velocity == (Vector{}) {
			continue
		}
		// The density factor scales a LOCAL copy of the velocity for this step's
		// integration only. ra.Velocity (sticky across steps) is never mutated —
		// mutating it would compound the slowdown geometrically across no-intent
		// steps and corrupt an agent that later stops receiving intents.
		vel := ra.Velocity
		if f, ok := factors[ra.A.ID]; ok {
			vel = Vector{DX: vel.DX * f, DY: vel.DY * f}
		}
		next, rem := integrateMovementBodyRem(*ra.A.Position, ra.rem, vel, bodyForAgent(ra), w, h, torus)
		ra.A.Position = &next
		ra.rem = rem
	}

	// Collision-response (block mode): resolve overlaps deterministically before accounting.
	if rs.Env != nil && rs.Env.CollisionMode == models.CollisionBlock {
		// Capture every agent's integrated position (its own integration result above,
		// or its unchanged pre-phase position). Any axis the collision layer corrects
		// invalidates that axis's sub-cell remainder (U4): the remainder must never
		// disagree with a collision-corrected integer position.
		integrated := make(map[uint64]common.Point, len(agents))
		for _, ra := range agents {
			if ra == nil || ra.A == nil || ra.removed || ra.A.Position == nil {
				continue
			}
			integrated[ra.A.ID] = *ra.A.Position
		}
		resolveCollisions(rs, agents, before)
		for _, ra := range agents {
			if ra == nil || ra.A == nil || ra.removed || ra.A.Position == nil {
				continue
			}
			ipos, ok := integrated[ra.A.ID]
			if !ok {
				continue
			}
			if ra.A.Position.X != ipos.X {
				ra.rem.X = 0
			}
			if ra.A.Position.Y != ipos.Y {
				ra.rem.Y = 0
			}
		}
	}

	// U10 flow-limit gate. Runs AFTER collision response — gate shapes are passable
	// (unfilled), so crowd push-out can legally move an agent into the gate, and
	// gating final positions closes the "pushed through unpaid" admission leak — and
	// before movement accounting so a held agent records zero movement. Because rules
	// (phase 3) read previous-tick-end positions, a `position in` remove rule only
	// ever fires for agents this gate admitted.
	if len(cong.limits) > 0 {
		applyFlowLimits(rs, agents, before, beforeRem, cong.limits)
	}

	// Record movement distance + a "moved" memory event for any agent whose position
	// changed this phase (self-move or push). In none mode only self-movers change, so
	// this is identical to the legacy per-mover accounting.
	for _, ra := range agents {
		if ra == nil || ra.A == nil || ra.removed || ra.A.Position == nil {
			continue
		}
		bpos, ok := before[ra.A.ID]
		if !ok || *ra.A.Position == bpos {
			continue
		}
		if rs.stats != nil {
			rs.stats.MovementDistance += Distance(bpos, *ra.A.Position, w, h, torus)
		}
		AppendMemory(ra, rs.Step, datasims.AgentMemoryMovement, "moved", nil, ra.memoryCap(), nowFn())
	}
}

// --- spawn / remove phase -------------------------------------------------------

// applyRemovals drops every agent queued for removal this tick from rs.Agents and
// rs.ByID, accumulating the removed ids so a later spawn never reuses one.
func applyRemovals(rs *RuntimeState) {
	if len(rs.removals) == 0 {
		return
	}
	remove := make(map[uint64]bool, len(rs.removals))
	for _, id := range rs.removals {
		remove[id] = true
	}
	kept := rs.Agents[:0]
	for _, ra := range rs.Agents {
		if ra == nil || ra.A == nil {
			continue
		}
		if remove[ra.A.ID] {
			if rs.stats != nil {
				rs.stats.AgentsRemoved++
				rs.stats.PopRemoved[ra.A.AgentUID]++
			}
			delete(rs.ByID, ra.A.ID)
			if rs.Index != nil {
				rs.Index.Remove(ra.A.ID)
			}
			continue
		}
		kept = append(kept, ra)
	}
	rs.Agents = kept
	rs.removals = rs.removals[:0]
}

// applySpawns materialises every queued spawn into a new live agent. Spawned agents
// receive a fresh instance id (rs.nextID++), the spawned archetype's resolved
// strategy, the parent's position (when the parent is known and the archetype is
// spatial), and the explicit/inherited property overrides. They only participate
// from the NEXT tick (they are appended after this tick's phases have run).
//
// Transform requests (req.transform, ActionTypeTransform — "turn the agent into
// archetype X") additionally REPLACE their source: once the replacement is
// materialised (with the source still live in ByID so spawn_position "same" and
// spawn_inherit_from resolve against it), the source agent is queued on
// rs.removals and drained by the applyRemovals call that follows applySpawns in
// RunStepRows — the same removal path as remove_agent, so stats/index/step rows
// see a normal removal. Edge cases (both deterministic — queue insertion order):
//   - source already removed this tick (another rule/interaction, or an earlier
//     transform of the same agent won): the transform is moot — the spawn is
//     skipped so a dead agent cannot leave a live replacement behind.
//   - spawn fails (unknown target archetype): the ORIGINAL agent stays alive —
//     safer than removing it with no replacement.
func applySpawns(rs *RuntimeState) {
	if len(rs.spawns) == 0 {
		return
	}
	for _, req := range rs.spawns {
		arch := archetypeByUID(rs.Model, req.archetypeUID)
		if arch == nil {
			continue
		}
		var src *RuntimeAgent
		if req.transform {
			src = rs.ByID[req.parentID]
			if src == nil || src.A == nil || src.removed {
				continue // source already gone this tick — nothing to transform
			}
		}
		for i := 0; i < req.count; i++ {
			id := rs.nextID
			rs.nextID++
			ra := newSpawnedAgent(rs, arch, req, id)
			rs.Agents = append(rs.Agents, ra)
			rs.ByID[id] = ra
			if rs.Index != nil && ra.A.Position != nil {
				rs.Index.Insert(id, *ra.A.Position)
			}
			if rs.stats != nil {
				rs.stats.AgentsSpawned++
				rs.stats.PopSpawned[arch.UID]++
			}
		}
		if req.transform {
			// Marking removed here (not just queueing) dedups a repeated transform
			// of the same source later in this drain.
			src.removed = true
			rs.removals = append(rs.removals, src.A.ID)
		}
	}
	rs.spawns = rs.spawns[:0]
}

// newSpawnedAgent builds a runtime agent for one spawn request: position resolved
// via resolveSpawnPosition (honoring req.position's SpawnPositionType — same,
// random, offset, nearby, or at_shape), properties seeded from the archetype
// defaults then overlaid with inherited parent values then explicit overrides, and
// the archetype's initial state.
func newSpawnedAgent(rs *RuntimeState, arch *models.Agent, req spawnRequest, id uint64) *RuntimeAgent {
	props := defaultProps(arch)

	if parent := rs.ByID[req.parentID]; parent != nil && parent.A != nil {
		for _, uid := range req.inheritFrom {
			if v, ok := parent.A.Properties[uid]; ok {
				props[uid] = v
			}
		}
	}
	for k, v := range req.properties {
		props[k] = v
	}

	state := arch.Initialization.InitialState

	a := &datasims.SimulationAgent{
		ID:         id,
		AgentUID:   arch.UID,
		Properties: props,
		State:      state,
	}
	if arch.MoveType == models.AgentMoveTypeMovable || rs.Model.IsSpatial {
		a.Position = resolveSpawnPosition(rs, req)
	}

	strat := resolveArchetypeStrategy(rs.Model, arch)
	return &RuntimeAgent{
		A:         a,
		Archetype: arch,
		Strategy:  selectStrategy(strat, arch.Type),
		Cooldowns: make(map[string]uint64),
		OnceFired: make(map[string]bool),
		memCap:    resolveMemoryCap(rs.Model, arch),
	}
}

// resolveSpawnPosition computes a spawned agent's position from the placement
// directive carried in req.position, defaulting to the parent's position when the
// directive is absent or of type SpawnPositionSame. The spatial guard (arch.MoveType
// or rs.Model.IsSpatial) is enforced by the caller; this function is only invoked
// when a position should be assigned.
//
// Type semantics:
//   - same (default): copy parent position (nil when parent is absent).
//   - random: uniform random point inside the world.
//   - offset: parent + (OffsetX, OffsetY), clamped/wrapped.
//   - nearby: uniform random point within Radius of parent, clamped/wrapped.
//   - at_shape: uniform random point inside the named shape; degrades to parent when
//     the shape is not found.
func resolveSpawnPosition(rs *RuntimeState, req spawnRequest) *common.Point {
	// Resolve parent position once; nil when parent is unknown or positionless.
	var parent *common.Point
	if p := rs.ByID[req.parentID]; p != nil && p.A != nil && p.A.Position != nil {
		pp := *p.A.Position
		parent = &pp
	}

	sp := req.position
	typ := models.SpawnPositionSame
	if sp != nil && sp.Type != "" {
		typ = sp.Type
	}

	switch typ {
	case models.SpawnPositionRandom:
		if rs.Env == nil {
			return parent
		}
		pt := randomPoint(rs.RNG, rs.Env)
		return &pt

	case models.SpawnPositionOffset:
		if parent == nil {
			return nil
		}
		dx, dy := 0.0, 0.0
		if sp.OffsetX != nil {
			dx = *sp.OffsetX
		}
		if sp.OffsetY != nil {
			dy = *sp.OffsetY
		}
		return clampedPoint(rs, float64(parent.X)+dx, float64(parent.Y)+dy)

	case models.SpawnPositionNearby:
		if parent == nil {
			return nil
		}
		radius := 0.0
		if sp.Radius != nil {
			radius = *sp.Radius
		}
		ang := rs.RNG.Float64() * 2 * math.Pi
		rad := rs.RNG.Float64() * radius
		return clampedPoint(rs, float64(parent.X)+rad*math.Cos(ang), float64(parent.Y)+rad*math.Sin(ang))

	case models.SpawnPositionAtShape:
		if sh := findShapeByUID(rs, sp.ShapeUID); sh != nil {
			if rs.Env != nil {
				pt := sampleInShape(rs.RNG, *sh, rs.Env)
				return &pt
			}
		}
		return parent // degrade: shape not found or no env

	default: // SpawnPositionSame and any unrecognized type
		return parent
	}
}

// findShapeByUID scans the environment shapes for the first shape whose ShapeUID
// matches uid. Returns nil when the environment has no shapes or none match.
func findShapeByUID(rs *RuntimeState, uid string) *datasims.SimulationShape {
	if rs.Env == nil || uid == "" {
		return nil
	}
	if rs.shapesByUID != nil {
		return rs.shapesByUID[uid]
	}
	// Linear fallback for states built without NewRuntimeState (test literals).
	for i := range rs.Env.Shapes {
		if rs.Env.Shapes[i].ShapeUID == uid {
			return &rs.Env.Shapes[i]
		}
	}
	return nil
}

// defaultProps seeds a fresh property map from an archetype's declared default
// values (skipping properties without a default).
func defaultProps(arch *models.Agent) map[string]interface{} {
	out := make(map[string]interface{}, len(arch.Properties))
	for _, p := range arch.Properties {
		if p.DefaultValue != nil {
			out[p.UID] = p.DefaultValue
		}
	}
	return out
}

// archetypeByUID resolves an archetype definition from the model snapshot by UID.
func archetypeByUID(model *models.Model, uid string) *models.Agent {
	if model == nil {
		return nil
	}
	for i := range model.Agents {
		if model.Agents[i].UID == uid {
			return &model.Agents[i]
		}
	}
	return nil
}

// accumulatePopulation samples the live agent count per archetype UID after the
// spawn/remove phase and updates the statsAccumulator's Peak and Min maps. Peak is
// updated whenever the current count exceeds the stored maximum. Min is lazy-init
// via popMinSeen so the first observation sets Min rather than competing with a
// stale zero from a tick before any agents of that archetype existed.
//
// accumulatePopulation is a no-op when rs.stats is nil (backward-compat).
func accumulatePopulation(rs *RuntimeState) {
	if rs == nil || rs.stats == nil {
		return
	}
	// Count live agents per archetype UID.
	counts := make(map[string]uint64, 4)
	for _, ra := range rs.Agents {
		if ra == nil || ra.A == nil || ra.removed {
			continue
		}
		counts[ra.A.AgentUID]++
	}
	for uid, cnt := range counts {
		// Peak: update when the current count exceeds the stored maximum.
		if cnt > rs.stats.PopPeak[uid] {
			rs.stats.PopPeak[uid] = cnt
		}
		// Min: lazy-init on the first observation for this archetype, then update
		// whenever the count is lower than the stored minimum.
		if !rs.stats.popMinSeen[uid] {
			rs.stats.PopMin[uid] = cnt
			rs.stats.popMinSeen[uid] = true
		} else if cnt < rs.stats.PopMin[uid] {
			rs.stats.PopMin[uid] = cnt
		}
	}
}

// pruneEvents drops events whose TTL has elapsed (no longer perceivable at step),
// using the model's EventDefinition.TTLSteps (events with no definition live only on
// their emit step). It compacts in place so the surviving events keep their order.
func pruneEvents(rs *RuntimeState, events []runtimeEvent, step uint64) []runtimeEvent {
	if len(events) == 0 {
		return events[:0]
	}
	kept := events[:0]
	for _, ev := range events {
		def := eventDefByUID(rs, ev.EventUID)
		if eventActive(def, &ev, step) {
			kept = append(kept, ev)
		}
	}
	return kept
}

// --- step document assembly -----------------------------------------------------

// shapeRecordable reports whether a shape definition can change during a run
// and therefore earns per-step timeseries rows: it declares at least one
// property AND (any property is mutable, or a non-none LLM shape strategy may
// rewrite them). Static decorative shapes are never recorded.
func shapeRecordable(def *models.Shape) bool {
	if def == nil || len(def.Properties) == 0 {
		return false
	}
	for i := range def.Properties {
		if !def.Properties[i].Immutable {
			return true
		}
	}
	return def.LLMConfig != nil &&
		def.LLMConfig.Strategy.Type != "" &&
		def.LLMConfig.Strategy.Type != models.LLMStrategyNone
}

// buildStepRows snapshots the post-tick state into per-entity SimulationStep rows:
// one row per live agent (id-sorted for determinism) carrying its position +
// properties + state (State is omitted for the environment row), plus one row per
// recordable shape instance (id-sorted) carrying its position + properties, plus one
// environment row carrying the global properties (when present) AND the per-step
// accumulated statistics snapshot. The environment row is emitted on EVERY step so
// replay can rebuild statistics-driven widgets (the text_log "events" kind) with the
// same numbers the run accumulated live; the live WS path ignores this row's stats
// and drops it when it carries no global properties, so the stream is unaffected.
func buildStepRows(rs *RuntimeState, step uint64) []datasims.SimulationStep {
	now := nowFn()
	simID := ""
	if rs.Sim != nil {
		simID = rs.Sim.ID
	}

	live := make([]*RuntimeAgent, 0, len(rs.Agents))
	for _, ra := range rs.Agents {
		if ra == nil || ra.A == nil || ra.removed {
			continue
		}
		live = append(live, ra)
	}
	sort.SliceStable(live, func(i, j int) bool { return live[i].A.ID < live[j].A.ID })

	rows := make([]datasims.SimulationStep, 0, len(live)+1)
	for _, ra := range live {
		var pos *common.Point
		if ra.A.Position != nil {
			p := *ra.A.Position
			pos = &p
		}
		rows = append(rows, datasims.SimulationStep{
			Metadata: datasims.StepMetadata{
				SimulationID: simID,
				Step:         int64(step),
				ReferenceID:  ra.A.ID,
				Type:         datasims.MetadataTypeAgent,
				UID:          ra.A.AgentUID,
			},
			Timestamp:  now,
			Position:   pos,
			Properties: copyProps(ra.A.Properties),
			State:      ra.A.State,
			Decision:   ra.stepDecision,
		})
	}

	if rs.Env != nil && len(rs.recordableShapes) > 0 {
		recShapes := make([]*datasims.SimulationShape, 0, len(rs.Env.Shapes))
		for i := range rs.Env.Shapes {
			sh := &rs.Env.Shapes[i]
			if !rs.recordableShapes[sh.ShapeUID] {
				continue
			}
			recShapes = append(recShapes, sh)
		}
		sort.SliceStable(recShapes, func(i, j int) bool { return recShapes[i].ID < recShapes[j].ID })
		for _, sh := range recShapes {
			pos := sh.Position
			rows = append(rows, datasims.SimulationStep{
				Metadata: datasims.StepMetadata{
					SimulationID: simID,
					Step:         int64(step),
					ReferenceID:  sh.ID,
					Type:         datasims.MetadataTypeShape,
					UID:          sh.ShapeUID,
				},
				Timestamp:  now,
				Position:   &pos,
				Properties: copyProps(sh.Properties),
			})
		}
	}

	// Environment/stats row — emitted every step. It carries the accumulated
	// statistics snapshot (for faithful replay of statistics-driven widgets) and
	// the global properties when the simulation declares any.
	var globalProps map[string]interface{}
	if rs.Env != nil && len(rs.Env.GlobalProperties) > 0 {
		globalProps = copyProps(rs.Env.GlobalProperties)
	}
	rows = append(rows, datasims.SimulationStep{
		Metadata: datasims.StepMetadata{
			SimulationID: simID,
			Step:         int64(step),
			ReferenceID:  0,
			Type:         datasims.MetadataTypeEnvironment,
			UID:          models.EnvironmentUID,
		},
		Timestamp:  now,
		Properties: globalProps,
		Statistics: ComputeStatistics(rs),
	})

	return rows
}
