package vk

import (
	"context"
	"errors"

	"ai-chatbot/services/bot-orchestrator/internal/application"
)

var ErrNotImplemented = errors.New("VK transport is not implemented")

type Application interface {
	HandleMessage(context.Context, application.InboundMessage) (application.OutboundMessage, error)
}

type Config struct {
	GroupID    string
	Token      string
	APIVersion string
}

type Transport struct {
	config Config
	app    Application
}

func New(config Config, app Application) *Transport {
	return &Transport{config: config, app: app}
}

// implement me
func (t *Transport) Run(context.Context) error {
	return ErrNotImplemented
}
