package models

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	datachats "intelli.sim.core/data/chats"
	"intelli.sim.core/data/common"
	datamodels "intelli.sim.core/data/models"
	"intelli.sim.core/modules/auth"
	"intelli.sim.core/modules/llm"
	"intelli.sim.core/pkg/httpapi"
	"intelli.sim.core/pkg/imageutil"
)

const defaultPageSize = 20

// ModelService is the models use-case layer. Every method requires an
// authenticated principal in ctx (auth.WithPrincipal); ownership is enforced
// here: owner or admin, others see auth.ErrForbidden.
//
// It covers model CRUD (Create/Get/Update/Delete/List), the optional companion
// chat (get-or-create + target), cursor-paged messages, async LLM generation
// (SendMessage → 202 + SSE), and image→environment generation.
type ModelService interface {
	CreateModel(ctx context.Context, req CreateModelRequest) (*ModelResponse, error)
	GetModel(ctx context.Context, id string) (*ModelResponse, error)
	UpdateModel(ctx context.Context, id string, req UpdateModelRequest) (*ModelResponse, error)
	DeleteModel(ctx context.Context, id string) error
	DuplicateModel(ctx context.Context, id string, req DuplicateModelRequest) (*ModelResponse, error)
	ListModels(ctx context.Context, q ListModelsQuery) (*ListModelsResponse, error)
	ListModelOptions(ctx context.Context) (*ListModelOptionsResponse, error)

	GetOrCreateChat(ctx context.Context, modelID string, target *ExecLLMTargetDTO) (*ModelChatResponse, error)
	SetChatTarget(ctx context.Context, modelID string, req SetModelChatTargetRequest) (*ModelChatResponse, error)
	ListMessages(ctx context.Context, modelID string, q ListMessagesQuery) (*MessagesPageResponse, error)
	SendMessage(ctx context.Context, modelID string, req SendModelMessageRequest) (*SendMessageResponse, error)

	AddModelImage(ctx context.Context, modelID string, req AddModelImageRequest) (*ModelResponse, error)
	DeleteModelImage(ctx context.Context, modelID, imageUID string) (*ModelResponse, error)
	GenerateEnvironment(ctx context.Context, modelID string, req GenerateEnvironmentRequest) (*SendMessageResponse, error)
}

type service struct {
	logger    *zap.Logger
	repo      datamodels.ModelsRepository
	chatsRepo datachats.ChatsRepository
	mapper    *ModelMapper
	llm       llm.Service
	gen       *Generator
	hub       *EventsHub
	pending   *pendingTracker
	cfg       ModuleConfig
}

func NewModelService(
	logger *zap.Logger,
	repo datamodels.ModelsRepository,
	chatsRepo datachats.ChatsRepository,
	mapper *ModelMapper,
	llmService llm.Service,
	gen *Generator,
	hub *EventsHub,
	cfg ModuleConfig,
) ModelService {
	return &service{
		logger:    logger.Named("model-service"),
		repo:      repo,
		chatsRepo: chatsRepo,
		mapper:    mapper,
		llm:       llmService,
		gen:       gen,
		hub:       hub,
		pending:   newPendingTracker(),
		cfg:       cfg,
	}
}

// ---- helpers -------------------------------------------------------------

// getOwnedModel loads a model and enforces owner-or-admin access. Missing and
// malformed-id models yield ErrModelNotFound (404); a model owned by another
// non-admin user yields auth.ErrForbidden (403).
func (s *service) getOwnedModel(ctx context.Context, id string) (*datamodels.Model, *auth.UserPrincipal, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, nil, err
	}
	model, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, common.ErrNotFound) || errors.Is(err, common.ErrInvalidID) {
			return nil, nil, ErrModelNotFound
		}
		return nil, nil, err
	}
	if err := p.Authorize(model.CreatedBy); err != nil {
		return nil, nil, err
	}
	return model, p, nil
}

// buildResponse attaches the advisory completeness validation to a model.
// Persisted models already passed ValidateShape at PUT time, so the response
// validation is the completeness result (errors + warnings + ready).
func (s *service) buildResponse(model datamodels.Model) *ModelResponse {
	v := ValidateCompleteness(model)
	resp := s.mapper.ToModelResponse(model, v)
	return &resp
}

// ---- CRUD ------------------------------------------------------------------

func (s *service) CreateModel(ctx context.Context, req CreateModelRequest) (*ModelResponse, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	model := datamodels.Model{
		Name:        strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
		IsSpatial:   req.IsSpatial,
		SimulationConfig: datamodels.SimulationConfig{
			UpdateMode: datamodels.UpdateModeSequential,
		},
		CreatedAt: now,
		UpdatedAt: now,
		CreatedBy: p.ID,
		UpdatedBy: p.ID,
	}
	// New models default archetype agents to distribution sampling (only empties are
	// stamped; this runs at create, never on generic update). Must precede Canonical.
	model.StampDefaultArchetypeMode()
	// Spatial models auto-get the conventional "color" property on every agent/shape
	// that lacks one (create-only, backward-safe; see StampDefaultColorProperty).
	model.StampDefaultColorProperty()
	// Canonicalize strategy flags by Type so persisted data is always consistent.
	model.NormalizeLLMStrategies()
	model.NormalizeSpatial()
	// Coerce numeric-looking string rule condition/action values ("1") on
	// declared-numeric properties so they behave identically to a numeric
	// submission in the run engine's compares/writes (ISS-06).
	NormalizeRuleValues(&model)
	if err := s.compressDisplayImages(&model); err != nil {
		return nil, err
	}

	id, err := s.repo.Create(ctx, model)
	if err != nil {
		s.logger.Error("failed to create model", zap.Error(err))
		return nil, err
	}
	model.ID = id
	s.logger.Info("model created", zap.String("model_id", id), zap.String("owner", p.ID))

	// LLM kickoff: with a target + an initial message, create the companion chat
	// and fire the first async generation, mirroring SendMessage. Failures here are
	// surfaced to the caller (the model itself is already persisted).
	if req.LLMTarget != nil && strings.TrimSpace(req.InitialMessage) != "" {
		chatResp, err := s.GetOrCreateChat(ctx, id, req.LLMTarget)
		if err != nil {
			return nil, err
		}
		chat, err := s.chatsRepo.GetByID(ctx, chatResp.ChatID)
		if err != nil {
			return nil, err
		}
		model.ChatID = chat.ID
		if _, err := s.startGeneration(ctx, model, *chat, p, req.InitialMessage, EditScope{}); err != nil {
			return nil, err
		}
	}

	return s.buildResponse(model), nil
}

