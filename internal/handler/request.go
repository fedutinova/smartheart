package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/fedutinova/smartheart/internal/auth"
	"github.com/fedutinova/smartheart/internal/apperr"
	"github.com/fedutinova/smartheart/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// GetUserRequests returns requests for the authenticated user with pagination.
// Query params: ?limit=N&offset=N (defaults: limit=50, offset=0)
func (h *RequestHandler) GetUserRequests(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := extractUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "no auth context")
		return
	}

	limit := 50
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	requests, err := h.Repo.GetRequestsByUserID(r.Context(), userID, limit, offset)
	if err != nil {
		slog.Error("failed to get user requests", "user_id", userID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	total, err := h.Repo.CountRequestsByUserID(r.Context(), userID)
	if err != nil {
		slog.Error("failed to count user requests", "user_id", userID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Ensure JSON serializes as [] instead of null for empty results.
	if requests == nil {
		requests = []models.Request{}
	}

	writeJSON(w, http.StatusOK, PaginatedResponse{
		Data:   requests,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

// GetRequest returns a specific request by ID
func (h *RequestHandler) GetRequest(w http.ResponseWriter, r *http.Request) {
	raw := chi.URLParam(r, "id")
	id, err := uuid.Parse(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad id")
		return
	}

	_, claims, ok := extractUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "no auth context")
		return
	}

	request, err := h.Repo.GetRequestByID(r.Context(), id)
	if err != nil {
		if apperr.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		slog.Error("failed to get request", "id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if !auth.CanAccessResource(claims, request.UserID) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	// Enrich and parse EKG responses
	if request.Response != nil && request.Response.Model == models.EKGModelDirect {
		enrichEKGResponse(r.Context(), h.Repo, request, claims)

		if parsed, err := request.Response.ParseContent(); err == nil {
			writeJSON(w, http.StatusOK, request.WithParsedResponse(parsed))
			return
		}
	}

	writeJSON(w, http.StatusOK, request)
}

// GetJob returns the status of a job by ID
func (h *RequestHandler) GetJob(w http.ResponseWriter, r *http.Request) {
	raw := chi.URLParam(r, "id")
	id, err := uuid.Parse(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad id")
		return
	}

	_, claims, ok := extractUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "no auth context")
		return
	}

	j, ok := h.Queue.Status(r.Context(), id)
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	// Check ownership unless admin or read-all
	var payload struct {
		UserID uuid.UUID `json:"user_id"`
	}
	if err := json.Unmarshal(j.Payload, &payload); err != nil {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	if !auth.CanAccessResource(claims, payload.UserID) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	writeJSON(w, http.StatusOK, j)
}

// ServeFiles serves static files from local storage
func (h *RequestHandler) ServeFiles(w http.ResponseWriter, r *http.Request) {
	filePath := strings.TrimPrefix(r.URL.Path, "/files/")
	if filePath == "" {
		writeError(w, http.StatusBadRequest, "file path required")
		return
	}

	// Resolve baseDir to absolute path to prevent bypass via symlinks
	baseDir, err := filepath.Abs(filepath.Clean(h.Config.Storage.LocalDir))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid storage config")
		return
	}

	// Clean the requested path and join with base
	cleaned := filepath.Clean("/" + filePath)
	fullPath := filepath.Join(baseDir, cleaned)

	// Resolve symlinks before checking prefix
	realPath, err := filepath.EvalSymlinks(fullPath)
	if err != nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}

	// Ensure resolved path is strictly inside base directory (not the directory itself).
	if !strings.HasPrefix(realPath, baseDir+string(os.PathSeparator)) {
		writeError(w, http.StatusBadRequest, "invalid file path")
		return
	}

	http.ServeFile(w, r, realPath)
}
