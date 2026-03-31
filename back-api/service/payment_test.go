package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fedutinova/smartheart/back-api/apperr"
	"github.com/fedutinova/smartheart/back-api/config"
	repomocks "github.com/fedutinova/smartheart/back-api/repository/mocks"
)

func newPaymentService(t *testing.T) (*paymentService, *repomocks.MockStore) {
	repo := repomocks.NewMockStore(t)
	svc := NewPaymentService(repo, config.YooKassaConfig{
		ShopID:                   "test-shop",
		SecretKey:                "test-secret",
		ReturnURL:                "https://example.com/return",
		PriceKopecks:             9900,
		SubscriptionPriceKopecks: 29900,
	}, 3).(*paymentService)
	return svc, repo
}

// --- GetQuotaInfo ---

func TestGetQuotaInfo_NoSubscription(t *testing.T) {
	svc, repo := newPaymentService(t)
	ctx := context.Background()
	userID := uuid.New()

	repo.EXPECT().GetDailyUsage(mock.Anything, userID).Return(2, nil)
	repo.EXPECT().GetPaidAnalysesRemaining(mock.Anything, userID).Return(5, nil)
	repo.EXPECT().GetSubscriptionExpiresAt(mock.Anything, userID).Return(nil, nil)

	info, err := svc.GetQuotaInfo(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, 3, info.DailyLimit)
	assert.Equal(t, 2, info.UsedToday)
	assert.Equal(t, 1, info.FreeRemaining)
	assert.Equal(t, 5, info.PaidAnalysesRemaining)
	assert.False(t, info.NeedsPayment)
	assert.Nil(t, info.SubscriptionExpiresAt)
	assert.Equal(t, 29900, info.SubscriptionPriceKopecks)
}

func TestGetQuotaInfo_ActiveSubscription(t *testing.T) {
	svc, repo := newPaymentService(t)
	ctx := context.Background()
	userID := uuid.New()
	expires := time.Now().Add(15 * 24 * time.Hour)

	repo.EXPECT().GetDailyUsage(mock.Anything, userID).Return(10, nil)
	repo.EXPECT().GetPaidAnalysesRemaining(mock.Anything, userID).Return(0, nil)
	repo.EXPECT().GetSubscriptionExpiresAt(mock.Anything, userID).Return(&expires, nil)

	info, err := svc.GetQuotaInfo(ctx, userID)
	require.NoError(t, err)
	assert.False(t, info.NeedsPayment)
	assert.NotNil(t, info.SubscriptionExpiresAt)
}

func TestGetQuotaInfo_ExpiredSubscription_NeedPayment(t *testing.T) {
	svc, repo := newPaymentService(t)
	ctx := context.Background()
	userID := uuid.New()
	expired := time.Now().Add(-24 * time.Hour)

	repo.EXPECT().GetDailyUsage(mock.Anything, userID).Return(5, nil)
	repo.EXPECT().GetPaidAnalysesRemaining(mock.Anything, userID).Return(0, nil)
	repo.EXPECT().GetSubscriptionExpiresAt(mock.Anything, userID).Return(&expired, nil)

	info, err := svc.GetQuotaInfo(ctx, userID)
	require.NoError(t, err)
	assert.True(t, info.NeedsPayment)
	assert.Equal(t, 0, info.FreeRemaining)
}

// --- HandleWebhook ---

func TestHandleWebhook_PaymentSucceeded(t *testing.T) {
	svc, repo := newPaymentService(t)
	ctx := context.Background()

	repo.EXPECT().ConfirmPayment(mock.Anything, "yk-123").Return(nil)

	body, _ := json.Marshal(map[string]any{
		"event":  "payment.succeeded",
		"object": map[string]string{"id": "yk-123", "status": "succeeded"},
	})

	err := svc.HandleWebhook(ctx, body)
	require.NoError(t, err)
}

func TestHandleWebhook_PaymentCanceled(t *testing.T) {
	svc, repo := newPaymentService(t)
	ctx := context.Background()

	repo.EXPECT().CancelPayment(mock.Anything, "yk-456").Return(nil)

	body, _ := json.Marshal(map[string]any{
		"event":  "payment.canceled",
		"object": map[string]string{"id": "yk-456", "status": "canceled"},
	})

	err := svc.HandleWebhook(ctx, body)
	require.NoError(t, err)
}