func (s *service) GetModel(ctx context.Context, id string) (*ModelResponse, error) {
	model, _, err := s.getOwnedModel(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.buildResponse(*model), nil
}

func (s *service) UpdateModel(ctx context.Context, id string, req UpdateModelRequest) (*ModelResponse, error) {
	model, p, err := s.getOwnedModel(ctx, id)
	if err != nil {
		return nil, err
	}

	updated := s.mapper.FromInputDTO(req.Model)
	// Carry over server-owned fields the editable body does not include.
	updated.ID = model.ID
	updated.ChatID = model.ChatID
	updated.CreatedAt = model.CreatedAt
	updated.CreatedBy = model.CreatedBy
	updated.UpdatedAt = time.Now().UTC()
	updated.UpdatedBy = p.ID
	// The environment-generation image library is managed exclusively via the
	// dedicated POST/DELETE /:id/images endpoints — never through a full-model PUT.
	// Keep the persisted library so an editor save can't clobber or wipe it.
	updated.Images = model.Images

	// Canonicalize strategy flags by Type before validation/persist so storage,
	// validateSimulationConfig and the run engine all observe the same flags.
	updated.NormalizeLLMStrategies()
	updated.NormalizeSpatial()
	// Coerce numeric-looking string rule condition/action values ("1") on
	// declared-numeric properties so they behave identically to a numeric
	// submission in the run engine's compares/writes (ISS-06).
	NormalizeRuleValues(&updated)
	if err := s.compressDisplayImages(&updated); err != nil {
		return nil, err
	}

	// Field-shape validation gates the save: a non-empty result is a 422.
	if issues := ValidateShape(updated); len(issues) > 0 {
		return nil, shapeValidationError(issues)
	}

	// Conditional LLM-config validation: a model that uses the LLM for generation
	// or execution must declare a valid target with the right capability.
	if err := s.validateSimulationConfig(updated); err != nil {
		return nil, err
	}

	if err := s.repo.Update(ctx, id, updated); err != nil {
		s.logger.Error("failed to update model", zap.String("id", id), zap.Error(err))
		return nil, err
	}

	resp := s.buildResponse(updated)
	s.hub.Publish(id, Event{Type: EventModelUpdated, ModelID: id,
		Payload: ModelUpdatedPayload{ModelID: id, Model: *resp}})
	return resp, nil
}

func (s *service) DeleteModel(ctx context.Context, id string) error {
	model, _, err := s.getOwnedModel(ctx, id)
	if err != nil {
		return err
	}
	if model.ChatID != "" {
		if err := s.chatsRepo.DeleteMessagesByChatID(ctx, model.ChatID); err != nil {
			s.logger.Error("failed to delete model chat messages", zap.String("id", id), zap.Error(err))
			return err
		}
		if err := s.chatsRepo.Delete(ctx, model.ChatID); err != nil {
			s.logger.Error("failed to delete model chat", zap.String("id", id), zap.Error(err))
			return err
		}
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		s.logger.Error("failed to delete model", zap.String("id", id), zap.Error(err))
		return err
	}
	s.hub.Publish(id, Event{Type: EventModelDeleted, ModelID: id, Payload: ModelDeletedPayload{ModelID: id}})
	s.hub.CloseChat(id)
	return nil
}

// DuplicateModel deep-copies a model (new id + name + ownership/timestamps) and,
// if the source has a companion chat, clones that chat — its target/config plus
// the full message history — re-linked to the new model. The chat clone is
// best-effort: a failure logs a warning and leaves the duplicate chat-less
// rather than failing the whole operation (the model copy already persisted).
func (s *service) DuplicateModel(ctx context.Context, id string, req DuplicateModelRequest) (*ModelResponse, error) {
	src, p, err := s.getOwnedModel(ctx, id)
	if err != nil {
		return nil, err
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = src.Name + " (copy)"
	}

	now := time.Now().UTC()
	// A shallow value-copy is sufficient: dup is serialized immediately by
	// repo.Create (ModelToDocument) and src is discarded, so no nested mutation
	// can alias back. This carries over Agents/Rules/Interactions/Events/
	// Environment/Images/Widgets and both LLM configs verbatim.
	dup := *src
	dup.ID = ""
	dup.ChatID = ""
	dup.Name = name
	dup.CreatedAt = now
	dup.UpdatedAt = now
	dup.CreatedBy = p.ID
	dup.UpdatedBy = p.ID

	newID, err := s.repo.Create(ctx, dup)
	if err != nil {
		s.logger.Error("failed to duplicate model", zap.String("source_id", id), zap.Error(err))
		return nil, err
	}
	dup.ID = newID

	if src.ChatID != "" {
		if newChatID, cerr := s.cloneChat(ctx, src.ChatID, newID, p.ID, now); cerr != nil {
			s.logger.Warn("failed to clone model chat on duplicate; duplicate left chat-less",
				zap.String("source_id", id), zap.String("new_id", newID), zap.Error(cerr))
		} else {
			dup.ChatID = newChatID
			if uerr := s.repo.Update(ctx, newID, dup); uerr != nil {
				s.logger.Warn("failed to attach cloned chat to duplicate model",
					zap.String("new_id", newID), zap.Error(uerr))
			}
		}
	}

	s.logger.Info("model duplicated", zap.String("source_id", id), zap.String("model_id", newID))
	return s.buildResponse(dup), nil
}

// cloneChat copies a chat (re-linked to newModelID via ReferenceID) and its full
// message history (re-inserted chronologically) into a fresh chat, returning the
// new chat id.
func (s *service) cloneChat(ctx context.Context, srcChatID, newModelID, ownerID string, now time.Time) (string, error) {
	srcChat, err := s.chatsRepo.GetByID(ctx, srcChatID)
	if err != nil {
		return "", err
	}
	newChat := *srcChat
	newChat.ID = ""
	newChat.ReferenceID = newModelID
	newChat.CreatedAt = now
	newChat.UpdatedAt = now
	newChat.CreatedBy = ownerID
	newChat.UpdatedBy = ownerID
	newChatID, err := s.chatsRepo.Create(ctx, newChat)
	if err != nil {
		return "", err
	}

	// Collect the full history. ListMessagesBefore returns newest-first pages;
	// page[len-1] is the oldest in the page → the cursor for the next page.
	var msgs []datachats.ChatMessage
	before := ""
	for {
		page, hasMore, perr := s.chatsRepo.ListMessagesBefore(ctx, srcChatID, 100, before)
		if perr != nil {
			return "", perr
		}
		msgs = append(msgs, page...)
		if !hasMore || len(page) == 0 {
			break
		}
		before = page[len(page)-1].ID
	}
	// Re-insert oldest-first so the copy reads chronologically; reset each id so
	// the store mints a fresh one, and re-link to the new chat.
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		m.ID = ""
		m.ChatID = newChatID
		if _, aerr := s.chatsRepo.AddMessage(ctx, newChatID, m); aerr != nil {
			return "", aerr
		}
	}
	return newChatID, nil
}

