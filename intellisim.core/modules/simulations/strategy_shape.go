package simulations

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// shapeGroup holds one LLM-active shape UID together with its resolved model
// shape, all runtime shape instances (members) sharing that UID, the resolved
// LLMShapeStrategy, and the resolved LLM target for the call.
type shapeGroup struct {
	ShapeUID   string
	ModelShape *models.Shape
	Members    []*datasims.SimulationShape
	Strategy   models.LLMShapeStrategy
	Target     common.LLMTarget
}

// groupShapesByUID iterates rs.Env.Shapes in slice order, resolves each runtime
// shape to its model shape (by ShapeUID via a scan of rs.Model.Environment.Shapes),
// and groups them by ShapeUID in first-seen deterministic order. Shapes whose model
// shape has no LLMConfig, or whose LLMConfig.Strategy.Type is empty or "none", are
// skipped. The resolved Target merges shape-level LLMConfig.Target over the model
// default (field-wise: a non-empty shape field wins, empty fields inherit from the
// model), mirroring the agent resolveTarget precedence.
func groupShapesByUID(rs *RuntimeState) []*shapeGroup {
	if rs == nil || rs.Env == nil || rs.Model == nil || rs.Model.Environment == nil {
		return nil
	}

	// Index the model shapes by UID for O(1) lookup per runtime shape. Reuse the
	// per-run map when the state was built by NewRuntimeState.
	modelShapeByUID := rs.modelShapeDefs
	if modelShapeByUID == nil {
		modelShapeByUID = make(map[string]*models.Shape, len(rs.Model.Environment.Shapes))
		for i := range rs.Model.Environment.Shapes {
			ms := &rs.Model.Environment.Shapes[i]
			modelShapeByUID[ms.UID] = ms
		}
	}

	order := make([]string, 0)
	byUID := make(map[string]*shapeGroup)

	for i := range rs.Env.Shapes {
		sh := &rs.Env.Shapes[i]
		ms, ok := modelShapeByUID[sh.ShapeUID]
		if !ok || ms == nil {
			continue
		}
		if ms.LLMConfig == nil {
			continue
		}
		if ms.LLMConfig.Strategy.Type == "" || ms.LLMConfig.Strategy.Type == models.LLMStrategyNone {
			continue
		}

		g, exists := byUID[sh.ShapeUID]
		if !exists {
			// Resolve Target: shape LLMConfig.Target fields overlay the execution default.
			target := resolveShapeTarget(rs.Model, rs.Sim, ms)
			g = &shapeGroup{
				ShapeUID:   sh.ShapeUID,
				ModelShape: ms,
				Strategy:   ms.LLMConfig.Strategy,
				Target:     target,
			}
			byUID[sh.ShapeUID] = g
			order = append(order, sh.ShapeUID)
		}
		g.Members = append(g.Members, sh)
	}

	out := make([]*shapeGroup, 0, len(order))
	for _, uid := range order {
		out = append(out, byUID[uid])
	}
	return out
}

// resolveShapeTarget computes the effective LLM target for a shape group using a
// field-wise merge mirroring the agent-level resolveTarget precedence
// (shape > sim-execution-override > model execution default): the model's
// SimulationConfig.LLMSimulationExecutionConfig.Target is the base, the simulation's
// LLMExecutionTargetOverride overlays it, and the shape's own LLMConfig.Target
// overlays that. The shape LLM phase is part of Level-2 execution, so it reads the
// execution config — not the model-generation config.
func resolveShapeTarget(model *models.Model, sim *datasims.Simulation, ms *models.Shape) common.LLMTarget {
	var merged common.LLMTarget
	if model != nil {
		overlayTarget(&merged, model.SimulationConfig.LLMSimulationExecutionConfig.Target)
	}
	if sim != nil && sim.LLMExecutionTargetOverride != nil {
		overlayTarget(&merged, *sim.LLMExecutionTargetOverride)
	}
	if ms != nil && ms.LLMConfig != nil {
		overlayTarget(&merged, ms.LLMConfig.Target)
	}
	return merged
}

// shapePropsResp is the response shape for a shape property-update call.
// The LLM returns a flat properties map keyed by property UID.
type shapePropsResp struct {
	Properties map[string]interface{} `json:"properties"`
}

// planShapeUpdates asks the LLM for property updates for one shape group. It
// builds a compact prompt from the shape type, the current properties of the
// group's first member (all members share the same UID so their properties are
// semantically equivalent), and any PromptContext from the model shape or its
// LLMShapeConfig. It calls callJSON with purpose LLMUsagePurposeSimulationShape,
// attributing tokens to the ShapeUID as the agentUID key (consistent with the agent
// phase that uses AgentUID as the attribution key). On any error (provider failure,
// malformed JSON) it fails-soft and returns nil — the caller leaves the shape
// unchanged for this tick.
func planShapeUpdates(ctx context.Context, rs *RuntimeState, llmRT *llmRuntime, g *shapeGroup) map[string]interface{} {
	if llmRT == nil || g == nil || g.ModelShape == nil {
		return nil
	}
	if g.Target.ProviderUID == "" || g.Target.ModelUID == "" {
		return nil
	}

	system, user := buildShapePrompt(rs, g)

	var resp shapePropsResp
	if ok := llmRT.callJSON(ctx, g.Target, common.ModelCapabilitySimulationExecution,
		datasims.LLMUsagePurposeSimulationShape, g.ShapeUID, system, user, nil, &resp); !ok {
		return nil
	}
	return resp.Properties
}

