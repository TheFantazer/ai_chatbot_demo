package application

import (
	"context"
	"errors"

	bookingcontract "ai-chatbot/contracts/bookingapi"
)

var ErrApplicationNotImplemented = errors.New("booking application is not implemented")

type Provider interface {
	ListServices(context.Context) ([]bookingcontract.Service, error)
	SearchSlots(context.Context, bookingcontract.SearchSlotsRequest) ([]bookingcontract.Slot, error)
	CreateBooking(context.Context, bookingcontract.CreateBookingRequest) (bookingcontract.BookingResult, error)
}

type Service struct {
	provider Provider
}

func NewService(provider Provider) *Service {
	return &Service{provider: provider}
}

// implement me
func (s *Service) ListServices(context.Context) ([]bookingcontract.Service, error) {
	return nil, ErrApplicationNotImplemented
}

// implement me
func (s *Service) SearchSlots(context.Context, bookingcontract.SearchSlotsRequest) ([]bookingcontract.Slot, error) {
	return nil, ErrApplicationNotImplemented
}

// implement me
func (s *Service) CreateBooking(context.Context, bookingcontract.CreateBookingRequest) (bookingcontract.BookingResult, error) {
	return bookingcontract.BookingResult{}, ErrApplicationNotImplemented
}
