package simulations

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
	"intelli.sim.core/modules/llm"
)

// defaultRunCallTimeout is the per-LLM-call ceiling used when neither the target
// nor the runtime supplies one. It mirrors the create/edit generation default
// (SIMULATIONS_GENERATION_TIMEOUT envDefault 300s) so a run never hangs on a
// stalled provider.
const defaultRunCallTimeout = 300 * time.Second

// runSystemHeader frames every Level-2 EXECUTION call (agent purposes,
// interaction mediation, shape phase) — the model is CONTROLLING a running
// simulation, not initializing one (the generation prompts keep systemHeader).
// The literal "Output STRICT JSON only" substring is asserted by tests and
// relied on by the providers to suppress prose/markdown.
const runSystemHeader = "You control agents in a RUNNING agent-based simulation, deciding one step of its execution. " +
	"Ground every choice in the provided state, memory, perception and guidance. " +
	"Output STRICT JSON only, matching the given schema exactly — no prose, no markdown."

// llmRuntime is the run-phase LLM orchestrator. It reuses the package-level
// llmInvoker interface (declared in generator.go) and folds per-call usage into
// the run's SimulationTokenUsage via AddRun. callJSON picks Reasoning()/Simple()
// per the target's advertised reasoning flag, sets the core-controlled JSON
// response format + the call purpose, applies an ordered fallback chain, and
// degrades to ok=false (caller falls back to rules) on exhaustion / malformed
// JSON — it never panics or aborts the run.
type llmRuntime struct {
	inv    llmInvoker
	cfg    RunConfig
	usage  *datasims.SimulationTokenUsage
	runIdx int
	logger *zap.Logger

	// mu guards usage.AddRun so parallel Plan goroutines (Task 13) never race on
	// the shared SimulationTokenUsage accumulator (which grows slices and is NOT
	// concurrency-safe).
	mu sync.Mutex

	// genTimeout is the per-call timeout ceiling applied when neither the
	// resolved target nor the run-level simCfg supplies one (sourced from
	// SIMULATIONS_GENERATION_TIMEOUT at wiring time; defaults to
	// defaultRunCallTimeout when zero).
	genTimeout time.Duration

	// simCfg is the run-level execution config (model SimulationConfig.LLMSimulationExecutionConfig).
	// It carries RetryTimeoutMs and MaxRetries used as fallback defaults when the
	// resolved target has no TimeoutMs and when the fallback policy has no retry step.
	simCfg *models.LLMSimulationExecutionConfig
}

// newLLMRuntime builds a run-phase LLM orchestrator. usage must be non-nil
// (the run's accumulator); a nil logger is replaced with a no-op.
// simCfg is the model's SimulationConfig.LLMSimulationExecutionConfig (may be nil); it
// supplies the run-level RetryTimeoutMs and MaxRetries fallback defaults.
func newLLMRuntime(inv llmInvoker, cfg RunConfig, usage *datasims.SimulationTokenUsage, runIdx int, genTimeout time.Duration, simCfg *models.LLMSimulationExecutionConfig, logger *zap.Logger) *llmRuntime {
	if logger == nil {
		logger = zap.NewNop()
	}
	if genTimeout <= 0 {
		genTimeout = defaultRunCallTimeout
	}
	return &llmRuntime{
		inv:        inv,
		cfg:        cfg,
		usage:      usage,
		runIdx:     runIdx,
		logger:     logger,
		genTimeout: genTimeout,
		simCfg:     simCfg,
	}
}

// --- engine-owned LLM response contracts (own set; distinct from the generator's) ---

