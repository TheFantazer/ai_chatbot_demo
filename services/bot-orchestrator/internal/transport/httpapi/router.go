package httpapi

import (
	"context"
	"net/http"

	"ai-chatbot/services/bot-orchestrator/internal/application"
)

type Application interface {
	HandleMessage(context.Context, application.InboundMessage) (application.OutboundMessage, error)
}

type server struct {
	app Application
}

func New(app Application) http.Handler {
	s := &server{app: app}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/messages", s.handleMessage)
	return mux
}
