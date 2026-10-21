package chats

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	datachats "intelli.sim.core/data/chats"
	"intelli.sim.core/data/common"
	"intelli.sim.core/modules/auth"
	"intelli.sim.core/modules/llm"
)

// ---- fakes -----------------------------------------------------------------

type fakeRepo struct {
	mu       sync.Mutex
	seq      int
	chats    map[string]datachats.Chat
	messages []datachats.ChatMessage // insertion order == chronological
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{chats: map[string]datachats.Chat{}}
}

func (f *fakeRepo) nextID() string {
	f.seq++
	return fmt.Sprintf("%024d", f.seq) // 24-char hex-ish ids
}

func (f *fakeRepo) Create(_ context.Context, chat datachats.Chat) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.nextID()
	chat.ID = id
	f.chats[id] = chat
	return id, nil
}

func (f *fakeRepo) GetByID(_ context.Context, id string) (*datachats.Chat, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	chat, ok := f.chats[id]
	if !ok {
		return nil, fmt.Errorf("chat %s: %w", id, common.ErrNotFound)
	}
	return &chat, nil
}

func (f *fakeRepo) Update(_ context.Context, id string, chat datachats.Chat) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	chat.ID = id
	f.chats[id] = chat
	return nil
}

func (f *fakeRepo) Delete(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.chats, id)
	return nil
}

func (f *fakeRepo) List(_ context.Context, filter datachats.ListFilter) ([]datachats.Chat, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []datachats.Chat
	for _, c := range f.chats {
		if filter.CreatedBy != "" && c.CreatedBy != filter.CreatedBy {
			continue
		}
		if filter.Type != "" && c.Type != filter.Type {
			continue
		}
		out = append(out, c)
	}
	return out, false, nil
}

// NOTE: orders by insertion only — does not model the real repo's compound (created_at, _id) cursor tiebreak; same-timestamp ordering is covered by e2e against real MongoDB.
func (f *fakeRepo) ListMessagesBefore(_ context.Context, chatID string, limit int, beforeID string) ([]datachats.ChatMessage, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	// newest-first slice of this chat's messages
	var all []datachats.ChatMessage
	for i := len(f.messages) - 1; i >= 0; i-- {
		if f.messages[i].ChatID == chatID {
			all = append(all, f.messages[i])
		}
	}
	start := 0
	if beforeID != "" {
		start = -1
		for i, m := range all {
			if m.ID == beforeID {
				start = i + 1
				break
			}
		}
		if start == -1 {
			return nil, false, common.ErrNotFound
		}
	}
	rest := all[start:]
	hasMore := len(rest) > limit
	if hasMore {
		rest = rest[:limit]
	}
	return rest, hasMore, nil
}

func (f *fakeRepo) GetMessageByID(_ context.Context, id string) (*datachats.ChatMessage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, m := range f.messages {
		if m.ID == id {
			return &m, nil
		}
	}
	return nil, fmt.Errorf("chat message %s: %w", id, common.ErrNotFound)
}

func (f *fakeRepo) AddMessage(_ context.Context, chatID string, msg datachats.ChatMessage) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	msg.ID = f.nextID()
	msg.ChatID = chatID
	f.messages = append(f.messages, msg)
	return msg.ID, nil
}

func (f *fakeRepo) DeleteMessagesByChatID(_ context.Context, chatID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	kept := f.messages[:0]
	for _, m := range f.messages {
		if m.ChatID != chatID {
			kept = append(kept, m)
		}
	}
	f.messages = kept
	return nil
}

type stubLLM struct {
	providers []llm.ProviderInfo
	chatFn    func(ctx context.Context, req llm.ChatRequest) (*llm.Completion, error)
}

func (s *stubLLM) Simple(context.Context, llm.SimpleRequest) (*llm.Completion, error) {
	return nil, errors.New("not implemented")
}
func (s *stubLLM) Chat(ctx context.Context, req llm.ChatRequest) (*llm.Completion, error) {
	return s.chatFn(ctx, req)
}
func (s *stubLLM) Vision(context.Context, llm.VisionRequest) (*llm.Completion, error) {
	return nil, errors.New("not implemented")
}
func (s *stubLLM) Reasoning(context.Context, llm.ReasoningRequest) (*llm.ReasoningCompletion, error) {
	return nil, errors.New("not implemented")
}
func (s *stubLLM) Providers() []llm.ProviderInfo { return s.providers }
func (s *stubLLM) ProvidersByCapability(common.ModelCapability) []llm.ProviderInfo {
	return s.providers
}
func (s *stubLLM) ProvidersByModel(string) []llm.ProviderInfo { return s.providers }
func (s *stubLLM) ModelDescriptors(string, string) ([]llm.ConfigParamDescriptor, bool, bool) {
	return nil, false, false
}
func (s *stubLLM) MaxImages(string, string) (uint32, bool) { return 0, false }
func (s *stubLLM) ValidateTarget(common.LLMTarget, common.ModelCapability) error { return nil }

// ---- harness -----------------------------------------------------------------

