package bookingapi

import "time"

type Service struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Staff struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Specialization string `json:"specialization,omitempty"`
}

type ListStaffRequest struct {
	ServiceID string `json:"service_id"`
}

type SearchSlotsRequest struct {
	ServiceID string `json:"service_id"`
	StaffID   string `json:"staff_id"`
	Date      string `json:"date"`
}

type Slot struct {
	ID        string    `json:"id"`
	ServiceID string    `json:"service_id"`
	StaffID   string    `json:"staff_id"`
	StartsAt  time.Time `json:"starts_at"`
}

type Customer struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

type CreateBookingRequest struct {
	OperationID string   `json:"operation_id"`
	ServiceID   string   `json:"service_id"`
	SlotID      string   `json:"slot_id"`
	Customer    Customer `json:"customer"`
}

type BookingOutcome string

const (
	BookingCreated       BookingOutcome = "created"
	BookingRejected      BookingOutcome = "rejected"
	BookingResultUnknown BookingOutcome = "unknown"
)

type BookingResult struct {
	OperationID string         `json:"operation_id"`
	Outcome     BookingOutcome `json:"outcome"`
	ExternalID  string         `json:"external_id,omitempty"`
	Reason      string         `json:"reason,omitempty"`
}
