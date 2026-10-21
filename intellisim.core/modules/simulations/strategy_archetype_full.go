package simulations

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// archetypeFullStrategy is the Strategy-3 (full × archetype) execution strategy:
// for each enabled purpose it asks the LLM for ONE directive PER ARCHETYPE (the
// concrete-action template the whole archetype follows), optionally batching ≤
// Strategy.BatchSize archetypes into one call (demuxed by archetype_uid). The
// single directive is then fanned out to every member with local-context
// resolution + bounded seeded jitter (ApplyDirectiveWithVariation), so agents of
// one archetype are never identical. Capability:
// ModelCapabilitySimulationExecution.
type archetypeFullStrategy struct{}

func (archetypeFullStrategy) Name() string { return "archetype_full" }

// Plan delegates to the cross-archetype batcher with this single group. When the
// engine batches several archetypes into one call it invokes planArchetypeBatch
// directly with all groups; the per-group entry point keeps the ExecutionStrategy
// interface intact (one archetype => one call per purpose under BatchSize ≤ 1).
func (s archetypeFullStrategy) Plan(sc *StepContext, group *AgentGroup) (PlanResult, error) {
	if group == nil {
		return PlanResult{Intents: map[uint64]*AgentIntent{}}, nil
	}
	byArch := planArchetypeBatch(sc, []*AgentGroup{group}, s)
	if pr, ok := byArch[group.ArchetypeUID]; ok {
		return pr, nil
	}
	return PlanResult{Intents: map[uint64]*AgentIntent{}}, nil
}

// archetypeKind distinguishes the two archetype-scope strategies for the shared
// batcher: full requests a concrete-action template per archetype; decision
// requests a single label per archetype.
type archetypeKind int

const (
	archKindFull archetypeKind = iota
	archKindDecision
)

// archetypePlanner is the small contract the shared batcher uses to drive a
// strategy without an import cycle between the two archetype files.
type archetypePlanner interface {
	archetypeKind() archetypeKind
	capability() common.ModelCapability
}

func (archetypeFullStrategy) archetypeKind() archetypeKind { return archKindFull }
func (archetypeFullStrategy) capability() common.ModelCapability {
	return common.ModelCapabilitySimulationExecution
}

// planArchetypeBatch is the shared archetype-scope planner. For each enabled
// purpose (gated by DecisionFrequency on the FIRST group's strategy — archetypes
// in one batch share a resolved strategy), it splits the groups into batches of
// max(1, BatchSize) ARCHETYPES, issues ONE LLM call per batch (the archetype is
// the demux unit, keyed by archetype_uid), builds a per-archetype directive
// (AgentIntent template), and fans it out to that archetype's members via
// ApplyDirectiveWithVariation. Per-purpose results are merged per agent. It
// returns one PlanResult per archetype UID.
func planArchetypeBatch(sc *StepContext, groups []*AgentGroup, planner archetypePlanner) map[string]PlanResult {
	out := make(map[string]PlanResult)
	if sc == nil || sc.RS == nil || sc.LLM == nil || len(groups) == 0 {
		return out
	}
	live := liveGroups(groups)
	if len(live) == 0 {
		return out
	}
	for _, g := range live {
		out[g.ArchetypeUID] = PlanResult{Intents: map[uint64]*AgentIntent{}}
	}

	// Archetypes batched together share one resolved strategy; the first group's
	// strategy drives the enabled purposes / frequency / batch size / capability.
	strat := live[0].Strategy
	purposes := enabledPurposes(strat)
	if len(purposes) == 0 {
		return out
	}
	cap := planner.capability()

	for _, p := range purposes {
		if p == purposeInteraction {
			// Interaction participation is mediated in the interactions phase.
			continue
		}
		participants := live
		if p == purposeDecision {
			// The decision purpose honors on-demand call_llm requests: off-schedule
			// steps include only groups with ≥1 pending member, and the directive is
			// fanned out ONLY to the pending members (shallow group copies), so
			// non-requesting siblings don't receive off-schedule decisions.
			participants = decisionDueGroups(sc.RS.Step, strat.DecisionFrequency, live)
		} else if !purposeDue(sc.RS.Step, strat.DecisionFrequency) {
			continue
		}
		if len(participants) == 0 {
			continue
		}
		for _, batch := range chunkGroups(participants, strat.BatchSize) {
			directives := callArchetypePurpose(sc, batch, p, cap, planner.archetypeKind())
			for _, g := range batch {
				dir, ok := directives[g.ArchetypeUID]
				if !ok || dir == nil {
					// No fresh directive (failed call or archetype absent from the
					// response). For the decision kind, replay each member's own
					// LastDecision when present — mirroring the individual-decision
					// fallback (in distribution mode these are the per-member sampled
					// labels, which is exactly right). No LLM call, no RNG draw. The
					// full kind is intentionally NOT replayed (parity with
					// individual_full); rules cover the purpose otherwise.
					if p == purposeDecision && planner.archetypeKind() == archKindDecision {
						replayLastDecisions(g, out[g.ArchetypeUID].Intents)
					}
					continue
				}
				fanned := ApplyDirectiveWithVariation(sc.RS, g, dir)
				if p == purposeDecision && planner.archetypeKind() == archKindDecision {
					recordArchetypeDecisions(g, fanned)
				}
				mergeIntents(out[g.ArchetypeUID].Intents, fanned)
			}
		}
	}
	return out
}

