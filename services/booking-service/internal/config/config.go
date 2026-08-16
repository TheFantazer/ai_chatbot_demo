package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	HTTPAddr    string
	HTTPTimeout time.Duration

	YClientsBaseURL      string
	YClientsPartnerToken string
	YClientsUserToken    string
	YClientsCompanyID    string
	BusinessTimezone     string
}

func Load() (Config, error) {
	httpTimeout, err := duration("HTTP_TIMEOUT", 5*time.Second)
	if err != nil {
		return Config{}, err
	}

	return Config{
		HTTPAddr:             value("BOOKING_HTTP_ADDR", ":8081"),
		HTTPTimeout:          httpTimeout,
		YClientsBaseURL:      value("YCLIENTS_BASE_URL", "https://api.yclients.com"),
		YClientsPartnerToken: os.Getenv("YCLIENTS_PARTNER_TOKEN"),
		YClientsUserToken:    os.Getenv("YCLIENTS_USER_TOKEN"),
		YClientsCompanyID:    os.Getenv("YCLIENTS_COMPANY_ID"),
		BusinessTimezone:     value("BUSINESS_TIMEZONE", "Europe/Moscow"),
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
