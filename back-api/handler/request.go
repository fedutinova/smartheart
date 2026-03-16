package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
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

	page, err := h.Service.GetUserRequests(r.Context(), userID, limit, offset)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, PaginatedResponse{
		Data:   page.Data,
		Total:  page.Total,
		Limit:  page.Limit,
		Offset: page.Offset,
	})
}

// GetRequest returns a specific request by ID
func (h *RequestHandler) GetRequest(w http.ResponseWriter, r *http.Request) {
	raw := chi.URLParam(r, "id")
	id, err := parseUUID(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad id")
		return
	}

	_, claims, ok := extractUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "no auth context")
		return
	}

	request, err := h.Service.GetRequest(r.Context(), id, claims)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, request)
}

// GetJob returns the status of a job by ID
func (h *RequestHandler) GetJob(w http.ResponseWriter, r *http.Request) {
	raw := chi.URLParam(r, "id")
	id, err := parseUUID(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad id")
		return
	}

	_, claims, ok := extractUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "no auth context")
		return
	}

	j, err := h.Service.GetJobStatus(r.Context(), id, claims)
	if err != nil {
		handleServiceError(w, err)
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
