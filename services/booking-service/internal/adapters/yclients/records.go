package yclients

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var ErrInvalidRecord = errors.New("invalid YCLIENTS record")

func (c *Client) ListRecords(
	ctx context.Context,
	filter ListRecordsFilter,
) (RecordPage, error) {
	query, err := filter.values()
	if err != nil {
		return RecordPage{}, err
	}

	var response listRecordsResponse
	if err := c.do(
		ctx,
		http.MethodGet,
		c.recordsEndpoint(),
		query,
		nil,
		&response,
		http.StatusOK,
	); err != nil {
		return RecordPage{}, err
	}
	if !response.Success {
		return RecordPage{}, ErrUnsuccessfulResponse
	}

	return RecordPage{
		Records:    response.Data,
		Page:       response.Meta.Page,
		TotalCount: response.Meta.TotalCount,
	}, nil
}

func (c *Client) GetRecord(
	ctx context.Context,
	recordID int64,
) (Record, error) {
	if recordID <= 0 {
		return Record{}, fmt.Errorf("%w: record ID must be positive", ErrInvalidRecord)
	}

	var response recordResponse
	if err := c.do(
		ctx,
		http.MethodGet,
		c.recordEndpoint(recordID),
		nil,
		nil,
		&response,
		http.StatusOK,
	); err != nil {
		return Record{}, err
	}
	if !response.Success {
		return Record{}, ErrUnsuccessfulResponse
	}
	return response.Data, nil
}

func (c *Client) CreateRecord(
	ctx context.Context,
	request CreateRecordRequest,
) (Record, error) {
	if err := validateRecordRequest(request); err != nil {
		return Record{}, err
	}

	var response recordResponse
	if err := c.do(
		ctx,
		http.MethodPost,
		c.recordsEndpoint(),
		nil,
		request,
		&response,
		http.StatusCreated,
	); err != nil {
		return Record{}, err
	}
	if !response.Success {
		return Record{}, ErrUnsuccessfulResponse
	}
	if response.Data.ID <= 0 {
		return Record{}, errors.New("YCLIENTS returned no created record")
	}
	return response.Data, nil
}

func (c *Client) UpdateRecord(
	ctx context.Context,
	recordID int64,
	request UpdateRecordRequest,
) (Record, error) {
	if recordID <= 0 {
		return Record{}, fmt.Errorf("%w: record ID must be positive", ErrInvalidRecord)
	}
	if err := validateRecordRequest(CreateRecordRequest(request)); err != nil {
		return Record{}, err
	}

	var response recordResponse
	if err := c.do(
		ctx,
		http.MethodPut,
		c.recordEndpoint(recordID),
		nil,
		request,
		&response,
		http.StatusCreated,
	); err != nil {
		return Record{}, err
	}
	if !response.Success {
		return Record{}, ErrUnsuccessfulResponse
	}
	return response.Data, nil
}

func (c *Client) DeleteRecord(ctx context.Context, recordID int64) error {
	if recordID <= 0 {
		return fmt.Errorf("%w: record ID must be positive", ErrInvalidRecord)
	}
	return c.do(
		ctx,
		http.MethodDelete,
		c.recordEndpoint(recordID),
		nil,
		nil,
		nil,
		http.StatusNoContent,
	)
}

func (f ListRecordsFilter) values() (url.Values, error) {
	if f.Page < 0 {
		return nil, fmt.Errorf("%w: page must not be negative", ErrInvalidRecord)
	}
	if f.Count < 0 {
		return nil, fmt.Errorf("%w: count must not be negative", ErrInvalidRecord)
	}

	values := make(url.Values)
	setPositiveInt(values, "page", int64(f.Page))
	setPositiveInt(values, "count", int64(f.Count))
	setPositiveInt(values, "staff_id", f.StaffID)
	setPositiveInt(values, "client_id", f.ClientID)
	setPositiveInt(values, "created_user_id", f.CreatedUserID)
	setTime(values, "start_date", f.StartDate, time.DateOnly)
	setTime(values, "end_date", f.EndDate, time.DateOnly)
	setTime(values, "changed_after", f.ChangedAfter, time.RFC3339)
	setTime(values, "changed_before", f.ChangedBefore, time.RFC3339)
	if f.WithDeleted {
		values.Set("with_deleted", "1")
	}
	return values, nil
}

func validateRecordRequest(request CreateRecordRequest) error {
	if request.StaffID <= 0 {
		return fmt.Errorf("%w: staff ID must be positive", ErrInvalidRecord)
	}
	if len(request.Services) == 0 {
		return fmt.Errorf("%w: at least one service is required", ErrInvalidRecord)
	}
	for _, service := range request.Services {
		if service.ID <= 0 {
			return fmt.Errorf("%w: service ID must be positive", ErrInvalidRecord)
		}
	}
	if strings.TrimSpace(request.Client.Phone) == "" {
		return fmt.Errorf("%w: client phone is required", ErrInvalidRecord)
	}
	if strings.TrimSpace(request.Client.Name) == "" {
		return fmt.Errorf("%w: client name is required", ErrInvalidRecord)
	}
	if strings.TrimSpace(request.DateTime) == "" {
		return fmt.Errorf("%w: datetime is required", ErrInvalidRecord)
	}
	if request.SeanceLength <= 0 {
		return fmt.Errorf("%w: seance length must be positive", ErrInvalidRecord)
	}
	return nil
}

func (c *Client) recordsEndpoint() string {
	return "/api/v1/records/" + url.PathEscape(c.config.CompanyID)
}

func (c *Client) recordEndpoint(recordID int64) string {
	return "/api/v1/record/" + url.PathEscape(c.config.CompanyID) + "/" +
		strconv.FormatInt(recordID, 10)
}

func setPositiveInt(values url.Values, key string, value int64) {
	if value > 0 {
		values.Set(key, strconv.FormatInt(value, 10))
	}
}

func setTime(values url.Values, key string, value time.Time, layout string) {
	if !value.IsZero() {
		values.Set(key, value.Format(layout))
	}
}
