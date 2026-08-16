package yclients

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	bookingcontract "ai-chatbot/contracts/bookingapi"
)

func (c *Client) CreateBooking(ctx context.Context, request bookingcontract.CreateBookingRequest) (bookingcontract.BookingResult, error) {
	reference, found := c.slotReference(request.SlotID)
	if !found {
		return bookingResult(request.OperationID, bookingcontract.BookingRejected, "slot_not_found"), nil
	}
	if reference.ServiceID != request.ServiceID {
		return bookingResult(request.OperationID, bookingcontract.BookingRejected, "slot_service_mismatch"), nil
	}
	if !reference.StartsAt.After(time.Now().UTC()) {
		return bookingResult(request.OperationID, bookingcontract.BookingRejected, "slot_expired"), nil
	}

	available, err := c.slotStillAvailable(ctx, reference)
	if err != nil {
		return bookingcontract.BookingResult{}, err
	}
	if !available {
		return bookingResult(request.OperationID, bookingcontract.BookingRejected, "slot_unavailable"), nil
	}

	serviceID, err := positiveID(reference.ServiceID)
	if err != nil {
		return bookingcontract.BookingResult{}, err
	}
	location, err := time.LoadLocation(c.config.Timezone)
	if err != nil {
		return bookingcontract.BookingResult{}, err
	}
	record, err := c.CreateRecord(ctx, CreateRecordRequest{
		StaffID:      reference.StaffID,
		Services:     []RecordServiceInput{{ID: serviceID}},
		Client:       RecordClientInput{Name: request.Customer.Name, Phone: request.Customer.Phone},
		SaveIfBusy:   false,
		DateTime:     reference.StartsAt.In(location).Format("2006-01-02 15:04:05"),
		SeanceLength: reference.SeanceLength,
		SendSMS:      false,
		Attendance:   0,
		APIID:        request.OperationID,
	})
	if err != nil {
		if isDefiniteCreateRejection(err) {
			return bookingResult(request.OperationID, bookingcontract.BookingRejected, "provider_rejected"), nil
		}
		return bookingResult(request.OperationID, bookingcontract.BookingResultUnknown, "provider_result_unknown"), nil
	}

	result := bookingResult(request.OperationID, bookingcontract.BookingCreated, "")
	result.ExternalID = strconv.FormatInt(record.ID, 10)
	return result, nil
}

func (c *Client) slotReference(slotID string) (slotReference, bool) {
	c.slotsMu.RLock()
	reference, found := c.slots[slotID]
	c.slotsMu.RUnlock()
	return reference, found
}

func (c *Client) slotStillAvailable(ctx context.Context, reference slotReference) (bool, error) {
	location, err := time.LoadLocation(c.config.Timezone)
	if err != nil {
		return false, err
	}
	times, err := c.listBookTimes(ctx, reference.ServiceID, reference.StaffID, reference.StartsAt.In(location))
	if err != nil {
		return false, err
	}
	for _, available := range times {
		if time.Unix(int64(available.DateTime), 0).UTC().Equal(reference.StartsAt) && available.SeanceLength == reference.SeanceLength {
			return true, nil
		}
	}
	return false, nil
}

func isDefiniteCreateRejection(err error) bool {
	var httpError *HTTPError
	if errors.As(err, &httpError) {
		return httpError.StatusCode >= http.StatusBadRequest && httpError.StatusCode < http.StatusInternalServerError
	}
	return errors.Is(err, ErrUnsuccessfulResponse)
}

func bookingResult(operationID string, outcome bookingcontract.BookingOutcome, reason string) bookingcontract.BookingResult {
	return bookingcontract.BookingResult{OperationID: operationID, Outcome: outcome, Reason: reason}
}
