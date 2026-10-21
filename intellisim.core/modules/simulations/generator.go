package simulations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math/rand"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
	"intelli.sim.core/modules/llm"
)

// llmInvoker is the narrow slice of llm.Service the generator depends on. The
// real llm.Service satisfies it; tests inject a fake. Keeping the surface small
// makes the orchestration deterministic and trivially testable.
type llmInvoker interface {
	Simple(ctx context.Context, req llm.SimpleRequest) (*llm.Completion, error)
	Reasoning(ctx context.Context, req llm.ReasoningRequest) (*llm.ReasoningCompletion, error)
	ModelDescriptors(providerUID, modelUID string) ([]llm.ConfigParamDescriptor, bool, bool)
	ValidateTarget(target common.LLMTarget, purpose common.ModelCapability) error
}

// Generator turns a frozen model snapshot into an initial simulation world. It
// orchestrates the same-package sampling / placement / mapping primitives plus an
// LLM, never reimplementing them. Generation is deterministic given Seed alone
// and identical LLM responses: the RNG always derives from (sim.Seed, a FIXED
// runSeq of 1) — see RunRNG in rng.go — so all randomness flows through one
// *rand.Rand and every order-sensitive loop iterates the snapshot's slices
// (never a map). runSeq is retained as a Generator-internal concept only for
// indexing GenerationRuns/token-usage bookkeeping (see Run below), never for
// seeding the RNG.
type Generator struct {
	llm llmInvoker
	cfg ModuleConfig
}

// NewGenerator builds a Generator over the narrow llm dependency and the module
// config (GenerationTimeout + LLMBatchSize).
func NewGenerator(invoker llmInvoker, cfg ModuleConfig) *Generator {
	return &Generator{llm: invoker, cfg: cfg}
}

// genRun carries the per-run mutable state so the stage methods don't have to
// thread it through every signature.
type genRun struct {
	g        *Generator
	ctx      context.Context
	sim      *datasims.Simulation
	snap     *models.Model
	rng      *rand.Rand
	runIdx   int
	warnings []datasims.GenerationWarning
	requests uint
	progress func(stage, detail string)
}

// Run executes one generation pass over sim, mutating sim.Agents / sim.Environment
// / sim.TokenUsage in place according to scope. It returns the aggregated
// non-fatal warnings, the number of LLM requests made, and an error only when the
// inputs are structurally unusable (nil sim / nil snapshot). All LLM failures are
// non-fatal: they degrade to deterministic-random fallbacks and record a warning.
func (g *Generator) Run(ctx context.Context, sim *datasims.Simulation, scope datasims.GenerationScope, progress func(stage, detail string)) ([]datasims.GenerationWarning, uint, error) {
	if sim == nil || sim.ModelSnapshot == nil {
		return nil, 0, errors.New("generator: missing model snapshot")
	}

	// runSeq/runIdx index this run's record within GenerationRuns (used for the
	// TokenUsage.ByGeneration slot and progress reporting) — this GROWS with every
	// regeneration and is unrelated to the RNG below.
	runSeq := uint64(len(sim.GenerationRuns))
	if runSeq == 0 {
		runSeq = 1
	}
	runIdx := int(runSeq) - 1

	if sim.TokenUsage == nil {
		sim.TokenUsage = &datasims.SimulationTokenUsage{}
	}

	gr := &genRun{
		g:    g,
		ctx:  ctx,
		sim:  sim,
		snap: sim.ModelSnapshot,
		// The generation RNG derives from sim.Seed ALONE (a FIXED runSeq of 1, never
		// GenerationRuns length): since C6 (ISS-03) every (re)generation rolls a fresh
		// sim.Seed before calling Run, so a full-scope regen that draws seed X produces
		// byte-identical output to CREATING a brand-new simulation with seed X — the
		// stored seed always faithfully describes the current world. See rng.go.
		rng:      RunRNG(sim.Seed, 1),
		runIdx:   runIdx,
		progress: progress,
	}

	doEnv := scope.Kind == datasims.ScopeFull || scope.Kind == datasims.ScopeEnvironment
	doAgents := scope.Kind == datasims.ScopeFull || scope.Kind == datasims.ScopeAgents

	if doEnv {
		gr.generateEnvironment()
	}
	if doAgents {
		gr.generateAgents(scope)
	}

	return gr.warnings, gr.requests, nil
}