func (s *service) ListModels(ctx context.Context, q ListModelsQuery) (*ListModelsResponse, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	page, pageSize := q.Page, q.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = defaultPageSize
	}
	owner := p.ID
	if p.IsAdmin {
		owner = "" // admins list everyone's models
	}
	filter := datamodels.ListFilter{
		CreatedBy: owner,
		Page:      page,
		PageSize:  pageSize,
		SortBy:    q.SortBy,
		SortOrder: q.SortOrder,
	}
	models, hasMore, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	total, err := s.repo.Count(ctx, filter)
	if err != nil {
		return nil, err
	}
	out := make([]ModelSummaryResponse, 0, len(models))
	for _, m := range models {
		v := ValidateCompleteness(m)
		out = append(out, s.mapper.ToSummary(m, m.ChatID != "", v.SimulationReady))
	}
	return &ListModelsResponse{Models: out, Page: page, PageSize: pageSize, Total: int(total), HasMore: hasMore}, nil
}

// ListModelOptions returns the id+name of every model the caller may see
// (admin: all), unpaginated — backs selector UIs like the simulations model filter.
func (s *service) ListModelOptions(ctx context.Context) (*ListModelOptionsResponse, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	owner := p.ID
	if p.IsAdmin {
		owner = "" // admins see everyone's models
	}
	opts, err := s.repo.ListOptions(ctx, owner)
	if err != nil {
		return nil, err
	}
	out := make([]ModelOptionResponse, len(opts))
	for i, o := range opts {
		out[i] = ModelOptionResponse{ID: o.ID, Name: o.Name}
	}
	return &ListModelOptionsResponse{Options: out}, nil
}

// ---- chat helpers ----------------------------------------------------------

const defaultMessageLimit = 30

// validateGenTarget validates a target for the model-generation capability,
// mapping the llm-layer sentinels to the models-module sentinels.
func (s *service) validateGenTarget(target common.LLMTarget) error {
	return mapTargetError(s.llm.ValidateTarget(target, common.ModelCapabilityModelGeneration))
}

// validateSimulationConfig enforces, at model save, that the simulation-generation and
// simulation-execution LLM targets are present and capability-valid whenever the model
// actually uses the LLM for those phases (per Model.UsesLLMForGeneration/Execution).
// A model-generation target, when set, is also validated for model_generation.
func (s *service) validateSimulationConfig(m datamodels.Model) error {
	if t := m.LLMConfig.Target; t.ProviderUID != "" {
		if err := s.validateGenTarget(t); err != nil {
			return err
		}
	}
	if m.UsesLLMForGeneration() {
		t := m.SimulationConfig.LLMSimulationGenerationConfig.Target
		if t.ProviderUID == "" || t.ModelUID == "" {
			return ErrSimGenTargetRequired
		}
		if err := mapTargetError(s.llm.ValidateTarget(t, common.ModelCapabilitySimulationGeneration)); err != nil {
			return err
		}
	}
	if m.UsesLLMForExecution() {
		t := m.SimulationConfig.LLMSimulationExecutionConfig.Target
		if t.ProviderUID == "" || t.ModelUID == "" {
			return ErrSimExecTargetRequired
		}
		for _, capb := range m.RequiredExecutionCapabilities() {
			if err := mapTargetError(s.llm.ValidateTarget(t, capb)); err != nil {
				return err
			}
		}
	}
	return nil
}

// mapTargetError translates llm.Service.ValidateTarget failures to the
// models-module sentinels the API layer maps to HTTP status codes.
func mapTargetError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, llm.ErrCapabilityNotSupported):
		return ErrTargetCapability
	case errors.Is(err, llm.ErrEffortNotAllowed):
		return ErrEffortNotAllowed
	default:
		return ErrModelUnavailable
	}
}

// toChatResponse builds the chat sub-resource view of a model + its companion
// chat (live status from the pending tracker, live availability re-check).
func (s *service) toChatResponse(modelID string, chat datachats.Chat) *ModelChatResponse {
	status := ModelChatStatusIdle
	if s.pending.isThinking(modelID) {
		status = ModelChatStatusThinking
	}
	available := s.llm.ValidateTarget(chat.LLMTarget, common.ModelCapabilityModelGeneration) == nil
	return &ModelChatResponse{
		ModelID:        modelID,
		ChatID:         chat.ID,
		LLMTarget:      execDTOFromTarget(chat.LLMTarget),
		ModelAvailable: available,
		Status:         status,
		TokenUsage:     chat.TokenUsage,
	}
}

