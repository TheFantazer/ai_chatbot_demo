package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	bookingcontract "ai-chatbot/contracts/bookingapi"
)

var ErrInvalidRequest = errors.New("invalid booking request")

type Provider interface {
	ListServices(context.Context) ([]bookingcontract.Service, error)
	ListStaff(context.Context, bookingcontract.ListStaffRequest) ([]bookingcontract.Staff, error)
	SearchSlots(context.Context, bookingcontract.SearchSlotsRequest) ([]bookingcontract.Slot, error)
	CreateBooking(context.Context, bookingcontract.CreateBookingRequest) (bookingcontract.BookingResult, error)
}

type Service struct {
	provider     Provider
	operationsMu sync.Mutex
	operations   map[string]*operation
}

type operation struct {
	request bookingcontract.CreateBookingRequest
	done    chan struct{}
	result  bookingcontract.BookingResult
	err     error
}

func NewService(provider Provider) *Service {
	return &Service{provider: provider, operations: make(map[string]*operation)}
}

func (s *Service) ListServices(ctx context.Context) ([]bookingcontract.Service, error) {
	return s.provider.ListServices(ctx)
}

func (s *Service) ListStaff(ctx context.Context, request bookingcontract.ListStaffRequest) ([]bookingcontract.Staff, error) {
	if strings.TrimSpace(request.ServiceID) == "" {
		return nil, fmt.Errorf("%w: service ID is required", ErrInvalidRequest)
	}
	return s.provider.ListStaff(ctx, request)
}

func (s *Service) SearchSlots(ctx context.Context, request bookingcontract.SearchSlotsRequest) ([]bookingcontract.Slot, error) {
	if strings.TrimSpace(request.ServiceID) == "" || strings.TrimSpace(request.StaffID) == "" {
		return nil, fmt.Errorf("%w: service ID and staff ID are required", ErrInvalidRequest)
	}
	if _, err := time.Parse(time.DateOnly, request.Date); err != nil {
		return nil, fmt.Errorf("%w: date must use YYYY-MM-DD", ErrInvalidRequest)
	}
	return s.provider.SearchSlots(ctx, request)
}

func (s *Service) CreateBooking(ctx context.Context, request bookingcontract.CreateBookingRequest) (bookingcontract.BookingResult, error) {
	request = normalizeCreateBookingRequest(request)
	if request.OperationID == "" || request.ServiceID == "" || request.SlotID == "" || request.Customer.Name == "" || request.Customer.Phone == "" {
		return bookingcontract.BookingResult{}, fmt.Errorf("%w: operation, service, slot and customer are required", ErrInvalidRequest)
	}

	s.operationsMu.Lock()
	if existing, found := s.operations[request.OperationID]; found {
		if existing.request != request {
			s.operationsMu.Unlock()
			return bookingcontract.BookingResult{}, fmt.Errorf("%w: operation ID is already associated with another booking", ErrInvalidRequest)
		}
		done := existing.done
		s.operationsMu.Unlock()
		select {
		case <-ctx.Done():
			return bookingcontract.BookingResult{}, ctx.Err()
		case <-done:
			return existing.result, existing.err
		}
	}

	current := &operation{request: request, done: make(chan struct{})}
	s.operations[request.OperationID] = current
	s.operationsMu.Unlock()

	current.result, current.err = s.provider.CreateBooking(ctx, request)
	close(current.done)

	return current.result, current.err
}

func normalizeCreateBookingRequest(request bookingcontract.CreateBookingRequest) bookingcontract.CreateBookingRequest {
	request.OperationID = strings.TrimSpace(request.OperationID)
	request.ServiceID = strings.TrimSpace(request.ServiceID)
	request.SlotID = strings.TrimSpace(request.SlotID)
	request.Customer.Name = strings.TrimSpace(request.Customer.Name)
	request.Customer.Phone = strings.TrimSpace(request.Customer.Phone)
	return request
}