// --- run-context helpers -------------------------------------------------------

func (gr *genRun) report(stage, detail string) {
	if gr.progress != nil {
		gr.progress(stage, detail)
	}
}

func (gr *genRun) warn(path, reason string) {
	gr.warnings = append(gr.warnings, datasims.GenerationWarning{Path: path, Reason: reason})
}

func (gr *genRun) addWarnings(w []datasims.GenerationWarning) {
	if len(w) > 0 {
		gr.warnings = append(gr.warnings, w...)
	}
}

// --- target resolution ---------------------------------------------------------

func (gr *genRun) resolveAgentTarget(agentLLM *models.LLMAgentConfig) common.LLMTarget {
	if ov := gr.sim.LLMGenerationTargetOverride; ov != nil && ov.ProviderUID != "" {
		return *ov
	}
	if agentLLM != nil && agentLLM.Target.ProviderUID != "" {
		return agentLLM.Target
	}
	if gr.snap.SimulationConfig.LLMSimulationGenerationConfig.Target.ProviderUID != "" {
		return gr.snap.SimulationConfig.LLMSimulationGenerationConfig.Target
	}
	return common.LLMTarget{}
}

func (gr *genRun) resolveModelTarget() common.LLMTarget {
	return gr.resolveAgentTarget(nil)
}

// --- LLM call ------------------------------------------------------------------

// callJSON issues one JSON-format LLM call against target, folding token usage and
// counting the request. It returns (text, true) on success and ("", false) when no
// target is configured or the provider call errored — the caller then degrades to
// a deterministic-random fallback. reasoningPreferred routes through Reasoning when
// the target model advertises reasoning support.
func (gr *genRun) callJSON(target common.LLMTarget, agentUID, system, user string, reasoningPreferred bool) (string, bool) {
	if target.ProviderUID == "" {
		return "", false
	}

	reasoning := false
	if reasoningPreferred {
		if _, r, ok := gr.g.llm.ModelDescriptors(target.ProviderUID, target.ModelUID); ok {
			reasoning = r
		}
	}

	callCtx := gr.ctx
	if target.TimeoutMs == nil {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(gr.ctx, gr.g.cfg.GenerationTimeout)
		defer cancel()
	}

	gr.requests++

	var (
		text  string
		usage common.TokenUsage
	)
	if reasoning {
		resp, err := gr.g.llm.Reasoning(callCtx, llm.ReasoningRequest{
			Target:         target,
			Purpose:        common.ModelCapabilitySimulationGeneration,
			Prompt:         user,
			SystemPrompt:   system,
			ResponseFormat: llm.ResponseFormatJSON,
		})
		if err != nil {
			gr.warn(warnPath(agentUID), fmt.Sprintf("llm reasoning call failed: %v; randomized", err))
			return "", false
		}
		text = resp.Analysis
		usage = resp.Usage
	} else {
		resp, err := gr.g.llm.Simple(callCtx, llm.SimpleRequest{
			Target:         target,
			Purpose:        common.ModelCapabilitySimulationGeneration,
			Message:        user,
			SystemPrompt:   system,
			ResponseFormat: llm.ResponseFormatJSON,
		})
		if err != nil {
			gr.warn(warnPath(agentUID), fmt.Sprintf("llm call failed: %v; randomized", err))
			return "", false
		}
		text = resp.Text
		usage = resp.Usage
	}

	gr.sim.TokenUsage.Add(usage, agentUID, datasims.LLMUsageSimulationGeneration, gr.runIdx)
	return text, true
}

func warnPath(agentUID string) string {
	if agentUID == "" {
		return "llm"
	}
	return "agents:" + agentUID
}

// --- environment stage ---------------------------------------------------------

