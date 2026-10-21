package simulations

import (
	"fmt"
	"sort"
	"strings"

	datamodels "intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// systemHeader is prepended to every generation system prompt. The literal
// "Output STRICT JSON only" substring is asserted by tests and relied on by the
// providers to suppress prose/markdown.
const systemHeader = "You initialize an agent-based simulation instance from its model. " +
	"Output STRICT JSON only, matching the given schema exactly — no prose, no markdown."

// userJSONReminder is appended to every generation USER message. The OpenAI
// Responses API (/responses) requires the literal word "json" to appear in the
// input (user) message — NOT the instructions/system field — before it will
// honour text.format=json_object. The system header alone is sent as
// instructions and does not satisfy this check, so the reminder lives here.
const userJSONReminder = "\nRespond with a single valid JSON object only.\n"

// guidanceChain assembles the "Modeling guidance:" body from an ordered chain of
// prompt-context strings (model → agent → field), skipping empties. Each
// non-empty entry becomes its own bullet so the LLM can weigh them
// independently. Returns "" when every entry is empty (callers omit the section
// entirely in that case).
func guidanceChain(parts ...string) string {
	var b strings.Builder
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		b.WriteString("- ")
		b.WriteString(p)
		b.WriteByte('\n')
	}
	return b.String()
}

// writeGuidance appends a "Modeling guidance:" section to b when chain is
// non-empty.
func writeGuidance(b *strings.Builder, chain string) {
	if chain == "" {
		return
	}
	b.WriteString("\nModeling guidance:\n")
	b.WriteString(chain)
}

// modelPromptContext returns the model-level SIM-GENERATION prompt context
// (SimulationConfig.LLMSimulationGenerationConfig — the config whose Target
// drives these calls). Model.LLMConfig.PromptContext is the MODEL-generation
// context and deliberately does NOT reach sim-generation prompts (B2).
func modelPromptContext(m datamodels.Model) string {
	return m.SimulationConfig.LLMSimulationGenerationConfig.PromptContext
}

// agentPromptContext returns the agent-level generation prompt context, guarding
// a nil LLMConfig.
func agentPromptContext(agent datamodels.Agent) string {
	if agent.LLMConfig == nil {
		return ""
	}
	return agent.LLMConfig.PromptContext
}

// propGuidance combines a property's two context hints into a single guidance
// string (LLMPromptContext + Generation.PromptContext), skipping empties.
func propGuidance(p datamodels.AgentProperty) string {
	parts := []string{strings.TrimSpace(p.LLMPromptContext)}
	if p.Generation != nil {
		parts = append(parts, strings.TrimSpace(p.Generation.PromptContext))
	}
	var kept []string
	for _, s := range parts {
		if s != "" {
			kept = append(kept, s)
		}
	}
	return strings.Join(kept, "; ")
}

// propSchemaLine renders one property as a compact schema/guidance line:
//
//	uid (type) [min..max] — description; guidance: <hints>
//
// Bounds and guidance segments are omitted when absent so prompts stay terse.
func propSchemaLine(p datamodels.AgentProperty) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s (%s)", p.UID, p.Type)

	if p.MinValue != nil || p.MaxValue != nil {
		lo, hi := "", ""
		if p.MinValue != nil {
			lo = trimFloat(*p.MinValue)
		}
		if p.MaxValue != nil {
			hi = trimFloat(*p.MaxValue)
		}
		fmt.Fprintf(&b, " [%s..%s]", lo, hi)
	}
	if len(p.AllowedValues) > 0 {
		b.WriteString(" allowed{")
		for i, v := range p.AllowedValues {
			if i > 0 {
				b.WriteString(",")
			}
			fmt.Fprintf(&b, "%v", v)
		}
		b.WriteString("}")
	}
	if d := strings.TrimSpace(p.Description); d != "" {
		fmt.Fprintf(&b, " — %s", d)
	}
	if g := propGuidance(p); g != "" {
		fmt.Fprintf(&b, "; guidance: %s", g)
	}
	return b.String()
}

func trimFloat(f float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.4f", f), "0"), ".")
}

