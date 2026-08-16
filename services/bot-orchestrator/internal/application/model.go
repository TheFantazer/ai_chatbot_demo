package application

import (
	"errors"
	"fmt"
	"strings"
	"time"

	bookingcontract "ai-chatbot/contracts/bookingapi"
)

type Step string

const (
	StepWaitingForService      Step = "waiting_for_service"
	StepWaitingForDate         Step = "waiting_for_date"
	StepWaitingForStaff        Step = "waiting_for_staff"
	StepWaitingForTime         Step = "waiting_for_time"
	StepWaitingForContact      Step = "waiting_for_contact"
	StepWaitingForConfirmation Step = "waiting_for_confirmation"
	StepBookingInProgress      Step = "booking_in_progress"
	StepBooked                 Step = "booked"
	StepBookingUnknown         Step = "booking_unknown"
	StepCancelled              Step = "cancelled"
)

type Requirement string

const (
	RequirementNone Requirement = ""
	RequireService  Requirement = "require_service"
	RequireDate     Requirement = "require_date"
	RequireStaff    Requirement = "require_staff"
	RequireTime     Requirement = "require_time"
	RequireName     Requirement = "require_name"
	RequirePhone    Requirement = "require_phone"
)

type SlotSnapshot struct {
	ID        string
	ServiceID string
	StaffID   string
	Date      string
	StartsAt  time.Time
}

type DateSnapshot struct {
	Date string
}

type ServiceSnapshot struct {
	ID   string
	Name string
}

type StaffSnapshot struct {
	ID             string
	Name           string
	Specialization string
}

type BookingAttempt struct {
	OperationID       string
	ResultOperationID string
	Outcome           bookingcontract.BookingOutcome
	ExternalID        string
}

type ConversationState struct {
	ID       string
	Revision uint64

	Step    Step
	Pending Requirement

	ServiceID       string
	OfferedServices map[string]ServiceSnapshot
	OfferedDates    map[string]DateSnapshot
	SelectedDate    string
	OfferedStaff    map[string]StaffSnapshot
	StaffID         string
	OfferedSlots    map[string]SlotSnapshot
	SelectedSlot    string
	CustomerName    string
	CustomerPhone   string

	Booking   BookingAttempt
	UpdatedAt time.Time
}

type ActionType string

const (
	ActionChooseService   ActionType = "choose_service"
	ActionChooseDate      ActionType = "choose_date"
	ActionChooseStaff     ActionType = "choose_staff"
	ActionChooseTime      ActionType = "choose_time"
	ActionProvideContact  ActionType = "provide_contact"
	ActionChangeService   ActionType = "change_service"
	ActionChangeDate      ActionType = "change_date"
	ActionChangeStaff     ActionType = "change_staff"
	ActionChangeTime      ActionType = "change_time"
	ActionStartNewBooking ActionType = "start_new_booking"
	ActionAskQuestion     ActionType = "ask_question"
	ActionCancelFlow      ActionType = "cancel_flow"
	ActionClarify         ActionType = "clarify"
)

type ActionEnvelope struct {
	Action        ActionType      `json:"action"`
	Arguments     ActionArguments `json:"arguments"`
	StateRevision uint64          `json:"state_revision"`
}

type ActionArguments struct {
	ServiceID string `json:"service_id,omitempty"`
	Date      string `json:"date,omitempty"`
	StaffID   string `json:"staff_id,omitempty"`
	SlotID    string `json:"slot_id,omitempty"`
	Name      string `json:"name,omitempty"`
	Phone     string `json:"phone,omitempty"`
	Topic     string `json:"topic,omitempty"`
}

type InboundMessage struct {
	EventID        string `json:"event_id"`
	ConversationID string `json:"conversation_id"`
	Text           string `json:"text"`
}

type OutboundKind string

const (
	OutboundText           OutboundKind = "text"
	OutboundBookingCreated OutboundKind = "booking_created"
	OutboundBookingFailed  OutboundKind = "booking_failed"
)

type OutboundMessage struct {
	Kind OutboundKind `json:"kind"`
	Text string       `json:"text"`
}

type InterpretationRequest struct {
	Message        string
	State          ConversationState
	AllowedActions []ActionType
}

var ErrInvalidState = errors.New("invalid conversation state")