// replayLastDecisions appends each live member's own LastDecision (when set)
// into the group's intents — the archetype-scope last-decision fallback for a
// failed/omitted decision directive. Members without a prior decision are left
// to the rules.
func replayLastDecisions(g *AgentGroup, intents map[uint64]*AgentIntent) {
	for _, ra := range g.Members {
		if ra == nil || ra.A == nil || ra.removed || ra.LastDecision == nil {
			continue
		}
		dec := *ra.LastDecision
		in := intents[ra.A.ID]
		if in == nil {
			in = &AgentIntent{}
			intents[ra.A.ID] = in
		}
		in.Decisions = append(in.Decisions, dec)
	}
}

// decisionDueGroups is the archetype-scope analogue of decisionDueMembers: when
// the frequency gate passes it returns all groups (consuming every member's
// pending call_llm flag); otherwise it returns shallow copies of the groups that
// have ≥1 pending member, with Members reduced to exactly those pending agents.
func decisionDueGroups(step uint64, freq int, groups []*AgentGroup) []*AgentGroup {
	if purposeDue(step, freq) {
		for _, g := range groups {
			for _, ra := range g.Members {
				if ra != nil {
					ra.llmCallPending = false
				}
			}
		}
		return groups
	}
	var out []*AgentGroup
	for _, g := range groups {
		var pending []*RuntimeAgent
		for _, ra := range g.Members {
			if ra != nil && ra.llmCallPending {
				ra.llmCallPending = false
				pending = append(pending, ra)
			}
		}
		if len(pending) > 0 {
			gg := *g
			gg.Members = pending
			out = append(out, &gg)
		}
	}
	return out
}