func (gr *genRun) generateEnvironment() {
	if gr.snap.Environment == nil {
		gr.sim.Environment = nil
		return
	}

	gr.report("environment", "globals")
	env := &datasims.SimulationEnvironment{
		Width:            gr.snap.Environment.Width,
		Height:           gr.snap.Environment.Height,
		Torus:            gr.snap.Environment.Torus,
		CollisionMode:    gr.snap.Environment.CollisionMode,
		CollisionConfig:  cloneCollisionConfig(gr.snap.Environment.CollisionConfig),
		GlobalProperties: map[string]interface{}{},
	}
	gr.sim.Environment = env

	gr.generateGlobals(env)
	gr.generateShapes(env)
}

func (gr *genRun) generateGlobals(env *datasims.SimulationEnvironment) {
	var llmProps []models.AgentProperty
	for _, p := range gr.snap.Environment.GlobalProperties {
		if p.Generation != nil && p.Generation.Mode == models.GenModeLLM {
			llmProps = append(llmProps, p)
			continue
		}
		v, w := ResolveValue(gr.rng, p, false)
		env.GlobalProperties[p.UID] = v
		if w != nil {
			gr.addWarnings([]datasims.GenerationWarning{*w})
		}
	}

	if len(llmProps) == 0 {
		return
	}

	target := gr.resolveModelTarget()
	if target.ProviderUID == "" {
		gr.warn("environment.globals", "no llm target for global values; randomized")
		gr.fallbackGlobals(env, llmProps)
		return
	}

	sys, usr := buildGlobalValuesPrompt(*gr.snap, llmProps)
	txt, ok := gr.callJSON(target, "", sys, usr, false)
	if !ok {
		gr.fallbackGlobals(env, llmProps)
		return
	}

	var resp LLMGlobalValuesResponse
	if err := json.Unmarshal([]byte(txt), &resp); err != nil {
		gr.warn("environment.globals", "llm global values not valid JSON; randomized")
		gr.fallbackGlobals(env, llmProps)
		return
	}

	vals, w := MapGlobalValues(resp, llmProps, gr.rng)
	maps.Copy(env.GlobalProperties, vals)
	gr.addWarnings(w)
}

func (gr *genRun) fallbackGlobals(env *datasims.SimulationEnvironment, llmProps []models.AgentProperty) {
	for _, p := range llmProps {
		v, _ := ResolveValue(gr.rng, p, true)
		env.GlobalProperties[p.UID] = v
	}
}

func (gr *genRun) generateShapes(env *datasims.SimulationEnvironment) {
	if !gr.snap.IsSpatial || len(gr.snap.Environment.Shapes) == 0 {
		return
	}
	gr.report("environment", "shapes")

	var nextShapeID uint64 = 1
	for _, shape := range gr.snap.Environment.Shapes {
		count := ResolveCount(gr.rng, shape.Initialization)

		var positions []common.Point
		if shape.Geometry != nil && shape.Geometry.Position != nil {
			positions = []common.Point{*shape.Geometry.Position}
			count = 1
		} else {
			pts, w := PlaceAgents(gr.rng, count, shape.Initialization.Placement, env)
			positions = pts
			gr.addWarnings(w)
		}

		for i := uint64(0); i < count && i < uint64(len(positions)); i++ {
			ss := datasims.SimulationShape{
				ID:       nextShapeID,
				ShapeUID: shape.UID,
				Name:     shape.Name,
				Type:     shape.Type,
				Layer:    shape.Layer,
				Visible:  shape.Visible,
				Position: positions[i],
			}
			nextShapeID++
			if count > 1 {
				ss.Name = fmt.Sprintf("%s #%d", shape.Name, i+1)
			}

			if g := shape.Geometry; g != nil {
				if g.Width != nil {
					ss.Width = ptrU64(ResolveScalar(gr.rng, g.Width))
				}
				if g.Height != nil {
					ss.Height = ptrU64(ResolveScalar(gr.rng, g.Height))
				}
				if g.Radius != nil {
					ss.Radius = ptrF64(float64(ResolveScalar(gr.rng, g.Radius)))
				}
				if len(g.Points) > 0 {
					ss.Points = append([]common.Point(nil), g.Points...)
				}
				ss.Thickness = g.Thickness
				ss.Fill = g.Fill
				ss.Stroke = g.Stroke
				ss.StrokeWidth = g.StrokeWidth
			}

			ss.Properties = gr.resolveShapeProperties(shape.Properties)
			env.Shapes = append(env.Shapes, ss)
		}
	}
}

