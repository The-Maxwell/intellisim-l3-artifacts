package sdk

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	llmbridgev1 "github.com/The-Maxwell/intellisim.llm.provider.go.sdk/proto/llmbridge/v1"
)

// ---------- fake core bridge ----------

type fakeSession struct {
	reg  *llmbridgev1.ConnectRequest
	send chan *llmbridgev1.Invocation
	stop chan struct{} // close to end the stream server-side
}

type fakeBridge struct {
	llmbridgev1.UnimplementedLLMBridgeServer
	mu              sync.Mutex
	rejectWith      error
	acceptAll       bool // ack.accepted value
	failCompletions int  // number of CompleteInvocation calls to fail first
	sessions        chan *fakeSession
	completions     chan *llmbridgev1.CompleteInvocationRequest
}

func newFakeBridge() *fakeBridge {
	return &fakeBridge{
		acceptAll:   true,
		sessions:    make(chan *fakeSession, 4),
		completions: make(chan *llmbridgev1.CompleteInvocationRequest, 16),
	}
}

func (f *fakeBridge) setRejectWith(err error) { f.mu.Lock(); f.rejectWith = err; f.mu.Unlock() }
func (f *fakeBridge) setAcceptAll(v bool)     { f.mu.Lock(); f.acceptAll = v; f.mu.Unlock() }

func (f *fakeBridge) Connect(req *llmbridgev1.ConnectRequest, stream llmbridgev1.LLMBridge_ConnectServer) error {
	f.mu.Lock()
	reject := f.rejectWith
	f.mu.Unlock()
	if reject != nil {
		return reject
	}
	if err := stream.SendHeader(metadata.MD{}); err != nil {
		return err
	}
	s := &fakeSession{reg: req, send: make(chan *llmbridgev1.Invocation, 8), stop: make(chan struct{})}
	f.sessions <- s
	for {
		select {
		case inv := <-s.send:
			if err := stream.Send(inv); err != nil {
				return err
			}
		case <-s.stop:
			return nil
		case <-stream.Context().Done():
			return nil
		}
	}
}

func (f *fakeBridge) CompleteInvocation(ctx context.Context, req *llmbridgev1.CompleteInvocationRequest) (*llmbridgev1.CompleteInvocationAck, error) {
	f.mu.Lock()
	if f.failCompletions > 0 {
		f.failCompletions--
		f.mu.Unlock()
		return nil, status.Error(codes.Unavailable, "transient")
	}
	accepted := f.acceptAll
	f.mu.Unlock()
	f.completions <- req
	return &llmbridgev1.CompleteInvocationAck{Accepted: accepted}, nil
}

func startFakeBridge(t *testing.T) (*fakeBridge, []grpc.DialOption) {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	fake := newFakeBridge()
	llmbridgev1.RegisterLLMBridgeServer(srv, fake)
	go srv.Serve(lis)
	t.Cleanup(srv.Stop)
	dialer := func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }
	return fake, []grpc.DialOption{
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
}

func fastOptions(fakeDial []grpc.DialOption, h Handler, reg Registration) Options {
	return Options{
		CoreAddr:       "passthrough:///bufconn", // passthrough: skip DNS, the dialer handles it
		Registration:   reg,
		Handler:        h,
		DialOptions:    fakeDial,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     50 * time.Millisecond,
		DefaultTimeout: 2 * time.Second,
		DrainTimeout:   2 * time.Second,
	}
}

func fullRegistration() Registration {
	return Registration{
		UID: "test", Name: "Test", Version: "1.0.0",
		Models: []Model{{
			UID: "m1", Name: "M1",
			Capabilities: []Capability{CapabilityChat, CapabilityModelGeneration,
				CapabilitySimulationGeneration, CapabilitySimulationExecution, CapabilitySimulationDecisions,
				CapabilityResultsAnalysis, CapabilityVisionAnalysis},
			Efforts: []Effort{EffortLow, EffortMedium, EffortHigh},
		}},
	}
}

func waitSession(t *testing.T, f *fakeBridge) *fakeSession {
	t.Helper()
	select {
	case s := <-f.sessions:
		return s
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for provider session")
		return nil
	}
}

func waitCompletion(t *testing.T, f *fakeBridge) *llmbridgev1.CompleteInvocationRequest {
	t.Helper()
	select {
	case c := <-f.completions:
		return c
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for completion")
		return nil
	}
}

// ---------- tests ----------