// callArchetypePurpose issues one batched LLM call for a purpose across a batch of
// archetypes and demuxes the response by archetype_uid into a per-archetype
// directive (AgentIntent template). Returns one directive per archetype UID that
// produced a usable payload; archetypes absent from the response (or on a failed
// call) are simply omitted (caller leaves them to the rules).
func callArchetypePurpose(sc *StepContext, batch []*AgentGroup, p purpose, capability common.ModelCapability, kind archetypeKind) map[string]*AgentIntent {
	rs := sc.RS
	// Archetypes in one batch share a resolved strategy (planArchetypeBatch uses the
	// first group's strategy), so the application mode is taken from batch[0].
	mode := resolveArchetypeMode(batch[0].Strategy)
	system, user := buildArchetypePrompt(rs, batch, p, kind, mode)
	// Fallback policy of the batch's first archetype (they share a resolved config).
	fb := resolveModelFallback(rs.Model, batch[0].Archetype)
	target := batch[0].Target

	out := make(map[string]*AgentIntent)
	switch p {
	case purposeDecision:
		if kind == archKindFull {
			if mode == models.ArchetypeModeDistribution {
				var resp fullArchetypeDistResp
				if !sc.LLM.callJSON(sc.Ctx, target, capability,
					datasims.LLMUsageSimulationDecision, "", system, user, fb, &resp) {
					return out
				}
				for _, a := range resp.Agents {
					g := groupByUID(batch, a.ArchetypeUID)
					if g == nil {
						continue
					}
					if variants := mapArchetypeVariants(rs, g, a.Variants); len(variants) > 0 {
						out[a.ArchetypeUID] = &AgentIntent{ActionVariants: variants}
					}
					// All variants dropped (no valid actions) → omit the archetype so rules cover it.
				}
				return out
			}
			var resp fullArchetypeResp
			if !sc.LLM.callJSON(sc.Ctx, target, capability,
				datasims.LLMUsageSimulationDecision, "", system, user, fb, &resp) {
				return out
			}
			for _, a := range resp.Agents {
				g := groupByUID(batch, a.ArchetypeUID)
				if g == nil {
					continue
				}
				out[a.ArchetypeUID] = mapArchetypeIntent(rs, g, a.Actions)
			}
		} else {
			if mode == models.ArchetypeModeDistribution {
				var resp decisionDistResp
				if !sc.LLM.callJSON(sc.Ctx, target, capability,
					datasims.LLMUsageSimulationDecision, "", system, user, fb, &resp) {
					return out
				}
				for _, a := range resp.Agents {
					if groupByUID(batch, a.ArchetypeUID) == nil {
						continue
					}
					if intent := mapArchetypeDecisionDist(rs, a.ArchetypeUID, a.Choices); intent != nil {
						out[a.ArchetypeUID] = intent
					}
				}
				return out
			}
			var resp decisionResp
			if !sc.LLM.callJSON(sc.Ctx, target, capability,
				datasims.LLMUsageSimulationDecision, "", system, user, fb, &resp) {
				return out
			}
			for _, a := range resp.Agents {
				if a.Label == "" || groupByUID(batch, a.ArchetypeUID) == nil {
					continue
				}
				intent := &AgentIntent{Decisions: []DecisionIntent{{Label: a.Label, Params: a.Params}}}
				// Task 17: translate optional emit from archetype-decision response.
				// Authorization uses the archetype UID (all members share one archetype).
				if a.EmitUID != "" {
					def := eventDefByUID(rs, a.EmitUID)
					if authorizeEmit(def, models.EventProducerLLM, a.ArchetypeUID) {
						intent.Emits = append(intent.Emits, EmitIntent{EventUID: a.EmitUID, Data: a.EmitData})
					}
				}
				out[a.ArchetypeUID] = intent
			}
		}
	case purposeProperties:
		var resp propsResp
		if !sc.LLM.callJSON(sc.Ctx, target, capability,
			datasims.LLMUsageSimulationProperties, "", system, user, fb, &resp) {
			return out
		}
		for _, a := range resp.Agents {
			g := groupByUID(batch, a.ArchetypeUID)
			if g == nil || len(a.Values) == 0 {
				continue
			}
			// Enforce the prompt's "unknown or immutable UIDs are dropped" promise
			// BEFORE the directive fans out to members (varyProperties copies keys
			// verbatim). A fully-dropped map omits the archetype (rules cover it).
			vals := filterSettableValues(g.Archetype, a.Values)
			if len(vals) == 0 {
				continue
			}
			out[a.ArchetypeUID] = &AgentIntent{Properties: vals}
		}
	case purposeMovement:
		var resp movementResp
		if !sc.LLM.callJSON(sc.Ctx, target, capability,
			datasims.LLMUsageSimulationMovement, "", system, user, fb, &resp) {
			return out
		}
		for _, a := range resp.Agents {
			if groupByUID(batch, a.ArchetypeUID) == nil {
				continue
			}
			out[a.ArchetypeUID] = &AgentIntent{Movement: &MovementIntent{Intent: a.Intent, TargetRef: a.TargetRef, Speed: a.Speed}}
		}
	}
	return out
}

// fullArchetypeResp is the full-control archetype response: one action template
// per archetype, keyed by archetype_uid (the demux unit for archetype batches).
type fullArchetypeResp struct {
	Agents []struct {
		ArchetypeUID string      `json:"archetype_uid"`
		Actions      []rawAction `json:"actions"`
	} `json:"agents"`
}

