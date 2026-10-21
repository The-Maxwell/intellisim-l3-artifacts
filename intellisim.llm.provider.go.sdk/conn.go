package sdk

import (
	"context"
	"fmt"
	"math/rand/v2"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"

	llmbridgev1 "github.com/The-Maxwell/intellisim.llm.provider.go.sdk/proto/llmbridge/v1"
)

const maxMessageSize = 32 * 1024 * 1024 // 32 MB — vision payloads exceed the 4 MB default

// Run connects to core and serves invocations until ctx is canceled.
// It reconnects automatically with exponential backoff (±20% jitter) on any
// transient failure. A registration rejection (gRPC InvalidArgument) is NOT
// retried — Run returns the error immediately.
//
// On shutdown (ctx canceled) Run waits up to DrainTimeout for in-flight
// handlers, then returns. Handlers exceeding DrainTimeout are abandoned but
// keep running detached; their completion delivery is best-effort and may
// fail against the then-closed connection. Do not assume Run returning means
// every handler goroutine has stopped.
func (p *Provider) Run(ctx context.Context) error {
	conn, err := grpc.NewClient(p.opts.CoreAddr, p.dialOptions()...)
	if err != nil {
		return fmt.Errorf("sdk: dial %s: %w", p.opts.CoreAddr, err)
	}
	defer conn.Close()
	client := llmbridgev1.NewLLMBridgeClient(conn)

	var inflight sync.WaitGroup
	defer p.drain(&inflight)

	backoff := p.opts.InitialBackoff
	for {
		if ctx.Err() != nil {
			return nil
		}

		connected, err := p.serve(ctx, client, &inflight)
		if ctx.Err() != nil {
			p.logger.Info("shutting down")
			return nil
		}
		if status.Code(err) == codes.InvalidArgument {
			return fmt.Errorf("sdk: registration rejected by core: %w", err)
		}

		if connected {
			backoff = p.opts.InitialBackoff
		} else {
			backoff = min(backoff*2, p.opts.MaxBackoff)
		}
		p.logger.Warn("connection to core lost, reconnecting",
			"error", err, "backoff", backoff.String())
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(jitter(backoff)):
		}
	}
}

// serve runs one Connect session: register, then receive and dispatch
// invocations until the stream breaks. connected reports whether the
// registration was accepted (used to reset the backoff).
func (p *Provider) serve(ctx context.Context, client llmbridgev1.LLMBridgeClient, inflight *sync.WaitGroup) (connected bool, err error) {
	stream, err := client.Connect(ctx, p.reg)
	if err != nil {
		return false, err
	}
	// Header() blocks until core accepts (sends headers). NOTE: on rejection,
	// grpc-go's Header() returns (nil, nil) — no metadata, no error — and the
	// status error (e.g. InvalidArgument for a rejected registration) surfaces
	// on the first Recv() below, which Run inspects for the no-retry decision.
	// Non-nil metadata is therefore the acceptance signal.
	md, err := stream.Header()
	if err != nil {
		return false, err
	}
	accepted := md != nil
	if accepted {
		p.logger.Info("connected to core", "core_addr", p.opts.CoreAddr,
			"models", len(p.opts.Registration.Models))
	}

	for {
		inv, err := stream.Recv()
		if err != nil {
			return accepted, err
		}
		p.dispatch(ctx, client, inv, inflight)
	}
}

func (p *Provider) dialOptions() []grpc.DialOption {
	if len(p.opts.DialOptions) > 0 {
		return p.opts.DialOptions
	}
	return []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                15 * time.Second,
			Timeout:             5 * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(maxMessageSize),
			grpc.MaxCallSendMsgSize(maxMessageSize),
		),
	}
}

// drain waits up to DrainTimeout for in-flight handlers to finish.
func (p *Provider) drain(inflight *sync.WaitGroup) {
	done := make(chan struct{})
	go func() {
		inflight.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(p.opts.DrainTimeout):
		p.logger.Warn("drain timeout: abandoning in-flight invocations",
			"drain_timeout", p.opts.DrainTimeout.String())
	}
}

// jitter returns d ±20%.
func jitter(d time.Duration) time.Duration {
	f := 0.8 + 0.4*rand.Float64()
	return time.Duration(float64(d) * f)
}
