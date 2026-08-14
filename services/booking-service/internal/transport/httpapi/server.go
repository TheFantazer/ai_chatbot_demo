package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	bookingcontract "ai-chatbot/contracts/bookingapi"
	"ai-chatbot/services/booking-service/internal/application"
)

const maxRequestBytes = 1 << 20

type Application interface {
	ListServices(context.Context) ([]bookingcontract.Service, error)
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
	mux.HandleFunc("POST /v1/slots/search", s.searchSlots)
	mux.HandleFunc("POST /v1/bookings", s.createBooking)
	return mux
}

func (s *server) listServices(w http.ResponseWriter, r *http.Request) {
	services, err := s.app.ListServices(r.Context())
	if err != nil {
		writeApplicationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, services)
}

func (s *server) searchSlots(w http.ResponseWriter, r *http.Request) {
	var request bookingcontract.SearchSlotsRequest
	if !decode(w, r, &request) {
		return
	}

	slots, err := s.app.SearchSlots(r.Context(), request)
	if err != nil {
		writeApplicationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, slots)
}

func (s *server) createBooking(w http.ResponseWriter, r *http.Request) {
	var request bookingcontract.CreateBookingRequest
	if !decode(w, r, &request) {
		return
	}

	result, err := s.app.CreateBooking(r.Context(), request)
	if err != nil {
		writeApplicationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return false
	}
	return true
}

func writeApplicationError(w http.ResponseWriter, err error) {
	if errors.Is(err, application.ErrApplicationNotImplemented) {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
