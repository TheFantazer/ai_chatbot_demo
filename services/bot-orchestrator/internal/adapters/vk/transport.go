package vk

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ai-chatbot/services/bot-orchestrator/internal/application"
)

const (
	defaultAPIBaseURL     = "https://api.vk.com/method/"
	defaultLongPollWait   = 25
	defaultEventCacheSize = 4096
)

var ErrInvalidConfiguration = errors.New("invalid VK configuration")

type Application interface {
	HandleMessage(context.Context, application.InboundMessage) (application.OutboundMessage, error)
}

type Config struct {
	GroupID        string
	Token          string
	APIVersion     string
	APIBaseURL     string
	LongPollWait   int
	EventCacheSize int
}

type cachedEvent struct {
	PeerID int64
	Reply  application.OutboundMessage
	Sent   bool
}

type Transport struct {
	config     Config
	app        Application
	http       *http.Client
	logger     *slog.Logger
	groupID    int64
	events     map[string]cachedEvent
	eventOrder []string
}

func New(config Config, app Application, httpClient *http.Client, logger *slog.Logger) *Transport {
	if strings.TrimSpace(config.APIBaseURL) == "" {
		config.APIBaseURL = defaultAPIBaseURL
	}
	if config.LongPollWait <= 0 {
		config.LongPollWait = defaultLongPollWait
	}
	if config.EventCacheSize <= 0 {
		config.EventCacheSize = defaultEventCacheSize
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: time.Duration(config.LongPollWait+10) * time.Second}
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Transport{config: config, app: app, http: httpClient, logger: logger, events: make(map[string]cachedEvent)}
}

func (t *Transport) Run(ctx context.Context) error {
	if err := t.validateConfiguration(); err != nil {
		return err
	}
	server, err := t.getLongPollServer(ctx)
	if err != nil {
		return err
	}

	for {
		response, err := t.poll(ctx, server)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			t.logger.Error("poll VK events", "error", err)
			if err := waitForRetry(ctx); err != nil {
				return nil
			}
			continue
		}

		switch response.Failed {
		case 0:
			server.TS = response.TS
		case 1:
			server.TS = response.TS
			continue
		case 2, 3:
			server, err = t.getLongPollServer(ctx)
			if err != nil {
				return err
			}
			continue
		default:
			return fmt.Errorf("VK long poll returned unsupported failure code %d", response.Failed)
		}

		for _, event := range response.Updates {
			if err := t.handleEvent(ctx, event); err != nil {
				t.logger.Error("handle VK event", "event_id", event.EventID, "error", err)
			}
		}
	}
}

func (t *Transport) handleEvent(ctx context.Context, event eventDTO) error {
	if event.Type != "message_new" || event.GroupID != t.groupID || event.Object.Message.Out != 0 || event.Object.Message.FromID <= 0 {
		return nil
	}
	eventID := strings.TrimSpace(event.EventID)
	text := strings.TrimSpace(event.Object.Message.Text)
	if eventID == "" || event.Object.Message.PeerID <= 0 || text == "" {
		return nil
	}

	if cached, found := t.events[eventID]; found {
		if cached.Sent {
			return nil
		}
		if err := t.sendMessage(ctx, cached.PeerID, eventID, cached.Reply.Text); err != nil {
			return err
		}
		t.markEventSent(eventID)
		return nil
	}

	reply, err := t.app.HandleMessage(ctx, application.InboundMessage{
		EventID:        "vk:" + strconv.FormatInt(t.groupID, 10) + ":" + eventID,
		ConversationID: "vk:" + strconv.FormatInt(t.groupID, 10) + ":" + strconv.FormatInt(event.Object.Message.PeerID, 10),
		Text:           text,
	})
	if err != nil {
		return fmt.Errorf("handle application message: %w", err)
	}
	if strings.TrimSpace(reply.Text) == "" {
		return errors.New("application returned an empty VK reply")
	}

	t.rememberEvent(eventID, cachedEvent{PeerID: event.Object.Message.PeerID, Reply: reply})
	if err := t.sendMessage(ctx, event.Object.Message.PeerID, eventID, reply.Text); err != nil {
		return err
	}
	t.markEventSent(eventID)
	return nil
}

func (t *Transport) markEventSent(eventID string) {
	event, found := t.events[eventID]
	if !found {
		return
	}
	event.Sent = true
	t.events[eventID] = event
}

func (t *Transport) rememberEvent(eventID string, event cachedEvent) {
	if _, found := t.events[eventID]; found {
		return
	}
	t.events[eventID] = event
	t.eventOrder = append(t.eventOrder, eventID)
	if len(t.eventOrder) <= t.config.EventCacheSize {
		return
	}
	oldest := t.eventOrder[0]
	t.eventOrder = t.eventOrder[1:]
	delete(t.events, oldest)
}

func (t *Transport) validateConfiguration() error {
	groupID, err := strconv.ParseInt(strings.TrimSpace(t.config.GroupID), 10, 64)
	if err != nil || groupID <= 0 {
		return fmt.Errorf("%w: group ID must be a positive integer", ErrInvalidConfiguration)
	}
	if strings.TrimSpace(t.config.Token) == "" {
		return fmt.Errorf("%w: token is required", ErrInvalidConfiguration)
	}
	if strings.TrimSpace(t.config.APIVersion) == "" {
		return fmt.Errorf("%w: API version is required", ErrInvalidConfiguration)
	}
	if t.app == nil {
		return fmt.Errorf("%w: application is required", ErrInvalidConfiguration)
	}
	if t.http == nil {
		return fmt.Errorf("%w: HTTP client is required", ErrInvalidConfiguration)
	}
	t.groupID = groupID
	return nil
}

func waitForRetry(ctx context.Context) error {
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
