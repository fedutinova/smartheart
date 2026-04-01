package handler

import (
	"net/http"
	"strconv"

	"github.com/fedutinova/smartheart/back-api/repository"
)

// AdminHandler handles admin dashboard endpoints.
type AdminHandler struct {
	Repo repository.Store
}

func adminPagination(r *http.Request) (limit, offset int) {
	limit = 20
	offset = 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	return limit, offset
}

// GetStats returns aggregate dashboard statistics.
func (h *AdminHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.Repo.GetAdminStats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load stats")
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// ListUsers returns a paginated list of users.
func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	limit, offset := adminPagination(r)
	search := r.URL.Query().Get("search")

	users, total, err := h.Repo.ListUsers(r.Context(), limit, offset, search)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load users")
		return
	}

	writeJSON(w, http.StatusOK, PaginatedResponse{
		Data:   users,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

// ListPayments returns a paginated list of all payments.
func (h *AdminHandler) ListPayments(w http.ResponseWriter, r *http.Request) {
	limit, offset := adminPagination(r)

	payments, total, err := h.Repo.ListPayments(r.Context(), limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load payments")
		return
	}

	writeJSON(w, http.StatusOK, PaginatedResponse{
		Data:   payments,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

// ListFeedback returns a paginated list of RAG feedback.
func (h *AdminHandler) ListFeedback(w http.ResponseWriter, r *http.Request) {
	limit, offset := adminPagination(r)

	feedback, total, err := h.Repo.ListRAGFeedback(r.Context(), limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load feedback")
		return
	}

	writeJSON(w, http.StatusOK, PaginatedResponse{
		Data:   feedback,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}
