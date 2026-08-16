package yclients

import (
	"context"
	"errors"
	"net/http"
	"sync"

	bookingcontract "ai-chatbot/contracts/bookingapi"
	"ai-chatbot/services/booking-service/internal/application"
)

var ErrNotImplemented = errors.New("YCLIENTS client is not implemented")

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

func (c *Client) CreateBooking(context.Context, bookingcontract.CreateBookingRequest) (bookingcontract.BookingResult, error) {
	return bookingcontract.BookingResult{}, ErrNotImplemented
}

var _ application.Provider = (*Client)(nil)
