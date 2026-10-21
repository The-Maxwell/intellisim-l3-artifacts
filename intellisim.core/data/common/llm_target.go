package common

// LLMEffort is the reasoning-effort level requested from the provider for a call
// (e.g. OpenAI reasoning effort). Kept an open string so provider-specific values
// pass through; empty means "provider default".
type LLMEffort string

const (
	LLMEffortMinimal LLMEffort = "minimal"
	LLMEffortLow     LLMEffort = "low"
	LLMEffortMedium  LLMEffort = "medium"
	LLMEffortHigh    LLMEffort = "high"
	LLMEffortMax     LLMEffort = "max"
)

// ConfigValue is a single dynamic generation-config value. Exactly one field is
// non-nil. JSON form is a single-key tagged object, e.g. {"double":0.7}.
//
// Not comparable (List is a slice): use reflect.DeepEqual or inspect the active
// field pointer directly rather than ==.
type ConfigValue struct {
	Double *float64 `json:"double,omitempty"`
	Int    *int64   `json:"int,omitempty"`
	Uint   *uint64  `json:"uint,omitempty"`
	Bool   *bool    `json:"bool,omitempty"`
	String *string  `json:"string,omitempty"`
	List   []string `json:"list,omitempty"`
}

// GenerationParams is the optional, provider-validated generation tuning map.
// Nil/empty for chats; set only for models/simulations.
type GenerationParams map[string]ConfigValue

// LLMTarget identifies WHICH LLM to call and how hard it should think: the provider,
// the provider's model id, and the requested effort. It is the single shared shape for
// LLM selection across chats, models, and simulations — including per-entity overrides
// (a core idea of the meta-system: route different cases to different models, e.g. a
// cheap model for simple archetypes and a strong one for complex agents).
//
// Resolution order for Level-2 (simulation execution) calls, most specific wins:
//
//	Agent/Shape/Event .LLMTarget  >  SimulationDocument.LLMTarget  >  models.LLMConfig.Target
//
// Not comparable (Generation is a map): use field-wise comparison or
// reflect.DeepEqual rather than ==.
type LLMTarget struct {
	ProviderUID string           `json:"provider_uid,omitempty"`
	ModelUID    string           `json:"model_uid,omitempty"`  // provider's model id, e.g. "gpt-4o"
	Effort      LLMEffort        `json:"effort,omitempty"`     // empty = provider default
	Generation  GenerationParams `json:"generation,omitempty"` // models/sim only; chat leaves nil
	TimeoutMs   *uint64          `json:"timeout_ms,omitempty"` // explicit per-call deadline (ms)
}