func TestRun_RegistersAndDispatchesAllTypes(t *testing.T) {
	fake, dial := startFakeBridge(t)
	p, err := New(fastOptions(dial, fullHandler{}, fullRegistration()))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- p.Run(ctx) }()

	sess := waitSession(t, fake)
	assert.Equal(t, "test", sess.reg.ProviderUid)
	require.Len(t, sess.reg.Models, 1)
	assert.Len(t, sess.reg.Models[0].Capabilities, 7)

	// simple
	sess.send <- &llmbridgev1.Invocation{InvocationId: "i1", ModelUid: "m1",
		Request: &llmbridgev1.Invocation_Simple{Simple: &llmbridgev1.SimpleRequest{Message: "hello"}}}
	c1 := waitCompletion(t, fake)
	assert.Equal(t, "i1", c1.InvocationId)
	assert.Equal(t, "test", c1.ProviderUid)
	require.NotNil(t, c1.GetCompletion())
	assert.Equal(t, "ok", c1.GetCompletion().Text)
	// usage auto-fill: model uid + latency
	assert.Equal(t, "m1", c1.GetCompletion().Usage.ModelUid)

	// chat
	sess.send <- &llmbridgev1.Invocation{InvocationId: "i2", ModelUid: "m1",
		Request: &llmbridgev1.Invocation_Chat{Chat: &llmbridgev1.ChatRequest{Message: "hi"}}}
	c2 := waitCompletion(t, fake)
	assert.Equal(t, "chat:hi", c2.GetCompletion().Text)

	// vision
	sess.send <- &llmbridgev1.Invocation{InvocationId: "i3", ModelUid: "m1",
		Request: &llmbridgev1.Invocation_Vision{Vision: &llmbridgev1.VisionRequest{Message: "img", Images: []*llmbridgev1.VisionImage{{ImageData: []byte{1}, ImageFormat: "png"}}}}}
	c3 := waitCompletion(t, fake)
	assert.Equal(t, "vision", c3.GetCompletion().Text)

	// reasoning — effort auto-fill check
	sess.send <- &llmbridgev1.Invocation{InvocationId: "i4", ModelUid: "m1",
		Effort:  llmbridgev1.Effort_EFFORT_HIGH,
		Request: &llmbridgev1.Invocation_Reasoning{Reasoning: &llmbridgev1.ReasoningRequest{Prompt: "why"}}}
	c4 := waitCompletion(t, fake)
	require.NotNil(t, c4.GetReasoning())
	assert.Equal(t, "analysis", c4.GetReasoning().Analysis)
	assert.Equal(t, llmbridgev1.Effort_EFFORT_HIGH, c4.GetReasoning().Usage.Effort)

	cancel()
	assert.NoError(t, <-done)
}

type erroringHandler struct{ fullHandler }

func (erroringHandler) Simple(ctx context.Context, req *SimpleRequest) (*Completion, error) {
	switch req.Message {
	case "typed":
		return nil, Errorf(ErrCodeModelNotSupported, "nope")
	case "panic":
		panic("boom")
	case "slow":
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(10 * time.Second):
			return &Completion{Text: "late"}, nil
		}
	}
	return nil, assert.AnError
}

func TestRun_ErrorMapping(t *testing.T) {
	fake, dial := startFakeBridge(t)
	p, err := New(fastOptions(dial, erroringHandler{}, fullRegistration()))
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- p.Run(ctx) }()
	sess := waitSession(t, fake)

	send := func(id, msg string, timeoutMs uint64) {
		inv := &llmbridgev1.Invocation{InvocationId: id, ModelUid: "m1",
			Request: &llmbridgev1.Invocation_Simple{Simple: &llmbridgev1.SimpleRequest{Message: msg}}}
		if timeoutMs > 0 {
			inv.TimeoutMs = &timeoutMs
		}
		sess.send <- inv
	}

	send("e1", "typed", 0)
	r1 := waitCompletion(t, fake)
	require.NotNil(t, r1.GetError())
	assert.Equal(t, llmbridgev1.ErrorCode_ERROR_CODE_MODEL_NOT_SUPPORTED, r1.GetError().Code)

	send("e2", "plain", 0)
	r2 := waitCompletion(t, fake)
	assert.Equal(t, llmbridgev1.ErrorCode_ERROR_CODE_UPSTREAM_ERROR, r2.GetError().Code)

	send("e3", "panic", 0)
	r3 := waitCompletion(t, fake)
	assert.Equal(t, llmbridgev1.ErrorCode_ERROR_CODE_INTERNAL, r3.GetError().Code)

	send("e4", "slow", 50) // 50ms timeout
	r4 := waitCompletion(t, fake)
	assert.Equal(t, llmbridgev1.ErrorCode_ERROR_CODE_TIMEOUT, r4.GetError().Code)
	cancel()
	assert.NoError(t, <-done)
}