func ValidateState(state ConversationState) error {
	if strings.TrimSpace(state.ID) == "" {
		return invalidState("conversation ID is required")
	}
	if !isKnownStep(state.Step) {
		return invalidState("unknown step %q", state.Step)
	}
	if !isKnownRequirement(state.Pending) {
		return invalidState("unknown pending requirement %q", state.Pending)
	}
	if err := validateServiceSnapshots(state); err != nil {
		return err
	}
	if err := validateDateSnapshots(state); err != nil {
		return err
	}
	if err := validateStaffSnapshots(state); err != nil {
		return err
	}
	if err := validateSlotSnapshots(state); err != nil {
		return err
	}

	switch state.Step {
	case StepWaitingForService:
		if state.Pending != RequireService {
			return invalidState("waiting_for_service requires require_service")
		}
		if !isEmptyBookingAttempt(state.Booking) {
			return invalidState("waiting_for_service cannot contain a booking attempt")
		}
	case StepWaitingForDate:
		if state.Pending != RequireDate {
			return invalidState("waiting_for_date requires require_date")
		}
		if err := validateSelectedService(state); err != nil {
			return err
		}
		if !isEmptyBookingAttempt(state.Booking) {
			return invalidState("waiting_for_date cannot contain a booking attempt")
		}
	case StepWaitingForStaff:
		if state.Pending != RequireStaff {
			return invalidState("waiting_for_staff requires require_staff")
		}
		if err := validateSelectedService(state); err != nil {
			return err
		}
		if err := validateSelectedDate(state); err != nil {
			return err
		}
		if !isEmptyBookingAttempt(state.Booking) {
			return invalidState("waiting_for_staff cannot contain a booking attempt")
		}
	case StepWaitingForTime:
		if state.Pending != RequireTime {
			return invalidState("waiting_for_time requires require_time")
		}
		if err := validateSelectedService(state); err != nil {
			return err
		}
		if err := validateSelectedDate(state); err != nil {
			return err
		}
		if err := validateSelectedStaff(state); err != nil {
			return err
		}
		if !isEmptyBookingAttempt(state.Booking) {
			return invalidState("waiting_for_time cannot contain a booking attempt")
		}
	case StepWaitingForContact:
		if state.Pending != RequireName && state.Pending != RequirePhone {
			return invalidState("waiting_for_contact requires require_name or require_phone")
		}
		if err := validateBookingDraft(state, false); err != nil {
			return err
		}
		if state.Pending == RequirePhone && strings.TrimSpace(state.CustomerName) == "" {
			return invalidState("require_phone requires customer name")
		}
		if !isEmptyBookingAttempt(state.Booking) {
			return invalidState("waiting_for_contact cannot contain a booking attempt")
		}
	case StepWaitingForConfirmation:
		if state.Pending != RequirementNone {
			return invalidState("waiting_for_confirmation cannot have a pending requirement")
		}
		if err := validateBookingDraft(state, true); err != nil {
			return err
		}
		if state.Booking.Outcome == bookingcontract.BookingCreated || strings.TrimSpace(state.Booking.ExternalID) != "" {
			return invalidState("waiting_for_confirmation cannot contain a successful booking")
		}
		if state.Booking.Outcome != "" && state.Booking.Outcome != bookingcontract.BookingRejected {
			return invalidState("waiting_for_confirmation can contain only a rejected booking result")
		}
		if state.Booking.Outcome == bookingcontract.BookingRejected && (strings.TrimSpace(state.Booking.OperationID) == "" || state.Booking.ResultOperationID != state.Booking.OperationID) {
			return invalidState("rejected booking result must belong to the current operation")
		}
	case StepBookingInProgress:
		if state.Pending != RequirementNone {
			return invalidState("booking_in_progress cannot have a pending requirement")
		}
		if err := validateBookingDraft(state, true); err != nil {
			return err
		}
		if strings.TrimSpace(state.Booking.OperationID) == "" {
			return invalidState("booking_in_progress requires operation ID")
		}
		if strings.TrimSpace(state.Booking.ResultOperationID) != "" || state.Booking.Outcome != "" || strings.TrimSpace(state.Booking.ExternalID) != "" {
			return invalidState("booking_in_progress cannot contain a booking result")
		}
	case StepBooked:
		if state.Pending != RequirementNone {
			return invalidState("booked cannot have a pending requirement")
		}
		if err := validateBookingDraft(state, true); err != nil {
			return err
		}
		if strings.TrimSpace(state.Booking.OperationID) == "" || state.Booking.ResultOperationID != state.Booking.OperationID || state.Booking.Outcome != bookingcontract.BookingCreated || strings.TrimSpace(state.Booking.ExternalID) == "" {
			return invalidState("booked requires a confirmed created result")
		}
	case StepBookingUnknown:
		if state.Pending != RequirementNone {
			return invalidState("booking_unknown cannot have a pending requirement")
		}
		if err := validateBookingDraft(state, true); err != nil {
			return err
		}
		if strings.TrimSpace(state.Booking.OperationID) == "" || state.Booking.Outcome != bookingcontract.BookingResultUnknown || strings.TrimSpace(state.Booking.ExternalID) != "" {
			return invalidState("booking_unknown requires an unknown result without external ID")
		}
		if state.Booking.ResultOperationID != "" && state.Booking.ResultOperationID != state.Booking.OperationID {
			return invalidState("unknown booking result belongs to another operation")
		}
	case StepCancelled:
		if state.Pending != RequirementNone {
			return invalidState("cancelled cannot have a pending requirement")
		}
	}

	return nil
}

