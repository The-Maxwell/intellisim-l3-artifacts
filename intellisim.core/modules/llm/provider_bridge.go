package llm

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	llmbridgev1 "github.com/The-Maxwell/intellisim.llm.provider.go.sdk/proto/llmbridge/v1"
)

// sendQueueSize buffers invocations per provider stream so Invoke does not
// block on the single stream writer under burst load.
const sendQueueSize = 64

// ProviderBridge implements the llmbridge.v1.LLMBridge gRPC service: it keeps
// one streaming session per connected provider, pushes invocations through the
// stream, and correlates unary completions back to waiting callers.
type ProviderBridge struct {
	llmbridgev1.UnimplementedLLMBridgeServer

	logger   *zap.Logger
	registry Registry

	mu       sync.Mutex
	sessions map[string]*providerSession
	pending  map[string]*pendingInvocation
}

type providerSession struct {
	uid    string
	sendCh chan *llmbridgev1.Invocation
	done   chan struct{} // closed when the session ends or is kicked
	once   sync.Once
}

func (s *providerSession) close() { s.once.Do(func() { close(s.done) }) }

type pendingInvocation struct {
	session *providerSession
	respCh  chan *llmbridgev1.CompleteInvocationRequest // buffered(1); nil delivery = disconnected
}

func NewProviderBridge(logger *zap.Logger, registry Registry) *ProviderBridge {
	return &ProviderBridge{
		logger:   logger.Named("provider-bridge"),
		registry: registry,
		sessions: make(map[string]*providerSession),
		pending:  make(map[string]*pendingInvocation),
	}
}

// Connect — provider registers and receives invocations on the returned stream.
// The stream IS liveness: when it ends, the provider is unregistered and its
// pending invocations fail.
func (b *ProviderBridge) Connect(req *llmbridgev1.ConnectRequest, stream llmbridgev1.LLMBridge_ConnectServer) error {
	if err := validateRegistration(req); err != nil {
		b.logger.Warn("Rejected provider registration", zap.Error(err))
		return status.Error(codes.InvalidArgument, err.Error())
	}

	sess := &providerSession{
		uid:    req.ProviderUid,
		sendCh: make(chan *llmbridgev1.Invocation, sendQueueSize),
		done:   make(chan struct{}),
	}

	b.mu.Lock()
	if old, ok := b.sessions[req.ProviderUid]; ok {
		b.logger.Warn("Provider reconnected while previous session alive — replacing (newest wins)",
			zap.String("provider_uid", req.ProviderUid))
		old.close()
		b.failPendingLocked(old)
	}
	b.sessions[req.ProviderUid] = sess
	b.mu.Unlock()

	// Header delivery = "registration accepted" signal; the SDK blocks on it.
	if err := stream.SendHeader(metadata.MD{}); err != nil {
		b.cleanupSession(sess)
		return err
	}

	b.registry.Register(NewRemoteProvider(b.logger, b, req, time.Now()))
	b.logger.Info("Provider connected",
		zap.String("provider_uid", req.ProviderUid), zap.Int("models", len(req.Models)))
	defer b.cleanupSession(sess)

	for {
		select {
		case inv := <-sess.sendCh:
			if err := stream.Send(inv); err != nil {
				return err
			}
		case <-sess.done: // kicked by a newer session
			return nil
		case <-stream.Context().Done():
			return nil
		}
	}
}

