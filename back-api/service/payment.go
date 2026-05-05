package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/fedutinova/smartheart/back-api/apperr"
	"github.com/fedutinova/smartheart/back-api/config"
	"github.com/fedutinova/smartheart/back-api/models"
	"github.com/fedutinova/smartheart/back-api/repository"
)

// PaymentService handles payment creation and webhook processing.
type PaymentService interface {
	// CreateSubscription creates a YooKassa payment for a monthly subscription.
	CreateSubscription(ctx context.Context, userID uuid.UUID) (*PaymentResult, error)
	// HandleWebhook processes a YooKassa webhook notification after verifying the signature.
	HandleWebhook(ctx context.Context, body []byte, signature string) error
	// GetQuotaInfo returns the user's current quota status.
	GetQuotaInfo(ctx context.Context, userID uuid.UUID) (*QuotaInfo, error)
	// ValidatePromoCode checks if a promo code is valid and returns discount info.
	ValidatePromoCode(ctx context.Context, userID uuid.UUID, code string) (*PromoDiscountInfo, error)
}

// PaymentResult is returned after creating a payment.
type PaymentResult struct {
	PaymentID       uuid.UUID `json:"payment_id"`
	ConfirmationURL string    `json:"confirmation_url"`
	AmountRub       string    `json:"amount_rub"`
}

// QuotaInfo describes the user's current quota state (lifetime free analyses).
type QuotaInfo struct {
	FreeLimit                int     `json:"free_limit"`
	FreeAnalysesUsed         int     `json:"free_analyses_used"`
	FreeRemaining            int     `json:"free_remaining"`
	PaidAnalysesRemaining    int     `json:"paid_analyses_remaining"`
	NeedsPayment             bool    `json:"needs_payment"`
	PricePerAnalysisKopecks  int     `json:"price_per_analysis_kopecks"`
	SubscriptionExpiresAt    *string `json:"subscription_expires_at,omitempty"`
	SubscriptionPriceKopecks int     `json:"subscription_price_kopecks"`
}

// PromoDiscountInfo contains information about a valid promo code discount.
type PromoDiscountInfo struct {
	Code            string `json:"code"`
	DiscountPercent int    `json:"discount_percent"`
	IsValid         bool   `json:"is_valid"`
	Reason          string `json:"reason,omitempty"` // if not valid
}

type paymentService struct {
	repo       repository.Store
	cfg        config.YooKassaConfig
	freeLimit  int
	httpClient *http.Client
}

func NewPaymentService(repo repository.Store, ykCfg config.YooKassaConfig, freeLimit int) PaymentService {
	return &paymentService{
		repo:       repo,
		cfg:        ykCfg,
		freeLimit:  freeLimit,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// StartStalePaymentCleaner launches a background goroutine that periodically
// cancels pending payments older than maxAge. It stops when ctx is canceled.
func StartStalePaymentCleaner(ctx context.Context, repo repository.Store, interval, maxAge time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				canceled, err := repo.CancelStalePayments(ctx, maxAge)
				if err != nil {
					slog.WarnContext(ctx, "Failed to cancel stale payments", "error", err)
				} else if canceled > 0 {
					slog.InfoContext(ctx, "Canceled stale payments", "count", canceled)
				}
			}
		}
	}()
}

// YooKassa API types

type yooKassaAmount struct {
	Value    string `json:"value"`
	Currency string `json:"currency"`
}

type yooKassaConfirmation struct {
	Type      string `json:"type"`
	ReturnURL string `json:"return_url,omitempty"`
	URL       string `json:"confirmation_url,omitempty"`
}

type yooKassaCreateRequest struct {
	Amount       yooKassaAmount       `json:"amount"`
	Confirmation yooKassaConfirmation `json:"confirmation"`
	Description  string               `json:"description"`
	Metadata     map[string]string    `json:"metadata,omitempty"`
}

type yooKassaPaymentResponse struct {
	ID           string                `json:"id"`
	Status       string                `json:"status"`
	Amount       yooKassaAmount        `json:"amount"`
	Confirmation *yooKassaConfirmation `json:"confirmation,omitempty"`
}

type yooKassaWebhook struct {
	Event  string `json:"event"`
	Object struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	} `json:"object"`
}