func validateServiceSnapshots(state ConversationState) error {
	for id, service := range state.OfferedServices {
		if strings.TrimSpace(id) == "" || strings.TrimSpace(service.ID) == "" || id != service.ID || strings.TrimSpace(service.Name) == "" {
			return invalidState("invalid offered service %q", id)
		}
	}
	return nil
}

func validateDateSnapshots(state ConversationState) error {
	for date, snapshot := range state.OfferedDates {
		if date == "" || snapshot.Date != date {
			return invalidState("invalid offered date %q", date)
		}
		if _, err := time.Parse(time.DateOnly, date); err != nil {
			return invalidState("invalid offered date %q", date)
		}
	}
	return nil
}

func validateStaffSnapshots(state ConversationState) error {
	for id, staff := range state.OfferedStaff {
		if strings.TrimSpace(id) == "" || strings.TrimSpace(staff.ID) == "" || id != staff.ID || strings.TrimSpace(staff.Name) == "" {
			return invalidState("invalid offered staff %q", id)
		}
	}
	return nil
}

func validateSlotSnapshots(state ConversationState) error {
	for id, slot := range state.OfferedSlots {
		if strings.TrimSpace(id) == "" || strings.TrimSpace(slot.ID) == "" || id != slot.ID || strings.TrimSpace(slot.ServiceID) == "" || strings.TrimSpace(slot.StaffID) == "" || slot.Date == "" || slot.StartsAt.IsZero() {
			return invalidState("invalid offered slot %q", id)
		}
		if _, err := time.Parse(time.DateOnly, slot.Date); err != nil {
			return invalidState("slot %q has invalid date", id)
		}
		if len(state.OfferedServices) > 0 {
			if _, ok := state.OfferedServices[slot.ServiceID]; !ok {
				return invalidState("slot %q refers to an unoffered service", id)
			}
		}
		if len(state.OfferedStaff) > 0 {
			if _, ok := state.OfferedStaff[slot.StaffID]; !ok {
				return invalidState("slot %q refers to unoffered staff", id)
			}
		}
	}
	return nil
}

func validateSelectedService(state ConversationState) error {
	serviceID := strings.TrimSpace(state.ServiceID)
	if serviceID == "" {
		return invalidState("selected service is required")
	}
	if _, ok := state.OfferedServices[serviceID]; !ok {
		return invalidState("selected service was not offered")
	}
	return nil
}

func validateSelectedDate(state ConversationState) error {
	date := strings.TrimSpace(state.SelectedDate)
	if date == "" {
		return invalidState("selected date is required")
	}
	if _, ok := state.OfferedDates[date]; !ok {
		return invalidState("selected date was not offered")
	}
	return nil
}

func validateSelectedStaff(state ConversationState) error {
	staffID := strings.TrimSpace(state.StaffID)
	if staffID == "" {
		return invalidState("selected staff is required")
	}
	if _, ok := state.OfferedStaff[staffID]; !ok {
		return invalidState("selected staff was not offered")
	}
	return nil
}

func validateBookingDraft(state ConversationState, requireContact bool) error {
	if err := validateSelectedService(state); err != nil {
		return err
	}
	if err := validateSelectedDate(state); err != nil {
		return err
	}
	if err := validateSelectedStaff(state); err != nil {
		return err
	}
	slot, ok := state.OfferedSlots[state.SelectedSlot]
	if strings.TrimSpace(state.SelectedSlot) == "" || !ok {
		return invalidState("selected slot was not offered")
	}
	if slot.ServiceID != state.ServiceID {
		return invalidState("selected slot belongs to another service")
	}
	if slot.Date != state.SelectedDate {
		return invalidState("selected slot belongs to another date")
	}
	if slot.StaffID != state.StaffID {
		return invalidState("selected slot belongs to another staff member")
	}
	if requireContact && (strings.TrimSpace(state.CustomerName) == "" || strings.TrimSpace(state.CustomerPhone) == "") {
		return invalidState("complete customer contact is required")
	}
	return nil
}

func isKnownStep(step Step) bool {
	switch step {
	case StepWaitingForService, StepWaitingForDate, StepWaitingForStaff, StepWaitingForTime, StepWaitingForContact, StepWaitingForConfirmation, StepBookingInProgress, StepBooked, StepBookingUnknown, StepCancelled:
		return true
	default:
		return false
	}
}

func isKnownRequirement(requirement Requirement) bool {
	switch requirement {
	case RequirementNone, RequireService, RequireDate, RequireStaff, RequireTime, RequireName, RequirePhone:
		return true
	default:
		return false
	}
}

func invalidState(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidState, fmt.Sprintf(format, args...))
}

func isEmptyBookingAttempt(attempt BookingAttempt) bool {
	return strings.TrimSpace(attempt.OperationID) == "" && strings.TrimSpace(attempt.ResultOperationID) == "" && attempt.Outcome == "" && strings.TrimSpace(attempt.ExternalID) == ""
}