// Invoke sends one invocation to a connected provider and waits for the
// completion, ctx cancellation, or provider disconnect.
// Invoke sets inv.TimeoutMs from ctx's deadline — pass a freshly built, non-shared Invocation.
func (b *ProviderBridge) Invoke(ctx context.Context, providerUID string, inv *llmbridgev1.Invocation) (*llmbridgev1.CompleteInvocationRequest, error) {
	b.mu.Lock()
	sess, ok := b.sessions[providerUID]
	if !ok {
		b.mu.Unlock()
		return nil, fmt.Errorf("%w: %q", ErrProviderNotFound, providerUID)
	}
	if deadline, has := ctx.Deadline(); has {
		if remaining := time.Until(deadline); remaining > 0 {
			inv.TimeoutMs = proto.Uint64(uint64(remaining.Milliseconds()))
		}
	}
	respCh := make(chan *llmbridgev1.CompleteInvocationRequest, 1)
	b.pending[inv.InvocationId] = &pendingInvocation{session: sess, respCh: respCh}
	b.mu.Unlock()

	removePending := func() {
		b.mu.Lock()
		delete(b.pending, inv.InvocationId)
		b.mu.Unlock()
	}

	select {
	case sess.sendCh <- inv:
	case <-sess.done:
		removePending()
		return nil, fmt.Errorf("%w: %q", ErrProviderDisconnected, providerUID)
	case <-ctx.Done():
		removePending()
		return nil, ctx.Err()
	}

	select {
	case resp := <-respCh:
		if resp == nil { // session cleanup delivered the disconnect signal
			return nil, fmt.Errorf("%w: %q", ErrProviderDisconnected, providerUID)
		}
		return resp, nil
	case <-sess.done:
		removePending()
		return nil, fmt.Errorf("%w: %q", ErrProviderDisconnected, providerUID)
	case <-ctx.Done():
		removePending()
		return nil, ctx.Err()
	}
}

// CompleteInvocation — provider delivers one result. accepted=false tells the
// provider that core no longer waits (timeout/disconnect); it must drop the result.
func (b *ProviderBridge) CompleteInvocation(ctx context.Context, req *llmbridgev1.CompleteInvocationRequest) (*llmbridgev1.CompleteInvocationAck, error) {
	b.mu.Lock()
	p, ok := b.pending[req.InvocationId]
	if ok && p.session.uid != req.ProviderUid {
		ok = false // response from a provider that does not own this invocation
	} else if ok {
		delete(b.pending, req.InvocationId)
	}
	b.mu.Unlock()

	if !ok {
		return &llmbridgev1.CompleteInvocationAck{Accepted: false}, nil
	}
	p.respCh <- req // buffered(1); entry-in-map guarantees an empty buffer
	return &llmbridgev1.CompleteInvocationAck{Accepted: true}, nil
}

// cleanupSession unregisters the provider (only if this session is still
// current — a newer session may have replaced it) and fails its pending work.
func (b *ProviderBridge) cleanupSession(sess *providerSession) {
	sess.close()
	b.mu.Lock()
	if current, ok := b.sessions[sess.uid]; ok && current == sess {
		delete(b.sessions, sess.uid)
		b.registry.Unregister(sess.uid)
	}
	b.failPendingLocked(sess)
	b.mu.Unlock()
	b.logger.Info("Provider session closed", zap.String("provider_uid", sess.uid))
}

// failPendingLocked delivers the disconnect signal (nil) to every pending
// invocation owned by sess. Caller must hold b.mu.
func (b *ProviderBridge) failPendingLocked(sess *providerSession) {
	for id, p := range b.pending {
		if p.session == sess {
			p.respCh <- nil
			delete(b.pending, id)
		}
	}
}

func validateRegistration(req *llmbridgev1.ConnectRequest) error {
	if req.ProviderUid == "" {
		return fmt.Errorf("provider_uid is required")
	}
	if len(req.Models) == 0 {
		return fmt.Errorf("at least one model is required")
	}
	seen := make(map[string]bool, len(req.Models))
	for i, m := range req.Models {
		if m.Uid == "" {
			return fmt.Errorf("model #%d: uid is required", i)
		}
		if seen[m.Uid] {
			return fmt.Errorf("duplicate model uid %q", m.Uid)
		}
		seen[m.Uid] = true
		if len(m.Capabilities) == 0 {
			return fmt.Errorf("model %q: at least one capability is required", m.Uid)
		}
		for _, c := range m.Capabilities {
			if capabilityFromProto(c) == "" {
				return fmt.Errorf("model %q: unknown capability value %d", m.Uid, c)
			}
		}
		for _, e := range m.Efforts {
			if effortFromProto(e) == "" {
				return fmt.Errorf("model %q: unknown effort value %d", m.Uid, e)
			}
		}
		for _, cp := range m.ConfigParams {
			if cp.GetKey() == "" {
				return fmt.Errorf("model %q: config param with empty key", m.Uid)
			}
			if configParamTypeFromProto(cp.GetType()) == "" {
				return fmt.Errorf("model %q: config param %q has unknown type", m.Uid, cp.GetKey())
			}
		}
	}
	return nil
}
