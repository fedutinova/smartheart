package handler

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

type fileURLResponse struct {
	URL string `json:"url"`
}

// GetUserRequests returns requests for the authenticated user with pagination.
// Query params: ?limit=N&offset=N (defaults: limit=50, offset=0).
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

// GetRequest returns a specific request by ID.
func (h *RequestHandler) GetRequest(w http.ResponseWriter, r *http.Request) {
	raw := chi.URLParam(r, "id")
	id, err := parseUUID(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request ID")
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

	// Fill in missing S3URL from storage config
	for i := range request.Files {
		if request.Files[i].S3URL == "" && request.Files[i].S3Key != "" {
			request.Files[i].S3URL = h.Config.Storage.LocalURL + "/" + request.Files[i].S3Key
		}
	}

	writeJSON(w, http.StatusOK, request)
}

// GetJob returns the status of a job by ID.
func (h *RequestHandler) GetJob(w http.ResponseWriter, r *http.Request) {
	raw := chi.URLParam(r, "id")
	id, err := parseUUID(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request ID")
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

// GetRequestFile serves a file belonging to a request.
// The caller must own the request. The file is streamed from storage.
func (h *RequestHandler) GetRequestFile(w http.ResponseWriter, r *http.Request) {
	s3Key, _, err := h.lookupOwnedRequestFile(r)
	if err != nil {
		writeFileLookupError(w, err)
		return
	}

	rc, contentType, err := h.Storage.GetFile(r.Context(), s3Key)
	if err != nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	defer func() { _ = rc.Close() }()

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	_, _ = io.Copy(w, rc)
}

// GetRequestFileURL returns a direct file URL when the storage backend supports it.
// This avoids proxying image bytes through the API and lets the browser load the file directly.
func (h *RequestHandler) GetRequestFileURL(w http.ResponseWriter, r *http.Request) {
	s3Key, file, err := h.lookupOwnedRequestFile(r)
	if err != nil {
		writeFileLookupError(w, err)
		return
	}

	url, err := h.Storage.GetPresignedURL(r.Context(), s3Key, h.Config.JWT.TTLAccess)
	if err == nil && url != "" {
		writeJSON(w, http.StatusOK, fileURLResponse{URL: url})
		return
	}

	if file.S3URL != "" {
		writeJSON(w, http.StatusOK, fileURLResponse{URL: file.S3URL})
		return
	}

	if (h.Config.Storage.Mode == "local" || h.Config.Storage.Mode == "filesystem") &&
		h.Config.Storage.LocalURL != "" && s3Key != "" {
		writeJSON(w, http.StatusOK, fileURLResponse{
			URL: fmt.Sprintf("%s/%s", strings.TrimRight(h.Config.Storage.LocalURL, "/"), s3Key),
		})
		return
	}

	writeError(w, http.StatusNotImplemented, "direct file url not supported")
}

func (h *RequestHandler) lookupOwnedRequestFile(r *http.Request) (string, fileRef, error) {
	requestIDRaw := chi.URLParam(r, "id")
	requestID, err := parseUUID(requestIDRaw)
	if err != nil {
		return "", fileRef{}, errBadRequest("invalid request ID")
	}

	fileIDRaw := chi.URLParam(r, "fileId")
	fileID, err := parseUUID(fileIDRaw)
	if err != nil {
		return "", fileRef{}, errBadRequest("invalid file ID")
	}

	_, claims, ok := extractUserID(r)
	if !ok {
		return "", fileRef{}, errUnauthorized("no auth context")
	}

	request, err := h.Service.GetRequest(r.Context(), requestID, claims)
	if err != nil {
		return "", fileRef{}, err
	}

	for i := range request.Files {
		if request.Files[i].ID == fileID {
			if request.Files[i].S3Key == "" {
				return "", fileRef{}, errNotFound("file not found")
			}
			return request.Files[i].S3Key, fileRef{
				S3URL: request.Files[i].S3URL,
			}, nil
		}
	}

	return "", fileRef{}, errNotFound("file not found")
}

type fileRef struct {
	S3URL string
}

type requestFileLookupError struct {
	status int
	msg    string
	err    error
}

func (e *requestFileLookupError) Error() string {
	if e.err != nil {
		return e.err.Error()
	}
	return e.msg
}

func errBadRequest(msg string) error {
	return &requestFileLookupError{status: http.StatusBadRequest, msg: msg}
}

func errUnauthorized(msg string) error {
	return &requestFileLookupError{status: http.StatusUnauthorized, msg: msg}
}

func errNotFound(msg string) error {
	return &requestFileLookupError{status: http.StatusNotFound, msg: msg}
}

func writeFileLookupError(w http.ResponseWriter, err error) {
	var lookupErr *requestFileLookupError
	if errors.As(err, &lookupErr) {
		writeError(w, lookupErr.status, lookupErr.msg)
		return
	}
	handleServiceError(w, err)
}

// ServeFiles serves static files from local storage.
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
