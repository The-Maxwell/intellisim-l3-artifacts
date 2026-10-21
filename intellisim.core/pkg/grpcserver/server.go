package grpcserver

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/caarlos0/env/v11"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

type GrpcServer struct {
	logger *zap.Logger
	server *grpc.Server
	config grpcConfig
}

func (s *GrpcServer) GetServer() *grpc.Server {
	return s.server
}

func (s *GrpcServer) Start(ctx context.Context) {
	s.logger.Info("Start GRPC Server")

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.config.Port))
	if err != nil {
		panic(err)
	}

	s.logger.Info("Listen GRPC port", zap.Int("port", s.config.Port))
	if err := s.server.Serve(lis); err != nil {
		panic(err)
	}
}

func (s *GrpcServer) Stop(ctx context.Context) {
	done := make(chan struct{})
	go func() {
		s.server.GracefulStop()
		close(done)
	}()

	select {
	case <-ctx.Done():
		s.logger.Warn("GRPC server shutdown timed out, forcing stop")
		s.server.Stop()
	case <-done:
		s.logger.Info("GRPC server stopped gracefully")
	}
}

func NewGrpcServer(lc fx.Lifecycle, logger *zap.Logger) *GrpcServer {
	var config grpcConfig
	if err := env.Parse(&config); err != nil {
		panic(err)
	}

	var kaep = keepalive.EnforcementPolicy{
		MinTime:             10 * time.Second, // SDK clients ping every 15s — compliant
		PermitWithoutStream: true,
	}

	// No MaxConnectionIdle/Age: provider streams are long-lived by design.
	var kasp = keepalive.ServerParameters{
		Time:    30 * time.Second, // ping idle clients
		Timeout: 10 * time.Second,
	}

	// Vision payloads exceed the 4 MB gRPC default.
	const maxMessageSize = 32 * 1024 * 1024

	s := grpc.NewServer(
		grpc.KeepaliveEnforcementPolicy(kaep),
		grpc.KeepaliveParams(kasp),
		grpc.MaxRecvMsgSize(maxMessageSize),
		grpc.MaxSendMsgSize(maxMessageSize),
		grpc.UnaryInterceptor(unaryLogger(logger)),
		grpc.StreamInterceptor(streamLogger(logger)),
	)

	srv := &GrpcServer{
		logger: logger.Named("grpc"),
		server: s,
		config: config,
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go srv.Start(ctx)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			srv.Stop(ctx)
			return nil
		},
	})

	return srv
}
