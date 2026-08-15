package yandexgpt

import (
	"context"
	"errors"
	"net/http"

	"ai-chatbot/services/bot-orchestrator/internal/application"
)

var ErrNotImplemented = errors.New("YandexGPT client is not implemented")

type Config struct {
	APIKey   string
	FolderID string
	Model    string
}

type Client struct {
	config Config
	http   *http.Client
}

func New(config Config, httpClient *http.Client) *Client {
	return &Client{config: config, http: httpClient}
}

// implement me
func (c *Client) Interpret(context.Context, application.InterpretationRequest) (application.ActionEnvelope, error) {
	return application.ActionEnvelope{}, ErrNotImplemented
}

var _ application.Interpreter = (*Client)(nil)
