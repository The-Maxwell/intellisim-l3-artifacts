package models

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	datachats "intelli.sim.core/data/chats"
	"intelli.sim.core/data/common"
	datamodels "intelli.sim.core/data/models"
	"intelli.sim.core/modules/auth"
	"intelli.sim.core/modules/llm"
	"intelli.sim.core/pkg/httpapi"
)

// fakeModelsRepo is an in-memory ModelsRepository for service tests.
type fakeModelsRepo struct {
	mu         sync.Mutex
	seq        int
	store      map[string]datamodels.Model
	lastFilter datamodels.ListFilter
}

func newFakeModelsRepo() *fakeModelsRepo {
	return &fakeModelsRepo{store: map[string]datamodels.Model{}}
}

func (r *fakeModelsRepo) Create(ctx context.Context, model datamodels.Model) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seq++
	id := fmt.Sprintf("%024d", r.seq)
	model.ID = id
	r.store[id] = model
	return id, nil
}

func (r *fakeModelsRepo) GetByID(ctx context.Context, id string) (*datamodels.Model, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.store[id]
	if !ok {
		return nil, common.ErrNotFound
	}
	return &m, nil
}

func (r *fakeModelsRepo) Update(ctx context.Context, id string, model datamodels.Model) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	model.ID = id
	r.store[id] = model
	return nil
}

func (r *fakeModelsRepo) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.store, id)
	return nil
}

func (r *fakeModelsRepo) List(ctx context.Context, filter datamodels.ListFilter) ([]datamodels.Model, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastFilter = filter
	out := make([]datamodels.Model, 0, len(r.store))
	for _, m := range r.store {
		if filter.CreatedBy == "" || m.CreatedBy == filter.CreatedBy {
			out = append(out, m)
		}
	}
	return out, false, nil
}

func (r *fakeModelsRepo) Count(ctx context.Context, filter datamodels.ListFilter) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var n int64
	for _, m := range r.store {
		if filter.CreatedBy == "" || m.CreatedBy == filter.CreatedBy {
			n++
		}
	}
	return n, nil
}

func (r *fakeModelsRepo) ListOptions(ctx context.Context, createdBy string) ([]datamodels.ModelOption, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]datamodels.ModelOption, 0, len(r.store))
	for _, m := range r.store {
		if createdBy == "" || m.CreatedBy == createdBy {
			out = append(out, datamodels.ModelOption{ID: m.ID, Name: m.Name})
		}
	}
	return out, nil
}

func (r *fakeModelsRepo) get(id string) datamodels.Model {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.store[id]
}

// fakeChatsRepo is an in-memory datachats.ChatsRepository for the model-chat path.
type fakeChatsRepo struct {
	mu       sync.Mutex
	seq      int
	chats    map[string]datachats.Chat
	messages []datachats.ChatMessage // insertion order == chronological
}

func newFakeChatsRepo() *fakeChatsRepo {
	return &fakeChatsRepo{chats: map[string]datachats.Chat{}}
}

func (f *fakeChatsRepo) nextID() string {
	f.seq++
	return fmt.Sprintf("c%023d", f.seq)
}

func (f *fakeChatsRepo) Create(_ context.Context, chat datachats.Chat) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.nextID()
	chat.ID = id
	f.chats[id] = chat
	return id, nil
}

func (f *fakeChatsRepo) GetByID(_ context.Context, id string) (*datachats.Chat, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	chat, ok := f.chats[id]
	if !ok {
		return nil, common.ErrNotFound
	}
	return &chat, nil
}

func (f *fakeChatsRepo) Update(_ context.Context, id string, chat datachats.Chat) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	chat.ID = id
	f.chats[id] = chat
	return nil
}

func (f *fakeChatsRepo) Delete(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.chats, id)
	return nil
}

func (f *fakeChatsRepo) List(_ context.Context, _ datachats.ListFilter) ([]datachats.Chat, bool, error) {
	return nil, false, nil
}

func (f *fakeChatsRepo) ListMessagesBefore(_ context.Context, chatID string, limit int, beforeID string) ([]datachats.ChatMessage, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var all []datachats.ChatMessage // newest first
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

func (f *fakeChatsRepo) GetMessageByID(_ context.Context, id string) (*datachats.ChatMessage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, m := range f.messages {
		if m.ID == id {
			return &m, nil
		}
	}
	return nil, common.ErrNotFound
}

func (f *fakeChatsRepo) AddMessage(_ context.Context, chatID string, msg datachats.ChatMessage) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seq++
	msg.ID = fmt.Sprintf("m%023d", f.seq)
	msg.ChatID = chatID
	f.messages = append(f.messages, msg)
	return msg.ID, nil
}