// decisionResp is the response shape for a decision/decision-batch call.
// Task 17: each agent entry may include an optional emit_uid + emit_data to
// request emitting a model-defined event alongside the chosen label. The emit
// is authorized via authorizeEmit(def, EventProducerLLM, archetypeUID) in
// demuxDecisionsWithState before it is appended to AgentIntent.Emits.
type decisionResp struct {
	Agents []struct {
		Instance     int            `json:"instance"`      // individual batches
		ArchetypeUID string         `json:"archetype_uid"` // archetype batches
		Label        string         `json:"label"`
		Params       map[string]any `json:"params"`
		EmitUID      string         `json:"emit_uid"`  // optional: event UID to emit
		EmitData     any            `json:"emit_data"` // optional: event payload
	} `json:"agents"`
}

// propsResp is the response shape for a property-update call.
type propsResp struct {
	Agents []struct {
		Instance     int            `json:"instance"`
		ArchetypeUID string         `json:"archetype_uid"`
		Values       map[string]any `json:"values"`
	} `json:"agents"`
}

// movementResp is the response shape for a movement call.
type movementResp struct {
	Agents []struct {
		Instance     int     `json:"instance"`
		ArchetypeUID string  `json:"archetype_uid"`
		Intent       string  `json:"intent"`     // toward|away|wander|hold
		TargetRef    string  `json:"target_ref"` // agentUID|shapeUID|"x,y"
		Speed        float64 `json:"speed"`      // 0..1
	} `json:"agents"`
}

// interactionResp is the response shape for a UseLLM-interaction mediation call.
type interactionResp struct {
	Fire   bool           `json:"fire"`
	Params map[string]any `json:"params"`
}

// --- target resolution ---------------------------------------------------------

// resolveTarget merges Agent.LLMConfig.Target over Sim.LLMExecutionTargetOverride over
// Model.SimulationConfig.LLMSimulationExecutionConfig.Target, field by field: a non-empty ProviderUID/ModelUID/Effort
// or a non-nil Generation/TimeoutMs at a more specific level overrides the value
// inherited from the lower-precedence level. This mirrors the generator's
// resolveAgentTarget precedence (agent > sim > model) but performs a field-wise
// merge so an agent may override only Effort/Generation/TimeoutMs while inheriting
// the provider/model from the model default.
//
// Returns ok=false (caller degrades to rule-based for the affected agents) when
// the merged target has no ProviderUID+ModelUID.
func resolveTarget(model *models.Model, sim *datasims.Simulation, agent *models.Agent) (common.LLMTarget, bool) {
	var merged common.LLMTarget

	// Lowest precedence first; each higher level overlays its non-empty fields.
	if model != nil {
		overlayTarget(&merged, model.SimulationConfig.LLMSimulationExecutionConfig.Target)
	}
	if sim != nil && sim.LLMExecutionTargetOverride != nil {
		overlayTarget(&merged, *sim.LLMExecutionTargetOverride)
	}
	if agent != nil && agent.LLMConfig != nil {
		overlayTarget(&merged, agent.LLMConfig.Target)
	}

	if merged.ProviderUID == "" || merged.ModelUID == "" {
		return common.LLMTarget{}, false
	}
	return merged, true
}

// overlayTarget copies src's non-empty / non-nil fields over dst, leaving dst's
// inherited values intact where src is unset.
func overlayTarget(dst *common.LLMTarget, src common.LLMTarget) {
	if src.ProviderUID != "" {
		dst.ProviderUID = src.ProviderUID
	}
	if src.ModelUID != "" {
		dst.ModelUID = src.ModelUID
	}
	if src.Effort != "" {
		dst.Effort = src.Effort
	}
	if src.Generation != nil {
		dst.Generation = src.Generation
	}
	if src.TimeoutMs != nil {
		dst.TimeoutMs = src.TimeoutMs
	}
}

// --- prompt assembly ------------------------------------------------------------