// reverseMessages flips the repository's newest-first order to chronological.
func reverseMessages(msgs []datachats.ChatMessage) []datachats.ChatMessage {
	out := make([]datachats.ChatMessage, len(msgs))
	for i, m := range msgs {
		out[len(msgs)-1-i] = m
	}
	return out
}

// ---- chat + generation -----------------------------------------------------

func (s *service) GetOrCreateChat(ctx context.Context, modelID string, target *ExecLLMTargetDTO) (*ModelChatResponse, error) {
	model, p, err := s.getOwnedModel(ctx, modelID)
	if err != nil {
		return nil, err
	}

	if model.ChatID != "" {
		chat, err := s.chatsRepo.GetByID(ctx, model.ChatID)
		if err != nil {
			return nil, err
		}
		return s.toChatResponse(modelID, *chat), nil
	}

	// No companion chat yet — a target is required to create one.
	if target == nil {
		return nil, ErrNoChat
	}
	t := targetFromExecDTO(*target)
	if err := s.validateGenTarget(t); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	name := strings.TrimSpace(model.Name)
	if name == "" {
		name = "Model"
	}
	chat := datachats.Chat{
		Type:        datachats.ChatTypeModel,
		Name:        name,
		LLMTarget:   t,
		ReferenceID: modelID,
		CreatedAt:   now,
		UpdatedAt:   now,
		CreatedBy:   p.ID,
		UpdatedBy:   p.ID,
	}
	chatID, err := s.chatsRepo.Create(ctx, chat)
	if err != nil {
		s.logger.Error("failed to create model chat", zap.String("model_id", modelID), zap.Error(err))
		return nil, err
	}
	chat.ID = chatID

	model.ChatID = chatID
	model.UpdatedAt = now
	if err := s.repo.Update(ctx, modelID, *model); err != nil {
		s.logger.Error("failed to attach chat to model", zap.String("model_id", modelID), zap.Error(err))
		return nil, err
	}
	s.logger.Info("model chat created", zap.String("model_id", modelID), zap.String("chat_id", chatID))
	return s.toChatResponse(modelID, chat), nil
}

func (s *service) SetChatTarget(ctx context.Context, modelID string, req SetModelChatTargetRequest) (*ModelChatResponse, error) {
	model, p, err := s.getOwnedModel(ctx, modelID)
	if err != nil {
		return nil, err
	}

	// No chat yet: create it with the requested target (validates inside).
	if model.ChatID == "" {
		return s.GetOrCreateChat(ctx, modelID, req.LLMTarget)
	}

	t := targetFromExecDTO(*req.LLMTarget)
	if err := s.validateGenTarget(t); err != nil {
		return nil, err
	}
	chat, err := s.chatsRepo.GetByID(ctx, model.ChatID)
	if err != nil {
		return nil, err
	}
	chat.LLMTarget = t
	chat.UpdatedAt = time.Now().UTC()
	chat.UpdatedBy = p.ID
	if err := s.chatsRepo.Update(ctx, chat.ID, *chat); err != nil {
		s.logger.Error("failed to update model chat target", zap.String("model_id", modelID), zap.Error(err))
		return nil, err
	}
	return s.toChatResponse(modelID, *chat), nil
}

func (s *service) ListMessages(ctx context.Context, modelID string, q ListMessagesQuery) (*MessagesPageResponse, error) {
	model, _, err := s.getOwnedModel(ctx, modelID)
	if err != nil {
		return nil, err
	}
	if model.ChatID == "" {
		return &MessagesPageResponse{Messages: []ModelChatMessageResponse{}, HasMore: false}, nil
	}

	limit := q.Limit
	if limit < 1 {
		limit = defaultMessageLimit
	}
	if q.Before != "" {
		cursorMsg, err := s.chatsRepo.GetMessageByID(ctx, q.Before)
		if err != nil || cursorMsg.ChatID != model.ChatID {
			return nil, ErrInvalidCursor
		}
	}
	msgs, hasMore, err := s.chatsRepo.ListMessagesBefore(ctx, model.ChatID, limit, q.Before)
	if err != nil {
		return nil, err
	}
	return &MessagesPageResponse{
		Messages: s.mapper.ToMessagesResponse(reverseMessages(msgs)),
		HasMore:  hasMore,
	}, nil
}

func (s *service) SendMessage(ctx context.Context, modelID string, req SendModelMessageRequest) (*SendMessageResponse, error) {
	model, p, err := s.getOwnedModel(ctx, modelID)
	if err != nil {
		return nil, err
	}
	if model.ChatID == "" {
		return nil, ErrNoChat
	}
	chat, err := s.chatsRepo.GetByID(ctx, model.ChatID)
	if err != nil {
		return nil, err
	}
	if err := s.validateGenTarget(chat.LLMTarget); err != nil {
		return nil, err
	}
	return s.startGeneration(ctx, *model, *chat, p, req.Content, scopeFromDTO(req.Scope))
}

