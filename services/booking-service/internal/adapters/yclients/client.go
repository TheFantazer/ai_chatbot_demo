package yclients

import (
	"net/http"
	"sync"

	"ai-chatbot/services/booking-service/internal/application"
)

type Config struct {
	BaseURL      string
	PartnerToken string
	UserToken    string
	CompanyID    string
	Timezone     string
}

type Client struct {
	config  Config
	http    *http.Client
	slotsMu sync.RWMutex
	slots   map[string]slotReference
}

func New(config Config, httpClient *http.Client) *Client {
	return &Client{config: config, http: httpClient, slots: make(map[string]slotReference)}
}

var _ application.Provider = (*Client)(nil)