func (f *fakeChatsRepo) DeleteMessagesByChatID(_ context.Context, chatID string) error {
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

func newTestService(repo datamodels.ModelsRepository) ModelService {
	return NewModelService(zap.NewNop(), repo, nil, NewModelMapper(), nil, nil, NewEventsHub(), ModuleConfig{})
}

func TestService_GetModel_ForeignNonAdmin_Forbidden(t *testing.T) {
	repo := &fakeModelsRepo{store: map[string]datamodels.Model{"1": {ID: "1", CreatedBy: "alice"}}}
	svc := newTestService(repo)
	ctx := auth.WithPrincipal(context.Background(), &auth.UserPrincipal{ID: "bob", IsAdmin: false})
	_, err := svc.GetModel(ctx, "1")
	require.ErrorIs(t, err, auth.ErrForbidden)
}

func TestService_ListModels_AdminSeesAll(t *testing.T) {
	repo := &fakeModelsRepo{store: map[string]datamodels.Model{
		"1": {ID: "1", CreatedBy: "alice"},
		"2": {ID: "2", CreatedBy: "bob"},
	}}
	svc := newTestService(repo)
	ctx := auth.WithPrincipal(context.Background(), &auth.UserPrincipal{ID: "admin", IsAdmin: true})
	resp, err := svc.ListModels(ctx, ListModelsQuery{})
	require.NoError(t, err)
	require.Equal(t, "", repo.lastFilter.CreatedBy) // admin => empty owner filter
	require.Len(t, resp.Models, 2)
}

func TestService_DuplicateModel_ClonesModelAndChat(t *testing.T) {
	repo := &fakeModelsRepo{store: map[string]datamodels.Model{}}
	chatsRepo := newFakeChatsRepo()
	svc, _ := newGenTestService(repo, chatsRepo, &fakeLLM{})
	ctx := ctxAs("alice", false)

	// Source model with a companion chat carrying two messages.
	chatID, err := chatsRepo.Create(ctx, datachats.Chat{
		Type: datachats.ChatTypeModel, Name: "Source", ReferenceID: "src1", CreatedBy: "alice",
	})
	require.NoError(t, err)
	_, err = chatsRepo.AddMessage(ctx, chatID, datachats.ChatMessage{Role: datachats.ChatMessageUserRole, TextContent: "hi", CreatedBy: "alice"})
	require.NoError(t, err)
	_, err = chatsRepo.AddMessage(ctx, chatID, datachats.ChatMessage{Role: datachats.ChatMessageAssistantRole, TextContent: "spec"})
	require.NoError(t, err)
	repo.store["src1"] = datamodels.Model{ID: "src1", Name: "Source", ChatID: chatID, CreatedBy: "alice"}

	resp, err := svc.DuplicateModel(ctx, "src1", DuplicateModelRequest{})
	require.NoError(t, err)
	require.NotEqual(t, "src1", resp.ID)
	require.Equal(t, "Source (copy)", resp.Model.Name)

	// The duplicate has its own, distinct chat.
	newModel := repo.get(resp.ID)
	require.NotEmpty(t, newModel.ChatID)
	require.NotEqual(t, chatID, newModel.ChatID)
	require.Equal(t, newModel.ChatID, resp.ChatID)

	// The cloned chat is re-linked to the new model and carries both messages.
	newChat, err := chatsRepo.GetByID(ctx, newModel.ChatID)
	require.NoError(t, err)
	require.Equal(t, resp.ID, newChat.ReferenceID)
	cloned, _, err := chatsRepo.ListMessagesBefore(ctx, newModel.ChatID, 100, "")
	require.NoError(t, err)
	require.Len(t, cloned, 2)

	// The source chat is untouched.
	srcMsgs, _, err := chatsRepo.ListMessagesBefore(ctx, chatID, 100, "")
	require.NoError(t, err)
	require.Len(t, srcMsgs, 2)
}

// ---- async chat/generation harness -----------------------------------------

func newGenTestService(repo *fakeModelsRepo, chatsRepo *fakeChatsRepo, fake *fakeLLM) (ModelService, *EventsHub) {
	hub := NewEventsHub()
	gen := NewGenerator(zap.NewNop(), fake)
	svc := NewModelService(zap.NewNop(), repo, chatsRepo, NewModelMapper(), fake, gen, hub,
		ModuleConfig{GenerationTimeout: 5 * time.Second, HistoryLimit: 50})
	return svc, hub
}

func ctxAs(id string, admin bool) context.Context {
	return auth.WithPrincipal(context.Background(), &auth.UserPrincipal{ID: id, IsAdmin: admin})
}

// ---- image library + environment generation --------------------------------

// makePNG builds a small real PNG and returns it as base64 — decodable by imageutil.Compress.
func makePNG(t *testing.T) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 60), G: uint8(y * 60), B: 100, A: 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func imageTestService(repo *fakeModelsRepo, fake *fakeLLM) ModelService {
	cfg := ModuleConfig{
		GenerationTimeout:  5 * time.Second,
		MaxModelImages:     3,
		MaxImageBytes:      6 << 20,
		ModelImageMaxDim:   1536,
		ModelImageMaxBytes: 1 << 20,
	}
	return NewModelService(zap.NewNop(), repo, &fakeChatsRepo{}, NewModelMapper(), fake,
		NewGenerator(zap.NewNop(), fake), NewEventsHub(), cfg)
}

