package yandexgpt

import "ai-chatbot/services/bot-orchestrator/internal/application"

// implement me
func (c *Client) buildCompletionRequest(application.InterpretationRequest) (completionRequest, error) {
	return completionRequest{}, ErrNotImplemented
}
