package application

import (
	"time"

	bookingcontract "ai-chatbot/contracts/bookingapi"
)

type Step string

const (
	StepWaitingForService Step = "waiting_for_service"
	StepWaitingForTime    Step = "waiting_for_time"
	StepWaitingForContact Step = "waiting_for_contact"
	StepBookingInProgress Step = "booking_in_progress"
	StepBooked            Step = "booked"
	StepBookingUnknown    Step = "booking_unknown"
	StepCancelled         Step = "cancelled"
)

type Requirement string

const (
	RequirementNone Requirement = ""
	RequireService  Requirement = "require_service"
	RequireTime     Requirement = "require_time"
	RequireName     Requirement = "require_name"
	RequirePhone    Requirement = "require_phone"
)

type SlotSnapshot struct {
	ID        string
	ServiceID string
	StartsAt  time.Time
}

type BookingAttempt struct {
	OperationID string
	Outcome     bookingcontract.BookingOutcome
	ExternalID  string
}

type ConversationState struct {
	ID       string
	Revision uint64

	Step    Step
	Pending Requirement

	ServiceID     string
	OfferedSlots  map[string]SlotSnapshot
	SelectedSlot  string
	CustomerName  string
	CustomerPhone string

	Booking   BookingAttempt
	UpdatedAt time.Time
}

type ActionType string

const (
	ActionChooseService  ActionType = "choose_service"
	ActionChooseTime     ActionType = "choose_time"
	ActionProvideContact ActionType = "provide_contact"
	ActionChangeService  ActionType = "change_service"
	ActionChangeTime     ActionType = "change_time"
	ActionAskQuestion    ActionType = "ask_question"
	ActionCancelFlow     ActionType = "cancel_flow"
	ActionClarify        ActionType = "clarify"
)

type ActionEnvelope struct {
	Action        ActionType      `json:"action"`
	Arguments     ActionArguments `json:"arguments"`
	StateRevision uint64          `json:"state_revision"`
}

type ActionArguments struct {
	ServiceID string `json:"service_id,omitempty"`
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
