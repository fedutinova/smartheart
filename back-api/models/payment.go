package models

import (
	"time"

	"github.com/google/uuid"
)

// Payment status constants.
const (
	PaymentPending   = "pending"
	PaymentSucceeded = "succeeded"
	PaymentCanceled  = "canceled"
)

// Payment type constants.
const (
	PaymentTypeAnalyses     = "analyses"
	PaymentTypeSubscription = "subscription"
)

// Payment represents a YooKassa payment record.
type Payment struct {
	ID             uuid.UUID  `json:"id"`
	UserID         uuid.UUID  `json:"user_id"`
	YooKassaID     string     `json:"yookassa_id"`
	Status         string     `json:"status"`
	AmountKopecks  int        `json:"amount_kopecks"`
	Description    string     `json:"description"`
	AnalysesCount  int        `json:"analyses_count"`
	PaymentType    string     `json:"payment_type"`
	CreatedAt      time.Time  `json:"created_at"`
	ConfirmedAt    *time.Time `json:"confirmed_at,omitempty"`
}
