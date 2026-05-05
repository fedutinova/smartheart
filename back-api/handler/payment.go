package handler

import (
	"io"
	"net/http"

	"github.com/fedutinova/smartheart/back-api/service"
)

// PaymentHandler handles payment and quota endpoints.
type PaymentHandler struct {
	Service service.PaymentService
}

type applyPromoCodeRequest struct {
	Code string `json:"code" validate:"required,min=1,max=50"`
}

// Webhook handles YooKassa payment notifications.
// This endpoint is public (called by YooKassa servers) and requires signature verification.
func (h *PaymentHandler) Webhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read body")
		return
	}

	signature := r.Header.Get("X-Webhook-Signature")
	if signature == "" {
		writeError(w, http.StatusUnauthorized, "missing webhook signature")
		return
	}

	if err := h.Service.HandleWebhook(r.Context(), body, signature); err != nil {
		handleServiceError(w, err)
		return
	}

	// YooKassa expects 200 OK to acknowledge webhook
	w.WriteHeader(http.StatusOK)
}

type createSubscriptionRequest struct {
	PromoCode string `json:"promo_code"`
}

// CreateSubscription creates a payment for a monthly subscription.
// If promo_code gives 100% discount, the subscription is activated directly.
func (h *PaymentHandler) CreateSubscription(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := extractUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "no auth context")
		return
	}

	var req createSubscriptionRequest
	if r.ContentLength > 0 {
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}

	result, err := h.Service.CreateSubscription(r.Context(), userID, req.PromoCode)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetQuota returns the user's current quota information.
func (h *PaymentHandler) GetQuota(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := extractUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "no auth context")
		return
	}

	info, err := h.Service.GetQuotaInfo(r.Context(), userID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, info)
}

// ApplyPromoCode validates and returns discount info for a promo code.
func (h *PaymentHandler) ApplyPromoCode(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	var req applyPromoCodeRequest
	if !decodeAndValidate(w, r, &req) {
		return
	}

	userID, _, ok := extractUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "no auth context")
		return
	}

	discount, err := h.Service.ValidatePromoCode(r.Context(), userID, req.Code)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, discount)
}
