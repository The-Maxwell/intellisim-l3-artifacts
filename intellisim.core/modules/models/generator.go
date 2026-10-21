package models

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"go.uber.org/zap"

	datachats "intelli.sim.core/data/chats"
	"intelli.sim.core/data/common"
	datamodels "intelli.sim.core/data/models"
	"intelli.sim.core/modules/llm"
)

// EditScope restricts an LLM model-generation edit to a section or single entity.
// An empty scope means the whole model. It is the domain form of EditScopeDTO.
type EditScope struct {
	Section string
	UID     string
}

// scopeFromDTO converts the API edit-scope DTO to the domain EditScope. A nil DTO
// yields the zero (whole-model) scope.
func scopeFromDTO(d *EditScopeDTO) EditScope {
	if d == nil {
		return EditScope{}
	}
	return EditScope{Section: d.Section, UID: d.UID}
}

// MergeWarning records a single fail-soft decision made while applying an LLM
// response to the model (e.g. a locked entity preserved, an invalid field kept).
type MergeWarning struct {
	Path   string
	Reason string
}

// GenerateInput is the full context for one model-generation LLM call.
type GenerateInput struct {
	Model       datamodels.Model
	History     []datachats.ChatMessage
	UserMessage string
	Scope       EditScope
	Target      common.LLMTarget
}

// GenerateOutput is the merged model + display message + bookkeeping returned by
// a model-generation call.
type GenerateOutput struct {
	DisplayMessage string
	Model          datamodels.Model
	Raw            string
	Warnings       []MergeWarning
	Usage          common.TokenUsage
}

// VisionInput is the context for an image->environment generation call.
type VisionInput struct {
	Model  datamodels.Model
	Images []datamodels.ModelImage
	Prompt string
	Target common.LLMTarget
}

// Generator wraps the LLM service to produce/edit ABM models from natural
// language and reference images: it builds the (scoped) generation prompt,
// invokes the LLM with JSON response format, parses the envelope, and applies
// the result through the fail-soft, lock/scope-aware merge.
type Generator struct {
	logger *zap.Logger
	llm    llm.Service
}

func NewGenerator(logger *zap.Logger, llmService llm.Service) *Generator {
	return &Generator{logger: logger.Named("model-generator"), llm: llmService}
}

// Generate runs one model-generation LLM completion and merges the structural
// result into the model. The system prompt is scope-trimmed; the response is a
// strict JSON envelope (display_message + model). On success it returns the
// merged model, the display message, the raw JSON (for RawContent), the
// fail-soft merge warnings and the call's token usage.
func (g *Generator) Generate(ctx context.Context, in GenerateInput) (GenerateOutput, error) {
	sys := buildSystemPrompt(in.Model, in.Scope)
	completion, err := g.llm.Chat(ctx, llm.ChatRequest{
		Target:           in.Target,
		Purpose:          common.ModelCapabilityModelGeneration,
		Message:          in.UserMessage,
		SystemMessage:    sys,
		Messages:         in.History,
		OverflowStrategy: llm.OverflowStrategyCut,
		ChatType:         "model",
		ResponseFormat:   llm.ResponseFormatJSON,
	})
	if err != nil {
		return GenerateOutput{}, err
	}

	env, softWarns, perr := parseEnvelopeSoft(completion.Text)
	if perr != nil {
		return GenerateOutput{}, fmt.Errorf("parse llm response: %w", perr)
	}

	merged, warns := llmMerge(in.Model, env, in.Scope)
	return GenerateOutput{
		DisplayMessage: env.DisplayMessage,
		Model:          merged,
		Raw:            completion.Text,
		Warnings:       append(softWarns, warns...),
		Usage:          completion.Usage,
	}, nil
}

// GenerateEnvironmentFromImage extracts an environment from one or more reference
// images via the vision capability and merges it into the model's environment
// section. ALL supplied images are decoded (base64 → raw bytes, format derived
// from MIME) and sent in a single vision call; each image's description, when
// set, is forwarded as a label so the model can tell the images apart.
func (g *Generator) GenerateEnvironmentFromImage(ctx context.Context, in VisionInput) (GenerateOutput, error) {
	if len(in.Images) == 0 {
		return GenerateOutput{}, fmt.Errorf("no reference image supplied")
	}

	var existingEnv datamodels.Environment
	if in.Model.Environment != nil {
		existingEnv = *in.Model.Environment
	}
	sys := buildVisionSystemPrompt(existingEnv, in.Model.LLMConfig.PromptContext)

	images := make([]llm.VisionImage, 0, len(in.Images))
	for i, img := range in.Images {
		decoded, derr := base64.StdEncoding.DecodeString(img.Data)
		if derr != nil {
			return GenerateOutput{}, fmt.Errorf("decode reference image %d: %w", i, derr)
		}
		images = append(images, llm.VisionImage{
			Data:        decoded,
			Format:      imageFormatFromMime(img.MimeType),
			Description: visionImageLabel(i, img),
		})
	}

	completion, err := g.llm.Vision(ctx, llm.VisionRequest{
		Target:         in.Target,
		Purpose:        common.ModelCapabilityVisionAnalysis,
		Message:        in.Prompt,
		Images:         images,
		ResponseFormat: llm.ResponseFormatJSON,
		SystemPrompt:   sys,
	})
	if err != nil {
		return GenerateOutput{}, err
	}

	env, softWarns, perr := parseEnvelopeSoft(completion.Text)
	if perr != nil {
		return GenerateOutput{}, fmt.Errorf("parse llm response: %w", perr)
	}

	merged, warns := llmMerge(in.Model, env, EditScope{Section: "environment"})
	return GenerateOutput{
		DisplayMessage: env.DisplayMessage,
		Model:          merged,
		Raw:            completion.Text,
		Warnings:       append(softWarns, warns...),
		Usage:          completion.Usage,
	}, nil
}