func TestRun_ReconnectsAfterStreamBreak(t *testing.T) {
	fake, dial := startFakeBridge(t)
	p, err := New(fastOptions(dial, fullHandler{}, fullRegistration()))
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- p.Run(ctx) }()

	s1 := waitSession(t, fake)
	close(s1.stop) // server ends the stream → SDK must reconnect

	s2 := waitSession(t, fake) // second registration proves reconnection
	assert.Equal(t, "test", s2.reg.ProviderUid)

	// new session still serves invocations
	s2.send <- &llmbridgev1.Invocation{InvocationId: "after", ModelUid: "m1",
		Request: &llmbridgev1.Invocation_Simple{Simple: &llmbridgev1.SimpleRequest{Message: "x"}}}
	c := waitCompletion(t, fake)
	assert.Equal(t, "after", c.InvocationId)
	cancel()
	assert.NoError(t, <-done)
}

func TestRun_RegistrationRejectedNoRetry(t *testing.T) {
	fake, dial := startFakeBridge(t)
	fake.setRejectWith(status.Error(codes.InvalidArgument, "bad registration"))
	p, err := New(fastOptions(dial, fullHandler{}, fullRegistration()))
	require.NoError(t, err)

	runErr := make(chan error, 1)
	go func() { runErr <- p.Run(context.Background()) }()
	select {
	case err := <-runErr:
		assert.ErrorContains(t, err, "registration rejected")
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return on registration rejection")
	}
}

func TestRun_ShutdownDrainsInFlight(t *testing.T) {
	fake, dial := startFakeBridge(t)
	slow := &drainHandler{started: make(chan struct{}), release: make(chan struct{})}
	reg := fullRegistration()
	reg.Models[0].Capabilities = []Capability{CapabilitySimulationDecisions}
	p, err := New(fastOptions(dial, slow, reg))
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- p.Run(ctx) }()
	sess := waitSession(t, fake)

	sess.send <- &llmbridgev1.Invocation{InvocationId: "d1", ModelUid: "m1",
		Request: &llmbridgev1.Invocation_Simple{Simple: &llmbridgev1.SimpleRequest{Message: "x"}}}
	<-slow.started
	cancel()                     // shutdown while handler in flight
	close(slow.release)          // let the handler finish now
	c := waitCompletion(t, fake) // completion still delivered during drain
	assert.Equal(t, "d1", c.InvocationId)
	assert.Greater(t, c.GetCompletion().Usage.LatencyMs, 0.0)
	assert.NoError(t, <-done)
}

type drainHandler struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (h *drainHandler) Simple(ctx context.Context, req *SimpleRequest) (*Completion, error) {
	h.once.Do(func() { close(h.started) })
	<-h.release
	time.Sleep(5 * time.Millisecond)
	return &Completion{Text: "drained"}, nil
}

func TestRun_NotAcceptedCompletionDropped(t *testing.T) {
	fake, dial := startFakeBridge(t)
	p, err := New(fastOptions(dial, fullHandler{}, fullRegistration()))
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- p.Run(ctx) }()

	sess := waitSession(t, fake)
	fake.setAcceptAll(false)

	sess.send <- &llmbridgev1.Invocation{InvocationId: "na1", ModelUid: "m1",
		Request: &llmbridgev1.Invocation_Simple{Simple: &llmbridgev1.SimpleRequest{Message: "hello"}}}

	// The fake records the completion before acking false.
	waitCompletion(t, fake)

	// Provider must NOT retry — assert no second completion arrives within 300ms.
	select {
	case extra := <-fake.completions:
		t.Fatalf("unexpected retry of completion %s", extra.InvocationId)
	case <-time.After(300 * time.Millisecond):
	}

	cancel()
	assert.NoError(t, <-done)
}

func TestRun_CompletionRetryOnTransientError(t *testing.T) {
	fake, dial := startFakeBridge(t)
	p, err := New(fastOptions(dial, fullHandler{}, fullRegistration()))
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- p.Run(ctx) }()

	sess := waitSession(t, fake)

	fake.mu.Lock()
	fake.failCompletions = 1
	fake.mu.Unlock()

	sess.send <- &llmbridgev1.Invocation{InvocationId: "tr1", ModelUid: "m1",
		Request: &llmbridgev1.Invocation_Simple{Simple: &llmbridgev1.SimpleRequest{Message: "hello"}}}

	// Allow up to 5s — the retry delay is 500ms.
	select {
	case c := <-fake.completions:
		assert.Equal(t, "tr1", c.InvocationId)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for retried completion")
	}

	cancel()
	assert.NoError(t, <-done)
}
