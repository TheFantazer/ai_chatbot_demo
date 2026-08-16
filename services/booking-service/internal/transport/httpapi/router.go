package httpapi

import (
	"context"
	"net/http"

	bookingcontract "ai-chatbot/contracts/bookingapi"
)

type Application interface {
	ListServices(context.Context) ([]bookingcontract.Service, error)
	ListStaff(context.Context, bookingcontract.ListStaffRequest) ([]bookingcontract.Staff, error)
	SearchSlots(context.Context, bookingcontract.SearchSlotsRequest) ([]bookingcontract.Slot, error)
	CreateBooking(context.Context, bookingcontract.CreateBookingRequest) (bookingcontract.BookingResult, error)
}

type server struct {
	app Application
}

func New(app Application) http.Handler {
	s := &server{app: app}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/services", s.listServices)
	mux.HandleFunc("POST /v1/staff/search", s.listStaff)
	mux.HandleFunc("POST /v1/slots/search", s.searchSlots)
	mux.HandleFunc("POST /v1/bookings", s.createBooking)
	return mux
}