func (gr *genRun) resolveShapeProperties(props []models.AgentProperty) map[string]interface{} {
	out := make(map[string]interface{}, len(props))
	// Deduplicate structural warnings (same pattern as resolvePropertiesForInstances):
	// if a shape appears multiple times the same structural issue would fire once per
	// instance. Cap at one occurrence per (path, reason) per call.
	seenWarns := map[string]struct{}{}
	for _, p := range props {
		v, w := ResolveValue(gr.rng, p, false)
		out[p.UID] = v
		if w != nil {
			key := w.Path + "|" + w.Reason
			if _, seen := seenWarns[key]; !seen {
				seenWarns[key] = struct{}{}
				gr.addWarnings([]datasims.GenerationWarning{*w})
			}
		}
	}
	return out
}

// --- agents stage --------------------------------------------------------------

func (gr *genRun) generateAgents(scope datasims.GenerationScope) {
	resolvedCounts := map[string]uint64{}

	// Group existing instances by archetype UID (stable order via snap slice).
	perArchetype := map[string][]datasims.SimulationAgent{}
	for _, a := range gr.sim.Agents {
		perArchetype[a.AgentUID] = append(perArchetype[a.AgentUID], a)
	}

	// Determine the next free instance ID above the max across the WHOLE sim, so
	// freshly built instances never collide with kept ones.
	var nextID uint64
	for _, a := range gr.sim.Agents {
		if a.ID > nextID {
			nextID = a.ID
		}
	}
	nextID++

	result := make([]datasims.SimulationAgent, 0, len(gr.sim.Agents))

	for i := range gr.snap.Agents {
		archetype := gr.snap.Agents[i]
		uid := archetype.UID

		if !archetypeInScope(scope, uid) {
			result = append(result, perArchetype[uid]...)
			continue
		}

		existing := perArchetype[uid]

		// --- population ---
		var instances []datasims.SimulationAgent
		if partActive(scope, datasims.PartPopulation) {
			count := gr.resolveCountFor(archetype, resolvedCounts)
			resolvedCounts[uid] = count
			instances = make([]datasims.SimulationAgent, 0, count)
			for n := uint64(0); n < count; n++ {
				instances = append(instances, datasims.SimulationAgent{
					ID:       nextID,
					AgentUID: uid,
					Name:     fmt.Sprintf("%s #%d", archetype.Name, n+1),
					State:    archetype.Initialization.InitialState,
				})
				nextID++
			}
		} else {
			// Reuse existing instances in-place so the properties/placement sub-stages can mutate them without rebuilding the population (IDs and count stay stable).
			instances = existing
			resolvedCounts[uid] = uint64(len(instances))
		}

		// --- properties ---
		if partActive(scope, datasims.PartProperties) {
			gr.resolvePropertiesForInstances(archetype, instances)
		}

		// --- placement ---
		// "manual" placement is resolved per-simulation: the user positions each
		// instance in the world (simulation editor) before the run. Leave the
		// instances unplaced (Position == nil) here — validation blocks the run
		// until they are placed. Every other placement type resolves a position
		// now, so a null-position spatial agent uniquely means "awaits manual
		// placement".
		if gr.snap.IsSpatial && partActive(scope, datasims.PartPlacement) && !isManualPlacement(archetype) {
			positions := gr.placeForAgents(archetype, uint64(len(instances)))
			for k := range instances {
				if k < len(positions) {
					p := clampFootprintPoint(positions[k], archetype.Footprint, gr.sim.Environment)
					instances[k].Position = &p
				}
			}
		}

		result = append(result, instances...)
	}

	gr.sim.Agents = result
}

