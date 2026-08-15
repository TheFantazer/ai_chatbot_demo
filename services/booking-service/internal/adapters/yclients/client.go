package yclients

import (
	"context"
	"errors"
	"net/http"

	bookingcontract "ai-chatbot/contracts/bookingapi"
	"ai-chatbot/services/booking-service/internal/application"
)

var ErrNotImplemented = errors.New("YCLIENTS client is not implemented")

type Config struct {
	BaseURL      string
	PartnerToken string
	UserToken    string
	CompanyID    string
}

type Client struct {
	config Config
	http   *http.Client
}

func New(config Config, httpClient *http.Client) *Client {
	return &Client{config: config, http: httpClient}
}

func (c *Client) SearchSlots(context.Context, bookingcontract.SearchSlotsRequest) ([]bookingcontract.Slot, error) {
	return nil, ErrNotImplemented
}

func (c *Client) CreateBooking(context.Context, bookingcontract.CreateBookingRequest) (bookingcontract.BookingResult, error) {
	return bookingcontract.BookingResult{}, ErrNotImplemented
}

var _ application.Provider = (*Client)(nil)