// startGeneration is the shared kickoff used by SendMessage and the CreateModel
// LLM-kickoff: it reserves the per-model generation slot, persists the user
// message, publishes the created + thinking events, and spawns the background
// generation (detached from the request so it survives the 202 return). The
// caller must have already validated the chat target.
func (s *service) startGeneration(
	ctx context.Context,
	model datamodels.Model,
	chat datachats.Chat,
	p *auth.UserPrincipal,
	content string,
	scope EditScope,
) (*SendMessageResponse, error) {
	if !s.pending.tryStart(model.ID) {
		return nil, ErrGenerationInProgress
	}

	now := time.Now().UTC()
	userMsg := datachats.ChatMessage{
		ChatID:      chat.ID,
		Role:        datachats.ChatMessageUserRole,
		TextContent: strings.TrimSpace(content),
		CreatedAt:   now,
		CreatedBy:   p.ID,
	}
	msgID, err := s.chatsRepo.AddMessage(ctx, chat.ID, userMsg)
	if err != nil {
		s.pending.finish(model.ID)
		s.logger.Error("failed to save user message", zap.String("model_id", model.ID), zap.Error(err))
		return nil, err
	}
	userMsg.ID = msgID

	userDTO := s.mapper.ToMessageResponse(userMsg)
	s.hub.Publish(model.ID, Event{Type: EventMessageCreated, ModelID: model.ID, Payload: userDTO})
	s.hub.Publish(model.ID, Event{Type: EventChatStatus, ModelID: model.ID,
		Payload: ChatStatusPayload{ModelID: model.ID, Status: ModelChatStatusThinking}})

	// Detach from the HTTP request: the generation must survive the 202 return.
	go s.generate(context.WithoutCancel(ctx), model, chat, userMsg, scope)

	return &SendMessageResponse{Message: userDTO, ChatStatus: ModelChatStatusThinking}, nil
}

// generate runs one background model-generation completion and persists the
// outcome: a normal assistant message + the merged model on success, or an
// error-marked assistant message on failure (so history stays truthful after
// reload). Always ends by publishing status idle. Mirrors chats.generate.
func (s *service) generate(
	ctx context.Context,
	model datamodels.Model,
	chat datachats.Chat,
	userMsg datachats.ChatMessage,
	scope EditScope,
) {
	defer func() {
		// finish BEFORE publishing idle so a client that reacts to the idle
		// event can never read a stale "thinking" status from the tracker.
		s.pending.finish(model.ID)
		s.hub.Publish(model.ID, Event{Type: EventChatStatus, ModelID: model.ID,
			Payload: ChatStatusPayload{ModelID: model.ID, Status: ModelChatStatusIdle}})
	}()

	callCtx, cancel := context.WithTimeout(ctx, s.cfg.GenerationTimeout)
	defer cancel()

	history, err := s.loadHistory(callCtx, chat.ID, userMsg.ID)
	if err != nil {
		s.logger.Error("failed to load model chat history", zap.String("model_id", model.ID), zap.Error(err))
		s.hub.Publish(model.ID, Event{Type: EventChatError, ModelID: model.ID,
			Payload: ChatErrorPayload{ModelID: model.ID, Code: "internal", Message: "failed to load chat history"}})
		return
	}

	out, llmErr := s.gen.Generate(callCtx, GenerateInput{
		Model:       model,
		History:     history,
		UserMessage: userMsg.TextContent,
		Scope:       scope,
		Target:      chat.LLMTarget,
	})

	now := time.Now().UTC()
	target := chat.LLMTarget
	assistantMsg := datachats.ChatMessage{
		ChatID:    chat.ID,
		Role:      datachats.ChatMessageAssistantRole,
		LLMTarget: &target,
		CreatedAt: now,
		CreatedBy: fmt.Sprintf("llm:%s", chat.LLMTarget.ProviderUID),
	}
	if llmErr != nil {
		code, message := classifyLLMError(llmErr)
		assistantMsg.Error = &datachats.MessageError{Code: code, Message: message}
		s.logger.Warn("model generation failed",
			zap.String("model_id", model.ID), zap.String("code", code), zap.Error(llmErr))
	} else {
		assistantMsg.TextContent = out.DisplayMessage
		assistantMsg.RawContent = out.Raw
		usage := out.Usage
		assistantMsg.TokenUsage = &usage
		for _, w := range out.Warnings {
			s.logger.Info("model merge warning",
				zap.String("model_id", model.ID), zap.String("path", w.Path), zap.String("reason", w.Reason))
		}
	}

	// Persist on a fresh deadline — callCtx may already be expired (timeout).
	persistCtx, persistCancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer persistCancel()

	msgID, err := s.chatsRepo.AddMessage(persistCtx, chat.ID, assistantMsg)
	if err != nil {
		s.logger.Error("failed to save assistant message", zap.String("model_id", model.ID), zap.Error(err))
		s.hub.Publish(model.ID, Event{Type: EventChatError, ModelID: model.ID,
			Payload: ChatErrorPayload{ModelID: model.ID, Code: "internal", Message: "failed to save assistant message"}})
		return
	}
	assistantMsg.ID = msgID

	// On a failed generation, publish the error-marked assistant message + a
	// chat.error and stop (no model change to persist).
	if llmErr != nil {
		s.hub.Publish(model.ID, Event{Type: EventMessageCreated, ModelID: model.ID,
			Payload: s.mapper.ToMessageResponse(assistantMsg)})
		s.hub.Publish(model.ID, Event{Type: EventChatError, ModelID: model.ID,
			Payload: ChatErrorPayload{ModelID: model.ID, Code: assistantMsg.Error.Code, Message: assistantMsg.Error.Message}})
		return
	}

	// Persist the merged model. out.Model is derived from model, so ID/ChatID/
	// CreatedAt/CreatedBy are already preserved; only freshness/authorship change.
	merged := out.Model
	merged.UpdatedAt = now
	merged.UpdatedBy = fmt.Sprintf("llm:%s", chat.LLMTarget.ProviderUID)
	// Generated models default archetype agents to distribution sampling (only empty
	// modes are stamped, so any explicit user choice is preserved). Must precede Canonical.
	merged.StampDefaultArchetypeMode()
	// Spatial models auto-get the conventional "color" property on every agent/shape that
	// lacks one, so LLM-generated entities always render with a colour (gen-only,
	// backward-safe; an LLM-supplied colour property is preserved).
	merged.StampDefaultColorProperty()
	// LLM-authored strategies are canonicalized too, so generated models obey the
	// same Type→flags rules as manually edited ones.
	merged.NormalizeLLMStrategies()
	merged.NormalizeSpatial()
	// Coerce numeric-looking string rule condition/action values ("1") on
	// declared-numeric properties so LLM-authored rules behave identically to a
	// numeric submission in the run engine's compares/writes (ISS-06).
	NormalizeRuleValues(&merged)
	// TODO(concurrency): full-document replace of a model snapshot taken at send
	// time — a concurrent manual UpdateModel (PUT) that lands during generation is
	// silently overwritten (higher impact than chats: it can drop manual edits or
	// resurrect deleted entities). ErrGenerationInProgress only guards against two
	// concurrent generations, not against a manual edit. Acceptable single-user dev
	// trade-off; revisit before multi-user (e.g. optimistic version check).
	if err := s.repo.Update(persistCtx, model.ID, merged); err != nil {
		s.logger.Error("failed to persist generated model", zap.String("model_id", model.ID), zap.Error(err))
		s.hub.Publish(model.ID, Event{Type: EventChatError, ModelID: model.ID,
			Payload: ChatErrorPayload{ModelID: model.ID, Code: "internal", Message: "failed to persist generated model"}})
		return
	}

	// Maintain the chat's running usage aggregate + freshness.
	if assistantMsg.TokenUsage != nil {
		if chat.TokenUsage == nil {
			chat.TokenUsage = &common.TokenUsageSummary{}
		}
		chat.TokenUsage.Add(*assistantMsg.TokenUsage)
	}
	chat.UpdatedAt = now
	if err := s.chatsRepo.Update(persistCtx, chat.ID, chat); err != nil {
		s.logger.Error("failed to update model chat aggregate", zap.String("model_id", model.ID), zap.Error(err))
	}

	s.hub.Publish(model.ID, Event{Type: EventMessageCreated, ModelID: model.ID,
		Payload: s.mapper.ToMessageResponse(assistantMsg)})
	s.hub.Publish(model.ID, Event{Type: EventModelUpdated, ModelID: model.ID,
		Payload: ModelUpdatedPayload{ModelID: model.ID, Model: *s.buildResponse(merged)}})
}