// buildContext assembles the mandatory-hint chain plus the driving agents' memory
// summaries into a compact prompt-context block. The chain order is fixed:
// model LLMConfig.PromptContext → archetype LLMConfig.PromptContext / strategy
// prompt contexts → recent agent memory summaries. The PromptContext of the entity
// driving the call is always included (mandatory hints). Returns "" when nothing
// is worth saying.
func buildContext(rs *RuntimeState, group *AgentGroup, members []*RuntimeAgent) string {
	var parts []string

	if rs != nil && rs.Model != nil {
		parts = append(parts, strings.TrimSpace(rs.Model.SimulationConfig.LLMSimulationExecutionConfig.PromptContext))
	}

	if group != nil && group.Archetype != nil && group.Archetype.LLMConfig != nil {
		alc := group.Archetype.LLMConfig
		parts = append(parts, strings.TrimSpace(alc.PromptContext))
		// Population/count + placement guidance lives on AgentInitialization
		// (Placement.PromptContext, used by buildCountPrompt/buildPlacementPrompt) —
		// a generation-time-only hint that must NOT leak into per-step runtime calls.
	}

	chain := guidanceChain(parts...)

	var b strings.Builder
	writeGuidance(&b, chain)

	// Per-driving-agent memory summaries (recent slice, summarized newest-first).
	memBudget := defaultMemoryCap
	for _, ra := range members {
		if ra == nil || ra.A == nil {
			continue
		}
		if cap := ra.memoryCap(); cap > 0 {
			memBudget = cap
		}
		summary := SummarizeMemory(ra, memBudget)
		if summary == "" {
			continue
		}
		fmt.Fprintf(&b, "\nRecent memory of agent %d (%s):\n%s\n", ra.A.ID, ra.A.AgentUID, summary)
	}

	// Prior LLM context feedback block (article "контроль контексту"): the most
	// recent llm_response + rule memory for each driving agent. Bounded to 5
	// entries so the prompt does not grow unboundedly. Skipped when empty.
	const priorLLMContextLimit = 5
	for _, ra := range members {
		if ra == nil || ra.A == nil {
			continue
		}
		prior := SummarizeMemoryByType(ra, []datasims.AgentMemoryType{datasims.AgentMemoryLLM, datasims.AgentMemoryRule}, priorLLMContextLimit)
		if prior == "" {
			continue
		}
		fmt.Fprintf(&b, "\nPrior LLM context for agent %d:\n%s\n", ra.A.ID, prior)
	}

	// Perception block (article "найближче оточення"): for each driving agent in
	// a spatial model that has a position and a non-zero perceptionRadius, gather
	// neighbour agents within that radius, sort them by ID for byte-stable prompts,
	// cap at perceptionNeighbourCap, and render a compact "Nearby:" listing showing
	// each neighbour's archetype UID, relative position (dx,dy), and state.
	// Skipped entirely for non-spatial models, agents without a position, radius==0,
	// or when no neighbours are found within the radius.
	const perceptionNeighbourCap = 10
	if rs != nil && rs.Model != nil && rs.Model.IsSpatial {
		for _, ra := range members {
			if ra == nil || ra.A == nil || ra.A.Position == nil {
				continue
			}
			pr := resolvedPerceptionRadius(rs, ra)
			if pr <= 0 {
				continue
			}
			neighbours := neighborsWithin(rs, ra, pr, nil)
			if len(neighbours) == 0 {
				continue
			}
			// Sort deterministically by neighbour agent ID.
			sort.Slice(neighbours, func(i, j int) bool {
				return neighbours[i].A.ID < neighbours[j].A.ID
			})
			// Cap to avoid over-filling the prompt context window.
			if len(neighbours) > perceptionNeighbourCap {
				neighbours = neighbours[:perceptionNeighbourCap]
			}
			b.WriteString("\nNearby:")
			for _, nb := range neighbours {
				archUID := nb.A.AgentUID
				dx := int64(nb.A.Position.X) - int64(ra.A.Position.X)
				dy := int64(nb.A.Position.Y) - int64(ra.A.Position.Y)
				fmt.Fprintf(&b, "\n  %s @ (%+d,%+d) state=%s", archUID, dx, dy, nb.A.State)
			}
			b.WriteByte('\n')
		}
	}

	return strings.TrimSpace(b.String())
}