// envOneLiner summarises the environment dimensions/topology for inclusion in a
// user message. Returns "" for non-spatial models or a nil environment.
func envOneLiner(m datamodels.Model) string {
	if !m.IsSpatial || m.Environment == nil {
		return ""
	}
	e := m.Environment
	torus := ""
	if e.Torus {
		torus = ", toroidal (wrap-around) edges"
	}
	collide := ""
	if e.CollisionMode == datamodels.CollisionBlock {
		collide = ", solid bodies (agents cannot overlap shapes/each other)"
	}
	return fmt.Sprintf("world %d×%d%s%s", e.Width, e.Height, torus, collide)
}

// footprintText renders an agent's simulation footprint for prompts (empty for a
// point/nil footprint).
func footprintText(a datamodels.Agent) string {
	f := a.Footprint
	if f == nil || f.Kind == datamodels.FootprintPoint || f.Kind == "" {
		return ""
	}
	switch f.Kind {
	case datamodels.FootprintCircle:
		if f.Radius != nil {
			return fmt.Sprintf(" [body: circle r=%s]", trimFloat(*f.Radius))
		}
	case datamodels.FootprintRectangle:
		if f.Width != nil && f.Height != nil {
			return fmt.Sprintf(" [body: %d×%d rectangle]", *f.Width, *f.Height)
		}
	case datamodels.FootprintPolygon:
		return fmt.Sprintf(" [body: %d-point polygon]", len(f.Points))
	}
	return ""
}

// --- 1. count -----------------------------------------------------------------

// buildCountPrompt builds the prompt that asks the LLM to choose a population
// count for one archetype. resolvedCounts carries the already-resolved counts of
// the other archetypes (uid -> count) so the LLM can balance the population.
func buildCountPrompt(m datamodels.Model, agent datamodels.Agent, resolvedCounts map[string]uint64) (string, string) {
	var sys strings.Builder
	sys.WriteString(systemHeader)
	sys.WriteString("\nTask: choose the initial population COUNT for one agent archetype.\n")
	sys.WriteString("Schema: {\"count\": <uint>, \"rationale\": \"<short>\"}\n")
	sys.WriteString("Example: {\"count\": 25, \"rationale\": \"balances the predator-prey ratio\"}\n")

	var user strings.Builder
	fmt.Fprintf(&user, "Model: %s — %s\n", m.Name, m.Description)
	if one := envOneLiner(m); one != "" {
		fmt.Fprintf(&user, "Environment: %s\n", one)
	}
	fmt.Fprintf(&user, "Archetype: %s — %s\n", agent.Name, agent.Description)

	if len(resolvedCounts) > 0 {
		user.WriteString("Already-resolved counts of other archetypes:\n")
		for _, uid := range sortedKeys(resolvedCounts) {
			fmt.Fprintf(&user, "- %s: %d\n", uid, resolvedCounts[uid])
		}
	}

	init := agent.Initialization
	if init.MinPopulation != nil || init.MaxPopulation != nil {
		lo, hi := "unbounded", "unbounded"
		if init.MinPopulation != nil {
			lo = fmt.Sprintf("%d", *init.MinPopulation)
		}
		if init.MaxPopulation != nil {
			hi = fmt.Sprintf("%d", *init.MaxPopulation)
		}
		fmt.Fprintf(&user, "Count bounds: min=%s, max=%s (stay within).\n", lo, hi)
	}
	fmt.Fprintf(&user, "Suggested baseline population: %d\n", init.PopulationCount)
	user.WriteString("Consider the environment's size/capacity and the other archetypes' counts; keep the ecosystem in plausible balance.\n")

	// Population/count guidance reuses the initialization placement prompt context — the
	// single initialization-level LLM hint for this agent/shape (spawn/population prompt
	// contexts no longer live on the runtime strategy).
	popCtx := ""
	if agent.Initialization.Placement != nil {
		popCtx = agent.Initialization.Placement.PromptContext
	}
	writeGuidance(&user, guidanceChain(modelPromptContext(m), agentPromptContext(agent), popCtx))

	user.WriteString(userJSONReminder)
	return sys.String(), user.String()
}

// --- 2. global values ---------------------------------------------------------

