package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"ai-chatbot/services/bot-orchestrator/internal/application"
)

const maxRequestBytes = 1 << 20

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

func (s *server) handleMessage(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var message application.InboundMessage
	if err := decoder.Decode(&message); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	if strings.TrimSpace(message.EventID) == "" ||
		strings.TrimSpace(message.ConversationID) == "" ||
		strings.TrimSpace(message.Text) == "" {
		writeError(w, http.StatusBadRequest, "event_id, conversation_id and text are required")
		return
	}

	reply, err := s.app.HandleMessage(r.Context(), message)
	if err != nil {
		if errors.Is(err, application.ErrWorkflowNotImplemented) {
			writeError(w, http.StatusNotImplemented, err.Error())
			return
		}

		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, reply)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