// loadHistory returns up to HistoryLimit model-chat messages OLDER than the new
// user message, chronological, with failed generations filtered out.
func (s *service) loadHistory(ctx context.Context, chatID, beforeMsgID string) ([]datachats.ChatMessage, error) {
	msgs, _, err := s.chatsRepo.ListMessagesBefore(ctx, chatID, s.cfg.HistoryLimit, beforeMsgID)
	if err != nil {
		return nil, err
	}
	history := make([]datachats.ChatMessage, 0, len(msgs))
	for _, m := range reverseMessages(msgs) {
		if m.Error == nil {
			history = append(history, m)
		}
	}
	return history, nil
}

// classifyLLMError maps llm-layer failures to persisted/API error codes.
// Mirrors chats.classifyLLMError.
func classifyLLMError(err error) (code, message string) {
	var providerErr *llm.ProviderError
	switch {
	case errors.Is(err, llm.ErrProviderNotFound),
		errors.Is(err, llm.ErrModelNotFound),
		errors.Is(err, llm.ErrProviderDisconnected):
		return "model_unavailable", err.Error()
	case errors.Is(err, llm.ErrEffortNotAllowed):
		return "effort_not_allowed", err.Error()
	case errors.Is(err, llm.ErrCapabilityNotSupported):
		return "capability_not_supported", err.Error()
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout", "generation timed out"
	case errors.As(err, &providerErr):
		return string(providerErr.Code), providerErr.Message
	default:
		return "internal", err.Error()
	}
}

// ---- image -> environment --------------------------------------------------

// AddModelImage compresses one uploaded image and appends it to the model's
// environment-generation image library (capped at cfg.MaxModelImages). Returns the
// full updated model so the frontend has a single state sink (updated_at advances,
// so the SSE model.updated echo is a clean no-op).
func (s *service) AddModelImage(ctx context.Context, modelID string, req AddModelImageRequest) (*ModelResponse, error) {
	model, p, err := s.getOwnedModel(ctx, modelID)
	if err != nil {
		return nil, err
	}
	if max := s.cfg.MaxModelImages; max > 0 && len(model.Images) >= max {
		return nil, httpapi.NewAPIError(http.StatusUnprocessableEntity, "too_many_images",
			fmt.Sprintf("a model may hold at most %d images — delete one before adding another", max))
	}
	// Reject an absurd upload before paying for a decode + compress.
	if s.cfg.MaxImageBytes > 0 && base64.StdEncoding.DecodedLen(len(req.Data)) > s.cfg.MaxImageBytes {
		return nil, httpapi.NewAPIError(http.StatusUnprocessableEntity, "image_too_large",
			fmt.Sprintf("image exceeds the %d byte upload limit", s.cfg.MaxImageBytes))
	}
	res, cerr := imageutil.Compress(req.Data, s.modelImageMaxDim(), s.modelImageMaxBytes())
	if cerr != nil {
		if errors.Is(cerr, imageutil.ErrTooLarge) {
			return nil, httpapi.NewAPIError(http.StatusUnprocessableEntity, "image_too_large",
				fmt.Sprintf("image exceeds the %d byte limit even after compression", s.modelImageMaxBytes()))
		}
		return nil, httpapi.NewAPIError(http.StatusUnprocessableEntity, "image_invalid",
			"image could not be decoded")
	}

	model.Images = append(model.Images, datamodels.ModelImage{
		UID:         uuid.NewString(),
		Name:        strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
		MimeType:    res.MimeType,
		Data:        res.DataB64,
	})
	model.UpdatedAt = time.Now().UTC()
	model.UpdatedBy = p.ID
	if err := s.repo.Update(ctx, model.ID, *model); err != nil {
		s.logger.Error("failed to persist added image", zap.String("model_id", model.ID), zap.Error(err))
		return nil, err
	}

	resp := s.buildResponse(*model)
	s.hub.Publish(model.ID, Event{Type: EventModelUpdated, ModelID: model.ID,
		Payload: ModelUpdatedPayload{ModelID: model.ID, Model: *resp}})
	return resp, nil
}

