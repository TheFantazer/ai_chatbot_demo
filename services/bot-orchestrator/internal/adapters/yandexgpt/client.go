package yandexgpt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"ai-chatbot/services/bot-orchestrator/internal/application"
)

var ErrInvalidModelResponse = errors.New("invalid YandexGPT model response")

const (
	finalAlternativeStatus = "ALTERNATIVE_STATUS_FINAL"
	assistantRole          = "assistant"
)

type Config struct {
	BaseURL  string
	APIKey   string
	FolderID string
	Model    string
}

type Client struct {
	config Config
	http   *http.Client
}

func New(config Config, httpClient *http.Client) *Client {
	if config.BaseURL == "" {
		config.BaseURL = defaultBaseURL
	}
	return &Client{config: config, http: httpClient}
}

func (c *Client) Interpret(ctx context.Context, req application.InterpretationRequest) (application.ActionEnvelope, error) {
	request, err := c.buildCompletionRequest(req)
	if err != nil {
		return application.ActionEnvelope{}, fmt.Errorf("build YandexGPT completion request: %w", err)
	}

	response, err := c.do(ctx, request)
	if err != nil {
		return application.ActionEnvelope{}, fmt.Errorf("request YandexGPT completion: %w", err)
	}

	action, err := decodeActionEnvelope(response)
	if err != nil {
		return application.ActionEnvelope{}, err
	}
	if err := validateActionEnvelope(req, action); err != nil {
		return application.ActionEnvelope{}, err
	}

	return action, nil
}

type actionEnvelopeResponse struct {
	Action        *application.ActionType      `json:"action"`
	Arguments     *application.ActionArguments `json:"arguments"`
	StateRevision *uint64                      `json:"state_revision"`
}

func decodeActionEnvelope(response completionResponse) (application.ActionEnvelope, error) {
	if len(response.Alternatives) != 1 {
		return application.ActionEnvelope{}, fmt.Errorf("%w: expected one alternative, got %d", ErrInvalidModelResponse, len(response.Alternatives))
	}

	alternative := response.Alternatives[0]
	if alternative.Status != finalAlternativeStatus {
		return application.ActionEnvelope{}, fmt.Errorf("%w: unexpected alternative status %q", ErrInvalidModelResponse, alternative.Status)
	}
	if alternative.Message.Role != assistantRole {
		return application.ActionEnvelope{}, fmt.Errorf("%w: unexpected message role %q", ErrInvalidModelResponse, alternative.Message.Role)
	}
	if strings.TrimSpace(alternative.Message.Text) == "" {
		return application.ActionEnvelope{}, fmt.Errorf("%w: empty message text", ErrInvalidModelResponse)
	}

	decoder := json.NewDecoder(strings.NewReader(alternative.Message.Text))
	decoder.DisallowUnknownFields()

	var decoded actionEnvelopeResponse
	if err := decoder.Decode(&decoded); err != nil {
		return application.ActionEnvelope{}, fmt.Errorf("%w: decode action: %v", ErrInvalidModelResponse, err)
	}

	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return application.ActionEnvelope{}, fmt.Errorf("%w: trailing JSON value", ErrInvalidModelResponse)
		}
		return application.ActionEnvelope{}, fmt.Errorf("%w: trailing data: %v", ErrInvalidModelResponse, err)
	}

	if decoded.Action == nil {
		return application.ActionEnvelope{}, fmt.Errorf("%w: action is required", ErrInvalidModelResponse)
	}
	if decoded.Arguments == nil {
		return application.ActionEnvelope{}, fmt.Errorf("%w: arguments are required", ErrInvalidModelResponse)
	}
	if decoded.StateRevision == nil {
		return application.ActionEnvelope{}, fmt.Errorf("%w: state_revision is required", ErrInvalidModelResponse)
	}

	return application.ActionEnvelope{
		Action:        *decoded.Action,
		Arguments:     *decoded.Arguments,
		StateRevision: *decoded.StateRevision,
	}, nil
}

func validateActionEnvelope(req application.InterpretationRequest, action application.ActionEnvelope) error {
	if !isKnownAction(action.Action) {
		return fmt.Errorf("%w: unknown action %q", ErrInvalidModelResponse, action.Action)
	}

	allowed := false
	for _, allowedAction := range req.AllowedActions {
		if action.Action == allowedAction {
			allowed = true
			break
		}
	}
	if !allowed {
		return fmt.Errorf("%w: action %q is not allowed", ErrInvalidModelResponse, action.Action)
	}

	if action.StateRevision != req.State.Revision {
		return fmt.Errorf("%w: state revision mismatch", ErrInvalidModelResponse)
	}

	if action.Action == application.ActionChooseTime {
		slotID := strings.TrimSpace(action.Arguments.SlotID)
		if slotID == "" {
			return fmt.Errorf("%w: choose_time requires slot_id", ErrInvalidModelResponse)
		}
		if _, ok := req.State.OfferedSlots[slotID]; !ok {
			return fmt.Errorf("%w: slot_id was not offered", ErrInvalidModelResponse)
		}
	}

	return nil
}

var _ application.Interpreter = (*Client)(nil)
