package llm

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	llmbridgev1 "github.com/The-Maxwell/intellisim.llm.provider.go.sdk/proto/llmbridge/v1"
)

func startBridge(t *testing.T) (*ProviderBridge, Registry, llmbridgev1.LLMBridgeClient) {
	t.Helper()
	logger := zap.NewNop()
	registry := NewRegistry(logger)
	bridge := NewProviderBridge(logger, registry)

	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	llmbridgev1.RegisterLLMBridgeServer(srv, bridge)
	go srv.Serve(lis)
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufconn", // passthrough: skip DNS, the dialer handles it
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	return bridge, registry, llmbridgev1.NewLLMBridgeClient(conn)
}

func validConnectRequest(uid string) *llmbridgev1.ConnectRequest {
	return &llmbridgev1.ConnectRequest{
		ProviderUid: uid, Name: "Test", Version: "1.0.0",
		Models: []*llmbridgev1.Model{{
			Uid: "m1", Name: "M1",
			Capabilities: []llmbridgev1.ModelCapability{
				llmbridgev1.ModelCapability_MODEL_CAPABILITY_SIMULATION_DECISIONS,
			},
		}},
	}
}

func connect(t *testing.T, client llmbridgev1.LLMBridgeClient, ctx context.Context, uid string) llmbridgev1.LLMBridge_ConnectClient {
	t.Helper()
	stream, err := client.Connect(ctx, validConnectRequest(uid))
	require.NoError(t, err)
	_, err = stream.Header() // blocks until accepted
	require.NoError(t, err)
	return stream
}

func simpleInvocation(id string) *llmbridgev1.Invocation {
	return &llmbridgev1.Invocation{
		InvocationId: id, ModelUid: "m1",
		Request: &llmbridgev1.Invocation_Simple{Simple: &llmbridgev1.SimpleRequest{Message: "hi"}},
	}
}

