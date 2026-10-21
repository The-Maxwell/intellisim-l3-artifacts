package main

import (
	llmbridgev1 "github.com/The-Maxwell/intellisim.llm.provider.go.sdk/proto/llmbridge/v1"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"
	"intelli.sim.core/api"
	"intelli.sim.core/data"
	"intelli.sim.core/modules/auth"
	chatsmodule "intelli.sim.core/modules/chats"
	"intelli.sim.core/modules/llm"
	"intelli.sim.core/pkg/grpcserver"
	"intelli.sim.core/pkg/httpserver"
	"intelli.sim.core/pkg/logs"
	"intelli.sim.core/pkg/mongodb"
	modelsmodule "intelli.sim.core/modules/models"
	simulationsmodule "intelli.sim.core/modules/simulations"
)

func InitializeIntelliSimCoreApp() *fx.App {
	return fx.New(
		auth.Module,
		llm.Module,
		data.Module,
		chatsmodule.Module,
		modelsmodule.Module,
		simulationsmodule.Module,
		fx.Provide(
			logs.NewLogger,
			mongodb.NewMongoDBClient,
			api.NewAPI,
			httpserver.NewHttpServer,
			grpcserver.NewGrpcServer,
		),
		fx.Invoke(func(*httpserver.HttpServer) {}),
		fx.Invoke(func(client *mongodb.MongoDBClient) { client.EnsureIndexes() }),
		fx.Invoke(registerGrpcServices),
		fx.WithLogger(func(log *zap.Logger) fxevent.Logger {
			return &fxevent.ZapLogger{Logger: log}
		}),
	)
}

func registerGrpcServices(grpcSrv *grpcserver.GrpcServer, bridge *llm.ProviderBridge) {
	llmbridgev1.RegisterLLMBridgeServer(grpcSrv.GetServer(), bridge)
}
