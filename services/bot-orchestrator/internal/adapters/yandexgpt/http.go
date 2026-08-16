package yandexgpt

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

const (
	defaultBaseURL   = "https://ai.api.cloud.yandex.net"
	completionPath   = "/foundationModels/v1/completion"
	maxResponseBytes = 1 << 20
)

var (
	ErrInvalidConfiguration = errors.New("invalid YandexGPT configuration")
	ErrResponseTooLarge     = errors.New("YandexGPT response is too large")
)

type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("YandexGPT returned HTTP status %d", e.StatusCode)
	}
	return fmt.Sprintf("YandexGPT returned HTTP status %d: %s", e.StatusCode, e.Body)
}

func (c *Client) do(ctx context.Context, requestBody completionRequest) (completionResponse, error) {
	if err := c.validateConfiguration(); err != nil {
		return completionResponse{}, err
	}

	baseURL, err := url.Parse(c.config.BaseURL)
	if err != nil || !baseURL.IsAbs() || baseURL.Host == "" {
		return completionResponse{}, fmt.Errorf("%w: invalid base URL", ErrInvalidConfiguration)
	}

	endpoint, err := url.Parse(completionPath)
	if err != nil {
		return completionResponse{}, fmt.Errorf("parse YandexGPT endpoint: %w", err)
	}

	encodedRequest, err := json.Marshal(requestBody)
	if err != nil {
		return completionResponse{}, fmt.Errorf("encode YandexGPT request: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL.ResolveReference(endpoint).String(), bytes.NewReader(encodedRequest))
	if err != nil {
		return completionResponse{}, fmt.Errorf("create YandexGPT request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Api-Key "+c.config.APIKey)
	request.Header.Set("x-folder-id", c.config.FolderID)

	response, err := c.http.Do(request)
	if err != nil {
		return completionResponse{}, fmt.Errorf("execute YandexGPT request: %w", err)
	}
	defer response.Body.Close()

	encodedResponse, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return completionResponse{}, fmt.Errorf("read YandexGPT response: %w", err)
	}
	if len(encodedResponse) > maxResponseBytes {
		return completionResponse{}, ErrResponseTooLarge
	}

	if response.StatusCode != http.StatusOK {
		return completionResponse{}, &HTTPError{
			StatusCode: response.StatusCode,
			Body:       compactErrorBody(encodedResponse),
		}
	}
	if len(bytes.TrimSpace(encodedResponse)) == 0 {
		return completionResponse{}, errors.New("YandexGPT returned an empty response body")
	}

	var result completionResponse
	if err := json.Unmarshal(encodedResponse, &result); err != nil {
		return completionResponse{}, fmt.Errorf("decode YandexGPT response: %w", err)
	}
	if len(result.Alternatives) == 0 && result.Result != nil {
		result.Alternatives = result.Result.Alternatives
		result.Usage = result.Result.Usage
		result.ModelVersion = result.Result.ModelVersion
	}
	return result, nil
}

func (c *Client) validateConfiguration() error {
	switch {
	case c.http == nil:
		return fmt.Errorf("%w: HTTP client is required", ErrInvalidConfiguration)
	case strings.TrimSpace(c.config.BaseURL) == "":
		return fmt.Errorf("%w: base URL is required", ErrInvalidConfiguration)
	case strings.TrimSpace(c.config.APIKey) == "":
		return fmt.Errorf("%w: API key is required", ErrInvalidConfiguration)
	case strings.TrimSpace(c.config.FolderID) == "":
		return fmt.Errorf("%w: folder ID is required", ErrInvalidConfiguration)
	case strings.TrimSpace(c.config.Model) == "":
		return fmt.Errorf("%w: model is required", ErrInvalidConfiguration)
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