func (gr *genRun) resolveCountFor(archetype models.Agent, resolvedCounts map[string]uint64) uint64 {
	gr.report("agents:"+archetype.UID, "population")

	if archetype.Initialization.CountMode != models.CountLLM {
		return ResolveCount(gr.rng, archetype.Initialization)
	}

	target := gr.resolveAgentTarget(archetype.LLMConfig)
	if target.ProviderUID == "" {
		gr.warn("agents:"+archetype.UID, "no llm target for count; used algorithmic count")
		return ResolveCount(gr.rng, archetype.Initialization)
	}

	sys, usr := buildCountPrompt(*gr.snap, archetype, resolvedCounts)
	txt, ok := gr.callJSON(target, archetype.UID, sys, usr, true)
	if !ok {
		return ResolveCount(gr.rng, archetype.Initialization)
	}

	var resp LLMCountResponse
	if err := json.Unmarshal([]byte(txt), &resp); err != nil {
		gr.warn("agents:"+archetype.UID, "llm count not valid JSON; used algorithmic count")
		return ResolveCount(gr.rng, archetype.Initialization)
	}

	c, w := MapCount(resp, archetype.Initialization)
	if w != nil {
		gr.addWarnings([]datasims.GenerationWarning{*w})
	}
	return c
}

func (gr *genRun) resolvePropertiesForInstances(archetype models.Agent, instances []datasims.SimulationAgent) {
	var llmProps, others []models.AgentProperty
	for _, p := range archetype.Properties {
		if p.Generation != nil && p.Generation.Mode == models.GenModeLLM {
			llmProps = append(llmProps, p)
		} else {
			others = append(others, p)
		}
	}

	// Non-LLM props first, per instance.
	// Warnings from ResolveValue are archetype-level structural issues (e.g.
	// "string property has no allowed values") that repeat identically for every
	// instance. We deduplicate by (path, reason) so each distinct warning fires at
	// most once per resolvePropertiesForInstances call while still drawing a fresh
	// random value for every instance.
	seenWarns := map[string]struct{}{}
	for k := range instances {
		if instances[k].Properties == nil {
			instances[k].Properties = make(map[string]interface{}, len(archetype.Properties))
		}
		for _, p := range others {
			v, w := ResolveValue(gr.rng, p, false)
			instances[k].Properties[p.UID] = v
			if w != nil {
				key := w.Path + "|" + w.Reason
				if _, seen := seenWarns[key]; !seen {
					seenWarns[key] = struct{}{}
					gr.addWarnings([]datasims.GenerationWarning{*w})
				}
			}
		}
	}

	if len(llmProps) == 0 || len(instances) == 0 {
		return
	}

	target := gr.resolveAgentTarget(archetype.LLMConfig)
	if target.ProviderUID == "" {
		gr.warn("agents:"+archetype.UID, "no llm target for property values; randomized")
		for k := range instances {
			for _, p := range llmProps {
				v, _ := ResolveValue(gr.rng, p, true)
				instances[k].Properties[p.UID] = v
			}
		}
		return
	}

	batchSize := gr.g.cfg.LLMBatchSize
	if batchSize < 1 {
		batchSize = 40
	}

	total := len(instances)
	batches := (total + batchSize - 1) / batchSize
	for start := 0; start < total; start += batchSize {
		end := start + batchSize
		if end > total {
			end = total
		}
		batchLen := end - start
		gr.report("agents:"+archetype.UID, fmt.Sprintf("properties batch %d/%d", start/batchSize+1, batches))

		ctxSummary := fmt.Sprintf("population %d; %s", total, gr.envSummaryLine())
		sys, usr := buildAgentBatchPrompt(*gr.snap, archetype, llmProps, start, batchLen, ctxSummary)

		var resp LLMAgentBatchResponse
		txt, ok := gr.callJSON(target, archetype.UID, sys, usr, false)
		if ok {
			if err := json.Unmarshal([]byte(txt), &resp); err != nil {
				gr.warn("agents:"+archetype.UID, "llm property batch not valid JSON; randomized")
				resp = LLMAgentBatchResponse{}
			}
		} else {
			resp = LLMAgentBatchResponse{}
		}

		maps, w := MapAgentBatch(resp, llmProps, batchLen, gr.rng)
		gr.addWarnings(w)
		for k, m := range maps {
			for _, p := range llmProps {
				instances[start+k].Properties[p.UID] = m[p.UID]
			}
		}
	}
}

