package models

import (
	"time"

	"github.com/google/uuid"
)

// RequestStatus is a typed string for request lifecycle states,
// distinct from job.Status which tracks queue lifecycle.
type RequestStatus = string

// Request status constants.
const (
	StatusPending    RequestStatus = "pending"
	StatusProcessing RequestStatus = "processing"
	StatusCompleted  RequestStatus = "completed"
	StatusFailed     RequestStatus = "failed"
)

// ValidRequestStatus reports whether s is a known request status.
func ValidRequestStatus(s RequestStatus) bool {
	switch s {
	case StatusPending, StatusProcessing, StatusCompleted, StatusFailed:
		return true
	default:
		return false
	}
}

// Request represents an EKG or GPT analysis request
type Request struct {
	ID        uuid.UUID     `json:"id"`
	UserID    uuid.UUID     `json:"user_id,omitempty"`
	TextQuery *string       `json:"text_query,omitempty"`
	Status    RequestStatus `json:"status"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
	Files     []File        `json:"files,omitempty"`
	Response  *Response     `json:"response,omitempty"`

	// ECG analysis parameters (nullable — only set for EKG requests)
	ECGAge          *int     `json:"ecg_age,omitempty"`
	ECGSex          *string  `json:"ecg_sex,omitempty"`
	ECGPaperSpeedMMS *float64 `json:"ecg_paper_speed_mms,omitempty"`
	ECGMmPerMvLimb  *float64 `json:"ecg_mm_per_mv_limb,omitempty"`
	ECGMmPerMvChest *float64 `json:"ecg_mm_per_mv_chest,omitempty"`
}

