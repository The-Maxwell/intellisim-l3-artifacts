// Copyright (c) 2026 Vladyslav Pryshchepa
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0
// Noncommercial use only. Commercial licensing: vlad.prishhepa@gmail.com

package main

import (
	"context"
	"log/slog"
	"syscall"

	"os/signal"

	"go.uber.org/zap"
	"go.uber.org/zap/exp/zapslog"

	sdk "github.com/The-Maxwell/intellisim.llm.provider.go.sdk"

	"intelli.sim.openai.llm.provider/internal/config"
	"intelli.sim.openai.llm.provider/internal/provider"
	"intelli.sim.openai.llm.provider/pkg/logs"
)

func main() {
	logger := logs.NewLogger()
	defer logger.Sync()

	logger.Info("Starting OpenAI LLM Provider")

	cfg, err := config.Load(logger)
	if err != nil {
		logger.Fatal("Failed to load config", zap.Error(err))
	}

	client := provider.NewOpenAIClient(logger, cfg)
	handler := provider.NewOpenAIHandler(logger, client)

	p, err := sdk.New(sdk.Options{
		CoreAddr: cfg.CoreAddr(),
		Logger:   slog.New(zapslog.NewHandler(logger.Core())),
		Handler:  handler,
		Registration: sdk.Registration{
			UID:         cfg.ProviderID,
			Name:        cfg.Name,
			Description: "OpenAI models via the IntelliSim provider SDK",
			Version:     "2.0.0",
			Models:      provider.Models,
		},
	})
	if err != nil {
		logger.Fatal("Invalid provider configuration", zap.Error(err))
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := p.Run(ctx); err != nil {
		logger.Fatal("Provider stopped with error", zap.Error(err))
	}
	logger.Info("Provider stopped")
}
