package service

import (
	"bytes"
	"context"
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
	// CreatePayment creates a YooKassa payment for analysis packs and returns the confirmation URL.
	CreatePayment(ctx context.Context, userID uuid.UUID, analysesCount int) (*PaymentResult, error)
	// CreateSubscription creates a YooKassa payment for a monthly subscription.
	CreateSubscription(ctx context.Context, userID uuid.UUID) (*PaymentResult, error)
	// HandleWebhook processes a YooKassa webhook notification.
	HandleWebhook(ctx context.Context, body []byte) error
	// GetQuotaInfo returns the user's current quota status.
	GetQuotaInfo(ctx context.Context, userID uuid.UUID) (*QuotaInfo, error)
}

// PaymentResult is returned after creating a payment.
type PaymentResult struct {
	PaymentID       uuid.UUID `json:"payment_id"`
	ConfirmationURL string    `json:"confirmation_url"`
	AmountRub       string    `json:"amount_rub"`
}

// QuotaInfo describes the user's current quota state.
type QuotaInfo struct {
	DailyLimit               int     `json:"daily_limit"`
	UsedToday                int     `json:"used_today"`
	FreeRemaining            int     `json:"free_remaining"`
	PaidAnalysesRemaining    int     `json:"paid_analyses_remaining"`
	NeedsPayment             bool    `json:"needs_payment"`
	PricePerAnalysisKopecks  int     `json:"price_per_analysis_kopecks"`
	SubscriptionExpiresAt    *string `json:"subscription_expires_at,omitempty"`
	SubscriptionPriceKopecks int     `json:"subscription_price_kopecks"`
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

func (s *paymentService) CreatePayment(ctx context.Context, userID uuid.UUID, analysesCount int) (*PaymentResult, error) {
	if analysesCount < 1 || analysesCount > 100 {
		return nil, fmt.Errorf("analyses count must be 1-100: %w", apperr.ErrValidation)
	}

	totalKopecks := s.cfg.PriceKopecks * analysesCount
	paymentID := uuid.New()

	return s.createYooKassaPayment(ctx, &models.Payment{
		ID:            paymentID,
		UserID:        userID,
		AmountKopecks: totalKopecks,
		Description:   fmt.Sprintf("SmartHeart: %d анализ(ов) ЭКГ", analysesCount),
		AnalysesCount: analysesCount,
		PaymentType:   models.PaymentTypeAnalyses,
	}, map[string]string{
		"payment_id": paymentID.String(),
		"user_id":    userID.String(),
	})
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

	paymentID := uuid.New()

	return s.createYooKassaPayment(ctx, &models.Payment{
		ID:            paymentID,
		UserID:        userID,
		AmountKopecks: s.cfg.SubscriptionPriceKopecks,
		Description:   "SmartHeart: подписка на 30 дней",
		AnalysesCount: 0,
		PaymentType:   models.PaymentTypeSubscription,
	}, map[string]string{
		"payment_id": paymentID.String(),
		"user_id":    userID.String(),
		"type":       "subscription",
	})
}

func (s *paymentService) HandleWebhook(ctx context.Context, body []byte) error {
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
		slog.InfoContext(ctx, "Payment confirmed", "yookassa_id", yookassaID)

	case "payment.canceled":
		if err := s.repo.CancelPayment(ctx, yookassaID); err != nil {
			slog.ErrorContext(ctx, "Failed to cancel payment", "yookassa_id", yookassaID, "error", err)
			return apperr.WrapInternal("cancel payment", err)
		}
		slog.InfoContext(ctx, "Payment canceled", "yookassa_id", yookassaID)

	default:
		slog.DebugContext(ctx, "Ignoring webhook event", "event", webhook.Event)
	}

	return nil
}

func (s *paymentService) GetQuotaInfo(ctx context.Context, userID uuid.UUID) (*QuotaInfo, error) {
	usedToday, err := s.repo.GetDailyUsage(ctx, userID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get daily usage", "user_id", userID, "error", err)
		usedToday = 0
	}

	paidRemaining, err := s.repo.GetPaidAnalysesRemaining(ctx, userID)
	if err != nil {
		slog.WarnContext(ctx, "Failed to get paid analyses", "user_id", userID, "error", err)
		paidRemaining = 0
	}

	subExpiresAt, err := s.repo.GetSubscriptionExpiresAt(ctx, userID)
	if err != nil {
		slog.WarnContext(ctx, "Failed to get subscription", "user_id", userID, "error", err)
	}

	hasActiveSubscription := subExpiresAt != nil && subExpiresAt.After(time.Now())

	freeRemaining := max(s.freeLimit-usedToday, 0)

	needsPayment := !hasActiveSubscription && freeRemaining == 0 && paidRemaining == 0

	info := &QuotaInfo{
		DailyLimit:               s.freeLimit,
		UsedToday:                usedToday,
		FreeRemaining:            freeRemaining,
		PaidAnalysesRemaining:    paidRemaining,
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