func availableProviders() []llm.ProviderInfo {
	return []llm.ProviderInfo{{
		UID: "openai",
		Models: []llm.ModelInfo{{
			UID:          "gpt-4o",
			Capabilities: []common.ModelCapability{common.ModelCapabilityChat},
			Efforts:      []common.LLMEffort{common.LLMEffortLow, common.LLMEffortHigh},
		}},
	}}
}

func newTestService(repo *fakeRepo, llmStub *stubLLM) (ChatService, *EventsHub) {
	hub := NewEventsHub()
	svc := NewChatService(zap.NewNop(), repo, NewChatMapper(), llmStub, hub,
		ModuleConfig{GenerationTimeout: 5 * time.Second, HistoryLimit: 50})
	return svc, hub
}

func ctxAs(id string, admin bool) context.Context {
	return auth.WithPrincipal(context.Background(), &auth.UserPrincipal{ID: id, IsAdmin: admin})
}

func target() *LLMTargetDTO { return &LLMTargetDTO{ProviderUID: "openai", ModelUID: "gpt-4o"} }

// waitForEvent drains the subscription until the wanted type arrives.
func waitForEvent(t *testing.T, sub <-chan Event, want EventType) Event {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case ev, ok := <-sub:
			require.True(t, ok)
			if ev.Type == want {
				return ev
			}
		case <-deadline:
			t.Fatalf("timed out waiting for %s", want)
		}
	}
}

// ---- tests -------------------------------------------------------------------

func TestCreateChat_RejectsUnavailableTarget(t *testing.T) {
	svc, _ := newTestService(newFakeRepo(), &stubLLM{providers: nil})
	_, err := svc.CreateChat(ctxAs("alice", false), CreateChatRequest{LLMTarget: target()})
	assert.ErrorIs(t, err, ErrModelUnavailable)
}

func TestCreateChat_RejectsDisallowedEffort(t *testing.T) {
	svc, _ := newTestService(newFakeRepo(), &stubLLM{providers: availableProviders()})
	_, err := svc.CreateChat(ctxAs("alice", false), CreateChatRequest{
		LLMTarget: &LLMTargetDTO{ProviderUID: "openai", ModelUID: "gpt-4o", Effort: "max"},
	})
	assert.ErrorIs(t, err, ErrEffortNotAllowed)
}

func TestCreateChat_DefaultsNameAndStampsOwner(t *testing.T) {
	svc, _ := newTestService(newFakeRepo(), &stubLLM{providers: availableProviders()})
	resp, err := svc.CreateChat(ctxAs("alice", false), CreateChatRequest{LLMTarget: target()})
	require.NoError(t, err)
	assert.Equal(t, "New Chat", resp.Name)
	assert.Equal(t, "alice", resp.CreatedBy)
	assert.True(t, resp.ModelAvailable)
	assert.Equal(t, ChatStatusIdle, resp.Status)
}

func TestOwnership_NonOwnerForbidden_MissingNotFound_AdminSeesAll(t *testing.T) {
	repo := newFakeRepo()
	svc, _ := newTestService(repo, &stubLLM{providers: availableProviders()})
	created, err := svc.CreateChat(ctxAs("alice", false), CreateChatRequest{LLMTarget: target()})
	require.NoError(t, err)

	// Foreign chat -> 403 (existence is disclosed by design).
	_, err = svc.GetChat(ctxAs("bob", false), created.ID)
	assert.ErrorIs(t, err, auth.ErrForbidden)

	// Genuinely missing chat -> 404, never 403.
	_, err = svc.GetChat(ctxAs("bob", false), "000000000000000000000000")
	assert.ErrorIs(t, err, ErrChatNotFound)

	_, err = svc.GetChat(ctxAs("root", true), created.ID)
	assert.NoError(t, err)

	bobList, err := svc.ListChats(ctxAs("bob", false), ListChatsQuery{})
	require.NoError(t, err)
	assert.Empty(t, bobList.Chats)

	adminList, err := svc.ListChats(ctxAs("root", true), ListChatsQuery{})
	require.NoError(t, err)
	assert.Len(t, adminList.Chats, 1)
}

func TestUpdateChat_NoChangesRejected(t *testing.T) {
	svc, _ := newTestService(newFakeRepo(), &stubLLM{providers: availableProviders()})
	created, _ := svc.CreateChat(ctxAs("alice", false), CreateChatRequest{LLMTarget: target()})
	_, err := svc.UpdateChat(ctxAs("alice", false), created.ID, UpdateChatRequest{})
	assert.ErrorIs(t, err, ErrNoChanges)
}