// archetypeVariant is one weighted action-set variant in a full distribution response.
type archetypeVariant struct {
	Weight  float64     `json:"weight"`
	Actions []rawAction `json:"actions"`
}

// fullArchetypeDistResp is the full-control DISTRIBUTION response (ArchetypeModeDistribution):
// a weighted set of whole action-set variants per archetype; each member samples one.
type fullArchetypeDistResp struct {
	Agents []struct {
		ArchetypeUID string             `json:"archetype_uid"`
		Variants     []archetypeVariant `json:"variants"`
	} `json:"agents"`
}

// archetypeDecisionChoice is one weighted decision label in a decision distribution response.
type archetypeDecisionChoice struct {
	Label   string         `json:"label"`
	Weight  float64        `json:"weight"`
	Params  map[string]any `json:"params"`
	EmitUID string         `json:"emit_uid"`
	EmitData any            `json:"emit_data"`
}

// decisionDistResp is the decision DISTRIBUTION response (ArchetypeModeDistribution):
// a weighted set of decision labels per archetype; each member samples one.
type decisionDistResp struct {
	Agents []struct {
		ArchetypeUID string                    `json:"archetype_uid"`
		Choices      []archetypeDecisionChoice `json:"choices"`
	} `json:"agents"`
}

// mapArchetypeIntent validates a full-control archetype action template and
// partitions the result into Actions vs Emits (Task 17). emit_event ActionIntents
// are placed into AgentIntent.Emits so ApplyDirectiveWithVariation copies them
// into per-agent intents and ApplyIntentActions routes them through
// applyEmitEvent(..., EventProducerLLM).
func mapArchetypeIntent(rs *RuntimeState, g *AgentGroup, raw []rawAction) *AgentIntent {
	ra := representativeMember(g)
	if ra == nil {
		return &AgentIntent{}
	}
	mapped := MapFullActions(rs, ra, raw)
	intent := &AgentIntent{}
	for _, ai := range mapped {
		if ai.Type == models.ActionTypeEmitEvent {
			intent.Emits = append(intent.Emits, EmitIntent{EventUID: ai.EventUID, Data: ai.EventData})
		} else {
			intent.Actions = append(intent.Actions, ai)
		}
	}
	return intent
}

// mapArchetypeVariants validates each full-control action-set variant against the
// archetype's representative member, partitioning emit_event actions into the
// variant's Emits (the same contract as mapArchetypeIntent). Variants that map to
// zero actions AND zero emits are dropped; the caller omits the archetype entirely
// when none survive (so the rule engine covers it). Weights are carried through.
func mapArchetypeVariants(rs *RuntimeState, g *AgentGroup, variants []archetypeVariant) []WeightedActionSet {
	ra := representativeMember(g)
	if ra == nil {
		return nil
	}
	out := make([]WeightedActionSet, 0, len(variants))
	for _, v := range variants {
		mapped := MapFullActions(rs, ra, v.Actions)
		ws := WeightedActionSet{Weight: v.Weight}
		for _, ai := range mapped {
			if ai.Type == models.ActionTypeEmitEvent {
				ws.Emits = append(ws.Emits, EmitIntent{EventUID: ai.EventUID, Data: ai.EventData})
			} else {
				ws.Actions = append(ws.Actions, ai)
			}
		}
		if len(ws.Actions) == 0 && len(ws.Emits) == 0 {
			continue // empty variant: drop so pickChoice renormalises over the survivors
		}
		out = append(out, ws)
	}
	return out
}

// mapArchetypeDecisionDist builds a decision-distribution directive from the LLM's
// weighted label choices: each non-empty label becomes a WeightedDecision, and any
// authorized optional emit is collected onto the directive's Emits (fanned verbatim
// to all members, matching the identical-mode "emits identical for all members"
// rule). Returns nil when no usable label survives (caller omits the archetype).
func mapArchetypeDecisionDist(rs *RuntimeState, archetypeUID string, choices []archetypeDecisionChoice) *AgentIntent {
	intent := &AgentIntent{}
	for _, c := range choices {
		if c.Label == "" {
			continue
		}
		intent.DecisionDist = append(intent.DecisionDist, WeightedDecision{
			Decision: DecisionIntent{Label: c.Label, Params: c.Params},
			Weight:   c.Weight,
		})
		if c.EmitUID != "" {
			def := eventDefByUID(rs, c.EmitUID)
			if authorizeEmit(def, models.EventProducerLLM, archetypeUID) {
				intent.Emits = append(intent.Emits, EmitIntent{EventUID: c.EmitUID, Data: c.EmitData})
			}
		}
	}
	if len(intent.DecisionDist) == 0 {
		return nil
	}
	return intent
}