func TestService_AddModelImage_CompressesAssignsUIDAndPersists(t *testing.T) {
	repo := &fakeModelsRepo{store: map[string]datamodels.Model{"1": {ID: "1", CreatedBy: "alice", IsSpatial: true}}}
	svc := imageTestService(repo, &fakeLLM{})

	resp, err := svc.AddModelImage(ctxAs("alice", false), "1", AddModelImageRequest{
		Name: "floor", Description: "floor plan", MimeType: "image/png", Data: makePNG(t),
	})
	require.NoError(t, err)
	require.Len(t, resp.Model.Images, 1)
	img := resp.Model.Images[0]
	require.NotEmpty(t, img.UID)
	require.Equal(t, "floor", img.Name)
	require.Equal(t, "floor plan", img.Description)
	require.NotEmpty(t, img.Data)
	require.Len(t, repo.get("1").Images, 1) // persisted
}

func TestService_AddModelImage_CapEnforced(t *testing.T) {
	full := []datamodels.ModelImage{{UID: "a"}, {UID: "b"}, {UID: "c"}} // == MaxModelImages (3)
	repo := &fakeModelsRepo{store: map[string]datamodels.Model{"1": {ID: "1", CreatedBy: "alice", IsSpatial: true, Images: full}}}
	svc := imageTestService(repo, &fakeLLM{})

	_, err := svc.AddModelImage(ctxAs("alice", false), "1", AddModelImageRequest{MimeType: "image/png", Data: makePNG(t)})
	var apiErr *httpapi.APIError
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, "too_many_images", apiErr.Code)
}

func TestService_DeleteModelImage(t *testing.T) {
	imgs := []datamodels.ModelImage{{UID: "keep"}, {UID: "drop"}}
	repo := &fakeModelsRepo{store: map[string]datamodels.Model{"1": {ID: "1", CreatedBy: "alice", IsSpatial: true, Images: imgs}}}
	svc := imageTestService(repo, &fakeLLM{})

	resp, err := svc.DeleteModelImage(ctxAs("alice", false), "1", "drop")
	require.NoError(t, err)
	require.Len(t, resp.Model.Images, 1)
	kept := repo.get("1").Images
	require.Len(t, kept, 1)
	require.Equal(t, "keep", kept[0].UID)

	_, err = svc.DeleteModelImage(ctxAs("alice", false), "1", "nope") // missing UID
	var apiErr *httpapi.APIError
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, http.StatusNotFound, apiErr.Status)
}

func TestService_GenerateEnvironment_NoImages(t *testing.T) {
	repo := &fakeModelsRepo{store: map[string]datamodels.Model{"1": {ID: "1", CreatedBy: "alice", IsSpatial: true}}}
	svc := imageTestService(repo, &fakeLLM{})
	_, err := svc.GenerateEnvironment(ctxAs("alice", false), "1", GenerateEnvironmentRequest{Target: execTarget()})
	var apiErr *httpapi.APIError
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, "no_images", apiErr.Code)
}

