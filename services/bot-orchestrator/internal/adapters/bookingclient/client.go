package bookingclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	bookingcontract "ai-chatbot/contracts/bookingapi"
	"ai-chatbot/services/bot-orchestrator/internal/application"
)

const maxResponseBytes = 4 << 20

var ErrInvalidConfiguration = errors.New("invalid booking-service configuration")

type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("booking-service returned HTTP status %d", e.StatusCode)
	}
	return fmt.Sprintf("booking-service returned HTTP status %d: %s", e.StatusCode, e.Body)
}

type Client struct {
	baseURL string
	http    *http.Client
}

func New(baseURL string, httpClient *http.Client) *Client {
	return &Client{baseURL: baseURL, http: httpClient}
}

func (c *Client) ListServices(ctx context.Context) ([]bookingcontract.Service, error) {
	var services []bookingcontract.Service
	if err := c.do(ctx, http.MethodGet, "/v1/services", nil, &services); err != nil {
		return nil, err
	}
	return services, nil
}

func (c *Client) ListStaff(ctx context.Context, request bookingcontract.ListStaffRequest) ([]bookingcontract.Staff, error) {
	var staff []bookingcontract.Staff
	if err := c.do(ctx, http.MethodPost, "/v1/staff/search", request, &staff); err != nil {
		return nil, err
	}
	return staff, nil
}

func (c *Client) SearchSlots(ctx context.Context, request bookingcontract.SearchSlotsRequest) ([]bookingcontract.Slot, error) {
	var slots []bookingcontract.Slot
	if err := c.do(ctx, http.MethodPost, "/v1/slots/search", request, &slots); err != nil {
		return nil, err
	}
	return slots, nil
}

func (c *Client) CreateBooking(ctx context.Context, request bookingcontract.CreateBookingRequest) (bookingcontract.BookingResult, error) {
	var result bookingcontract.BookingResult
	if err := c.do(ctx, http.MethodPost, "/v1/bookings", request, &result); err != nil {
		return bookingcontract.BookingResult{}, err
	}
	return result, nil
}

func (c *Client) do(ctx context.Context, method string, endpoint string, requestBody any, responseBody any) error {
	if c.http == nil {
		return fmt.Errorf("%w: HTTP client is required", ErrInvalidConfiguration)
	}
	baseURL, err := url.Parse(strings.TrimSpace(c.baseURL))
	if err != nil || !baseURL.IsAbs() || baseURL.Host == "" {
		return fmt.Errorf("%w: invalid base URL", ErrInvalidConfiguration)
	}
	relativeURL, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("parse booking-service endpoint: %w", err)
	}

	var body io.Reader
	if requestBody != nil {
		encoded, err := json.Marshal(requestBody)
		if err != nil {
			return fmt.Errorf("encode booking-service request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, baseURL.ResolveReference(relativeURL).String(), body)
	if err != nil {
		return fmt.Errorf("create booking-service request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	if requestBody != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := c.http.Do(request)
	if err != nil {
		return fmt.Errorf("execute booking-service request: %w", err)
	}
	defer response.Body.Close()
	encodedResponse, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return fmt.Errorf("read booking-service response: %w", err)
	}
	if len(encodedResponse) > maxResponseBytes {
		return errors.New("booking-service response is too large")
	}
	if response.StatusCode != http.StatusOK {
		return &HTTPError{StatusCode: response.StatusCode, Body: compactBody(encodedResponse)}
	}

	decoder := json.NewDecoder(bytes.NewReader(encodedResponse))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(responseBody); err != nil {
		return fmt.Errorf("decode booking-service response: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("booking-service returned trailing response data")
	}
	return nil
}

func compactBody(body []byte) string {
	const maxErrorBytes = 1024
	trimmed := strings.TrimSpace(string(body))
	if len(trimmed) <= maxErrorBytes {
		return trimmed
	}
	return trimmed[:maxErrorBytes] + "..."
}

var _ application.BookingGateway = (*Client)(nil)