// recordArchetypeDecisions stamps each member's sampled decision label as its
// LastDecision (for the last_decision fallback / decision conditions). It reads from
// the FANNED per-agent intents (not the directive) so it is correct for both
// identical mode (every member shares the label) and distribution mode (each member
// sampled its own). The fanned intents already hold per-agent param copies.
func recordArchetypeDecisions(g *AgentGroup, fanned map[uint64]*AgentIntent) {
	for _, ra := range g.Members {
		if ra == nil || ra.A == nil || ra.removed {
			continue
		}
		in := fanned[ra.A.ID]
		if in == nil || len(in.Decisions) == 0 {
			continue
		}
		cp := in.Decisions[0]
		ra.LastDecision = &cp
	}
}

// --- shared archetype helpers --------------------------------------------------

// liveGroups returns the groups that still have at least one non-removed member.
func liveGroups(groups []*AgentGroup) []*AgentGroup {
	out := make([]*AgentGroup, 0, len(groups))
	for _, g := range groups {
		if g == nil || g.Archetype == nil {
			continue
		}
		if representativeMember(g) == nil {
			continue
		}
		out = append(out, g)
	}
	return out
}

// representativeMember returns the first live member of a group (used to validate
// archetype-wide action/property templates), or nil when the group is empty.
func representativeMember(g *AgentGroup) *RuntimeAgent {
	if g == nil {
		return nil
	}
	for _, ra := range g.Members {
		if ra != nil && ra.A != nil && !ra.removed {
			return ra
		}
	}
	return nil
}

// chunkGroups splits archetype groups into batches of at most max(1,size). Empty
// groups are dropped so a batch never carries an archetype with no live members.
func chunkGroups(groups []*AgentGroup, size int) [][]*AgentGroup {
	n := size
	if n < 1 {
		n = 1
	}
	var out [][]*AgentGroup
	for i := 0; i < len(groups); i += n {
		end := i + n
		if end > len(groups) {
			end = len(groups)
		}
		out = append(out, groups[i:end])
	}
	return out
}

// groupByUID finds the group in a batch with the given archetype UID, or nil.
func groupByUID(batch []*AgentGroup, uid string) *AgentGroup {
	for _, g := range batch {
		if g.ArchetypeUID == uid {
			return g
		}
	}
	return nil
}

// mergeIntents merges per-agent intents from src into dst, combining the per-agent
// fields so several per-purpose passes accumulate onto one AgentIntent.
func mergeIntents(dst map[uint64]*AgentIntent, src map[uint64]*AgentIntent) {
	for id, in := range src {
		if in == nil {
			continue
		}
		cur := dst[id]
		if cur == nil {
			dst[id] = in
			continue
		}
		cur.Decisions = append(cur.Decisions, in.Decisions...)
		cur.Actions = append(cur.Actions, in.Actions...)
		cur.Emits = append(cur.Emits, in.Emits...) // Task 17: merge LLM-emit intents
		if in.Movement != nil {
			cur.Movement = in.Movement
		}
		if len(in.Properties) > 0 {
			if cur.Properties == nil {
				cur.Properties = make(map[string]any, len(in.Properties))
			}
			for k, v := range in.Properties {
				cur.Properties[k] = v
			}
		}
	}
}