// createYooKassaPayment is the shared logic for creating a payment in YooKassa and saving it locally.
func (s *paymentService) createYooKassaPayment(ctx context.Context, payment *models.Payment, metadata map[string]string) (*PaymentResult, error) {
	if s.cfg.ShopID == "" || s.cfg.SecretKey == "" {
		return nil, fmt.Errorf("payments not configured: %w", apperr.ErrInternal)
	}

	amountRub := fmt.Sprintf("%d.%02d", payment.AmountKopecks/100, payment.AmountKopecks%100)

	reqBody := yooKassaCreateRequest{
		Amount: yooKassaAmount{
			Value:    amountRub,
			Currency: "RUB",
		},
		Confirmation: yooKassaConfirmation{
			Type:      "redirect",
			ReturnURL: s.cfg.ReturnURL,
		},
		Description: payment.Description,
		Metadata:    metadata,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, apperr.WrapInternal("marshal payment request", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.yookassa.ru/v3/payments", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, apperr.WrapInternal("create payment request", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotence-Key", payment.ID.String())
	req.SetBasicAuth(s.cfg.ShopID, s.cfg.SecretKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, apperr.WrapInternal("call YooKassa API", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, apperr.WrapInternal("read YooKassa response", err)
	}

	if resp.StatusCode != http.StatusOK {
		slog.ErrorContext(ctx, "YooKassa API error", "status", resp.StatusCode, "body", string(respBody))
		return nil, fmt.Errorf("yookassa returned %d: %w", resp.StatusCode, apperr.ErrInternal)
	}

	var ykResp yooKassaPaymentResponse
	if err := json.Unmarshal(respBody, &ykResp); err != nil {
		return nil, apperr.WrapInternal("parse YooKassa response", err)
	}

	if ykResp.Confirmation == nil || ykResp.Confirmation.URL == "" {
		return nil, fmt.Errorf("no confirmation URL in response: %w", apperr.ErrInternal)
	}

	payment.YooKassaID = ykResp.ID
	payment.Status = models.PaymentPending
	if err := s.repo.CreatePayment(ctx, payment); err != nil {
		return nil, apperr.WrapInternal("save payment record", err)
	}

	slog.InfoContext(ctx, "Payment created", "payment_id", payment.ID, "yookassa_id", ykResp.ID, "user_id", payment.UserID, "type", payment.PaymentType, "amount", amountRub)

	return &PaymentResult{
		PaymentID:       payment.ID,
		ConfirmationURL: ykResp.Confirmation.URL,
		AmountRub:       amountRub,
	}, nil
}

func (s *paymentService) CreateSubscription(ctx context.Context, userID uuid.UUID) (*PaymentResult, error) {
	// Reject if user already has an active subscription.
	subExpires, err := s.repo.GetSubscriptionExpiresAt(ctx, userID)
	if err != nil {
		return nil, apperr.WrapInternal("check subscription", err)
	}
	if subExpires != nil && subExpires.After(time.Now()) {
		return nil, fmt.Errorf("subscription is already active until %s: %w",
			subExpires.Format("2006-01-02"), apperr.ErrConflict)
	}

	// Reject if there is already a pending subscription payment to prevent
	// duplicate charges from concurrent requests.
	hasPending, err := s.repo.HasPendingPayment(ctx, userID, models.PaymentTypeSubscription)
	if err != nil {
		return nil, apperr.WrapInternal("check pending subscription", err)
	}
	if hasPending {
		return nil, fmt.Errorf("subscription payment is already in progress: %w", apperr.ErrConflict)
	}

	paymentID := uuid.New()

	return s.createYooKassaPayment(ctx, &models.Payment{
		ID:            paymentID,
		UserID:        userID,
		AmountKopecks: s.cfg.SubscriptionPriceKopecks,
		Description:   "Умное сердце: подписка на 30 дней",
		AnalysesCount: 0,
		PaymentType:   models.PaymentTypeSubscription,
	}, map[string]string{
		"payment_id": paymentID.String(),
		"user_id":    userID.String(),
		"type":       "subscription",
	})
}

func (s *paymentService) HandleWebhook(ctx context.Context, body []byte, signature string) error {
	// Verify webhook signature using HMAC-SHA256
	// YooKassa sends: X-Webhook-Signature = base64(HMAC-SHA256(body, secret_key))
	if s.cfg.SecretKey == "" {
		return fmt.Errorf("webhook secret key not configured: %w", apperr.ErrInternal)
	}

	expectedSig := base64.StdEncoding.EncodeToString(
		hmac.New(sha256.New, []byte(s.cfg.SecretKey)).Sum(body),
	)

	if !hmac.Equal([]byte(signature), []byte(expectedSig)) {
		sigPreview := signature
		if len(signature) > 10 {
			sigPreview = signature[:10]
		}
		slog.WarnContext(ctx, "Webhook signature verification failed", "provided", sigPreview, "expected", expectedSig[:10])
		return fmt.Errorf("invalid webhook signature: %w", apperr.ErrValidation)
	}

	var webhook yooKassaWebhook
	if err := json.Unmarshal(body, &webhook); err != nil {
		return fmt.Errorf("parse webhook: %w", apperr.ErrValidation)
	}

	yookassaID := webhook.Object.ID
	if yookassaID == "" {
		return fmt.Errorf("empty payment ID in webhook: %w", apperr.ErrValidation)
	}

	switch webhook.Event {
	case "payment.succeeded":
		if err := s.repo.ConfirmPayment(ctx, yookassaID); err != nil {
			slog.ErrorContext(ctx, "Failed to confirm payment", "yookassa_id", yookassaID, "error", err)
			return apperr.WrapInternal("confirm payment", err)
		}
		slog.InfoContext(ctx, "Payment confirmed via webhook", "yookassa_id", yookassaID)

	case "payment.canceled":
		if err := s.repo.CancelPayment(ctx, yookassaID); err != nil {
			slog.ErrorContext(ctx, "Failed to cancel payment", "yookassa_id", yookassaID, "error", err)
			return apperr.WrapInternal("cancel payment", err)
		}
		slog.InfoContext(ctx, "Payment canceled via webhook", "yookassa_id", yookassaID)

	default:
		slog.DebugContext(ctx, "Ignoring webhook event", "event", webhook.Event)
	}

	return nil
}

func (s *paymentService) GetQuotaInfo(ctx context.Context, userID uuid.UUID) (*QuotaInfo, error) {
	freeUsed, err := s.repo.GetFreeAnalysesUsed(ctx, userID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get free analyses used", "user_id", userID, "error", err)
		freeUsed = 0
	}

	subExpiresAt, err := s.repo.GetSubscriptionExpiresAt(ctx, userID)
	if err != nil {
		slog.WarnContext(ctx, "Failed to get subscription", "user_id", userID, "error", err)
	}

	hasActiveSubscription := subExpiresAt != nil && subExpiresAt.After(time.Now())

	freeRemaining := max(s.freeLimit-freeUsed, 0)

	needsPayment := !hasActiveSubscription && freeRemaining == 0

	info := &QuotaInfo{
		FreeLimit:                s.freeLimit,
		FreeAnalysesUsed:         freeUsed,
		FreeRemaining:            freeRemaining,
		PaidAnalysesRemaining:    0,
		NeedsPayment:             needsPayment,
		PricePerAnalysisKopecks:  s.cfg.PriceKopecks,
		SubscriptionPriceKopecks: s.cfg.SubscriptionPriceKopecks,
	}

	if subExpiresAt != nil {
		formatted := subExpiresAt.Format(time.RFC3339)
		info.SubscriptionExpiresAt = &formatted
	}

	return info, nil
}

func (s *paymentService) ValidatePromoCode(ctx context.Context, userID uuid.UUID, code string) (*PromoDiscountInfo, error) {
	result := &PromoDiscountInfo{
		Code:    code,
		IsValid: false,
	}

	promoCode, err := s.repo.GetPromoCodeByCode(ctx, code)
	if err != nil {
		result.Reason = "Промокод не найден"
		return result, nil
	}

	if !promoCode.IsValid() {
		if promoCode.ExpiresAt != nil && time.Now().After(*promoCode.ExpiresAt) {
			result.Reason = "Промокод истёк"
		} else if promoCode.MaxUses > 0 && promoCode.UsedCount >= promoCode.MaxUses {
			result.Reason = "Промокод использован максимальное количество раз"
		}
		return result, nil
	}

	result.IsValid = true
	result.DiscountPercent = promoCode.DiscountPercent
	return result, nil
}
