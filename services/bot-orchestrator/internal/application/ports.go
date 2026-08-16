package application

import (
	"context"

	bookingcontract "ai-chatbot/contracts/bookingapi"
)

type Interpreter interface {
	Interpret(context.Context, InterpretationRequest) (ActionEnvelope, error)
}

type BookingGateway interface {
	ListServices(context.Context) ([]bookingcontract.Service, error)
	ListStaff(context.Context, bookingcontract.ListStaffRequest) ([]bookingcontract.Staff, error)
	SearchSlots(context.Context, bookingcontract.SearchSlotsRequest) ([]bookingcontract.Slot, error)
	CreateBooking(context.Context, bookingcontract.CreateBookingRequest) (bookingcontract.BookingResult, error)
}

type ConversationStore interface {
	Load(context.Context, string) (ConversationState, bool, error)
	Save(context.Context, ConversationState) error
}