// DeleteModelImage removes one image (by UID) from the model's image library and
// returns the full updated model (symmetric with AddModelImage, so the frontend
// has a single state sink and the SSE echo is a clean no-op).
func (s *service) DeleteModelImage(ctx context.Context, modelID, imageUID string) (*ModelResponse, error) {
	model, p, err := s.getOwnedModel(ctx, modelID)
	if err != nil {
		return nil, err
	}
	kept := make([]datamodels.ModelImage, 0, len(model.Images))
	for _, img := range model.Images {
		if img.UID != imageUID {
			kept = append(kept, img)
		}
	}
	if len(kept) == len(model.Images) {
		return nil, httpapi.NewAPIError(http.StatusNotFound, "image_not_found", "image not found on this model")
	}
	model.Images = kept
	model.UpdatedAt = time.Now().UTC()
	model.UpdatedBy = p.ID
	if err := s.repo.Update(ctx, model.ID, *model); err != nil {
		s.logger.Error("failed to persist image deletion", zap.String("model_id", model.ID), zap.Error(err))
		return nil, err
	}

	resp := s.buildResponse(*model)
	s.hub.Publish(model.ID, Event{Type: EventModelUpdated, ModelID: model.ID,
		Payload: ModelUpdatedPayload{ModelID: model.ID, Model: *resp}})
	return resp, nil
}

// GenerateEnvironment starts async environment generation from the model's STORED
// image library. The selected vision model's advertised max-images limit gates the
// attached count (422 too_many_images_for_model when exceeded).
func (s *service) GenerateEnvironment(ctx context.Context, modelID string, req GenerateEnvironmentRequest) (*SendMessageResponse, error) {
	model, _, err := s.getOwnedModel(ctx, modelID)
	if err != nil {
		return nil, err
	}
	if !model.IsSpatial {
		return nil, ErrNotSpatial
	}
	if len(model.Images) == 0 {
		return nil, httpapi.NewAPIError(http.StatusUnprocessableEntity, "no_images",
			"attach at least one image before generating the environment")
	}

	// Pick + validate the vision target. An explicit request target is validated
	// for vision_analysis; otherwise fall back to the model chat's target if it
	// supports vision. Either way the chosen target must support vision.
	target, err := s.resolveVisionTarget(ctx, *model, req.Target)
	if err != nil {
		return nil, err
	}

	// Gate the attached image count against the model's advertised max.
	advertised, ok := s.llm.MaxImages(target.ProviderUID, target.ModelUID)
	max := resolveMaxImages(advertised, ok)
	if len(model.Images) > max {
		return nil, &httpapi.APIError{
			Status:  http.StatusUnprocessableEntity,
			Code:    "too_many_images_for_model",
			Message: fmt.Sprintf("the selected model %q accepts at most %d image(s); remove %d before generating",
				target.ModelUID, max, len(model.Images)-max),
			Fields: map[string]string{"images": fmt.Sprintf("remove %d image(s)", len(model.Images)-max)},
		}
	}

	if !s.pending.tryStart(modelID) {
		return nil, ErrGenerationInProgress
	}

	// Detached background vision call. We run WITHOUT a chat message here: the
	// image path may use an ad-hoc vision target that differs from the
	// model-generation chat target, so we only emit model.updated and do not
	// persist a conversation turn.
	go s.generateEnvironment(context.WithoutCancel(ctx), *model, target, req.Prompt)

	return &SendMessageResponse{ChatStatus: ModelChatStatusThinking}, nil
}

// resolveMaxImages resolves the effective per-request image cap for a vision
// model. An unregistered model or an unset (0) advertised value resolves to 1 —
// the conservative single-image default that preserves legacy behavior.
func resolveMaxImages(advertised uint32, ok bool) int {
	if !ok || advertised == 0 {
		return 1
	}
	return int(advertised)
}

func (s *service) modelImageMaxDim() int {
	if s.cfg.ModelImageMaxDim > 0 {
		return s.cfg.ModelImageMaxDim
	}
	return 1536
}

func (s *service) modelImageMaxBytes() int {
	if s.cfg.ModelImageMaxBytes > 0 {
		return s.cfg.ModelImageMaxBytes
	}
	return 1572864
}

// resolveVisionTarget chooses and validates the vision target for an image->env
// call: an explicit request target (validated for vision) wins; otherwise the
// model chat's target is reused if it supports vision. Capability mismatch maps
// to ErrTargetCapability.
func (s *service) resolveVisionTarget(ctx context.Context, model datamodels.Model, reqTarget *ExecLLMTargetDTO) (common.LLMTarget, error) {
	if reqTarget != nil {
		t := targetFromExecDTO(*reqTarget)
		if err := s.llm.ValidateTarget(t, common.ModelCapabilityVisionAnalysis); err != nil {
			return common.LLMTarget{}, mapVisionTargetError(err)
		}
		return t, nil
	}
	if model.ChatID == "" {
		return common.LLMTarget{}, ErrTargetCapability
	}
	chat, err := s.chatsRepo.GetByID(ctx, model.ChatID)
	if err != nil {
		return common.LLMTarget{}, err
	}
	if err := s.llm.ValidateTarget(chat.LLMTarget, common.ModelCapabilityVisionAnalysis); err != nil {
		return common.LLMTarget{}, mapVisionTargetError(err)
	}
	return chat.LLMTarget, nil
}

// mapVisionTargetError maps llm validation failures for the vision path. A
// capability or model/provider mismatch is reported as ErrTargetCapability (the
// chosen target cannot do vision); effort issues map through.
func mapVisionTargetError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, llm.ErrEffortNotAllowed):
		return ErrEffortNotAllowed
	default:
		return ErrTargetCapability
	}
}