// buildShapePrompt assembles the system and user messages for a shape
// property-update call: shared runtime blocks, shape identity, the settable
// property schema (immutable properties are excluded — the engine skips their
// writes anyway), current values, and the execution-config + shape guidance.
func buildShapePrompt(rs *RuntimeState, g *shapeGroup) (string, string) {
	var sys strings.Builder
	sys.WriteString(runSystemHeader)
	sys.WriteString("\nTask: update properties for an environment shape this step.\n")
	sys.WriteString("Only the listed settable property UIDs are accepted; unknown or immutable UIDs are dropped.\n")
	sys.WriteString(`Schema: {"properties": {"<uid>": <value>}}`)
	sys.WriteString("\nExample: {\"properties\": {\"level\": 5}}\n")

	var user strings.Builder
	user.WriteString(buildSharedRuntimeBlocks(rs))
	fmt.Fprintf(&user, "Shape: %s (uid=%s, %s)", g.ModelShape.Name, g.ShapeUID, string(g.ModelShape.Type))
	if d := strings.TrimSpace(g.ModelShape.Description); d != "" {
		fmt.Fprintf(&user, " — %s", d)
	}
	user.WriteByte('\n')

	// Settable property schema (declared, mutable only).
	var schemaLines strings.Builder
	for _, p := range g.ModelShape.Properties {
		if p.Immutable {
			continue
		}
		fmt.Fprintf(&schemaLines, "- %s\n", propSchemaLine(p))
	}
	if schemaLines.Len() > 0 {
		user.WriteString("Properties (settable; values must fit type/bounds):\n")
		user.WriteString(schemaLines.String())
	}

	// Current property values from the first member (all members share the same UID).
	if len(g.Members) > 0 && g.Members[0] != nil {
		props := g.Members[0].Properties
		if len(props) > 0 {
			user.WriteString("Current values:\n")
			propKeys := make([]string, 0, len(props))
			for k := range props {
				propKeys = append(propKeys, k)
			}
			sort.Strings(propKeys)
			for _, k := range propKeys {
				fmt.Fprintf(&user, "  %s: %v\n", k, props[k])
			}
		}
	}

	// Guidance: EXECUTION-config context (B3 — was Model.LLMConfig, the
	// model-generation config), then the shape's own context.
	var guidanceParts []string
	if rs != nil && rs.Model != nil {
		guidanceParts = append(guidanceParts, strings.TrimSpace(rs.Model.SimulationConfig.LLMSimulationExecutionConfig.PromptContext))
	}
	if g.ModelShape.LLMConfig != nil {
		guidanceParts = append(guidanceParts, strings.TrimSpace(g.ModelShape.LLMConfig.PromptContext))
	}
	writeGuidance(&user, guidanceChain(guidanceParts...))

	return sys.String(), user.String()
}

// applyShapeUpdates writes the property map returned by planShapeUpdates onto
// every group member's Properties slice in rs.Env.Shapes. Only UIDs DECLARED on
// the model shape AND not marked Immutable are written — the shape prompt
// promises "unknown or immutable UIDs are dropped", and this enforces it. A nil
// updates map is a no-op.
func applyShapeUpdates(rs *RuntimeState, g *shapeGroup, updates map[string]interface{}) {
	if updates == nil || g == nil || rs == nil || rs.Env == nil {
		return
	}

	// Build the set of settable (declared, mutable) property UIDs from the model
	// shape. An undeclared model shape yields an empty set → every update dropped.
	settable := make(map[string]bool)
	if g.ModelShape != nil {
		for _, p := range g.ModelShape.Properties {
			if !p.Immutable {
				settable[p.UID] = true
			}
		}
	}

	// Write updates onto each runtime member.
	for i := range rs.Env.Shapes {
		sh := &rs.Env.Shapes[i]
		if sh.ShapeUID != g.ShapeUID {
			continue
		}
		if sh.Properties == nil {
			sh.Properties = make(map[string]interface{}, len(updates))
		}
		for k, v := range updates {
			if !settable[k] {
				continue
			}
			sh.Properties[k] = v
		}
	}
}

// runShapePhase is the per-step shape-LLM phase. It runs ONCE on the real rs
// before agents are cloned, and shapes are iterated in fixed (first-seen UID)
// group order, so shape-LLM writes are deterministic WITHOUT touching the agent
// clone/commit cycle. This placement (after the perceive/index rebuild and before
// runLLMPhase) ensures that agents perceive the updated shape state this tick.
//
// Determinism note: because the shape phase runs on the real rs before any agent
// clones are made, and each group is processed sequentially in first-seen order, the
// result is identical across reruns with the same inputs. Interaction-driven shape
// mutation (e.g. via ActionTypeShapeProperty rules) keeps its existing semantics
// and is out of scope for this phase.
//
// Fail-soft: an LLM error for one group degrades that group to no-update for this
// tick; other groups and the rest of the step are unaffected.
func runShapePhase(ctx context.Context, rs *RuntimeState, llmRT *llmRuntime) {
	if llmRT == nil {
		return
	}
	groups := groupShapesByUID(rs)
	for _, g := range groups {
		updates := planShapeUpdates(ctx, rs, llmRT, g)
		applyShapeUpdates(rs, g, updates)
	}
}
