package yclients

import (
	"encoding/json"
	"time"
)

type ListRecordsFilter struct {
	Page          int
	Count         int
	StaffID       int64
	ClientID      int64
	CreatedUserID int64
	StartDate     time.Time
	EndDate       time.Time
	ChangedAfter  time.Time
	ChangedBefore time.Time
	WithDeleted   bool
}

type RecordPage struct {
	Records    []Record
	Page       int
	TotalCount int
}

type Record struct {
	ID           int64           `json:"id"`
	CompanyID    int64           `json:"company_id"`
	StaffID      int64           `json:"staff_id"`
	Services     []RecordService `json:"services"`
	Client       *RecordClient   `json:"client"`
	DateTime     string          `json:"datetime"`
	SeanceLength int             `json:"seance_length"`
	Comment      string          `json:"comment"`
	Attendance   int             `json:"attendance"`
	APIID        string          `json:"api_id"`
	Deleted      bool            `json:"deleted"`
}

type RecordService struct {
	ID        int64   `json:"id"`
	Title     string  `json:"title"`
	FirstCost float64 `json:"first_cost"`
	Discount  float64 `json:"discount"`
	Cost      float64 `json:"cost"`
}

type RecordClient struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}

type RecordServiceInput struct {
	ID        int64   `json:"id"`
	FirstCost float64 `json:"first_cost,omitempty"`
	Discount  float64 `json:"discount,omitempty"`
	Cost      float64 `json:"cost,omitempty"`
}

type RecordClientInput struct {
	Phone      string `json:"phone"`
	Name       string `json:"name"`
	Surname    string `json:"surname,omitempty"`
	Patronymic string `json:"patronymic,omitempty"`
	Email      string `json:"email,omitempty"`
}

type CreateRecordRequest struct {
	StaffID      int64                `json:"staff_id"`
	Services     []RecordServiceInput `json:"services"`
	Client       RecordClientInput    `json:"client"`
	SaveIfBusy   bool                 `json:"save_if_busy"`
	DateTime     string               `json:"datetime"`
	SeanceLength int                  `json:"seance_length"`
	SendSMS      bool                 `json:"send_sms"`
	Comment      string               `json:"comment,omitempty"`
	Attendance   int                  `json:"attendance"`
	APIID        string               `json:"api_id,omitempty"`
}

type UpdateRecordRequest CreateRecordRequest

type listRecordsResponse struct {
	Success bool       `json:"success"`
	Data    []Record   `json:"data"`
	Meta    recordMeta `json:"meta"`
}

type recordsResponse struct {
	Success bool            `json:"success"`
	Data    []Record        `json:"data"`
	Meta    json.RawMessage `json:"meta"`
}

type recordResponse struct {
	Success bool            `json:"success"`
	Data    Record          `json:"data"`
	Meta    json.RawMessage `json:"meta"`
}

type recordMeta struct {
	Page       int `json:"page"`
	TotalCount int `json:"total_count"`
}
