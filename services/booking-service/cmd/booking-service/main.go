package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"ai-chatbot/services/booking-service/internal/adapters/yclients"
	"ai-chatbot/services/booking-service/internal/application"
	"ai-chatbot/services/booking-service/internal/config"
	"ai-chatbot/services/booking-service/internal/platform/httpserver"
	"ai-chatbot/services/booking-service/internal/transport/httpapi"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}

	httpClient := &http.Client{Timeout: cfg.HTTPTimeout}
	provider := yclients.New(yclients.Config{
		BaseURL:      cfg.YClientsBaseURL,
		PartnerToken: cfg.YClientsPartnerToken,
		UserToken:    cfg.YClientsUserToken,
		CompanyID:    cfg.YClientsCompanyID,
		Timezone:     cfg.BusinessTimezone,
	}, httpClient)
	bookingService := application.NewService(provider)

	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	if err := httpserver.Run(ctx, cfg.HTTPAddr, httpapi.New(bookingService), logger); err != nil {
		logger.Error("booking-service stopped", "error", err)
		os.Exit(1)
	}
}
