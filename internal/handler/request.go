package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/fedutinova/smartheart/internal/auth"
	"github.com/fedutinova/smartheart/internal/common"
	"github.com/fedutinova/smartheart/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// GetUserRequests returns all requests for the authenticated user
func (h *Handlers) GetUserRequests(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		http.Error(w, "no auth context", http.StatusUnauthorized)
		return
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		http.Error(w, "invalid user ID", http.StatusBadRequest)
		return
	}

	requests, err := h.Repo.GetRequestsByUserID(r.Context(), userID)
	if err != nil {
		slog.Error("failed to get user requests", "user_id", userID, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(requests); err != nil {
		slog.Warn("encode requests", "err", err)
	}
}

// GetRequest returns a specific request by ID
func (h *Handlers) GetRequest(w http.ResponseWriter, r *http.Request) {
	raw := chi.URLParam(r, "id")
	id, err := uuid.Parse(raw)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}

	claims, ok := auth.FromContext(r.Context())
	if !ok {
		http.Error(w, "no auth context", http.StatusUnauthorized)
		return
	}

	request, err := h.Repo.GetRequestByID(r.Context(), id)
	if err != nil {
		if common.IsNotFound(err) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		slog.Error("failed to get request", "id", id, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	perms := auth.PermsForRoles(claims.Roles)

	// Check access permissions
	if _, hasAdminPerm := perms[auth.PermAdminAll]; !hasAdminPerm {
		if _, hasReadAllPerm := perms[auth.PermJobReadAll]; !hasReadAllPerm {
			userID, err := uuid.Parse(claims.UserID)
			if err != nil || request.UserID != userID {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
		}
	}

	// Enrich and parse EKG responses
	if request.Response != nil && request.Response.Model == "ekg_preprocessor_v1" {
		h.enrichEKGResponse(r, request, claims, perms)

		if parsed, err := request.Response.ParseContent(); err == nil {
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(request.WithParsedResponse(parsed)); err != nil {
				slog.Warn("encode request", "err", err)
			}
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(request); err != nil {
		slog.Warn("encode request", "err", err)
	}
}

// enrichEKGResponse adds GPT interpretation to EKG response
func (h *Handlers) enrichEKGResponse(r *http.Request, request *models.Request, claims *auth.Claims, perms map[string]struct{}) {
	var ekgResponseData map[string]any
	if err := json.Unmarshal([]byte(request.Response.Content), &ekgResponseData); err != nil {
		return
	}

	gptRequestIDStr, ok := ekgResponseData["gpt_request_id"].(string)
	if !ok || gptRequestIDStr == "" {
		return
	}

	gptRequestID, err := uuid.Parse(gptRequestIDStr)
	if err != nil {
		return
	}

	gptRequest, err := h.Repo.GetRequestByID(r.Context(), gptRequestID)
	if err != nil {
		return
	}

	// Check permissions for GPT request
	hasAccess := false
	if _, hasAdminPerm := perms[auth.PermAdminAll]; hasAdminPerm {
		hasAccess = true
	} else if _, hasReadAllPerm := perms[auth.PermJobReadAll]; hasReadAllPerm {
		hasAccess = true
	} else {
		userID, _ := uuid.Parse(claims.UserID)
		hasAccess = (gptRequest.UserID == userID)
	}

	if !hasAccess {
		return
	}

	ekgResponseData["gpt_interpretation_status"] = gptRequest.Status
	if gptRequest.Status == "completed" && gptRequest.Response != nil {
		gptContent := gptRequest.Response.Content
		conclusion := extractConclusion(gptContent)
		ekgResponseData["gpt_interpretation"] = conclusion
		ekgResponseData["gpt_full_response"] = gptContent
	} else if gptRequest.Status == "failed" {
		ekgResponseData["gpt_interpretation"] = "GPT analysis failed"
	} else {
		ekgResponseData["gpt_interpretation"] = nil
	}

	if updatedContent, err := json.Marshal(ekgResponseData); err == nil {
		request.Response.Content = string(updatedContent)
	}
}

// GetJob returns the status of a job by ID
func (h *Handlers) GetJob(w http.ResponseWriter, r *http.Request) {
	raw := chi.URLParam(r, "id")
	id, err := uuid.Parse(raw)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}

	claims, ok := auth.FromContext(r.Context())
	if !ok {
		http.Error(w, "no auth context", http.StatusUnauthorized)
		return
	}

	j, ok := h.Q.Status(r.Context(), id)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	// Check ownership unless admin or read-all
	perms := auth.PermsForRoles(claims.Roles)
	if _, hasAdmin := perms[auth.PermAdminAll]; !hasAdmin {
		if _, hasReadAll := perms[auth.PermJobReadAll]; !hasReadAll {
			var payload struct {
				UserID string `json:"user_id"`
			}
			if err := json.Unmarshal(j.Payload, &payload); err == nil && payload.UserID != claims.UserID {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(j); err != nil {
		slog.Warn("encode job", "err", err)
	}
}

// ServeFiles serves static files from local storage
func (h *Handlers) ServeFiles(w http.ResponseWriter, r *http.Request) {
	filePath := strings.TrimPrefix(r.URL.Path, "/files/")
	if filePath == "" {
		http.Error(w, "file path required", http.StatusBadRequest)
		return
	}

	baseDir := filepath.Clean(h.Config.LocalStorageDir)
	fullPath := filepath.Join(baseDir, filepath.Clean("/"+filePath))
	if !strings.HasPrefix(fullPath, baseDir+string(os.PathSeparator)) {
		http.Error(w, "invalid file path", http.StatusBadRequest)
		return
	}

	http.ServeFile(w, r, fullPath)
}