// worldShapeCap bounds the per-prompt shape listing in the shared world block;
// instances beyond the cap collapse into one "+N more" line.
const worldShapeCap = 12

// buildSharedRuntimeBlocks renders the per-call shared context every Level-2
// execution prompt starts with: the step line (temporal awareness), the world
// summary with the shape instances the LLM may reference as movement targets
// (spatial models only), and the CURRENT environment global values (the LLM
// may read them and, where allowed, write them via target_uid "environment").
// It deliberately lives OUTSIDE buildContext: buildContext runs once per
// archetype group in a batched archetype call and would duplicate these.
// Deterministic: env slice order for shapes, key-sorted globals.
func buildSharedRuntimeBlocks(rs *RuntimeState) string {
	if rs == nil {
		return ""
	}
	var b strings.Builder

	if rs.Sim != nil && rs.Sim.Steps > 0 {
		fmt.Fprintf(&b, "Step %d of %d.\n", rs.Step, rs.Sim.Steps)
	} else {
		fmt.Fprintf(&b, "Step %d.\n", rs.Step)
	}

	if rs.Model != nil && rs.Model.IsSpatial && rs.Env != nil {
		torus := ""
		if rs.Env.Torus {
			torus = ", toroidal (wrap-around) edges"
		}
		collide := ""
		if rs.Env.CollisionMode == models.CollisionBlock {
			collide = ", solid bodies (agents cannot overlap shapes/each other)"
		}
		fmt.Fprintf(&b, "World: %d×%d%s%s.\n", rs.Env.Width, rs.Env.Height, torus, collide)
		if n := len(rs.Env.Shapes); n > 0 {
			b.WriteString("Shapes (usable as movement target_ref by uid):\n")
			for i := range rs.Env.Shapes {
				if i == worldShapeCap {
					fmt.Fprintf(&b, "- … and %d more\n", n-worldShapeCap)
					break
				}
				s := rs.Env.Shapes[i]
				fmt.Fprintf(&b, "- %s (uid=%s, %s) center=(%d,%d)%s\n",
					s.Name, s.ShapeUID, s.Type, s.Position.X, s.Position.Y, shapeSizeText(s))
			}
		}
	}

	if rs.Env != nil && len(rs.Env.GlobalProperties) > 0 {
		keys := make([]string, 0, len(rs.Env.GlobalProperties))
		for k := range rs.Env.GlobalProperties {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b.WriteString("Environment globals:")
		for _, k := range keys {
			fmt.Fprintf(&b, " %s=%v", k, rs.Env.GlobalProperties[k])
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// --- LLM call helper ------------------------------------------------------------

// callJSON runs one purpose call against target and decodes the strict-JSON
// response into out. It:
//   - picks Reasoning() when the target advertises reasoning (ModelDescriptors),
//     else Simple();
//   - sets the call Purpose to capability (validated provider-side) and the
//     core-controlled JSON response format;
//   - appends the OpenAI Responses "json"-in-user-message reminder
//     (userJSONReminder, reused from the generator path);
//   - applies the ordered fallback chain on error (retry with MaxRetries +
//     RetryDelayMs → rules_only/last_decision/skip → ok=false);
//   - bounds each call with a per-call timeout (target.TimeoutMs else the run's
//     genTimeout ceiling);
//   - folds token usage into usage.AddRun on every successful provider call,
//     attributed to usagePurpose (the integration sub-purpose:
//     movement/properties/decision/interaction).
//
// capability (the provider-validated model capability — full→SimulationExecution,
// decision→SimulationDecisions) is kept SEPARATE from usagePurpose (the
// token-attribution dimension) so spend is bucketed per sub-purpose regardless of
// the validated capability.
//
// It returns ok=false (caller degrades to rules for those agents) on an empty
// target, exhausted fallback, or malformed JSON — never panicking and never
// aborting the run.
func (r *llmRuntime) callJSON(ctx context.Context, target common.LLMTarget, capability common.ModelCapability,
	usagePurpose datasims.LLMUsagePurpose, agentUID, system, user string, fallback *models.FallbackPolicy, out any) (ok bool) {

	if target.ProviderUID == "" || target.ModelUID == "" {
		return false
	}

	userMsg := user + userJSONReminder

	// Resolve the retry budget + post-retry stages from the fallback policy.
	// If the policy has no FallbackRetry step, fall back to the run-level MaxRetries
	// from simCfg (model DefaultSimulationConfig → no per-sim override exists).
	policyRetries, retryDelay, stages := resolveFallback(fallback)
	maxRetries := resolveRunMaxRetries(r.simCfg, policyRetries, hasFallbackRetryStep(fallback))

	// Resolve the per-call timeout once for all attempts in this invocation.
	callTimeout := resolveCallTimeout(target, r.simCfg, r.genTimeout)

	attempt := func() (string, bool) {
		return r.invokeOnce(ctx, target, capability, agentUID, system, userMsg, usagePurpose, callTimeout)
	}

	// Initial attempt + retries.
	totalAttempts := int(maxRetries) + 1
	if totalAttempts < 1 {
		totalAttempts = 1
	}
	for i := 0; i < totalAttempts; i++ {
		if i > 0 && retryDelay > 0 {
			select {
			case <-ctx.Done():
				return false
			case <-time.After(retryDelay):
			}
		}
		text, called := attempt()
		if !called {
			continue // provider error; retry/exhaust
		}
		if err := json.Unmarshal([]byte(text), out); err != nil {
			r.logger.Warn("simulation llm response not valid JSON",
				zap.String("agent_uid", agentUID), zap.Error(err))
			// Malformed JSON is not retriable in a meaningful way here; treat as a
			// failed stage and fall through to the post-retry fallback stages.
			break
		}
		return true
	}

	// Retries exhausted (or malformed JSON): walk the post-retry fallback stages.
	for _, st := range stages {
		switch st.Type {
		case models.FallbackRulesOnly, models.FallbackSkip, models.FallbackLastDecision:
			// All of these mean "no fresh LLM intent this step" from callJSON's
			// perspective: the caller decides what to do (run rules / replay the
			// last decision / skip). Returning ok=false signals "no intent".
			return false
		}
	}
	return false
}

// invokeOnce issues a single provider call (Reasoning when advertised, else
// Simple), folds usage on success, and returns (text, true) or ("", false) on a
// provider error. callTimeout is the pre-resolved per-call deadline (from
// resolveCallTimeout in callJSON); it is always applied as context.WithTimeout so
// every call is bounded regardless of whether target.TimeoutMs was set.
func (r *llmRuntime) invokeOnce(ctx context.Context, target common.LLMTarget, capability common.ModelCapability,
	agentUID, system, user string, purposeKind datasims.LLMUsagePurpose, callTimeout time.Duration) (string, bool) {

	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()

	reasoning := false
	if _, adv, ok := r.inv.ModelDescriptors(target.ProviderUID, target.ModelUID); ok {
		reasoning = adv
	}

	var (
		text  string
		usage common.TokenUsage
	)
	if reasoning {
		resp, err := r.inv.Reasoning(callCtx, llm.ReasoningRequest{
			Target:         target,
			Purpose:        capability,
			Prompt:         user,
			SystemPrompt:   system,
			ResponseFormat: llm.ResponseFormatJSON,
		})
		if err != nil {
			r.logger.Warn("simulation reasoning call failed",
				zap.String("agent_uid", agentUID), zap.Error(err))
			return "", false
		}
		text = resp.Analysis
		usage = resp.Usage
	} else {
		resp, err := r.inv.Simple(callCtx, llm.SimpleRequest{
			Target:         target,
			Purpose:        capability,
			Message:        user,
			SystemPrompt:   system,
			ResponseFormat: llm.ResponseFormatJSON,
		})
		if err != nil {
			r.logger.Warn("simulation simple call failed",
				zap.String("agent_uid", agentUID), zap.Error(err))
			return "", false
		}
		text = resp.Text
		usage = resp.Usage
	}

	if r.usage != nil {
		// AddRun grows slices and is NOT concurrency-safe; the mutex ensures that
		// parallel Plan goroutines (Task 13) never race on the shared accumulator.
		r.mu.Lock()
		r.usage.AddRun(usage, agentUID, purposeKind, r.runIdx)
		r.mu.Unlock()
	}
	return text, true
}

// resolveFallback splits a fallback policy into the leading retry budget (max
// retries + delay, taken from the first FallbackRetry step) and the remaining
// post-retry stages in order. A nil/empty policy yields no retries and a single
// implicit rules_only stage so a missing config degrades gracefully.
func resolveFallback(fb *models.FallbackPolicy) (maxRetries uint, retryDelay time.Duration, stages []models.FallbackStep) {
	if fb == nil || len(fb.Steps) == 0 {
		return 0, 0, []models.FallbackStep{{Type: models.FallbackRulesOnly}}
	}
	for _, st := range fb.Steps {
		if st.Type == models.FallbackRetry {
			maxRetries = st.MaxRetries
			retryDelay = time.Duration(st.RetryDelayMs) * time.Millisecond
			continue
		}
		stages = append(stages, st)
	}
	if len(stages) == 0 {
		stages = []models.FallbackStep{{Type: models.FallbackRulesOnly}}
	}
	return maxRetries, retryDelay, stages
}

// --- timeout and retry resolution -----------------------------------------------

// resolveCallTimeout computes the per-LLM-call timeout for one invocation.
// Precedence (most specific wins):
//  1. target.TimeoutMs  — explicit per-target override (ms → Duration)
//  2. simCfg.RetryTimeoutMs > 0 — run-level config (ms → Duration)
//  3. genTimeout > 0  — SIMULATIONS_GENERATION_TIMEOUT ceiling
//  4. defaultRunCallTimeout — built-in last-resort ceiling
func resolveCallTimeout(target common.LLMTarget, simCfg *models.LLMSimulationExecutionConfig, genTimeout time.Duration) time.Duration {
	if target.TimeoutMs != nil {
		return time.Duration(*target.TimeoutMs) * time.Millisecond
	}
	if simCfg != nil && simCfg.RetryTimeoutMs > 0 {
		return time.Duration(simCfg.RetryTimeoutMs) * time.Millisecond
	}
	if genTimeout > 0 {
		return genTimeout
	}
	return defaultRunCallTimeout
}

// resolveRunMaxRetries returns the effective retry budget for one callJSON
// invocation. If the fallback policy contained an explicit FallbackRetry step
// (hasPolicyRetryStep == true), the policy's value (policyRetries) takes
// precedence. Otherwise, the run-level MaxRetries from simCfg is used. If both
// are absent or zero, 0 is returned (no retries — pure rules-only degradation).
func resolveRunMaxRetries(simCfg *models.LLMSimulationExecutionConfig, policyRetries uint, hasPolicyRetryStep bool) uint {
	if hasPolicyRetryStep {
		return policyRetries
	}
	if simCfg != nil && simCfg.MaxRetries > 0 {
		return simCfg.MaxRetries
	}
	return 0
}

// hasFallbackRetryStep reports whether a fallback policy contains at least one
// FallbackRetry step. Used by resolveRunMaxRetries to distinguish "policy
// explicitly set MaxRetries=0" from "policy has no retry step at all".
func hasFallbackRetryStep(fb *models.FallbackPolicy) bool {
	if fb == nil {
		return false
	}
	for _, st := range fb.Steps {
		if st.Type == models.FallbackRetry {
			return true
		}
	}
	return false
}

// --- interaction mediation ------------------------------------------------------

// MediateInteraction is the UseLLM-interaction hook used by ProcessInteractions:
// it asks the LLM whether (and how) an interaction fires between initiator and
// target. It builds a small mediation prompt, calls the LLM (decision capability),
// and returns the decided fire flag + any mediation params. It fails soft: on any
// error / malformed response it returns fire=true (the interaction's declared
// Conditions already gated it) so a provider outage never silently disables the
// model's interactions.
func (r *llmRuntime) MediateInteraction(ctx context.Context, rs *RuntimeState, in models.Interaction,
	initiator, target *RuntimeAgent) (fire bool, params map[string]any) {

	if rs == nil {
		return true, nil
	}

	var arch *models.Agent
	if initiator != nil {
		arch = initiator.Archetype
	}
	tgt, okT := resolveTarget(rs.Model, rs.Sim, arch)
	if !okT {
		// No LLM target: degrade to "fire" (conditions already held).
		return true, nil
	}

	system, user := buildInteractionPrompt(rs, in, initiator, target)
	fb := resolveModelFallback(rs.Model, arch)

	var resp interactionResp
	if ok := r.callJSON(ctx, tgt, common.ModelCapabilitySimulationDecisions, datasims.LLMUsageSimulationInteraction,
		interactionAgentUID(initiator), system, user, fb, &resp); !ok {
		return true, nil // fail-soft
	}
	return resp.Fire, resp.Params
}

// buildInteractionPrompt assembles the mediation system + user messages for a
// UseLLM interaction.
func buildInteractionPrompt(rs *RuntimeState, in models.Interaction, initiator, target *RuntimeAgent) (string, string) {
	var sys strings.Builder
	sys.WriteString(runSystemHeader)
	sys.WriteString("\nTask: decide whether an agent interaction fires this step, and any parameters.\n")
	sys.WriteString("Schema: {\"fire\": <bool>, \"params\": {\"<key>\": <value>}}\n")
	sys.WriteString("Example: {\"fire\": true, \"params\": {}}\n")

	var user strings.Builder
	user.WriteString(buildSharedRuntimeBlocks(rs))
	name := in.Name
	if name == "" {
		name = in.ID
	}
	fmt.Fprintf(&user, "Interaction: %s — %s\n", name, in.Description)
	writeInteractionAgent(&user, "Initiator", initiator)
	writeInteractionAgent(&user, "Target", target)

	// Footprint- and torus-aware distance between the two bodies.
	if initiator != nil && initiator.A != nil && initiator.A.Position != nil &&
		target != nil && target.A != nil && target.A.Position != nil {
		w, h, torus := envDims(rs)
		d := BodyDistance(bodyForAgent(initiator), bodyForAgent(target), w, h, torus)
		fmt.Fprintf(&user, "Distance: %s\n", trimFloat(d))
	}

	// What firing would do (declared action types) — the stakes of "fire".
	if s := interactionEffectSummary(in); s != "" {
		user.WriteString(s)
	}

	var arch *models.Agent
	if initiator != nil {
		arch = initiator.Archetype
	}
	group := &AgentGroup{Archetype: arch}
	members := []*RuntimeAgent{}
	if initiator != nil {
		members = append(members, initiator)
	}
	ctxText := buildContext(rs, group, members)
	if hint := strings.TrimSpace(in.LLMPromptContext); hint != "" {
		if ctxText != "" {
			ctxText += "\n"
		}
		ctxText += "- " + hint
	}
	if ctxText != "" {
		user.WriteString(ctxText)
		user.WriteByte('\n')
	}

	return sys.String(), user.String()
}

// writeInteractionAgent renders one side of a mediated interaction with its
// position and current properties (canonical JSON, key-sorted by json.Marshal).
func writeInteractionAgent(b *strings.Builder, role string, ra *RuntimeAgent) {
	if ra == nil || ra.A == nil {
		return
	}
	fmt.Fprintf(b, "%s: %s #%d state=%q", role, ra.A.AgentUID, ra.A.ID, ra.A.State)
	if ra.A.Position != nil {
		fmt.Fprintf(b, " pos=(%d,%d)", ra.A.Position.X, ra.A.Position.Y)
	}
	if len(ra.A.Properties) > 0 {
		if props, err := json.Marshal(ra.A.Properties); err == nil {
			fmt.Fprintf(b, " properties=%s", props)
		}
	}
	b.WriteByte('\n')
}

// interactionEffectSummary summarizes the interaction's declared consequences
// as first-seen-deduped action-type lists per side. "" when both sides are empty.
func interactionEffectSummary(in models.Interaction) string {
	ini := actionTypeList(in.InitiatorActions)
	tgt := actionTypeList(in.TargetActions)
	if ini == "" && tgt == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString("On fire:")
	if ini != "" {
		b.WriteString(" initiator gets [" + ini + "]")
	}
	if tgt != "" {
		if ini != "" {
			b.WriteString(";")
		}
		b.WriteString(" target gets [" + tgt + "]")
	}
	b.WriteByte('\n')
	return b.String()
}

func actionTypeList(actions []models.Action) string {
	seen := map[string]bool{}
	var out []string
	for _, a := range actions {
		s := string(a.Type)
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return strings.Join(out, ", ")
}

// interactionAgentUID returns the initiator's archetype UID for token attribution
// (empty when unknown).
func interactionAgentUID(initiator *RuntimeAgent) string {
	if initiator != nil && initiator.A != nil {
		return initiator.A.AgentUID
	}
	return ""
}

// resolveModelFallback returns the effective fallback policy for a runtime call:
// the per-agent override (LLMSimulationAgentConfig.FallbackPolicy) when present,
// else the model's DefaultSimulationConfig policy, else nil (callJSON applies its
// implicit rules_only default).
func resolveModelFallback(model *models.Model, agent *models.Agent) *models.FallbackPolicy {
	if agent != nil && agent.LLMConfig != nil && agent.LLMConfig.SimulationConfig != nil &&
		agent.LLMConfig.SimulationConfig.FallbackPolicy != nil {
		return agent.LLMConfig.SimulationConfig.FallbackPolicy
	}
	if model != nil {
		return model.SimulationConfig.LLMSimulationExecutionConfig.FallbackPolicy
	}
	return nil
}

// perceptionRadius resolves the effective perception radius for a runtime agent:
//  1. agent archetype LLMConfig.SimulationConfig.PerceptionRadius if > 0, else
//  2. 0 (callers supply the model-level fallback via the RuntimeState).
//
// The model-level fallback (DefaultSimulationConfig.PerceptionRadius) is applied
// in buildContext where RuntimeState is available, mirroring the interactionRadius
// resolver pattern in interactions.go (which is purely per-agent).
//
// Note: PerceptionRadius is distinct from InteractionRadius — InteractionRadius
// gates physical agent-agent interactions; PerceptionRadius controls what the LLM
// perceives via the "Nearby:" prompt block.
func perceptionRadius(ra *RuntimeAgent) float64 {
	if ra == nil || ra.Archetype == nil {
		return 0
	}
	if ra.Archetype.LLMConfig == nil {
		return 0
	}
	if ra.Archetype.LLMConfig.SimulationConfig == nil {
		return 0
	}
	return ra.Archetype.LLMConfig.SimulationConfig.PerceptionRadius
}

// resolvedPerceptionRadius is the full three-level resolver: agent archetype
// PerceptionRadius (if >0) → model DefaultSimulationConfig PerceptionRadius (if
// >0) → 0. The rs parameter provides the model-level fallback.
func resolvedPerceptionRadius(rs *RuntimeState, ra *RuntimeAgent) float64 {
	if r := perceptionRadius(ra); r > 0 {
		return r
	}
	// Model-level fallback.
	if rs != nil && rs.Model != nil {
		if r := rs.Model.SimulationConfig.LLMSimulationExecutionConfig.PerceptionRadius; r > 0 {
			return r
		}
	}
	return 0
}