// buildGlobalValuesPrompt builds the prompt that asks the LLM to initialise the
// environment global properties.
func buildGlobalValuesPrompt(m datamodels.Model, props []datamodels.AgentProperty) (string, string) {
	var sys strings.Builder
	sys.WriteString(systemHeader)
	sys.WriteString("\nTask: assign an initial value to every listed global property.\n")
	sys.WriteString("Schema: {\"values\": {\"<uid>\": <value>}}\n")
	if ex := globalsExample(props); ex != "" {
		fmt.Fprintf(&sys, "Example: %s\n", ex)
	} else {
		sys.WriteString("Example: {\"values\": {\"temperature\": 20}}\n")
	}

	var user strings.Builder
	fmt.Fprintf(&user, "Model: %s — %s\n", m.Name, m.Description)
	if one := envOneLiner(m); one != "" {
		fmt.Fprintf(&user, "Environment: %s\n", one)
	}
	user.WriteString("Global properties:\n")
	for _, p := range props {
		fmt.Fprintf(&user, "- %s\n", propSchemaLine(p))
	}

	envCtx := ""
	if m.Environment != nil {
		envCtx = m.Environment.LLMPromptContext
	}
	writeGuidance(&user, guidanceChain(modelPromptContext(m), envCtx))

	user.WriteString(userJSONReminder)
	return sys.String(), user.String()
}

// globalsExample emits a one-property JSON example using the first prop so the
// system prompt always carries a parseable LLMGlobalValuesResponse sample.
func globalsExample(props []datamodels.AgentProperty) string {
	if len(props) == 0 {
		return ""
	}
	p := props[0]
	return fmt.Sprintf(`{"values": {%q: %s}}`, p.UID, exampleValue(p))
}

// --- 3. agent batch -----------------------------------------------------------

// buildAgentBatchPrompt builds the prompt for one batch of agent instances.
// ctxSummary is a pre-assembled one-paragraph summary (final count, environment
// one-liner, already-resolved non-LLM property stats) computed by the generator.
func buildAgentBatchPrompt(m datamodels.Model, agent datamodels.Agent, llmProps []datamodels.AgentProperty, batchStart, batchLen int, ctxSummary string) (string, string) {
	var sys strings.Builder
	sys.WriteString(systemHeader)
	fmt.Fprintf(&sys, "\nTask: produce exactly %d agent instances, numbered 0..%d.\n", batchLen, batchLen-1)
	sys.WriteString("Schema: {\"instances\":[{\"instance\":<0-based>,\"values\":{\"<uid>\":<value>}}]}\n")
	fmt.Fprintf(&sys, "Return one entry per instance index 0..%d; do not skip or repeat indices.\n", batchLen-1)
	// batchExample always returns a non-empty parseable JSON string (even for
	// empty props it produces {"instances":[{"instance":0,"values":{}}]}), so
	// no fallback is needed.
	fmt.Fprintf(&sys, "Example: %s\n", batchExample(llmProps))

	var user strings.Builder
	fmt.Fprintf(&user, "Archetype: %s — %s\n", agent.Name, agent.Description)
	fmt.Fprintf(&user, "This batch covers instances %d..%d of the population.\n", batchStart, batchStart+batchLen-1)
	if ctxSummary != "" {
		fmt.Fprintf(&user, "Context: %s\n", ctxSummary)
	}
	user.WriteString("Properties to generate per instance:\n")
	for _, p := range llmProps {
		fmt.Fprintf(&user, "- %s\n", propSchemaLine(p))
	}
	user.WriteString("Produce DIVERSE, plausible values across the instances; avoid identical rows. Keep each instance internally consistent (correlated properties should agree).\n")

	writeGuidance(&user, guidanceChain(modelPromptContext(m), agentPromptContext(agent)))

	user.WriteString(userJSONReminder)
	return sys.String(), user.String()
}

// batchExample emits a single-instance JSON example covering the supplied
// properties so the system prompt carries a parseable LLMAgentBatchResponse
// sample.
func batchExample(props []datamodels.AgentProperty) string {
	var b strings.Builder
	b.WriteString(`{"instances":[{"instance":0,"values":{`)
	for i, p := range props {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, "%q: %s", p.UID, exampleValue(p))
	}
	b.WriteString("}}]}")
	return b.String()
}

// --- 4. placement -------------------------------------------------------------