// visionImageLabel builds the per-image label sent before each image in a
// vision call, so the LLM can refer to each reference distinctly. It combines
// the positional index with the image's name and/or description when present.
func visionImageLabel(i int, img datamodels.ModelImage) string {
	desc := strings.TrimSpace(img.Description)
	name := strings.TrimSpace(img.Name)
	switch {
	case desc != "" && name != "":
		return fmt.Sprintf("Reference image %d — %s: %s", i+1, name, desc)
	case desc != "":
		return fmt.Sprintf("Reference image %d: %s", i+1, desc)
	case name != "":
		return fmt.Sprintf("Reference image %d — %s", i+1, name)
	default:
		return fmt.Sprintf("Reference image %d", i+1)
	}
}

// imageFormatFromMime maps a MIME type to the short image format the vision
// request expects ("png" | "jpeg" | "webp" | "gif"). Unknown types fall back to
// the subtype after the slash (or the raw value).
func imageFormatFromMime(mime string) string {
	switch strings.ToLower(strings.TrimSpace(mime)) {
	case "image/png":
		return "png"
	case "image/jpeg", "image/jpg":
		return "jpeg"
	case "image/webp":
		return "webp"
	case "image/gif":
		return "gif"
	default:
		if i := strings.LastIndex(mime, "/"); i >= 0 && i+1 < len(mime) {
			return mime[i+1:]
		}
		return mime
	}
}

// parseEnvelope is the warning-discarding form of parseEnvelopeSoft, kept for
// call sites (and tests) that only need the envelope.
func parseEnvelope(text string) (LLMGenerationEnvelope, error) {
	env, _, err := parseEnvelopeSoft(text)
	return env, err
}

// parseEnvelopeSoft tolerantly extracts the LLM generation envelope from a raw
// completion. It strips surrounding whitespace and an optional ```json / ```
// code fence, then locates the OUTERMOST balanced {...} object (ignoring braces
// inside JSON strings) and unmarshals it. It never panics on malformed input —
// it returns a descriptive error instead.
//
// FAIL-SOFT NET (U6b): encoding/json does not abort on a type mismatch — it
// skips the offending leaf, keeps decoding, and reports the first
// *json.UnmarshalTypeError. When that happens and the partial envelope still
// carries content (a model or a display message), the envelope is accepted and
// the skipped field is surfaced as a MergeWarning instead of failing the whole
// generation — one type-loose token from the LLM must not kill the draft.
// Structurally broken JSON still fails hard.
func parseEnvelopeSoft(text string) (LLMGenerationEnvelope, []MergeWarning, error) {
	cleaned := stripCodeFence(strings.TrimSpace(text))

	obj, ok := extractJSONObject(cleaned)
	if !ok {
		return LLMGenerationEnvelope{}, nil, fmt.Errorf("no JSON object found in response")
	}

	var env LLMGenerationEnvelope
	if err := json.Unmarshal([]byte(obj), &env); err != nil {
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) && (env.Model != nil || env.DisplayMessage != "") {
			warn := MergeWarning{
				Path:   typeErr.Field,
				Reason: fmt.Sprintf("field skipped: JSON %s does not fit its schema type (default kept)", typeErr.Value),
			}
			return env, []MergeWarning{warn}, nil
		}
		return LLMGenerationEnvelope{}, nil, fmt.Errorf("unmarshal envelope: %w", err)
	}
	return env, nil, nil
}

// stripCodeFence removes a single surrounding ```...``` markdown fence (with an
// optional language tag such as ```json) if present. Input must be trimmed.
func stripCodeFence(s string) string {
	if !strings.HasPrefix(s, "```") {
		return s
	}
	// Drop the opening fence line (```), including any language tag.
	if nl := strings.IndexByte(s, '\n'); nl >= 0 {
		s = s[nl+1:]
	} else {
		return s
	}
	// Drop a trailing closing fence if present.
	if idx := strings.LastIndex(s, "```"); idx >= 0 {
		s = s[:idx]
	}
	return strings.TrimSpace(s)
}

// extractJSONObject returns the first OUTERMOST balanced {...} substring of s,
// tracking string literals and escapes so braces inside strings do not throw
// off the depth count. Returns ok=false when no balanced object exists.
func extractJSONObject(s string) (string, bool) {
	start := strings.IndexByte(s, '{')
	if start < 0 {
		return "", false
	}
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if inString {
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1], true
			}
		}
	}
	return "", false
}
