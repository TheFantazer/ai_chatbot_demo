package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	HTTPAddr          string
	BookingServiceURL string
	HTTPTimeout       time.Duration
	ConversationTTL   time.Duration
	BusinessTimezone  string

	YandexAPIKey   string
	YandexFolderID string
	YandexModel    string

	VKGroupID    string
	VKToken      string
	VKAPIVersion string
}

func Load() (Config, error) {
	httpTimeout, err := duration("HTTP_TIMEOUT", 5*time.Second)
	if err != nil {
		return Config{}, err
	}

	conversationTTL, err := duration("CONVERSATION_TTL", 30*time.Minute)
	if err != nil {
		return Config{}, err
	}

	return Config{
		HTTPAddr:          value("BOT_HTTP_ADDR", ":8080"),
		BookingServiceURL: value("BOOKING_SERVICE_URL", "http://localhost:8081"),
		HTTPTimeout:       httpTimeout,
		ConversationTTL:   conversationTTL,
		BusinessTimezone:  value("BUSINESS_TIMEZONE", "Europe/Moscow"),
		YandexAPIKey:      os.Getenv("YANDEX_API_KEY"),
		YandexFolderID:    os.Getenv("YANDEX_FOLDER_ID"),
		YandexModel:       os.Getenv("YANDEX_MODEL"),
		VKGroupID:         os.Getenv("VK_GROUP_ID"),
		VKToken:           os.Getenv("VK_TOKEN"),
		VKAPIVersion:      os.Getenv("VK_API_VERSION"),
	}, nil
}

func value(name, fallback string) string {
	if current := os.Getenv(name); current != "" {
		return current
	}
	return fallback
}

func duration(name string, fallback time.Duration) (time.Duration, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback, nil
	}

	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}
	return parsed, nil
}
