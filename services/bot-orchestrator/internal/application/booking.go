package application

import (
	"errors"
	"fmt"
	"strings"

	bookingcontract "ai-chatbot/contracts/bookingapi"
)

var ErrInvalidBookingResult = errors.New("invalid booking result")

func BeginBooking(state ConversationState, operationID string) (ConversationState, bookingcontract.CreateBookingRequest, error) {
	if err := ValidateState(state); err != nil {
		return ConversationState{}, bookingcontract.CreateBookingRequest{}, err
	}
	if state.Step != StepWaitingForConfirmation {
		return ConversationState{}, bookingcontract.CreateBookingRequest{}, fmt.Errorf("%w: confirmation is required", ErrActionNotAllowed)
	}
	operationID = strings.TrimSpace(operationID)
	if operationID == "" {
		return ConversationState{}, bookingcontract.CreateBookingRequest{}, fmt.Errorf("%w: operation ID is required", ErrInvalidState)
	}

	next := cloneState(state)
	next.Revision++
	next.Step = StepBookingInProgress
	next.Pending = RequirementNone
	next.Booking = BookingAttempt{OperationID: operationID}

	request := bookingcontract.CreateBookingRequest{
		OperationID: operationID,
		ServiceID:   next.ServiceID,
		SlotID:      next.SelectedSlot,
		Customer: bookingcontract.Customer{
			Name:  next.CustomerName,
			Phone: next.CustomerPhone,
		},
	}

	if err := ValidateState(next); err != nil {
		return ConversationState{}, bookingcontract.CreateBookingRequest{}, err
	}
	return next, request, nil
}

func CompleteBooking(state ConversationState, result bookingcontract.BookingResult) (ConversationState, error) {
	if err := ValidateState(state); err != nil {
		return ConversationState{}, err
	}
	if state.Step != StepBookingInProgress {
		return ConversationState{}, fmt.Errorf("%w: booking is not in progress", ErrInvalidBookingResult)
	}
	if strings.TrimSpace(result.OperationID) == "" || result.OperationID != state.Booking.OperationID {
		return bookingUnknownState(state, ""), fmt.Errorf("%w: operation ID mismatch", ErrInvalidBookingResult)
	}

	next := cloneState(state)
	next.Booking.ResultOperationID = result.OperationID
	next.Booking.Outcome = result.Outcome
	next.Booking.ExternalID = strings.TrimSpace(result.ExternalID)

	switch result.Outcome {
	case bookingcontract.BookingCreated:
		if next.Booking.ExternalID == "" {
			return bookingUnknownState(state, result.OperationID), fmt.Errorf("%w: created result requires external ID", ErrInvalidBookingResult)
		}
		next.Step = StepBooked
	case bookingcontract.BookingRejected:
		if next.Booking.ExternalID != "" {
			return bookingUnknownState(state, result.OperationID), fmt.Errorf("%w: rejected result cannot contain external ID", ErrInvalidBookingResult)
		}
		next.Step = StepWaitingForConfirmation
	case bookingcontract.BookingResultUnknown:
		if next.Booking.ExternalID != "" {
			return bookingUnknownState(state, result.OperationID), fmt.Errorf("%w: unknown result cannot contain external ID", ErrInvalidBookingResult)
		}
		next.Step = StepBookingUnknown
	default:
		return bookingUnknownState(state, result.OperationID), fmt.Errorf("%w: unsupported outcome %q", ErrInvalidBookingResult, result.Outcome)
	}

	if err := ValidateState(next); err != nil {
		return bookingUnknownState(state, result.OperationID), err
	}
	return next, nil
}

func MarkBookingUnknown(state ConversationState) (ConversationState, error) {
	if err := ValidateState(state); err != nil {
		return ConversationState{}, err
	}
	if state.Step != StepBookingInProgress {
		return ConversationState{}, fmt.Errorf("%w: booking is not in progress", ErrInvalidBookingResult)
	}
	next := bookingUnknownState(state, "")
	if err := ValidateState(next); err != nil {
		return ConversationState{}, err
	}
	return next, nil
}

func bookingUnknownState(state ConversationState, resultOperationID string) ConversationState {
	next := cloneState(state)
	next.Step = StepBookingUnknown
	next.Pending = RequirementNone
	next.Booking.ResultOperationID = resultOperationID
	next.Booking.Outcome = bookingcontract.BookingResultUnknown
	next.Booking.ExternalID = ""
	return next
}
