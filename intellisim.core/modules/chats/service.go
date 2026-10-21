package chats

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"go.uber.org/zap"

	datachats "intelli.sim.core/data/chats"
	"intelli.sim.core/data/common"
	"intelli.sim.core/modules/auth"
	"intelli.sim.core/modules/llm"
)

const (
	defaultPageSize     = 20
	defaultMessageLimit = 30
)

// ChatService is the chats use-case layer. Every method requires an
// authenticated principal in ctx (auth.WithPrincipal); ownership is enforced
// here: owner or admin, others see ErrChatNotFound.
type ChatService interface {
	CreateChat(ctx context.Context, req CreateChatRequest) (*ChatResponse, error)
	GetChat(ctx context.Context, id string) (*ChatResponse, error)
	UpdateChat(ctx context.Context, id string, req UpdateChatRequest) (*ChatResponse, error)
	DeleteChat(ctx context.Context, id string) error
	ListChats(ctx context.Context, q ListChatsQuery) (*ListChatsResponse, error)
	ListMessages(ctx context.Context, chatID string, q ListMessagesQuery) (*MessagesPageResponse, error)
	SendMessage(ctx context.Context, chatID string, req SendMessageRequest) (*SendMessageResponse, error)
}

type service struct {
	logger  *zap.Logger
	repo    datachats.ChatsRepository
	mapper  *ChatMapper
	llm     llm.Service
	hub     *EventsHub
	pending *pendingTracker
	cfg     ModuleConfig
}

func NewChatService(
	logger *zap.Logger,
	repo datachats.ChatsRepository,
	mapper *ChatMapper,
	llmService llm.Service,
	hub *EventsHub,
	cfg ModuleConfig,
) ChatService {
	return &service{
		logger:  logger.Named("chat-service"),
		repo:    repo,
		mapper:  mapper,
		llm:     llmService,
		hub:     hub,
		pending: newPendingTracker(),
		cfg:     cfg,
	}
}

// ---- helpers -------------------------------------------------------------

// getOwnedChat loads a chat and enforces owner-or-admin access. Missing and
// malformed-id chats yield ErrChatNotFound (404); a chat owned by another
// non-admin user yields auth.ErrForbidden (403).
func (s *service) getOwnedChat(ctx context.Context, id string) (*datachats.Chat, *auth.UserPrincipal, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, nil, err
	}
	chat, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, common.ErrNotFound) || errors.Is(err, common.ErrInvalidID) {
			return nil, nil, ErrChatNotFound
		}
		return nil, nil, err
	}
	if err := p.Authorize(chat.CreatedBy); err != nil {
		return nil, nil, err
	}
	return chat, p, nil
}

// checkTarget validates the target against the LIVE provider registry,
// requiring the chat capability (model availability principle: re-check at
// use-time, not only creation-time).
func (s *service) checkTarget(target common.LLMTarget) error {
	for _, provider := range s.llm.Providers() {
		if provider.UID != target.ProviderUID {
			continue
		}
		for _, model := range provider.Models {
			if model.UID != target.ModelUID {
				continue
			}
			if !slices.Contains(model.Capabilities, common.ModelCapabilityChat) {
				return ErrModelUnavailable
			}
			if target.Effort != "" && !slices.Contains(model.Efforts, target.Effort) {
				return ErrEffortNotAllowed
			}
			return nil
		}
		return ErrModelUnavailable
	}
	return ErrModelUnavailable
}

func (s *service) toChatResponse(chat datachats.Chat) *ChatResponse {
	status := ChatStatusIdle
	if s.pending.isThinking(chat.ID) {
		status = ChatStatusThinking
	}
	resp := s.mapper.ToChatResponse(chat, s.checkTarget(chat.LLMTarget) == nil, status)
	return &resp
}

// reverseMessages flips the repository's newest-first order to chronological.
func reverseMessages(msgs []datachats.ChatMessage) []datachats.ChatMessage {
	out := make([]datachats.ChatMessage, len(msgs))
	for i, m := range msgs {
		out[len(msgs)-1-i] = m
	}
	return out
}

// ---- CRUD ------------------------------------------------------------------

func (s *service) CreateChat(ctx context.Context, req CreateChatRequest) (*ChatResponse, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	target := targetFromDTO(*req.LLMTarget)
	if err := s.checkTarget(target); err != nil {
		return nil, err
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "New Chat"
	}
	now := time.Now().UTC()
	chat := datachats.Chat{
		Type:        datachats.ChatTypePlain,
		Name:        name,
		Description: strings.TrimSpace(req.Description),
		LLMTarget:   target,
		CreatedAt:   now,
		UpdatedAt:   now,
		CreatedBy:   p.ID,
		UpdatedBy:   p.ID,
	}
	id, err := s.repo.Create(ctx, chat)
	if err != nil {
		s.logger.Error("failed to create chat", zap.Error(err))
		return nil, err
	}
	chat.ID = id
	s.logger.Info("chat created", zap.String("chat_id", id), zap.String("owner", p.ID))
	return s.toChatResponse(chat), nil
}

