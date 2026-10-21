package sdk

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	llmbridgev1 "github.com/The-Maxwell/intellisim.llm.provider.go.sdk/proto/llmbridge/v1"
)

const (
	completeRetries    = 2
	completeRetryDelay = 500 * time.Millisecond
	completeTimeout    = 10 * time.Second
)

// dispatch runs one invocation in its own goroutine and sends the result back
// via CompleteInvocation. Handlers run detached from the stream context
// (context.WithoutCancel) so a reconnect does not abort in-flight work; the
// per-invocation timeout still applies, and shutdown waits via drain().
func (p *Provider) dispatch(ctx context.Context, client llmbridgev1.LLMBridgeClient, inv *llmbridgev1.Invocation, inflight *sync.WaitGroup) {
	inflight.Add(1)
	go func() {
		defer inflight.Done()

		timeout := p.opts.DefaultTimeout
		if ms := inv.GetTimeoutMs(); ms > 0 {
			timeout = time.Duration(ms) * time.Millisecond
		}
		callCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
		defer cancel()

		started := time.Now()
		req := p.handle(callCtx, inv)
		p.fillUsageDefaults(inv, started, req)

		p.complete(client, req)
	}()
}

// handle routes the invocation to the matching handler and builds the full
// CompleteInvocationRequest. Panics are recovered into ERROR_CODE_INTERNAL.
//
// NOTE: the generated oneof interface isCompleteInvocationRequest_Result is
// unexported, so handle builds *llmbridgev1.CompleteInvocationRequest directly
// (assigning out.Result per branch) rather than returning the oneof.
func (p *Provider) handle(ctx context.Context, inv *llmbridgev1.Invocation) (out *llmbridgev1.CompleteInvocationRequest) {
	out = &llmbridgev1.CompleteInvocationRequest{
		ProviderUid:  p.opts.Registration.UID,
		InvocationId: inv.InvocationId,
	}
	defer func() {
		if r := recover(); r != nil {
			p.logger.Error("handler panic", "invocation_id", inv.InvocationId, "panic", fmt.Sprint(r))
			out.Result = errorResult(ErrCodeInternal, fmt.Sprintf("handler panic: %v", r))
		}
	}()

	switch req := inv.Request.(type) {
	case *llmbridgev1.Invocation_Simple:
		comp, err := p.handler.Simple(ctx, simpleRequestFromProto(inv, req.Simple))
		p.setCompletionResult(ctx, out, comp, err)

	case *llmbridgev1.Invocation_Chat:
		if p.chatH == nil {
			out.Result = errorResult(ErrCodeCapabilityNotSupported, "provider has no chat handler")
			return out
		}
		comp, err := p.chatH.Chat(ctx, chatRequestFromProto(inv, req.Chat))
		p.setCompletionResult(ctx, out, comp, err)

	case *llmbridgev1.Invocation_Vision:
		if p.visionH == nil {
			out.Result = errorResult(ErrCodeCapabilityNotSupported, "provider has no vision handler")
			return out
		}
		comp, err := p.visionH.Vision(ctx, visionRequestFromProto(inv, req.Vision))
		p.setCompletionResult(ctx, out, comp, err)

	case *llmbridgev1.Invocation_Reasoning:
		if p.reasonH == nil {
			out.Result = errorResult(ErrCodeCapabilityNotSupported, "provider has no reasoning handler")
			return out
		}
		comp, err := p.reasonH.Reasoning(ctx, reasoningRequestFromProto(inv, req.Reasoning))
		if err != nil {
			out.Result = p.errorResultFor(ctx, err)
			return out
		}
		out.Result = &llmbridgev1.CompleteInvocationRequest_Reasoning{Reasoning: reasoningCompletionToProto(comp)}

	default:
		out.Result = errorResult(ErrCodeInvalidRequest, "unknown invocation request type")
	}
	return out
}

// setCompletionResult assigns out.Result from a completion handler outcome. The
// generated oneof field type is unexported, so the concrete wrapper is assigned
// directly here rather than returned through a (non-nameable) interface.
func (p *Provider) setCompletionResult(ctx context.Context, out *llmbridgev1.CompleteInvocationRequest, comp *Completion, err error) {
	if err != nil {
		out.Result = p.errorResultFor(ctx, err)
		return
	}
	out.Result = &llmbridgev1.CompleteInvocationRequest_Completion{Completion: completionToProto(comp)}
}

// errorResultFor maps a handler error to the wire error wrapper.
func (p *Provider) errorResultFor(ctx context.Context, err error) *llmbridgev1.CompleteInvocationRequest_Error {
	var typed *Error
	switch {
	case errors.As(err, &typed):
		return errorResult(typed.Code, typed.Message)
	case errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil:
		return errorResult(ErrCodeTimeout, err.Error())
	default:
		return errorResult(ErrCodeUpstreamError, err.Error())
	}
}

func errorResult(code ErrorCode, message string) *llmbridgev1.CompleteInvocationRequest_Error {
	return &llmbridgev1.CompleteInvocationRequest_Error{
		Error: &llmbridgev1.Error{Code: errorCodeToProto(code), Message: message},
	}
}

// fillUsageDefaults sets ModelUid/Effort from the invocation and LatencyMs
// from the measured duration when the handler left them empty.
func (p *Provider) fillUsageDefaults(inv *llmbridgev1.Invocation, started time.Time, out *llmbridgev1.CompleteInvocationRequest) {
	var usage *llmbridgev1.TokenUsage
	switch r := out.Result.(type) {
	case *llmbridgev1.CompleteInvocationRequest_Completion:
		usage = r.Completion.Usage
	case *llmbridgev1.CompleteInvocationRequest_Reasoning:
		usage = r.Reasoning.Usage
	default:
		return
	}
	if usage == nil {
		return
	}
	if usage.ModelUid == "" {
		usage.ModelUid = inv.ModelUid
	}
	if usage.Effort == llmbridgev1.Effort_EFFORT_UNSPECIFIED {
		usage.Effort = inv.Effort
	}
	if usage.LatencyMs == 0 {
		usage.LatencyMs = float64(time.Since(started).Milliseconds())
	}
}

// complete delivers one result, retrying transient transport failures.
// ack.accepted == false means core no longer waits — drop silently.
func (p *Provider) complete(client llmbridgev1.LLMBridgeClient, req *llmbridgev1.CompleteInvocationRequest) {
	for attempt := 0; ; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), completeTimeout)
		ack, err := client.CompleteInvocation(ctx, req)
		cancel()
		if err == nil {
			if !ack.Accepted {
				p.logger.Debug("completion not accepted (core moved on)",
					"invocation_id", req.InvocationId)
			}
			return
		}
		if status.Code(err) == codes.Canceled {
			p.logger.Debug("completion dropped: connection closing",
				"invocation_id", req.InvocationId)
			return
		}
		if attempt >= completeRetries {
			p.logger.Error("failed to deliver completion",
				"invocation_id", req.InvocationId, "error", err)
			return
		}
		time.Sleep(completeRetryDelay)
	}
}
