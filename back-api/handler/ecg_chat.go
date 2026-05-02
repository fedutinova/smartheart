package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/fedutinova/smartheart/back-api/service"
)

// ECGChatHandler exposes contextual chat endpoints anchored to a specific ECG.
type ECGChatHandler struct {
	Service service.ECGChatService
}

type ecgChatMessageRequest struct {
	Content string `json:"content" validate:"required,min=1,max=2000"`
}

// GetMessages handles GET /v1/ecg/{id}/chat — returns the chat history for an ECG.
func (h *ECGChatHandler) GetMessages(w http.ResponseWriter, r *http.Request) {
	requestID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request ID")
		return
	}

	userID, _, ok := extractUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "no auth context")
		return
	}

	messages, err := h.Service.GetMessages(r.Context(), requestID, userID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	if messages == nil {
		messages = nil
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": messages})
}

// PostMessage handles POST /v1/ecg/{id}/chat/messages — sends a user question
// and returns the assistant's reply (with citations).
func (h *ECGChatHandler) PostMessage(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	requestID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request ID")
		return
	}

	userID, _, ok := extractUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "no auth context")
		return
	}

	var req ecgChatMessageRequest
	if !decodeAndValidate(w, r, &req) {
		return
	}

	reply, err := h.Service.SendMessage(r.Context(), requestID, userID, req.Content)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, reply)
}