func (s *service) GetChat(ctx context.Context, id string) (*ChatResponse, error) {
	chat, _, err := s.getOwnedChat(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.toChatResponse(*chat), nil
}

func (s *service) UpdateChat(ctx context.Context, id string, req UpdateChatRequest) (*ChatResponse, error) {
	chat, p, err := s.getOwnedChat(ctx, id)
	if err != nil {
		return nil, err
	}
	if !req.HasChanges() {
		return nil, ErrNoChanges
	}
	if req.Name != nil {
		chat.Name = strings.TrimSpace(*req.Name)
	}
	if req.Description != nil {
		chat.Description = strings.TrimSpace(*req.Description)
	}
	if req.LLMTarget != nil {
		target := targetFromDTO(*req.LLMTarget)
		if err := s.checkTarget(target); err != nil {
			return nil, err
		}
		chat.LLMTarget = target
	}
	chat.UpdatedAt = time.Now().UTC()
	chat.UpdatedBy = p.ID
	if err := s.repo.Update(ctx, id, *chat); err != nil {
		s.logger.Error("failed to update chat", zap.String("id", id), zap.Error(err))
		return nil, err
	}
	resp := s.toChatResponse(*chat)
	s.hub.Publish(id, Event{Type: EventChatUpdated, ChatID: id, Payload: *resp})
	return resp, nil
}

func (s *service) DeleteChat(ctx context.Context, id string) error {
	chat, _, err := s.getOwnedChat(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.DeleteMessagesByChatID(ctx, chat.ID); err != nil {
		s.logger.Error("failed to delete chat messages", zap.String("id", id), zap.Error(err))
		return err
	}
	if err := s.repo.Delete(ctx, chat.ID); err != nil {
		s.logger.Error("failed to delete chat", zap.String("id", id), zap.Error(err))
		return err
	}
	s.hub.Publish(id, Event{Type: EventChatDeleted, ChatID: id, Payload: ChatDeletedPayload{ChatID: id}})
	s.hub.CloseChat(id)
	return nil
}

func (s *service) ListChats(ctx context.Context, q ListChatsQuery) (*ListChatsResponse, error) {
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
		owner = "" // admins list everyone's chats
	}
	chats, hasMore, err := s.repo.List(ctx, datachats.ListFilter{
		CreatedBy: owner,
		Type:      datachats.ChatTypePlain,
		Page:      page,
		PageSize:  pageSize,
	})
	if err != nil {
		return nil, err
	}
	out := make([]ChatResponse, 0, len(chats))
	for _, chat := range chats {
		out = append(out, *s.toChatResponse(chat))
	}
	return &ListChatsResponse{Chats: out, Page: page, PageSize: pageSize, HasMore: hasMore}, nil
}

func (s *service) ListMessages(ctx context.Context, chatID string, q ListMessagesQuery) (*MessagesPageResponse, error) {
	chat, _, err := s.getOwnedChat(ctx, chatID)
	if err != nil {
		return nil, err
	}
	limit := q.Limit
	if limit < 1 {
		limit = defaultMessageLimit
	}
	if q.Before != "" {
		cursorMsg, err := s.repo.GetMessageByID(ctx, q.Before)
		if err != nil || cursorMsg.ChatID != chat.ID {
			return nil, ErrInvalidCursor
		}
	}
	msgs, hasMore, err := s.repo.ListMessagesBefore(ctx, chat.ID, limit, q.Before)
	if err != nil {
		return nil, err
	}
	return &MessagesPageResponse{
		Messages: s.mapper.ToMessagesResponse(reverseMessages(msgs)),
		HasMore:  hasMore,
	}, nil
}

// ---- async send ------------------------------------------------------------

func (s *service) SendMessage(ctx context.Context, chatID string, req SendMessageRequest) (*SendMessageResponse, error) {
	chat, p, err := s.getOwnedChat(ctx, chatID)
	if err != nil {
		return nil, err
	}
	if err := s.checkTarget(chat.LLMTarget); err != nil {
		return nil, err
	}
	if !s.pending.tryStart(chat.ID) {
		return nil, ErrGenerationInProgress
	}

	now := time.Now().UTC()
	userMsg := datachats.ChatMessage{
		ChatID:      chat.ID,
		Role:        datachats.ChatMessageUserRole,
		TextContent: strings.TrimSpace(req.Content),
		CreatedAt:   now,
		CreatedBy:   p.ID,
	}
	msgID, err := s.repo.AddMessage(ctx, chat.ID, userMsg)
	if err != nil {
		s.pending.finish(chat.ID)
		s.logger.Error("failed to save user message", zap.String("chat_id", chat.ID), zap.Error(err))
		return nil, err
	}
	userMsg.ID = msgID

	userDTO := s.mapper.ToMessageResponse(userMsg)
	s.hub.Publish(chat.ID, Event{Type: EventMessageCreated, ChatID: chat.ID, Payload: userDTO})
	s.hub.Publish(chat.ID, Event{Type: EventChatStatus, ChatID: chat.ID,
		Payload: ChatStatusPayload{ChatID: chat.ID, Status: ChatStatusThinking}})

	// Detach from the HTTP request: the generation must survive the 202 return.
	go s.generate(context.WithoutCancel(ctx), *chat, userMsg)

	return &SendMessageResponse{Message: userDTO, ChatStatus: ChatStatusThinking}, nil
}

// generate runs one background LLM completion and persists the outcome —
// a normal assistant message, or an error-marked one so history stays
// truthful after reload. Always ends by publishing status idle.
func (s *service) generate(ctx context.Context, chat datachats.Chat, userMsg datachats.ChatMessage) {
	defer func() {
		// finish BEFORE publishing idle so a client that reacts to the idle
		// event can never read a stale "thinking" status from the tracker.
		s.pending.finish(chat.ID)
		s.hub.Publish(chat.ID, Event{Type: EventChatStatus, ChatID: chat.ID,
			Payload: ChatStatusPayload{ChatID: chat.ID, Status: ChatStatusIdle}})
	}()

	callCtx, cancel := context.WithTimeout(ctx, s.cfg.GenerationTimeout)
	defer cancel()

	history, err := s.loadHistory(callCtx, chat.ID, userMsg.ID)
	if err != nil {
		s.logger.Error("failed to load chat history", zap.String("chat_id", chat.ID), zap.Error(err))
		s.hub.Publish(chat.ID, Event{Type: EventChatError, ChatID: chat.ID,
			Payload: ChatErrorPayload{ChatID: chat.ID, Code: "internal", Message: "failed to load chat history"}})
		return
	}

	completion, llmErr := s.llm.Chat(callCtx, llm.ChatRequest{
		Target:           chat.LLMTarget,
		Purpose:          common.ModelCapabilityChat,
		Message:          userMsg.TextContent,
		Messages:         history,
		OverflowStrategy: llm.OverflowStrategyCut,
		ChatType:         string(chat.Type),
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
		s.logger.Warn("llm generation failed",
			zap.String("chat_id", chat.ID), zap.String("code", code), zap.Error(llmErr))
	} else {
		assistantMsg.TextContent = completion.Text
		usage := completion.Usage
		assistantMsg.TokenUsage = &usage
	}

	// Persist on a fresh deadline — callCtx may already be expired (timeout).
	persistCtx, persistCancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer persistCancel()

	msgID, err := s.repo.AddMessage(persistCtx, chat.ID, assistantMsg)
	if err != nil {
		s.logger.Error("failed to save assistant message", zap.String("chat_id", chat.ID), zap.Error(err))
		s.hub.Publish(chat.ID, Event{Type: EventChatError, ChatID: chat.ID,
			Payload: ChatErrorPayload{ChatID: chat.ID, Code: "internal", Message: "failed to save assistant message"}})
		return
	}
	assistantMsg.ID = msgID

	// Maintain the chat's running usage aggregate + freshness.
	if assistantMsg.TokenUsage != nil {
		if chat.TokenUsage == nil {
			chat.TokenUsage = &common.TokenUsageSummary{}
		}
		chat.TokenUsage.Add(*assistantMsg.TokenUsage)
	}
	chat.UpdatedAt = now
	// TODO(concurrency): this is a full-document replace of a chat snapshot
	// taken at SendMessage time — a concurrent UpdateChat (rename/retarget)
	// would be silently overwritten. Acceptable single-user dev trade-off;
	// replace with a targeted token-usage increment before multi-user.
	if err := s.repo.Update(persistCtx, chat.ID, chat); err != nil {
		s.logger.Error("failed to update chat aggregate", zap.String("chat_id", chat.ID), zap.Error(err))
	}

	s.hub.Publish(chat.ID, Event{Type: EventMessageCreated, ChatID: chat.ID,
		Payload: s.mapper.ToMessageResponse(assistantMsg)})
}

// loadHistory returns up to HistoryLimit messages OLDER than the new user
// message, chronological, with failed generations filtered out.
func (s *service) loadHistory(ctx context.Context, chatID, beforeMsgID string) ([]datachats.ChatMessage, error) {
	msgs, _, err := s.repo.ListMessagesBefore(ctx, chatID, s.cfg.HistoryLimit, beforeMsgID)
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
func classifyLLMError(err error) (code, message string) {
	var providerErr *llm.ProviderError
	switch {
	case errors.Is(err, llm.ErrProviderNotFound),
		errors.Is(err, llm.ErrModelNotFound),
		errors.Is(err, llm.ErrProviderDisconnected):
		return "model_unavailable", err.Error()
	case errors.Is(err, llm.ErrEffortNotAllowed):
		return "effort_not_allowed", err.Error()
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout", "generation timed out"
	case errors.As(err, &providerErr):
		return string(providerErr.Code), providerErr.Message
	default:
		return "internal", err.Error()
	}
}