func TestService_GenerateEnvironment_NonSpatial(t *testing.T) {
	repo := &fakeModelsRepo{store: map[string]datamodels.Model{"1": {ID: "1", CreatedBy: "alice", IsSpatial: false, Images: []datamodels.ModelImage{{UID: "a"}}}}}
	svc := imageTestService(repo, &fakeLLM{maxImages: 5})
	_, err := svc.GenerateEnvironment(ctxAs("alice", false), "1", GenerateEnvironmentRequest{Target: execTarget()})
	require.ErrorIs(t, err, ErrNotSpatial)
}

func TestService_GenerateEnvironment_OverModelMax(t *testing.T) {
	imgs := []datamodels.ModelImage{{UID: "a"}, {UID: "b"}, {UID: "c"}}
	repo := &fakeModelsRepo{store: map[string]datamodels.Model{"1": {ID: "1", CreatedBy: "alice", IsSpatial: true, Images: imgs}}}
	svc := imageTestService(repo, &fakeLLM{maxImages: 2}) // model advertises 2 < 3 attached
	_, err := svc.GenerateEnvironment(ctxAs("alice", false), "1", GenerateEnvironmentRequest{Target: execTarget()})
	var apiErr *httpapi.APIError
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, "too_many_images_for_model", apiErr.Code)
	require.Equal(t, http.StatusUnprocessableEntity, apiErr.Status)
}

func execTarget() *ExecLLMTargetDTO {
	return &ExecLLMTargetDTO{ProviderUID: "openai", ModelUID: "gpt-4o"}
}

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

func TestService_SetChatTarget_RejectsNonModelGenCapability(t *testing.T) {
	repo := newFakeModelsRepo()
	chatsRepo := newFakeChatsRepo()
	fake := &fakeLLM{validateErr: llm.ErrCapabilityNotSupported}
	svc, _ := newGenTestService(repo, chatsRepo, fake)
	ctx := ctxAs("alice", false)

	created, err := svc.CreateModel(ctx, CreateModelRequest{Name: "M"})
	require.NoError(t, err)

	_, err = svc.SetChatTarget(ctx, created.ID, SetModelChatTargetRequest{LLMTarget: execTarget()})
	require.ErrorIs(t, err, ErrTargetCapability)
}

func TestService_SendMessage_202AndAsyncUpdatesModel(t *testing.T) {
	repo := newFakeModelsRepo()
	chatsRepo := newFakeChatsRepo()
	fake := &fakeLLM{
		resp: `{"display_message":"added prey","model":{"agents":[{"uid":"prey","name":"Prey","type":"agent","initialization":{"population_count":10}}]}}`,
	}
	svc, hub := newGenTestService(repo, chatsRepo, fake)
	ctx := ctxAs("alice", false)

	created, err := svc.CreateModel(ctx, CreateModelRequest{Name: "M"})
	require.NoError(t, err)

	_, err = svc.SetChatTarget(ctx, created.ID, SetModelChatTargetRequest{LLMTarget: execTarget()})
	require.NoError(t, err)

	// Subscribe BEFORE sending so the async events are not missed.
	sub, cancel := hub.Subscribe(created.ID)
	defer cancel()

	resp, err := svc.SendMessage(ctx, created.ID, SendModelMessageRequest{Content: "add prey"})
	require.NoError(t, err)
	require.Equal(t, ModelChatStatusThinking, resp.ChatStatus)
	require.Equal(t, "user", resp.Message.Role)

	// user message, assistant message, then model.updated (deterministic wait).
	first := waitForEvent(t, sub, EventMessageCreated)
	require.Equal(t, "user", first.Payload.(ModelChatMessageResponse).Role)

	second := waitForEvent(t, sub, EventMessageCreated)
	asMsg := second.Payload.(ModelChatMessageResponse)
	require.Equal(t, "assistant", asMsg.Role)
	require.Equal(t, "added prey", asMsg.Content)
	require.NotEmpty(t, asMsg.Raw) // RawContent carries the full spec json

	updated := waitForEvent(t, sub, EventModelUpdated)
	payload := updated.Payload.(ModelUpdatedPayload)
	require.Len(t, payload.Model.Model.Agents, 1)
	require.Equal(t, "prey", payload.Model.Model.Agents[0].UID)

	// The persisted model in the repo gained the new agent.
	persisted := repo.get(created.ID)
	require.Len(t, persisted.Agents, 1)
	require.Equal(t, "prey", persisted.Agents[0].UID)

	// Drain to idle so the goroutine fully completes within the test.
	idle := waitForEvent(t, sub, EventChatStatus)
	require.Equal(t, ModelChatStatusIdle, idle.Payload.(ChatStatusPayload).Status)
}

