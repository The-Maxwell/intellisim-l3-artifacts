package models

import "intelli.sim.core/data/common"

// ExecLLMTargetDTO is the models-module API form of common.LLMTarget. Unlike the
// chats LLMTargetDTO it carries dynamic generation params + an explicit timeout,
// because model generation is an "execution" call that the user can fine-tune.
//
// provider_uid/model_uid are optional at the DTO layer because a draft model (or
// a per-agent/shape override that inherits the model default) may carry an empty
// target. When a target is actually USED to talk to the LLM (set chat target /
// create-with-LLM / send message / image gen), the service validates it via
// llm.Service.ValidateTarget, which rejects empty provider/model with a clear error.
type ExecLLMTargetDTO struct {
	ProviderUID string                    `json:"provider_uid" validate:"omitempty,max=100"`
	ModelUID    string                    `json:"model_uid" validate:"omitempty,max=100"`
	Effort      string                    `json:"effort,omitempty" validate:"omitempty,oneof=minimal low medium high max"`
	Generation  map[string]ConfigValueDTO `json:"generation,omitempty"`
	TimeoutMs   *uint64                   `json:"timeout_ms,omitempty" validate:"omitempty,min=1"`
}

// ConfigValueDTO is the JSON form of a single dynamic config value (exactly one set).
// Mirrors common.ConfigValue field-by-field.
type ConfigValueDTO struct {
	Double *float64 `json:"double,omitempty"`
	Int    *int64   `json:"int,omitempty"`
	Uint   *uint64  `json:"uint,omitempty"`
	Bool   *bool    `json:"bool,omitempty"`
	String *string  `json:"string,omitempty"`
	List   []string `json:"list,omitempty"`
}

// EditScopeDTO restricts an LLM edit to a section or single entity. Empty = whole model.
type EditScopeDTO struct {
	Section string `json:"section,omitempty" validate:"omitempty,oneof=agents environment rules interactions events widgets"`
	UID     string `json:"uid,omitempty" validate:"omitempty,max=100"`
}

// CreateModelRequest creates a model. With LLMTarget+InitialMessage it also creates
// the companion chat and fires the first async generation.
type CreateModelRequest struct {
	Name           string            `json:"name" validate:"omitempty,max=120"`
	Description    string            `json:"description" validate:"omitempty,max=2000"`
	IsSpatial      bool              `json:"is_spatial"`
	LLMTarget      *ExecLLMTargetDTO `json:"llm_target" validate:"omitempty"`
	InitialMessage string            `json:"initial_message" validate:"omitempty,max=10000"`
}

// UpdateModelRequest is a FULL manual replace of the editable model (form or JSON).
// LLM routing/strategy/config + locks + ownership are accepted here (this is the
// user-facing API) but are validated and persisted as-is.
type UpdateModelRequest struct {
	Model ModelInputDTO `json:"model" validate:"required"`
}

type SetModelChatTargetRequest struct {
	LLMTarget *ExecLLMTargetDTO `json:"llm_target" validate:"required"`
}

type SendModelMessageRequest struct {
	Content string        `json:"content" validate:"required,notblank,max=10000"`
	Scope   *EditScopeDTO `json:"scope,omitempty"`
}

// AddModelImageRequest adds one image to the model's environment-generation
// image library. Data is base64 (pre-compression); the service compresses it.
type AddModelImageRequest struct {
	Name        string `json:"name,omitempty" validate:"omitempty,max=200"`
	Description string `json:"description,omitempty" validate:"omitempty,max=2000"`
	MimeType    string `json:"mime_type" validate:"required,oneof=image/png image/jpeg image/webp image/gif"`
	Data        string `json:"data" validate:"required"` // base64
}

// GenerateEnvironmentRequest starts environment generation from the model's
// STORED image library (images are managed via the add/delete endpoints).
type GenerateEnvironmentRequest struct {
	Target *ExecLLMTargetDTO `json:"llm_target,omitempty"` // must support vision_analysis; nil = model chat target
	Prompt string            `json:"prompt" validate:"omitempty,max=4000"`
}

type ListModelsQuery struct {
	Page      int    `query:"page" json:"page" validate:"omitempty,min=1"`
	PageSize  int    `query:"page_size" json:"page_size" validate:"omitempty,min=1,max=100"`
	SortBy    string `query:"sort_by" json:"sort_by" validate:"omitempty,oneof=name updated_at created_by"`
	SortOrder string `query:"sort_order" json:"sort_order" validate:"omitempty,oneof=asc desc"`
}

// DuplicateModelRequest optionally overrides the name of the duplicated model
// (default: "<source name> (copy)").
type DuplicateModelRequest struct {
	Name string `json:"name" validate:"omitempty,max=120"`
}

type ListMessagesQuery struct {
	Limit  int    `query:"limit" json:"limit" validate:"omitempty,min=1,max=100"`
	Before string `query:"before" json:"before" validate:"omitempty,len=24,hexadecimal"`
}

// targetFromExecDTO converts to the domain target (validation done by llm.Service).
func targetFromExecDTO(d ExecLLMTargetDTO) common.LLMTarget {
	t := common.LLMTarget{ProviderUID: d.ProviderUID, ModelUID: d.ModelUID, Effort: common.LLMEffort(d.Effort), TimeoutMs: d.TimeoutMs}
	if len(d.Generation) > 0 {
		t.Generation = make(common.GenerationParams, len(d.Generation))
		for k, v := range d.Generation {
			t.Generation[k] = configValueFromDTO(v)
		}
	}
	return t
}

func configValueFromDTO(v ConfigValueDTO) common.ConfigValue {
	return common.ConfigValue{Double: v.Double, Int: v.Int, Uint: v.Uint, Bool: v.Bool, String: v.String, List: v.List}
}

func execDTOFromTarget(t common.LLMTarget) ExecLLMTargetDTO {
	d := ExecLLMTargetDTO{ProviderUID: t.ProviderUID, ModelUID: t.ModelUID, Effort: string(t.Effort), TimeoutMs: t.TimeoutMs}
	if len(t.Generation) > 0 {
		d.Generation = make(map[string]ConfigValueDTO, len(t.Generation))
		for k, v := range t.Generation {
			d.Generation[k] = configValueDTOFromCommon(v)
		}
	}
	return d
}

func configValueDTOFromCommon(v common.ConfigValue) ConfigValueDTO {
	return ConfigValueDTO{Double: v.Double, Int: v.Int, Uint: v.Uint, Bool: v.Bool, String: v.String, List: v.List}
}
