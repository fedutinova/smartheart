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

type createPaymentRequest struct {
	AnalysesCount int `json:"analyses_count" validate:"required,gte=1,lte=100"`
}

// CreatePayment creates a YooKassa payment and returns the redirect URL.
func (h *PaymentHandler) CreatePayment(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	var req createPaymentRequest
	if !decodeAndValidate(w, r, &req) {
		return
	}

	userID, _, ok := extractUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "no auth context")
		return
	}

	result, err := h.Service.CreatePayment(r.Context(), userID, req.AnalysesCount)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// Webhook handles YooKassa payment notifications.
// This endpoint is public (called by YooKassa servers).
func (h *PaymentHandler) Webhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read body")
		return
	}

	if err := h.Service.HandleWebhook(r.Context(), body); err != nil {
		handleServiceError(w, err)
		return
	}

	// YooKassa expects 200 OK to acknowledge webhook
	w.WriteHeader(http.StatusOK)
}

// CreateSubscription creates a YooKassa payment for a monthly subscription.
func (h *PaymentHandler) CreateSubscription(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := extractUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "no auth context")
		return
	}

	result, err := h.Service.CreateSubscription(r.Context(), userID)
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