// TestService_SendMessage_GenerationFailurePublishesError verifies that when the
// LLM returns an error the service publishes a chat.error event followed by
// chat.status idle, and does NOT write a merged model to the repo.
func TestService_SendMessage_GenerationFailurePublishesError(t *testing.T) {
	repo := newFakeModelsRepo()
	chatsRepo := newFakeChatsRepo()
	fake := &fakeLLM{chatErr: errors.New("upstream LLM exploded")}
	svc, hub := newGenTestService(repo, chatsRepo, fake)
	ctx := ctxAs("alice", false)

	created, err := svc.CreateModel(ctx, CreateModelRequest{Name: "M"})
	require.NoError(t, err)
	_, err = svc.SetChatTarget(ctx, created.ID, SetModelChatTargetRequest{LLMTarget: execTarget()})
	require.NoError(t, err)

	// Snapshot the model state BEFORE generation so we can check it is unchanged.
	before := repo.get(created.ID)

	sub, cancel := hub.Subscribe(created.ID)
	defer cancel()

	resp, err := svc.SendMessage(ctx, created.ID, SendModelMessageRequest{Content: "do something"})
	require.NoError(t, err)
	require.Equal(t, ModelChatStatusThinking, resp.ChatStatus)

	// Expect: user message.created, then assistant message.created (error-marked),
	// then chat.error, then chat.status idle.
	waitForEvent(t, sub, EventMessageCreated) // user turn
	assistantEv := waitForEvent(t, sub, EventMessageCreated)
	asMsg := assistantEv.Payload.(ModelChatMessageResponse)
	require.Equal(t, "assistant", asMsg.Role)
	require.Empty(t, asMsg.Content) // no display text on error
	require.NotNil(t, asMsg.Error)  // error envelope must be present

	errEv := waitForEvent(t, sub, EventChatError)
	errPayload := errEv.Payload.(ChatErrorPayload)
	require.NotEmpty(t, errPayload.Code)

	idleEv := waitForEvent(t, sub, EventChatStatus)
	require.Equal(t, ModelChatStatusIdle, idleEv.Payload.(ChatStatusPayload).Status)

	// The model in the repo must be UNCHANGED — no partial merge applied.
	after := repo.get(created.ID)
	require.Equal(t, before.Agents, after.Agents)
	require.Equal(t, before.Rules, after.Rules)
	require.Equal(t, before.UpdatedAt, after.UpdatedAt)
}

// TestService_SendMessage_SecondConcurrentRejected verifies that a second
// SendMessage call while a generation is in-flight returns ErrGenerationInProgress.
// Determinism is achieved by pre-occupying the pending tracker directly (no
// goroutine races, no sleeps needed).
func TestService_SendMessage_SecondConcurrentRejected(t *testing.T) {
	repo := newFakeModelsRepo()
	chatsRepo := newFakeChatsRepo()
	fake := &fakeLLM{
		resp: `{"display_message":"ok","model":{}}`,
	}
	svc, _ := newGenTestService(repo, chatsRepo, fake)
	ctx := ctxAs("alice", false)

	created, err := svc.CreateModel(ctx, CreateModelRequest{Name: "M"})
	require.NoError(t, err)
	_, err = svc.SetChatTarget(ctx, created.ID, SetModelChatTargetRequest{LLMTarget: execTarget()})
	require.NoError(t, err)

	// Directly occupy the generation slot for this model, simulating an in-flight
	// background goroutine — no actual goroutine required, making the test fully
	// deterministic.
	inner := svc.(*service)
	ok := inner.pending.tryStart(created.ID)
	require.True(t, ok, "tryStart should succeed on first call")

	_, err = svc.SendMessage(ctx, created.ID, SendModelMessageRequest{Content: "concurrent request"})
	require.ErrorIs(t, err, ErrGenerationInProgress)

	// Release the slot so the tracker is left clean.
	inner.pending.finish(created.ID)
}