func TestBridge_RegisterInvokeComplete(t *testing.T) {
	bridge, registry, client := startBridge(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream := connect(t, client, ctx, "p1")

	require.Eventually(t, func() bool { _, ok := registry.Get("p1"); return ok },
		2*time.Second, 10*time.Millisecond)

	// caller side
	type result struct {
		resp *llmbridgev1.CompleteInvocationRequest
		err  error
	}
	resCh := make(chan result, 1)
	invokeCtx, invokeCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer invokeCancel()
	go func() {
		resp, err := bridge.Invoke(invokeCtx, "p1", simpleInvocation("i1"))
		resCh <- result{resp, err}
	}()

	// provider side: receive, then complete
	inv, err := stream.Recv()
	require.NoError(t, err)
	assert.Equal(t, "i1", inv.InvocationId)
	assert.Greater(t, inv.GetTimeoutMs(), uint64(0), "deadline must be propagated")

	ack, err := client.CompleteInvocation(context.Background(), &llmbridgev1.CompleteInvocationRequest{
		ProviderUid: "p1", InvocationId: "i1",
		Result: &llmbridgev1.CompleteInvocationRequest_Completion{
			Completion: &llmbridgev1.Completion{Text: "done"},
		},
	})
	require.NoError(t, err)
	assert.True(t, ack.Accepted)

	r := <-resCh
	require.NoError(t, r.err)
	assert.Equal(t, "done", r.resp.GetCompletion().Text)
}

func TestBridge_WrongProviderAndUnknownInvocation(t *testing.T) {
	bridge, registry, client := startBridge(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream := connect(t, client, ctx, "p1")
	require.Eventually(t, func() bool { _, ok := registry.Get("p1"); return ok },
		2*time.Second, 10*time.Millisecond)

	go bridge.Invoke(context.Background(), "p1", simpleInvocation("i2"))
	_, err := stream.Recv()
	require.NoError(t, err)

	// wrong provider uid -> not accepted, invocation still pending
	ack, err := client.CompleteInvocation(context.Background(), &llmbridgev1.CompleteInvocationRequest{
		ProviderUid: "impostor", InvocationId: "i2",
		Result: &llmbridgev1.CompleteInvocationRequest_Error{
			Error: &llmbridgev1.Error{Code: llmbridgev1.ErrorCode_ERROR_CODE_INTERNAL},
		},
	})
	require.NoError(t, err)
	assert.False(t, ack.Accepted)

	// unknown invocation id -> not accepted
	ack, err = client.CompleteInvocation(context.Background(), &llmbridgev1.CompleteInvocationRequest{
		ProviderUid: "p1", InvocationId: "ghost",
		Result: &llmbridgev1.CompleteInvocationRequest_Completion{
			Completion: &llmbridgev1.Completion{Text: "x"},
		},
	})
	require.NoError(t, err)
	assert.False(t, ack.Accepted)
}

func TestBridge_DisconnectFailsPending(t *testing.T) {
	bridge, registry, client := startBridge(t)
	ctx, cancel := context.WithCancel(context.Background())
	stream := connect(t, client, ctx, "p1")
	require.Eventually(t, func() bool { _, ok := registry.Get("p1"); return ok },
		2*time.Second, 10*time.Millisecond)

	errCh := make(chan error, 1)
	go func() {
		_, err := bridge.Invoke(context.Background(), "p1", simpleInvocation("i3"))
		errCh <- err
	}()
	_, err := stream.Recv()
	require.NoError(t, err)

	cancel() // client drops the stream -> provider disconnect

	select {
	case err := <-errCh:
		assert.ErrorIs(t, err, ErrProviderDisconnected)
	case <-time.After(5 * time.Second):
		t.Fatal("Invoke did not fail after provider disconnect")
	}
	require.Eventually(t, func() bool { _, ok := registry.Get("p1"); return !ok },
		2*time.Second, 10*time.Millisecond, "provider must be unregistered")
}

func TestBridge_NewestSessionWins(t *testing.T) {
	_, registry, client := startBridge(t)
	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()
	stream1 := connect(t, client, ctx1, "p1")
	require.Eventually(t, func() bool { _, ok := registry.Get("p1"); return ok },
		2*time.Second, 10*time.Millisecond)

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	_ = connect(t, client, ctx2, "p1") // same uid — kicks session 1

	_, err := stream1.Recv() // first stream must end
	assert.Error(t, err)

	_, ok := registry.Get("p1") // provider stays registered (new session)
	assert.True(t, ok)
}

func TestBridge_InvalidRegistrationRejected(t *testing.T) {
	_, _, client := startBridge(t)
	bad := validConnectRequest("") // empty uid
	stream, err := client.Connect(context.Background(), bad)
	require.NoError(t, err)
	// grpc-go surfaces a pre-header RPC failure via Recv, not Header():
	// Header() returns (nil, nil) on trailers-only responses.
	_, err = stream.Recv()
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestBridge_InvokeUnknownProvider(t *testing.T) {
	bridge, _, _ := startBridge(t)
	_, err := bridge.Invoke(context.Background(), "ghost", simpleInvocation("x"))
	assert.ErrorIs(t, err, ErrProviderNotFound)
}

// A session that dies between lookup and response-wait must fail the invoke
// promptly via sess.done — not hang until the caller's ctx deadline.
func TestBridge_InvokeFailsFastWhenSessionDiesMidFlight(t *testing.T) {
	bridge, registry, client := startBridge(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream := connect(t, client, ctx, "p1")
	require.Eventually(t, func() bool { _, ok := registry.Get("p1"); return ok },
		2*time.Second, 10*time.Millisecond)

	// Launch Invoke in background so it gets past the session lookup and
	// enqueues the invocation into sendCh.
	type result struct {
		resp *llmbridgev1.CompleteInvocationRequest
		err  error
	}
	resCh := make(chan result, 1)
	invokeCtx, invokeCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer invokeCancel()
	go func() {
		resp, err := bridge.Invoke(invokeCtx, "p1", simpleInvocation("mid-flight"))
		resCh <- result{resp, err}
	}()

	// Drain the invocation from the stream so Invoke is past the first select
	// and is now waiting in the second select (for the response).
	_, err := stream.Recv()
	require.NoError(t, err)

	// Now kill the session. Invoke is already past the sendCh write and is
	// blocked in the second select waiting for respCh or sess.done.
	bridge.mu.Lock()
	sess := bridge.sessions["p1"]
	bridge.mu.Unlock()
	require.NotNil(t, sess)

	start := time.Now()
	sess.close() // triggers the sess.done case in the second select

	r := <-resCh
	require.Error(t, r.err)
	assert.ErrorIs(t, r.err, ErrProviderDisconnected)
	assert.Less(t, time.Since(start), time.Second, "must fail fast via sess.done, not wait for ctx")
}

func TestBridge_InvokeWithoutDeadlineLeavesTimeoutZero(t *testing.T) {
	bridge, registry, client := startBridge(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream := connect(t, client, ctx, "p1")
	require.Eventually(t, func() bool { _, ok := registry.Get("p1"); return ok },
		2*time.Second, 10*time.Millisecond)

	type result struct {
		resp *llmbridgev1.CompleteInvocationRequest
		err  error
	}
	resCh := make(chan result, 1)
	go func() {
		resp, err := bridge.Invoke(context.Background(), "p1", simpleInvocation("no-deadline"))
		resCh <- result{resp, err}
	}()

	// Receive the invocation on the provider stream and check TimeoutMs.
	inv, err := stream.Recv()
	require.NoError(t, err)
	assert.Equal(t, uint64(0), inv.GetTimeoutMs(), "no deadline → TimeoutMs must be zero")

	// Complete it so the goroutine exits cleanly.
	ack, err := client.CompleteInvocation(context.Background(), &llmbridgev1.CompleteInvocationRequest{
		ProviderUid:  "p1",
		InvocationId: inv.InvocationId,
		Result: &llmbridgev1.CompleteInvocationRequest_Completion{
			Completion: &llmbridgev1.Completion{Text: "done"},
		},
	})
	require.NoError(t, err)
	assert.True(t, ack.Accepted)
	r := <-resCh
	require.NoError(t, r.err)
}
