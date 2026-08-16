package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	bookingcontract "ai-chatbot/contracts/bookingapi"
	"ai-chatbot/services/booking-service/internal/application"
)

const maxRequestBytes = 1 << 20

func (s *server) listServices(w http.ResponseWriter, r *http.Request) {
	services, err := s.app.ListServices(r.Context())
	if err != nil {
		writeApplicationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, services)
}

func (s *server) listStaff(w http.ResponseWriter, r *http.Request) {
	var request bookingcontract.ListStaffRequest
	if !decode(w, r, &request) {
		return
	}

	staff, err := s.app.ListStaff(r.Context(), request)
	if err != nil {
		writeApplicationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, staff)
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
	if errors.Is(err, application.ErrInvalidRequest) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
