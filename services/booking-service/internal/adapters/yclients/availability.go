package yclients

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"

	bookingcontract "ai-chatbot/contracts/bookingapi"
)

const maxBookingHorizonDays = 31

var ErrInvalidSlotSearch = errors.New("invalid slot search")

type slotReference struct {
	ID           string
	ServiceID    string
	StaffID      int64
	StartsAt     time.Time
	SeanceLength int
}

func (c *Client) SearchSlots(ctx context.Context, request bookingcontract.SearchSlotsRequest) ([]bookingcontract.Slot, error) {
	serviceID := strings.TrimSpace(request.ServiceID)
	if _, err := positiveID(serviceID); err != nil {
		return nil, fmt.Errorf("%w: invalid service ID", ErrInvalidSlotSearch)
	}
	location, err := time.LoadLocation(c.config.Timezone)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid timezone", ErrInvalidConfiguration)
	}

	selectedDate, err := time.ParseInLocation(time.DateOnly, request.Date, location)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid date", ErrInvalidSlotSearch)
	}
	today := beginningOfDay(time.Now().In(location))
	if selectedDate.Before(today) || selectedDate.After(today.AddDate(0, 0, maxBookingHorizonDays)) {
		return nil, fmt.Errorf("%w: date is outside booking horizon", ErrInvalidSlotSearch)
	}

	staff, err := c.listBookableStaff(ctx, serviceID)
	if err != nil {
		return nil, err
	}
	from := selectedDate.UTC()
	to := selectedDate.AddDate(0, 0, 1).UTC()
	referencesByStart := make(map[int64]slotReference)

	for _, staffID := range staff {
		times, err := c.listBookTimes(ctx, serviceID, staffID, selectedDate)
		if err != nil {
			return nil, err
		}
		for _, available := range times {
			startsAt := time.Unix(int64(available.DateTime), 0).UTC()
			if startsAt.Before(from) || !startsAt.Before(to) || available.SeanceLength <= 0 {
				continue
			}
			key := startsAt.Unix()
			if _, exists := referencesByStart[key]; exists {
				continue
			}
			reference := slotReference{ServiceID: serviceID, StaffID: staffID, StartsAt: startsAt, SeanceLength: available.SeanceLength}
			reference.ID = slotID(reference)
			referencesByStart[key] = reference
		}
	}

	references := make([]slotReference, 0, len(referencesByStart))
	for _, reference := range referencesByStart {
		references = append(references, reference)
	}
	sort.Slice(references, func(i int, j int) bool {
		if references[i].StartsAt.Equal(references[j].StartsAt) {
			return references[i].ID < references[j].ID
		}
		return references[i].StartsAt.Before(references[j].StartsAt)
	})

	slots := make([]bookingcontract.Slot, 0, len(references))
	c.slotsMu.Lock()
	for id, reference := range c.slots {
		if reference.StartsAt.Before(time.Now().UTC()) {
			delete(c.slots, id)
		}
	}
	for _, reference := range references {
		c.slots[reference.ID] = reference
		slots = append(slots, bookingcontract.Slot{ID: reference.ID, ServiceID: reference.ServiceID, StartsAt: reference.StartsAt})
	}
	c.slotsMu.Unlock()

	return slots, nil
}

func (c *Client) listBookableStaff(ctx context.Context, serviceID string) ([]int64, error) {
	query := make(url.Values)
	query.Add("service_ids[]", serviceID)
	var response bookStaffResponse
	if err := c.do(ctx, http.MethodGet, c.bookStaffEndpoint(), query, nil, &response, http.StatusOK); err != nil {
		return nil, err
	}
	if !response.Success {
		return nil, ErrUnsuccessfulResponse
	}

	staff := make([]int64, 0, len(response.Data))
	for _, item := range response.Data {
		if item.Bookable != nil && !*item.Bookable {
			continue
		}
		id, err := positiveID(string(item.ID))
		if err != nil {
			return nil, fmt.Errorf("decode YCLIENTS staff ID: %w", err)
		}
		staff = append(staff, id)
	}
	sort.Slice(staff, func(i int, j int) bool { return staff[i] < staff[j] })
	return staff, nil
}

func (c *Client) listBookTimes(ctx context.Context, serviceID string, staffID int64, date time.Time) ([]bookTimeDTO, error) {
	query := make(url.Values)
	query.Add("service_ids[]", serviceID)
	var response bookTimesResponse
	if err := c.do(ctx, http.MethodGet, c.bookTimesEndpoint(staffID, date), query, nil, &response, http.StatusOK); err != nil {
		return nil, err
	}
	if !response.Success {
		return nil, ErrUnsuccessfulResponse
	}
	return response.Data, nil
}

func (c *Client) bookStaffEndpoint() string {
	return "/api/v1/book_staff/" + url.PathEscape(c.config.CompanyID)
}

func (c *Client) bookTimesEndpoint(staffID int64, date time.Time) string {
	return "/api/v1/book_times/" + url.PathEscape(c.config.CompanyID) + "/" + strconv.FormatInt(staffID, 10) + "/" + date.Format(time.DateOnly)
}

func beginningOfDay(value time.Time) time.Time {
	year, month, day := value.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, value.Location())
}

func positiveID(value string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("ID must be a positive integer")
	}
	return id, nil
}

func slotID(reference slotReference) string {
	value := fmt.Sprintf("%s|%d|%d|%d", reference.ServiceID, reference.StaffID, reference.StartsAt.Unix(), reference.SeanceLength)
	digest := sha256.Sum256([]byte(value))
	return "slot_" + hex.EncodeToString(digest[:16])
}
