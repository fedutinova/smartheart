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

	"github.com/fedutinova/smartheart/back-api/apperr"
	"github.com/fedutinova/smartheart/back-api/config"
	"github.com/fedutinova/smartheart/back-api/models"
	"github.com/fedutinova/smartheart/back-api/repository"
	"github.com/google/uuid"
)

// PaymentService handles payment creation and webhook processing.
type PaymentService interface {
	// CreatePayment creates a YooKassa payment and returns the confirmation URL.
	CreatePayment(ctx context.Context, userID uuid.UUID, analysesCount int) (*PaymentResult, error)
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
	DailyLimit             int  `json:"daily_limit"`
	UsedToday              int  `json:"used_today"`
	FreeRemaining          int  `json:"free_remaining"`
	PaidAnalysesRemaining  int  `json:"paid_analyses_remaining"`
	NeedsPayment           bool `json:"needs_payment"`
	PricePerAnalysisKopecks int `json:"price_per_analysis_kopecks"`
}

type paymentService struct {
	repo         repository.Store
	cfg          config.YooKassaConfig
	freeLimit    int
	httpClient   *http.Client
}

func NewPaymentService(repo repository.Store, ykCfg config.YooKassaConfig, freeLimit int) PaymentService {
	return &paymentService{
		repo:      repo,
		cfg:       ykCfg,
		freeLimit: freeLimit,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
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
	ID           string               `json:"id"`
	Status       string               `json:"status"`
	Amount       yooKassaAmount       `json:"amount"`
	Confirmation *yooKassaConfirmation `json:"confirmation,omitempty"`
}

type yooKassaWebhook struct {
	Event  string `json:"event"`
	Object struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	} `json:"object"`
}

func (s *paymentService) CreatePayment(ctx context.Context, userID uuid.UUID, analysesCount int) (*PaymentResult, error) {
	if s.cfg.ShopID == "" || s.cfg.SecretKey == "" {
		return nil, fmt.Errorf("payments not configured: %w", apperr.ErrInternal)
	}
	if analysesCount < 1 || analysesCount > 100 {
		return nil, fmt.Errorf("analyses count must be 1-100: %w", apperr.ErrValidation)
	}

	totalKopecks := s.cfg.PriceKopecks * analysesCount
	amountRub := fmt.Sprintf("%d.%02d", totalKopecks/100, totalKopecks%100)
	description := fmt.Sprintf("SmartHeart: %d анализ(ов) ЭКГ", analysesCount)

	paymentID := uuid.New()

	// Create payment in YooKassa
	reqBody := yooKassaCreateRequest{
		Amount: yooKassaAmount{
			Value:    amountRub,
			Currency: "RUB",
		},
		Confirmation: yooKassaConfirmation{
			Type:      "redirect",
			ReturnURL: s.cfg.ReturnURL,
		},
		Description: description,
		Metadata: map[string]string{
			"payment_id": paymentID.String(),
			"user_id":    userID.String(),
		},
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
	req.Header.Set("Idempotence-Key", paymentID.String())
	req.SetBasicAuth(s.cfg.ShopID, s.cfg.SecretKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, apperr.WrapInternal("call YooKassa API", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, apperr.WrapInternal("read YooKassa response", err)
	}

	if resp.StatusCode != http.StatusOK {
		slog.Error("YooKassa API error", "status", resp.StatusCode, "body", string(respBody))
		return nil, fmt.Errorf("YooKassa returned %d: %w", resp.StatusCode, apperr.ErrInternal)
	}

	var ykResp yooKassaPaymentResponse
	if err := json.Unmarshal(respBody, &ykResp); err != nil {
		return nil, apperr.WrapInternal("parse YooKassa response", err)
	}

	if ykResp.Confirmation == nil || ykResp.Confirmation.URL == "" {
		return nil, fmt.Errorf("no confirmation URL in response: %w", apperr.ErrInternal)
	}

	// Save payment record
	payment := &models.Payment{
		ID:            paymentID,
		UserID:        userID,
		YooKassaID:    ykResp.ID,
		Status:        models.PaymentPending,
		AmountKopecks: totalKopecks,
		Description:   description,
		AnalysesCount: analysesCount,
	}
	if err := s.repo.CreatePayment(ctx, payment); err != nil {
		return nil, apperr.WrapInternal("save payment record", err)
	}

	slog.Info("payment created", "payment_id", paymentID, "yookassa_id", ykResp.ID, "user_id", userID, "amount", amountRub)

	return &PaymentResult{
		PaymentID:       paymentID,
		ConfirmationURL: ykResp.Confirmation.URL,
		AmountRub:       amountRub,
	}, nil
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
			slog.Error("failed to confirm payment", "yookassa_id", yookassaID, "error", err)
			return apperr.WrapInternal("confirm payment", err)
		}
		slog.Info("payment confirmed", "yookassa_id", yookassaID)

	case "payment.canceled":
		if err := s.repo.CancelPayment(ctx, yookassaID); err != nil {
			slog.Error("failed to cancel payment", "yookassa_id", yookassaID, "error", err)
			return apperr.WrapInternal("cancel payment", err)
		}
		slog.Info("payment canceled", "yookassa_id", yookassaID)

	default:
		slog.Debug("ignoring webhook event", "event", webhook.Event)
	}

	return nil
}

func (s *paymentService) GetQuotaInfo(ctx context.Context, userID uuid.UUID) (*QuotaInfo, error) {
	usedToday, err := s.repo.GetDailyUsage(ctx, userID)
	if err != nil {
		slog.Warn("failed to get daily usage", "user_id", userID, "error", err)
		usedToday = 0
	}

	paidRemaining, err := s.repo.GetPaidAnalysesRemaining(ctx, userID)
	if err != nil {
		slog.Warn("failed to get paid analyses", "user_id", userID, "error", err)
		paidRemaining = 0
	}

	freeRemaining := s.freeLimit - usedToday
	if freeRemaining < 0 {
		freeRemaining = 0
	}

	needsPayment := freeRemaining == 0 && paidRemaining == 0

	return &QuotaInfo{
		DailyLimit:              s.freeLimit,
		UsedToday:               usedToday,
		FreeRemaining:           freeRemaining,
		PaidAnalysesRemaining:   paidRemaining,
		NeedsPayment:            needsPayment,
		PricePerAnalysisKopecks: s.cfg.PriceKopecks,
	}, nil
}
