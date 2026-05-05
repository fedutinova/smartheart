package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/fedutinova/smartheart/back-api/models"
)

func (r *Repository) GetPromoCodeByCode(ctx context.Context, code string) (*models.PromoCode, error) {
	const query = `
		SELECT id, code, discount_percent, max_uses, used_count, expires_at, created_at, updated_at
		FROM promo_codes
		WHERE code = $1
	`
	row := r.querier.QueryRow(ctx, query, code)
	var promo models.PromoCode
	if err := row.Scan(
		&promo.ID, &promo.Code, &promo.DiscountPercent, &promo.MaxUses,
		&promo.UsedCount, &promo.ExpiresAt, &promo.CreatedAt, &promo.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &promo, nil
}

func (r *Repository) CreatePromoCode(ctx context.Context, promo *models.PromoCode) error {
	const query = `
		INSERT INTO promo_codes (id, code, discount_percent, max_uses, used_count, expires_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.querier.Exec(ctx, query,
		promo.ID, promo.Code, promo.DiscountPercent, promo.MaxUses,
		promo.UsedCount, promo.ExpiresAt, promo.CreatedAt, promo.UpdatedAt,
	)
	return err
}

func (r *Repository) UpdatePromoCodeUsedCount(ctx context.Context, promoCodeID uuid.UUID) error {
	const query = `
		UPDATE promo_codes
		SET used_count = used_count + 1, updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.querier.Exec(ctx, query, promoCodeID)
	return err
}

func (r *Repository) RecordPromoCodeUsage(ctx context.Context, usage *models.PromoCodeUsage) error {
	const query = `
		INSERT INTO promo_code_usage (id, user_id, promo_code_id, payment_id, discount_amount_kopecks, used_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.querier.Exec(ctx, query,
		usage.ID, usage.UserID, usage.PromoCodeID, usage.PaymentID,
		usage.DiscountAmountKopeks, usage.UsedAt,
	)
	return err
}