// buildArchetypePrompt assembles the system + user messages for one archetype
// batch call. Each archetype is enumerated by its UID (the demux key) with its
// member count + a representative state, followed by the shared mandatory-hint
// context (model/archetype PromptContext) + the members' memory summaries.
func buildArchetypePrompt(rs *RuntimeState, batch []*AgentGroup, p purpose, kind archetypeKind, mode models.ArchetypeMode) (string, string) {
	var sys strings.Builder
	sys.WriteString(runSystemHeader)
	sys.WriteString(archetypeSystemSpec(p, kind, mode, rs, batch))

	var user strings.Builder
	user.WriteString(buildSharedRuntimeBlocks(rs))
	fmt.Fprintf(&user, "%d archetype(s) acting this step (one directive each):\n", len(batch))
	for _, g := range batch {
		fmt.Fprintf(&user, "- archetype %q: %d agent(s)", g.ArchetypeUID, len(g.Members))
		if rep := representativeMember(g); rep != nil {
			fmt.Fprintf(&user, " example state=%q", rep.A.State)
			if rep.A.Position != nil {
				fmt.Fprintf(&user, " example pos=(%d,%d)", rep.A.Position.X, rep.A.Position.Y)
			}
		}
		user.WriteByte('\n')
		if agg := archetypeAggregateBlock(rs, g); agg != "" {
			user.WriteString(agg)
		}
		if wantsPropertySchema(p, g.Strategy) {
			if lines := settablePropertyLines(g.Archetype); lines != "" {
				user.WriteString("Properties (settable; values are clamped to bounds):\n")
				user.WriteString(lines)
			}
		}
		// Per-archetype mandatory-hint context + memory of its members.
		if ctxText := buildContext(rs, g, g.Members); ctxText != "" {
			user.WriteString(ctxText)
			user.WriteByte('\n')
		}
	}
	return sys.String(), user.String()
}

// archetypeAggregateBlock renders the article's aggregated-statistics view of
// one archetype population (Strategies 3/5: "the LLM analyses aggregated
// statistics for the type"): live count, state distribution, numeric property
// mean/min/max, and spatial centroid + RMS spread. Deterministic: declared
// property order, key-sorted states. The per-member memory/perception blocks
// are intentionally KEPT alongside (softer variant per the design spec).
func archetypeAggregateBlock(rs *RuntimeState, g *AgentGroup) string {
	live := make([]*RuntimeAgent, 0, len(g.Members))
	for _, ra := range g.Members {
		if ra != nil && ra.A != nil && !ra.removed {
			live = append(live, ra)
		}
	}
	if len(live) == 0 {
		return ""
	}
	var b strings.Builder

	states := map[string]int{}
	for _, ra := range live {
		states[ra.A.State]++
	}
	stateKeys := make([]string, 0, len(states))
	for k := range states {
		stateKeys = append(stateKeys, k)
	}
	sort.Strings(stateKeys)
	fmt.Fprintf(&b, "Population stats: %d live; states:", len(live))
	for _, k := range stateKeys {
		fmt.Fprintf(&b, " %s=%d", k, states[k])
	}
	b.WriteByte('\n')

	if g.Archetype != nil {
		wrote := false
		for _, p := range g.Archetype.Properties {
			if p.Type != models.PropertyTypeNumber {
				continue
			}
			var n int
			var sum, min, max float64
			for _, ra := range live {
				v, ok := coerceFloat(ra.A.Properties[p.UID])
				if !ok {
					continue
				}
				if n == 0 || v < min {
					min = v
				}
				if n == 0 || v > max {
					max = v
				}
				sum += v
				n++
			}
			if n == 0 {
				continue
			}
			if !wrote {
				b.WriteString("Property stats:")
				wrote = true
			}
			fmt.Fprintf(&b, " %s mean=%s min=%s max=%s;", p.UID, trimFloat(sum/float64(n)), trimFloat(min), trimFloat(max))
		}
		if wrote {
			b.WriteByte('\n')
		}
	}

	if rs != nil && rs.Model != nil && rs.Model.IsSpatial {
		var n int
		var sx, sy float64
		for _, ra := range live {
			if ra.A.Position == nil {
				continue
			}
			sx += float64(ra.A.Position.X)
			sy += float64(ra.A.Position.Y)
			n++
		}
		if n > 0 {
			cx, cy := sx/float64(n), sy/float64(n)
			var acc float64
			for _, ra := range live {
				if ra.A.Position == nil {
					continue
				}
				dx := float64(ra.A.Position.X) - cx
				dy := float64(ra.A.Position.Y) - cy
				acc += dx*dx + dy*dy
			}
			fmt.Fprintf(&b, "Spatial: centroid=(%d,%d) spread=%s\n",
				int64(math.Round(cx)), int64(math.Round(cy)), trimFloat(math.Sqrt(acc/float64(n))))
		}
	}
	return b.String()
}