// compressDisplayImages downscales/re-encodes every entity display image in place before
// persistence. It is idempotent (already-small PNG/JPEG images are unchanged). An image still
// over the byte cap after compression, or one that cannot be decoded, is a 422.
func (s *service) compressDisplayImages(m *datamodels.Model) error {
	if m == nil {
		return nil
	}
	maxDim := s.cfg.DisplayImageMaxDim
	if maxDim <= 0 {
		maxDim = 256
	}
	maxBytes := s.cfg.DisplayImageMaxBytes
	if maxBytes <= 0 {
		maxBytes = 262144
	}
	process := func(path string, d *datamodels.Display) error {
		if d == nil || d.Kind != datamodels.DisplayImage || d.Image == nil || d.Image.Data == "" {
			return nil
		}
		res, err := imageutil.Compress(d.Image.Data, maxDim, maxBytes)
		if err != nil {
			if errors.Is(err, imageutil.ErrTooLarge) {
				return httpapi.NewAPIError(http.StatusUnprocessableEntity, "display_image_too_large",
					fmt.Sprintf("%s.display.image exceeds the %d byte limit even after compression", path, maxBytes))
			}
			return httpapi.NewAPIError(http.StatusUnprocessableEntity, "display_image_invalid",
				fmt.Sprintf("%s.display.image could not be decoded", path))
		}
		d.Image.Data = res.DataB64
		d.Image.MimeType = res.MimeType
		d.Image.Width = res.Width
		d.Image.Height = res.Height
		return nil
	}
	for i := range m.Agents {
		if err := process(fmt.Sprintf("agents[%d]", i), m.Agents[i].Display); err != nil {
			return err
		}
	}
	if m.Environment != nil {
		for i := range m.Environment.Shapes {
			if err := process(fmt.Sprintf("environment.shapes[%d]", i), m.Environment.Shapes[i].Display); err != nil {
				return err
			}
		}
	}
	return nil
}

// generateEnvironment runs the background vision call and persists the merged
// model. Mirrors generate's context discipline (fresh persist deadline,
// finish-before-idle), but emits only model.updated (no chat turn).
func (s *service) generateEnvironment(ctx context.Context, model datamodels.Model, target common.LLMTarget, prompt string) {
	defer func() {
		s.pending.finish(model.ID)
		s.hub.Publish(model.ID, Event{Type: EventChatStatus, ModelID: model.ID,
			Payload: ChatStatusPayload{ModelID: model.ID, Status: ModelChatStatusIdle}})
	}()

	callCtx, cancel := context.WithTimeout(ctx, s.cfg.GenerationTimeout)
	defer cancel()

	out, llmErr := s.gen.GenerateEnvironmentFromImage(callCtx, VisionInput{
		Model:  model,
		Images: model.Images,
		Prompt: prompt,
		Target: target,
	})
	if llmErr != nil {
		code, message := classifyLLMError(llmErr)
		s.logger.Warn("environment generation failed",
			zap.String("model_id", model.ID), zap.String("code", code), zap.Error(llmErr))
		s.hub.Publish(model.ID, Event{Type: EventChatError, ModelID: model.ID,
			Payload: ChatErrorPayload{ModelID: model.ID, Code: code, Message: message}})
		return
	}
	for _, w := range out.Warnings {
		s.logger.Info("environment merge warning",
			zap.String("model_id", model.ID), zap.String("path", w.Path), zap.String("reason", w.Reason))
	}

	persistCtx, persistCancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer persistCancel()

	merged := out.Model
	merged.UpdatedAt = time.Now().UTC()
	merged.UpdatedBy = fmt.Sprintf("llm:%s", target.ProviderUID)
	// Generated models default archetype agents to distribution sampling (only empty
	// modes are stamped, so any explicit user choice is preserved). Must precede Canonical.
	merged.StampDefaultArchetypeMode()
	// Spatial models auto-get the conventional "color" property on every agent/shape that
	// lacks one, so LLM-generated entities always render with a colour (gen-only,
	// backward-safe; an LLM-supplied colour property is preserved).
	merged.StampDefaultColorProperty()
	// LLM-authored strategies are canonicalized too, so generated models obey the
	// same Type→flags rules as manually edited ones.
	merged.NormalizeLLMStrategies()
	merged.NormalizeSpatial()
	// Coerce numeric-looking string rule condition/action values ("1") on
	// declared-numeric properties so LLM-authored rules behave identically to a
	// numeric submission in the run engine's compares/writes (ISS-06).
	NormalizeRuleValues(&merged)
	// TODO(concurrency): full-document replace of a model snapshot taken at send
	// time — a concurrent manual UpdateModel (PUT) that lands during generation is
	// silently overwritten (higher impact than chats: it can drop manual edits or
	// resurrect deleted entities). ErrGenerationInProgress only guards against two
	// concurrent generations, not against a manual edit. Acceptable single-user dev
	// trade-off; revisit before multi-user (e.g. optimistic version check).
	if err := s.repo.Update(persistCtx, model.ID, merged); err != nil {
		s.logger.Error("failed to persist generated environment", zap.String("model_id", model.ID), zap.Error(err))
		s.hub.Publish(model.ID, Event{Type: EventChatError, ModelID: model.ID,
			Payload: ChatErrorPayload{ModelID: model.ID, Code: "internal", Message: "failed to persist generated environment"}})
		return
	}

	s.hub.Publish(model.ID, Event{Type: EventModelUpdated, ModelID: model.ID,
		Payload: ModelUpdatedPayload{ModelID: model.ID, Model: *s.buildResponse(merged)}})
}

// shapeValidationError builds the 422 envelope for field-shape failures, with a
// fields map of path -> message drawn from the issues.
func shapeValidationError(issues []ValidationIssue) *httpapi.APIError {
	fields := make(map[string]string, len(issues))
	for _, iss := range issues {
		fields[iss.Path] = iss.Message
	}
	return &httpapi.APIError{
		Status:  422,
		Code:    "validation_failed",
		Message: "model failed field-shape validation",
		Fields:  fields,
	}
}