// buildPlacementPrompt builds the prompt that asks the LLM for placement
// directives for one archetype population.
func buildPlacementPrompt(m datamodels.Model, agent datamodels.Agent, count uint64, env *datasims.SimulationEnvironment, ctxSummary string) (string, string) {
	var sys strings.Builder
	sys.WriteString(systemHeader)
	sys.WriteString("\nTask: distribute the agents across the environment via placement directives.\n")
	sys.WriteString("Schema: {\"directives\":[{\"kind\":\"cluster|region|at_shape|explicit\"," +
		"\"fraction\":<0..1>,\"center\":{\"x\":<num>,\"y\":<num>},\"radius\":<num>," +
		"\"polygon\":[{\"x\":<num>,\"y\":<num>}],\"shape_uid\":\"<uid>\",\"points\":[{\"x\":<num>,\"y\":<num>}]}]}\n")
	sys.WriteString("Rules: use FEW directives (1–6). fraction is the share of the population for that directive (region/cluster/at_shape). " +
		"explicit points are allowed ONLY when the total count ≤ 50.\n")
	sys.WriteString("Example: {\"directives\":[{\"kind\":\"cluster\",\"fraction\":0.6,\"center\":{\"x\":20,\"y\":20},\"radius\":8}," +
		"{\"kind\":\"at_shape\",\"fraction\":0.4,\"shape_uid\":\"lake\"}]}\n")

	var user strings.Builder
	user.WriteString(placementEnvSummary(env))
	fmt.Fprintf(&user, "Archetype: %s — %s%s\n", agent.Name, agent.Description, footprintText(agent))
	fmt.Fprintf(&user, "Agents to place: %d\n", count)
	if ctxSummary != "" {
		fmt.Fprintf(&user, "Context: %s\n", ctxSummary)
	}

	placeCtx := ""
	if agent.Initialization.Placement != nil {
		placeCtx = agent.Initialization.Placement.PromptContext
	}
	writeGuidance(&user, guidanceChain(modelPromptContext(m), agentPromptContext(agent), placeCtx))

	user.WriteString(userJSONReminder)
	return sys.String(), user.String()
}

// placementEnvSummary renders the environment dimensions, topology, and each
// shape instance (name/type/center/size) into a compact block. Guards a nil
// environment.
func placementEnvSummary(env *datasims.SimulationEnvironment) string {
	if env == nil {
		return "Environment: (none)\n"
	}
	var b strings.Builder
	torus := ""
	if env.Torus {
		torus = ", toroidal edges"
	}
	collide := ""
	if env.CollisionMode == datamodels.CollisionBlock {
		collide = " Bodies are solid: avoid overlapping shapes/other agents."
	}
	fmt.Fprintf(&b, "Environment: %d×%d%s. Coordinates are integers in [0,%d]×[0,%d].%s\n",
		env.Width, env.Height, torus, maxCoord(env.Width), maxCoord(env.Height), collide)
	if len(env.Shapes) > 0 {
		b.WriteString("Shapes (use shape_uid for at_shape directives):\n")
		for _, s := range env.Shapes {
			fmt.Fprintf(&b, "- %s (uid=%s, %s) center=(%d,%d)%s\n",
				s.Name, s.ShapeUID, s.Type, s.Position.X, s.Position.Y, shapeSizeText(s))
		}
	}
	return b.String()
}

func shapeSizeText(s datasims.SimulationShape) string {
	switch {
	case s.Radius != nil:
		return fmt.Sprintf(" radius=%s", trimFloat(*s.Radius))
	case s.Width != nil && s.Height != nil:
		return fmt.Sprintf(" size=%d×%d", *s.Width, *s.Height)
	default:
		return ""
	}
}

func maxCoord(dim uint64) uint64 {
	if dim == 0 {
		return 0
	}
	return dim - 1
}

// --- shared helpers -----------------------------------------------------------

// exampleValue produces a representative JSON literal for a property, used to
// build parseable JSON examples in the system prompts.
func exampleValue(p datamodels.AgentProperty) string {
	switch p.Type {
	case datamodels.PropertyTypeNumber:
		if p.MinValue != nil {
			return trimFloat(*p.MinValue)
		}
		return "0"
	case datamodels.PropertyTypeBoolean:
		return "true"
	case datamodels.PropertyTypeEnum, datamodels.PropertyTypeString:
		if len(p.AllowedValues) > 0 {
			return fmt.Sprintf("%q", fmt.Sprintf("%v", p.AllowedValues[0]))
		}
		return `"example"`
	case datamodels.PropertyTypeColor:
		return `"#FF8800"`
	default:
		return `"example"`
	}
}

// sortedKeys returns the keys of a uint64-valued map in deterministic order.
func sortedKeys(m map[string]uint64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
