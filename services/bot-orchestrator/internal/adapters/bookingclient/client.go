package bookingclient

import (
	"context"
	"errors"
	"net/http"

	bookingcontract "ai-chatbot/contracts/bookingapi"
	"ai-chatbot/services/bot-orchestrator/internal/application"
)

var ErrNotImplemented = errors.New("booking-service client is not implemented")

type Client struct {
	baseURL string
	http    *http.Client
}

func New(baseURL string, httpClient *http.Client) *Client {
	return &Client{baseURL: baseURL, http: httpClient}
}

// implement me
func (c *Client) ListServices(context.Context) ([]bookingcontract.Service, error) {
	return nil, ErrNotImplemented
}

func (c *Client) SearchSlots(context.Context, bookingcontract.SearchSlotsRequest) ([]bookingcontract.Slot, error) {
	return nil, ErrNotImplemented
}

func (c *Client) CreateBooking(context.Context, bookingcontract.CreateBookingRequest) (bookingcontract.BookingResult, error) {
	return bookingcontract.BookingResult{}, ErrNotImplemented
}

var _ application.BookingGateway = (*Client)(nil)