func TestListMessages_InvalidCursorFromOtherChat(t *testing.T) {
	repo := newFakeRepo()
	llmStub := &stubLLM{providers: availableProviders(),
		chatFn: func(context.Context, llm.ChatRequest) (*llm.Completion, error) {
			return &llm.Completion{Text: "ok"}, nil
		}}
	svc, _ := newTestService(repo, llmStub)
	ctx := ctxAs("alice", false)
	c1, _ := svc.CreateChat(ctx, CreateChatRequest{LLMTarget: target()})
	c2, _ := svc.CreateChat(ctx, CreateChatRequest{LLMTarget: target()})

	sent, err := svc.SendMessage(ctx, c1.ID, SendMessageRequest{Content: "hi"})
	require.NoError(t, err)

	_, err = svc.ListMessages(ctx, c2.ID, ListMessagesQuery{Before: sent.Message.ID})
	assert.ErrorIs(t, err, ErrInvalidCursor)
}

func TestSendMessage_AsyncFlowDeliversAssistantReply(t *testing.T) {
	repo := newFakeRepo()
	llmStub := &stubLLM{providers: availableProviders(),
		chatFn: func(_ context.Context, req llm.ChatRequest) (*llm.Completion, error) {
			return &llm.Completion{
				Text:  "echo: " + req.Message,
				Usage: common.TokenUsage{LLMTarget: req.Target, TotalTokens: 7, Calls: 1},
			}, nil
		}}
	svc, hub := newTestService(repo, llmStub)
	ctx := ctxAs("alice", false)
	chat, _ := svc.CreateChat(ctx, CreateChatRequest{LLMTarget: target()})

	sub, cancel := hub.Subscribe(chat.ID)
	defer cancel()

	resp, err := svc.SendMessage(ctx, chat.ID, SendMessageRequest{Content: "hello"})
	require.NoError(t, err)
	assert.Equal(t, ChatStatusThinking, resp.ChatStatus)
	assert.Equal(t, "user", resp.Message.Role)

	// user message event, thinking, assistant message, idle
	first := waitForEvent(t, sub, EventMessageCreated)
	assert.Equal(t, "user", first.Payload.(ChatMessageResponse).Role)
	second := waitForEvent(t, sub, EventMessageCreated)
	asMsg := second.Payload.(ChatMessageResponse)
	assert.Equal(t, "assistant", asMsg.Role)
	assert.Equal(t, "echo: hello", asMsg.Content)
	idle := waitForEvent(t, sub, EventChatStatus)
	assert.Equal(t, ChatStatusIdle, idle.Payload.(ChatStatusPayload).Status)

	// chat aggregate updated
	updated, err := svc.GetChat(ctx, chat.ID)
	require.NoError(t, err)
	require.NotNil(t, updated.TokenUsage)
	assert.Equal(t, uint64(7), updated.TokenUsage.Total.TotalTokens)
}

func TestSendMessage_RejectsWhileThinking(t *testing.T) {
	repo := newFakeRepo()
	release := make(chan struct{})
	llmStub := &stubLLM{providers: availableProviders(),
		chatFn: func(context.Context, llm.ChatRequest) (*llm.Completion, error) {
			<-release
			return &llm.Completion{Text: "done"}, nil
		}}
	svc, hub := newTestService(repo, llmStub)
	ctx := ctxAs("alice", false)
	chat, _ := svc.CreateChat(ctx, CreateChatRequest{LLMTarget: target()})

	sub, cancel := hub.Subscribe(chat.ID)
	defer cancel()

	_, err := svc.SendMessage(ctx, chat.ID, SendMessageRequest{Content: "first"})
	require.NoError(t, err)
	_, err = svc.SendMessage(ctx, chat.ID, SendMessageRequest{Content: "second"})
	assert.ErrorIs(t, err, ErrGenerationInProgress)
	close(release)

	// Drain until idle arrives so the background goroutine finishes within the test.
	for {
		ev := waitForEvent(t, sub, EventChatStatus)
		if ev.Payload.(ChatStatusPayload).Status == ChatStatusIdle {
			break
		}
	}
}

func TestGenerate_FailurePersistsErrorMessage(t *testing.T) {
	repo := newFakeRepo()
	llmStub := &stubLLM{providers: availableProviders(),
		chatFn: func(context.Context, llm.ChatRequest) (*llm.Completion, error) {
			return nil, fmt.Errorf("wrapped: %w", llm.ErrProviderDisconnected)
		}}
	svc, hub := newTestService(repo, llmStub)
	ctx := ctxAs("alice", false)
	chat, _ := svc.CreateChat(ctx, CreateChatRequest{LLMTarget: target()})

	sub, cancel := hub.Subscribe(chat.ID)
	defer cancel()

	_, err := svc.SendMessage(ctx, chat.ID, SendMessageRequest{Content: "hello"})
	require.NoError(t, err)

	waitForEvent(t, sub, EventMessageCreated) // user msg
	ev := waitForEvent(t, sub, EventMessageCreated)
	failed := ev.Payload.(ChatMessageResponse)
	require.NotNil(t, failed.Error)
	assert.Equal(t, "model_unavailable", failed.Error.Code)

	// persisted error messages are excluded from future LLM history
	page, err := svc.ListMessages(ctx, chat.ID, ListMessagesQuery{})
	require.NoError(t, err)
	assert.Len(t, page.Messages, 2) // but still visible in the API page
	assert.True(t, strings.HasPrefix(failed.CreatedBy, "llm:"))
}
