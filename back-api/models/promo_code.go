package models

import (
	"time"

	"github.com/google/uuid"
)

// PromoCode represents a discount promo code.
type PromoCode struct {
	ID              uuid.UUID  `json:"id"`
	Code            string     `json:"code"`
	DiscountPercent int        `json:"discount_percent"` // 0-100
	MaxUses         int        `json:"max_uses"`         // 0 = unlimited
	UsedCount       int        `json:"used_count"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// IsValid checks if the promo code is still valid.
func (p *PromoCode) IsValid() bool {
	if p.ExpiresAt != nil && time.Now().After(*p.ExpiresAt) {
		return false
	}
	if p.MaxUses > 0 && p.UsedCount >= p.MaxUses {
		return false
	}
	return true
}

// PromoCodeUsage tracks when a user uses a promo code.
type PromoCodeUsage struct {
	ID                   uuid.UUID
	UserID               uuid.UUID
	PromoCodeID          uuid.UUID
	PaymentID            *uuid.UUID // optional, set after payment
	DiscountAmountKopeks int
	UsedAt               time.Time
}