// archetypeSystemSpec returns the per-purpose system-prompt fragment for an
// archetype batch (role + strict JSON output contract, demuxed by archetype_uid).
// Under ArchetypeModeDistribution the decision/full schemas request a WEIGHTED
// distribution (choices/variants) so each member can sample a distinct outcome;
// under identical mode (the default) the legacy single-directive schemas are emitted
// byte-for-byte. Properties/movement schemas are mode-independent.
func archetypeSystemSpec(p purpose, kind archetypeKind, mode models.ArchetypeMode, rs *RuntimeState, batch []*AgentGroup) string {
	dist := mode == models.ArchetypeModeDistribution
	switch p {
	case purposeProperties:
		return "\nTask: update each archetype's agent property values (one directive per archetype).\n" +
			"Only the DECLARED settable property UIDs are accepted; values are clamped to the property's bounds; unknown or immutable UIDs are dropped.\n" +
			"Schema: {\"agents\":[{\"archetype_uid\":\"<uid>\",\"values\":{\"<propUID>\":<value>}}]}\n" +
			"Example: {\"agents\":[{\"archetype_uid\":\"wolf\",\"values\":{\"energy\":42}}]}\n"
	case purposeMovement:
		return "\nTask: choose each archetype's movement directive (one per archetype).\n" +
			"target_ref: an archetype or shape uid (each agent resolves the NEAREST matching instance), a literal \"x,y\" point, or empty (nearest agent of any archetype).\n" +
			"speed is 0..1 (fraction of the agent's max per-step distance). intent hold = stay; wander = random direction.\n" +
			"Schema: {\"agents\":[{\"archetype_uid\":\"<uid>\",\"intent\":\"toward|away|wander|hold\",\"target_ref\":\"<archetypeUID|shapeUID|x,y>\",\"speed\":<0..1>}]}\n" +
			"Example: {\"agents\":[{\"archetype_uid\":\"wolf\",\"intent\":\"toward\",\"target_ref\":\"lake\",\"speed\":0.8}]}\n"
	case purposeDecision:
		// Union of the LLM-emittable event labels (UID + optional prompt context)
		// across the batch (first-seen dedup), advertised on both the decision and
		// full-control specs when non-empty.
		emitSeen := map[string]struct{}{}
		var emitLabels []string
		for _, g := range batch {
			for _, label := range llmEmittableEventLabels(rs.Model, g.ArchetypeUID) {
				if _, dup := emitSeen[label]; dup {
					continue
				}
				emitSeen[label] = struct{}{}
				emitLabels = append(emitLabels, label)
			}
		}
		if kind == archKindDecision {
			if dist {
				spec := "\nTask: return a WEIGHTED DISTRIBUTION of decision labels per archetype; each agent independently samples one (so members of an archetype diverge)."
				if menu := unionDecisionMenuWithHints(rs, batch); len(menu) > 0 {
					spec += " Allowed labels: " + strings.Join(menu, ", ") + "."
				} else {
					spec += " You may return free labels."
				}
				spec += " Weights are relative (need not sum to 1)."
				spec += "\nSchema: {\"agents\":[{\"archetype_uid\":\"<uid>\",\"choices\":[{\"label\":\"<label>\",\"weight\":<number>,\"params\":{}}]}]}\n"
				if len(emitLabels) > 0 {
					spec += "Optionally add \"emit_uid\":\"<uid>\" (+ \"emit_data\":<payload>) to a choice entry (emit fields are read per choice) to emit one of: " + strings.Join(emitLabels, ", ") + ".\n"
				}
				spec += "Example: {\"agents\":[{\"archetype_uid\":\"wolf\",\"choices\":[{\"label\":\"forage\",\"weight\":3,\"params\":{}},{\"label\":\"rest\",\"weight\":1,\"params\":{}}]}]}\n"
				return spec
			}
			spec := "\nTask: choose one decision label per archetype."
			if menu := unionDecisionMenuWithHints(rs, batch); len(menu) > 0 {
				spec += " Allowed labels: " + strings.Join(menu, ", ") + "."
			} else {
				spec += " You may return a free label."
			}
			spec += "\nSchema: {\"agents\":[{\"archetype_uid\":\"<uid>\",\"label\":\"<label>\",\"params\":{}}]}\n"
			if len(emitLabels) > 0 {
				spec += "Optionally add \"emit_uid\":\"<uid>\" (+ \"emit_data\":<payload>) to emit one of: " + strings.Join(emitLabels, ", ") + ".\n"
			}
			spec += "Example: {\"agents\":[{\"archetype_uid\":\"wolf\",\"label\":\"forage\",\"params\":{}}]}\n"
			return spec
		}
		// full × archetype: concrete-action template(s) per archetype, including LLM-emitted events.
		// emit_event is only valid when the EventDefinition's Producers include "llm"
		// and (if ProducerUIDs is non-empty) the archetype UID is listed.
		actionTypes := "Allowed action types: set_property, modify_property, move, move_toward, move_away, interact, transition, emit_event.\n" +
			"For emit_event: {\"type\":\"emit_event\",\"event_uid\":\"<uid>\",\"event_data\":<payload|null>}\n" +
			"emit_event is only valid for events whose Producers include \"llm\"; unauthorized emits are silently dropped.\n"
		if len(emitLabels) > 0 {
			actionTypes += "Emittable events: " + strings.Join(emitLabels, ", ") + ".\n"
		}
		actionTypes += "set_property/modify_property may target a DECLARED environment global property by adding \"target_uid\":\"environment\".\n" +
			"move_toward/move_away target_uid follows the movement target_ref semantics (archetype/shape uid → nearest instance).\n"
		actionFields := "{\"type\":\"<type>\",\"property_uid\":\"...\",\"value\":...,\"modifier\":\"set|add|subtract|multiply|divide|increment|decrement|toggle\",\"target_uid\":\"...\",\"speed\":<0..1>,\"new_state\":\"...\",\"event_uid\":\"...\",\"event_data\":...}"
		if dist {
			return "\nTask: return a WEIGHTED set of action-set VARIANTS per archetype; each agent independently samples ONE whole variant (so members of an archetype diverge). Weights are relative (need not sum to 1).\n" +
				actionTypes +
				"Schema: {\"agents\":[{\"archetype_uid\":\"<uid>\",\"variants\":[{\"weight\":<number>,\"actions\":[" + actionFields + "]}]}]}\n" +
				"Example: {\"agents\":[{\"archetype_uid\":\"wolf\",\"variants\":[{\"weight\":2,\"actions\":[{\"type\":\"move_toward\",\"target_uid\":\"lake\",\"speed\":0.7}]},{\"weight\":1,\"actions\":[{\"type\":\"transition\",\"new_state\":\"resting\"}]}]}]}\n"
		}
		return "\nTask: choose a concrete action template per archetype (applied to all its agents with variation).\n" +
			actionTypes +
			"Schema: {\"agents\":[{\"archetype_uid\":\"<uid>\",\"actions\":[" + actionFields + "]}]}\n" +
			"Example: {\"agents\":[{\"archetype_uid\":\"wolf\",\"actions\":[{\"type\":\"move_toward\",\"target_uid\":\"lake\",\"speed\":0.7}]}]}\n"
	default:
		return ""
	}
}

// unionDecisionMenuWithHints returns the de-duplicated union of the per-archetype
// decision menus across the batch (preserving first-seen order) with rule-derived
// hints, so a batched decision call tells the LLM every allowed label; the hint
// comes from the first archetype in the batch that carries the label.
func unionDecisionMenuWithHints(rs *RuntimeState, batch []*AgentGroup) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, g := range batch {
		for _, label := range DecisionMenu(rs.Model, g.ArchetypeUID) {
			if _, dup := seen[label]; dup {
				continue
			}
			seen[label] = struct{}{}
			out = append(out, label+decisionLabelHint(rs.Model, g.ArchetypeUID, label))
		}
	}
	return out
}