// isManualPlacement reports whether an archetype uses manual placement, i.e. its
// instances are positioned by hand per-simulation before the run rather than
// resolved algorithmically at generation time.
func isManualPlacement(archetype models.Agent) bool {
	pl := archetype.Initialization.Placement
	return pl != nil && pl.Type == models.PlacementManual
}

func (gr *genRun) placeForAgents(archetype models.Agent, count uint64) []common.Point {
	gr.report("agents:"+archetype.UID, "placement")

	pl := archetype.Initialization.Placement

	if pl != nil && pl.Type == models.PlacementLLM {
		target := gr.resolveAgentTarget(archetype.LLMConfig)
		if target.ProviderUID == "" {
			gr.warn("agents:"+archetype.UID, "no llm target for placement; random")
			return gr.placeRandomFallback(count)
		}

		ctxSummary := fmt.Sprintf("population %d", count)
		sys, usr := buildPlacementPrompt(*gr.snap, archetype, count, gr.sim.Environment, ctxSummary)
		txt, ok := gr.callJSON(target, archetype.UID, sys, usr, true)
		if !ok {
			return gr.placeRandomFallback(count)
		}

		var resp LLMPlacementResponse
		if err := json.Unmarshal([]byte(txt), &resp); err != nil {
			gr.warn("agents:"+archetype.UID, "llm placement not valid JSON; random")
			return gr.placeRandomFallback(count)
		}

		dirs, w := MapDirectives(resp, gr.sim.Environment, count)
		gr.addWarnings(w)
		if len(dirs) == 0 {
			gr.warn("agents:"+archetype.UID, "no usable placement directives; random")
			return gr.placeRandomFallback(count)
		}
		return SampleDirectives(gr.rng, count, dirs, gr.sim.Environment)
	}

	positions, w := PlaceAgents(gr.rng, count, pl, gr.sim.Environment)
	gr.addWarnings(w)
	return positions
}

func (gr *genRun) placeRandomFallback(count uint64) []common.Point {
	positions, w := PlaceAgents(gr.rng, count, &models.Placement{Type: models.PlacementRandom}, gr.sim.Environment)
	gr.addWarnings(w)
	return positions
}

// envSummaryLine returns a brief one-liner describing the environment for prompt
// context, or "non-spatial" when there is none.
func (gr *genRun) envSummaryLine() string {
	if gr.sim.Environment == nil {
		return "non-spatial"
	}
	return fmt.Sprintf("world %dx%d", gr.sim.Environment.Width, gr.sim.Environment.Height)
}

// --- scope predicates ----------------------------------------------------------

func archetypeInScope(scope datasims.GenerationScope, uid string) bool {
	if scope.Kind == datasims.ScopeFull {
		return true
	}
	if scope.Kind != datasims.ScopeAgents {
		return false
	}
	if len(scope.AgentUIDs) == 0 {
		return true
	}
	return containsUID(scope.AgentUIDs, uid)
}

func partActive(scope datasims.GenerationScope, part datasims.GenerationPart) bool {
	if scope.Kind == datasims.ScopeFull {
		return true
	}
	if scope.Kind != datasims.ScopeAgents {
		return false
	}
	return len(scope.Parts) == 0 || containsPart(scope.Parts, part)
}

func containsPart(parts []datasims.GenerationPart, part datasims.GenerationPart) bool {
	for _, p := range parts {
		if p == part {
			return true
		}
	}
	return false
}

func containsUID(uids []string, uid string) bool {
	for _, u := range uids {
		if u == uid {
			return true
		}
	}
	return false
}

// --- pointer helpers -----------------------------------------------------------

func ptrU64(v uint64) *uint64   { return &v }
func ptrF64(v float64) *float64 { return &v }
