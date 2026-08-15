package yclients

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
)

const maxResponseBytes = 4 << 20

var (
	ErrInvalidConfiguration = errors.New("invalid YCLIENTS configuration")
	ErrUnsuccessfulResponse = errors.New("YCLIENTS returned success=false")
	ErrResponseTooLarge     = errors.New("YCLIENTS response is too large")
)

type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("YCLIENTS returned HTTP status %d", e.StatusCode)
	}
	return fmt.Sprintf("YCLIENTS returned HTTP status %d: %s", e.StatusCode, e.Body)
}

func (c *Client) do(ctx context.Context, method string, endpoint string, query url.Values, requestBody any, responseBody any, expectedStatus int) error {
	if err := c.validateConfiguration(); err != nil {
		return err
	}

	baseURL, err := url.Parse(c.config.BaseURL)
	if err != nil || !baseURL.IsAbs() || baseURL.Host == "" {
		return fmt.Errorf("%w: invalid base URL", ErrInvalidConfiguration)
	}

	relativeURL, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("parse YCLIENTS endpoint: %w", err)
	}
	targetURL := baseURL.ResolveReference(relativeURL)
	if query != nil {
		targetURL.RawQuery = query.Encode()
	}

	var body io.Reader
	if requestBody != nil {
		encoded, err := json.Marshal(requestBody)
		if err != nil {
			return fmt.Errorf("encode YCLIENTS request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}

	request, err := http.NewRequestWithContext(ctx, method, targetURL.String(), body)
	if err != nil {
		return fmt.Errorf("create YCLIENTS request: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.yclients.v2+json")
	request.Header.Set(
		"Authorization",
		"Bearer "+c.config.PartnerToken+", User "+c.config.UserToken,
	)
	if requestBody != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := c.http.Do(request)
	if err != nil {
		return fmt.Errorf("execute YCLIENTS request: %w", err)
	}
	defer response.Body.Close()

	encodedResponse, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return fmt.Errorf("read YCLIENTS response: %w", err)
	}
	if len(encodedResponse) > maxResponseBytes {
		return ErrResponseTooLarge
	}

	if response.StatusCode != expectedStatus {
		return &HTTPError{
			StatusCode: response.StatusCode,
			Body:       compactErrorBody(encodedResponse),
		}
	}

	if responseBody == nil {
		return nil
	}
	if len(bytes.TrimSpace(encodedResponse)) == 0 {
		return errors.New("YCLIENTS returned an empty response body")
	}
	if err := json.Unmarshal(encodedResponse, responseBody); err != nil {
		return fmt.Errorf("decode YCLIENTS response: %w", err)
	}
	return nil
}

func (c *Client) validateConfiguration() error {
	switch {
	case c.http == nil:
		return fmt.Errorf("%w: HTTP client is required", ErrInvalidConfiguration)
	case strings.TrimSpace(c.config.BaseURL) == "":
		return fmt.Errorf("%w: base URL is required", ErrInvalidConfiguration)
	case strings.TrimSpace(c.config.PartnerToken) == "":
		return fmt.Errorf("%w: partner token is required", ErrInvalidConfiguration)
	case strings.TrimSpace(c.config.UserToken) == "":
		return fmt.Errorf("%w: user token is required", ErrInvalidConfiguration)
	case strings.TrimSpace(c.config.CompanyID) == "":
		return fmt.Errorf("%w: company ID is required", ErrInvalidConfiguration)
	default:
		return nil
	}
}

func compactErrorBody(body []byte) string {
	const maxErrorBytes = 1024

	trimmed := strings.TrimSpace(string(body))
	if len(trimmed) <= maxErrorBytes {
		return trimmed
	}
	return trimmed[:maxErrorBytes] + "..."
}