func TestHandleWebhook_InvalidJSON(t *testing.T) {
	svc, _ := newPaymentService(t)

	err := svc.HandleWebhook(context.Background(), []byte("not json"))
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.ErrValidation)
}

func TestHandleWebhook_EmptyPaymentID(t *testing.T) {
	svc, _ := newPaymentService(t)

	body, _ := json.Marshal(map[string]any{
		"event":  "payment.succeeded",
		"object": map[string]string{"id": "", "status": "succeeded"},
	})

	err := svc.HandleWebhook(context.Background(), body)
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.ErrValidation)
}

func TestHandleWebhook_UnknownEvent_Ignored(t *testing.T) {
	svc, _ := newPaymentService(t)

	body, _ := json.Marshal(map[string]any{
		"event":  "refund.succeeded",
		"object": map[string]string{"id": "yk-789", "status": "succeeded"},
	})

	err := svc.HandleWebhook(context.Background(), body)
	require.NoError(t, err)
}

// --- CreateSubscription ---

func TestCreateSubscription_RejectsActiveSubscription(t *testing.T) {
	svc, repo := newPaymentService(t)
	ctx := context.Background()
	userID := uuid.New()
	expires := time.Now().Add(15 * 24 * time.Hour)

	repo.EXPECT().GetSubscriptionExpiresAt(mock.Anything, userID).Return(&expires, nil)

	_, err := svc.CreateSubscription(ctx, userID)
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.ErrConflict)
}

func TestCreateSubscription_AllowsExpiredSubscription(t *testing.T) {
	svc, repo := newPaymentService(t)
	ctx := context.Background()
	userID := uuid.New()
	expired := time.Now().Add(-24 * time.Hour)

	repo.EXPECT().GetSubscriptionExpiresAt(mock.Anything, userID).Return(&expired, nil)

	// Will fail at the HTTP call to YooKassa (no real server), but passes the subscription check.
	_, err := svc.CreateSubscription(ctx, userID)
	require.Error(t, err)
	assert.NotErrorIs(t, err, apperr.ErrConflict)
}

// --- checkQuota (via submission service) ---

func TestCheckQuota_ActiveSubscription_Unlimited(t *testing.T) {
	repo := repomocks.NewMockStore(t)
	svc := &submissionService{repo: repo, dailyLimit: 3}
	ctx := context.Background()
	userID := uuid.New()
	expires := time.Now().Add(15 * 24 * time.Hour)

	repo.EXPECT().IncrementDailyUsage(mock.Anything, userID).Return(10, nil)
	repo.EXPECT().GetSubscriptionExpiresAt(mock.Anything, userID).Return(&expires, nil)

	err := svc.checkQuota(ctx, userID)
	require.NoError(t, err)
}

func TestCheckQuota_ExpiredSubscription_FallsToFreeQuota(t *testing.T) {
	repo := repomocks.NewMockStore(t)
	svc := &submissionService{repo: repo, dailyLimit: 3}
	ctx := context.Background()
	userID := uuid.New()
	expired := time.Now().Add(-24 * time.Hour)

	repo.EXPECT().GetSubscriptionExpiresAt(mock.Anything, userID).Return(&expired, nil)
	repo.EXPECT().IncrementDailyUsage(mock.Anything, userID).Return(1, nil)

	err := svc.checkQuota(ctx, userID)
	require.NoError(t, err)
}

func TestCheckQuota_NoSubscription_QuotaExceeded_NoPaid(t *testing.T) {
	repo := repomocks.NewMockStore(t)
	svc := &submissionService{repo: repo, dailyLimit: 3}
	ctx := context.Background()
	userID := uuid.New()

	repo.EXPECT().GetSubscriptionExpiresAt(mock.Anything, userID).Return(nil, nil)
	repo.EXPECT().IncrementDailyUsage(mock.Anything, userID).Return(4, nil)
	repo.EXPECT().DecrementPaidAnalyses(mock.Anything, userID).Return(0, assert.AnError)

	err := svc.checkQuota(ctx, userID)
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.ErrPaymentRequired)
}
