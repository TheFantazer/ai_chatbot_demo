package vk

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const maxResponseBytes = 4 << 20

func (t *Transport) getLongPollServer(ctx context.Context) (longPollServerDTO, error) {
	values := t.apiValues()
	values.Set("group_id", strconv.FormatInt(t.groupID, 10))
	var response apiResponse[longPollServerDTO]
	if err := t.postForm(ctx, "groups.getLongPollServer", values, &response); err != nil {
		return longPollServerDTO{}, err
	}
	if response.Error != nil {
		return longPollServerDTO{}, response.Error
	}
	if strings.TrimSpace(response.Response.Key) == "" || strings.TrimSpace(response.Response.Server) == "" || strings.TrimSpace(response.Response.TS) == "" {
		return longPollServerDTO{}, errors.New("VK returned incomplete long poll server data")
	}
	return response.Response, nil
}

func (t *Transport) poll(ctx context.Context, server longPollServerDTO) (longPollResponse, error) {
	target, err := url.Parse(server.Server)
	if err != nil || !target.IsAbs() || target.Host == "" {
		return longPollResponse{}, errors.New("VK returned invalid long poll server URL")
	}
	query := target.Query()
	query.Set("act", "a_check")
	query.Set("key", server.Key)
	query.Set("ts", server.TS)
	query.Set("wait", strconv.Itoa(t.config.LongPollWait))
	target.RawQuery = query.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return longPollResponse{}, fmt.Errorf("create VK long poll request: %w", err)
	}
	var response longPollResponse
	if err := t.do(request, &response); err != nil {
		return longPollResponse{}, err
	}
	if strings.TrimSpace(response.TS) == "" && response.Failed != 2 && response.Failed != 3 {
		return longPollResponse{}, errors.New("VK long poll returned no ts")
	}
	return response, nil
}

func (t *Transport) sendMessage(ctx context.Context, peerID int64, eventID string, message string) error {
	if len([]rune(message)) > 9000 {
		return errors.New("VK message exceeds 9000 characters")
	}
	values := t.apiValues()
	values.Set("peer_id", strconv.FormatInt(peerID, 10))
	values.Set("random_id", strconv.FormatInt(int64(randomID(eventID)), 10))
	values.Set("message", message)
	var response sendMessageResponse
	if err := t.postForm(ctx, "messages.send", values, &response); err != nil {
		return err
	}
	if response.Error != nil {
		return response.Error
	}
	if strings.TrimSpace(response.Response.String()) == "" {
		return errors.New("VK returned no sent message ID")
	}
	return nil
}

func (t *Transport) apiValues() url.Values {
	values := make(url.Values)
	values.Set("access_token", t.config.Token)
	values.Set("v", t.config.APIVersion)
	return values
}

func (t *Transport) postForm(ctx context.Context, method string, values url.Values, target any) error {
	baseURL, err := url.Parse(t.config.APIBaseURL)
	if err != nil || !baseURL.IsAbs() || baseURL.Host == "" {
		return fmt.Errorf("%w: invalid API base URL", ErrInvalidConfiguration)
	}
	relativeURL, err := url.Parse(method)
	if err != nil {
		return fmt.Errorf("parse VK API method: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL.ResolveReference(relativeURL).String(), strings.NewReader(values.Encode()))
	if err != nil {
		return fmt.Errorf("create VK API request: %w", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return t.do(request, target)
}

func (t *Transport) do(request *http.Request, target any) error {
	response, err := t.http.Do(request)
	if err != nil {
		return fmt.Errorf("execute VK request: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return fmt.Errorf("read VK response: %w", err)
	}
	if len(body) > maxResponseBytes {
		return errors.New("VK response is too large")
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("VK returned HTTP status %d", response.StatusCode)
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode VK response: %w", err)
	}
	return nil
}

func (e *apiError) Error() string {
	return fmt.Sprintf("VK API error %d: %s", e.Code, e.Message)
}

func randomID(eventID string) int32 {
	digest := sha256.Sum256([]byte(eventID))
	value := int32(binary.BigEndian.Uint32(digest[:4]) & 0x7fffffff)
	if value == 0 {
		return 1
	}
	return value
}
