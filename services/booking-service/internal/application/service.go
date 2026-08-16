package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	bookingcontract "ai-chatbot/contracts/bookingapi"
)

var ErrInvalidRequest = errors.New("invalid booking request")

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

func (s *Service) ListServices(ctx context.Context) ([]bookingcontract.Service, error) {
	return s.provider.ListServices(ctx)
}

func (s *Service) SearchSlots(ctx context.Context, request bookingcontract.SearchSlotsRequest) ([]bookingcontract.Slot, error) {
	if strings.TrimSpace(request.ServiceID) == "" {
		return nil, fmt.Errorf("%w: service ID is required", ErrInvalidRequest)
	}
	if _, err := time.Parse(time.DateOnly, request.Date); err != nil {
		return nil, fmt.Errorf("%w: date must use YYYY-MM-DD", ErrInvalidRequest)
	}
	return s.provider.SearchSlots(ctx, request)
}

func (s *Service) CreateBooking(ctx context.Context, request bookingcontract.CreateBookingRequest) (bookingcontract.BookingResult, error) {
	if strings.TrimSpace(request.OperationID) == "" || strings.TrimSpace(request.ServiceID) == "" || strings.TrimSpace(request.SlotID) == "" || strings.TrimSpace(request.Customer.Name) == "" || strings.TrimSpace(request.Customer.Phone) == "" {
		return bookingcontract.BookingResult{}, fmt.Errorf("%w: operation, service, slot and customer are required", ErrInvalidRequest)
	}
	return s.provider.CreateBooking(ctx, request)
}
