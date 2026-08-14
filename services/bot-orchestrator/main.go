package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"ai-chatbot/services/bot-orchestrator/internal/adapters/bookingclient"
	"ai-chatbot/services/bot-orchestrator/internal/adapters/yandexgpt"
	"ai-chatbot/services/bot-orchestrator/internal/application"
	"ai-chatbot/services/bot-orchestrator/internal/config"
	"ai-chatbot/services/bot-orchestrator/internal/platform/httpserver"
	"ai-chatbot/services/bot-orchestrator/internal/transport/httpapi"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}

	httpClient := &http.Client{Timeout: cfg.HTTPTimeout}
	store := application.NewMemoryStore()
	interpreter := yandexgpt.New(yandexgpt.Config{
		APIKey:   cfg.YandexAPIKey,
		FolderID: cfg.YandexFolderID,
		Model:    cfg.YandexModel,
	}, httpClient)
	bookingGateway := bookingclient.New(cfg.BookingServiceURL, httpClient)
	workflow := application.NewWorkflow(store, interpreter, bookingGateway)

	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	if err := httpserver.Run(ctx, cfg.HTTPAddr, httpapi.New(workflow), logger); err != nil {
		logger.Error("bot-orchestrator stopped", "error", err)
		os.Exit(1)
	}
}
