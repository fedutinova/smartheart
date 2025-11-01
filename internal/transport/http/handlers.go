package http

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/fedutinova/smartheart/internal/auth"
	"github.com/fedutinova/smartheart/internal/config"
	"github.com/fedutinova/smartheart/internal/gpt"
	"github.com/fedutinova/smartheart/internal/job"
	"github.com/fedutinova/smartheart/internal/memq"
	"github.com/fedutinova/smartheart/internal/models"
	"github.com/fedutinova/smartheart/internal/redis"
	"github.com/fedutinova/smartheart/internal/repository"
	"github.com/fedutinova/smartheart/internal/storage"
	"github.com/fedutinova/smartheart/internal/validation"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// extractConclusion extracts structured conclusion from GPT response
// Returns the full response if it's already structured with bullet points or numbered list
func extractConclusion(gptResponse string) string {
	response := strings.TrimSpace(gptResponse)

	// Check if response is already structured (starts with number or bullet point)
	if strings.HasPrefix(response, "1.") || strings.HasPrefix(response, "•") ||
		strings.HasPrefix(response, "-") || strings.HasPrefix(response, "*") {
		return response
	}

	// Try to find conclusion section
	markers := []string{
		"### Заключение\n",
		"### Заключение",
		"## Заключение\n",
		"## Заключение",
		"Заключение:\n",
		"Заключение:",
		"Заключение\n",
	}

	for _, marker := range markers {
		idx := strings.Index(response, marker)
		if idx != -1 {
			conclusion := strings.TrimSpace(response[idx+len(marker):])
			// Remove disclaimer at the end if present
			disclaimers := []string{
				"\n\nИнтерпретация носит информационный характер",
				"\nИнтерпретация носит информационный характер",
				"\n\nThis is for informational purposes",
			}
			for _, disclaimer := range disclaimers {
				if idx := strings.Index(conclusion, disclaimer); idx != -1 {
					conclusion = strings.TrimSpace(conclusion[:idx])
				}
			}
			return strings.TrimSpace(conclusion)
		}
	}

	// If no marker found, return full response (it should already be structured)
	return response
}

type Handlers struct {
	Q       memq.JobQueue
	Repo    *repository.Repository
	Storage storage.Storage
	Redis   *redis.Service
	Config  config.Config
}

func (h *Handlers) Routers(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Post("/v1/auth/register", h.register)
		r.Post("/v1/auth/login", h.login)
		r.Post("/v1/auth/refresh", h.refresh)
	})

	// for static file serving for local storage
	if h.Config.StorageMode == "local" || h.Config.StorageMode == "filesystem" {
		r.Get("/files/*", h.serveFiles)
	}

	r.Group(func(r chi.Router) {
		r.Use(auth.JWTMiddleware(h.Config.JWTSecret, h.Config.JWTIssuer))

		r.Post("/v1/auth/logout", h.logout)

		r.With(auth.RequirePerm(auth.PermEKGSubmit)).Post("/v1/ekg/analyze", h.submitAnalyze)
		r.With(auth.RequirePerm(auth.PermEKGSubmit)).Post("/v1/gpt/process", h.submitGPTRequest)

		r.With(auth.RequirePerm(auth.PermJobReadOwn)).Get("/v1/jobs/{id}", h.getJob)
		r.With(auth.RequirePerm(auth.PermJobReadOwn)).Get("/v1/requests/{id}", h.getRequest)
		r.With(auth.RequirePerm(auth.PermJobReadOwn)).Get("/v1/requests", h.getUserRequests)
	})
}

func (h *Handlers) register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Username == "" || req.Email == "" || req.Password == "" {
		http.Error(w, "username, email, and password are required", http.StatusBadRequest)
		return
	}

	if !strings.Contains(req.Email, "@") {
		http.Error(w, "invalid email format", http.StatusBadRequest)
		return
	}

	if len(req.Password) < 6 {
		http.Error(w, "password must be at least 6 characters", http.StatusBadRequest)
		return
	}

	passwordHash, err := h.Repo.HashPassword(req.Password)
	if err != nil {
		slog.Error("failed to hash password", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	user := &models.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: passwordHash,
	}

	if err := h.Repo.CreateUser(r.Context(), user); err != nil {
		if strings.Contains(err.Error(), "unique constraint") || strings.Contains(err.Error(), "duplicate key") {
			http.Error(w, "username or email already exists", http.StatusConflict)
			return
		}
		slog.Error("failed to create user", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := h.Repo.AssignRoleToUser(r.Context(), user.ID, "user"); err != nil {
		slog.Error("failed to assign role to user", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"message": "user registered successfully",
		"user_id": user.ID,
	})
}

func (h *Handlers) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Email == "" || req.Password == "" {
		http.Error(w, "email and password are required", http.StatusBadRequest)
		return
	}

	user, err := h.Repo.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		slog.Warn("login attempt with invalid email", "email", req.Email)
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	if !h.Repo.CheckPassword(req.Password, user.PasswordHash) {
		slog.Warn("login attempt with invalid password", "email", req.Email)
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	roleNames := make([]string, len(user.Roles))
	for i, role := range user.Roles {
		roleNames[i] = role.Name
	}

	tokens, err := auth.NewTokenPair(
		h.Config.JWTSecret,
		h.Config.JWTIssuer,
		user.ID,
		roleNames,
		h.Config.JWTTTLAccess,
		h.Config.JWTTTLRefresh,
	)
	if err != nil {
		slog.Error("failed to create token pair", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	tokenHash := h.Repo.HashRefreshToken(tokens.RefreshToken)

	if err := h.Redis.StoreRefreshToken(r.Context(), user.ID.String(), tokenHash, h.Config.JWTTTLRefresh); err != nil {
		slog.Error("failed to store refresh token", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	refreshToken := &models.RefreshToken{
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(h.Config.JWTTTLRefresh),
	}

	if err := h.Repo.CreateRefreshToken(r.Context(), refreshToken); err != nil {
		slog.Error("failed to create refresh token record", "error", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tokens)
}

func (h *Handlers) refresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.RefreshToken == "" {
		http.Error(w, "refresh_token is required", http.StatusBadRequest)
		return
	}

	tokenHash := h.Repo.HashRefreshToken(req.RefreshToken)

	userID, err := h.Redis.GetRefreshTokenUserID(r.Context(), tokenHash)
	if err != nil {
		http.Error(w, "invalid refresh token", http.StatusUnauthorized)
		return
	}

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		slog.Error("invalid user ID from refresh token", "user_id", userID)
		http.Error(w, "invalid refresh token", http.StatusUnauthorized)
		return
	}

	user, err := h.Repo.GetUserByID(r.Context(), userUUID)
	if err != nil {
		slog.Error("failed to get user", "error", err)
		http.Error(w, "invalid refresh token", http.StatusUnauthorized)
		return
	}

	roleNames := make([]string, len(user.Roles))
	for i, role := range user.Roles {
		roleNames[i] = role.Name
	}

	tokens, err := auth.NewTokenPair(
		h.Config.JWTSecret,
		h.Config.JWTIssuer,
		user.ID,
		roleNames,
		h.Config.JWTTTLAccess,
		h.Config.JWTTTLRefresh,
	)
	if err != nil {
		slog.Error("failed to create token pair", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := h.Redis.RevokeRefreshToken(r.Context(), tokenHash); err != nil {
		slog.Error("failed to revoke old refresh token", "error", err)
	}

	newTokenHash := h.Repo.HashRefreshToken(tokens.RefreshToken)
	if err := h.Redis.StoreRefreshToken(r.Context(), user.ID.String(), newTokenHash, h.Config.JWTTTLRefresh); err != nil {
		slog.Error("failed to store new refresh token", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	newRefreshToken := &models.RefreshToken{
		UserID:    user.ID,
		TokenHash: newTokenHash,
		ExpiresAt: time.Now().Add(h.Config.JWTTTLRefresh),
	}

	if err := h.Repo.CreateRefreshToken(r.Context(), newRefreshToken); err != nil {
		slog.Error("failed to create new refresh token record", "error", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tokens)
}

func (h *Handlers) logout(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.RefreshToken != "" {
		tokenHash := h.Repo.HashRefreshToken(req.RefreshToken)
		if err := h.Redis.RevokeRefreshToken(r.Context(), tokenHash); err != nil {
			slog.Error("failed to revoke refresh token", "error", err)
		}
		if err := h.Repo.RevokeRefreshToken(r.Context(), tokenHash); err != nil {
			slog.Error("failed to revoke refresh token in db", "error", err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "logged out successfully"})
}

func (h *Handlers) serveFiles(w http.ResponseWriter, r *http.Request) {
	filePath := strings.TrimPrefix(r.URL.Path, "/files/")
	if filePath == "" {
		http.Error(w, "file path required", http.StatusBadRequest)
		return
	}

	if strings.Contains(filePath, "..") {
		http.Error(w, "invalid file path", http.StatusBadRequest)
		return
	}

	fullPath := filepath.Join(h.Config.LocalStorageDir, filePath)
	http.ServeFile(w, r, fullPath)
}

func (h *Handlers) submitAnalyze(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ImageTempURL string `json:"image_temp_url"`
		Notes        string `json:"notes,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ImageTempURL == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// Get user ID from JWT context
	var userID string
	if claims, ok := auth.FromContext(r.Context()); ok {
		userID = claims.UserID
	}

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		http.Error(w, "invalid user ID", http.StatusBadRequest)
		return
	}

	// Create request record BEFORE enqueueing job so we can return request_id immediately
	requestID := uuid.New()
	request := &models.Request{
		ID:     requestID,
		UserID: userUUID,
		Status: models.StatusPending,
	}
	if req.Notes != "" {
		request.TextQuery = &req.Notes
	}

	if err := h.Repo.CreateRequest(r.Context(), request); err != nil {
		slog.Error("failed to create request", "error", err)
		http.Error(w, "failed to create request", http.StatusInternalServerError)
		return
	}

	// Create EKG job payload with user ID and request ID
	ekgPayload := map[string]interface{}{
		"image_temp_url": req.ImageTempURL,
		"notes":          req.Notes,
		"user_id":        userID,
		"request_id":     requestID.String(),
	}

	payload, err := json.Marshal(ekgPayload)
	if err != nil {
		slog.Error("failed to marshal EKG payload", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	j := &job.Job{
		Type:    job.TypeEKGAnalyze,
		Payload: payload,
	}
	id, err := h.Q.Enqueue(r.Context(), j)
	if err != nil {
		slog.Error("failed to enqueue EKG job", "error", err)
		http.Error(w, "enqueue failed", http.StatusServiceUnavailable)
		return
	}

	slog.Info("EKG analysis job enqueued",
		"job_id", id,
		"request_id", requestID,
		"user_id", userID,
		"image_url", req.ImageTempURL)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"job_id":     id.String(),
		"request_id": requestID.String(),
		"status":     string(j.Status),
		"message":    "EKG analysis job submitted successfully",
	})
}

func (h *Handlers) submitGPTRequest(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	textQuery := r.FormValue("text_query")
	files := r.MultipartForm.File["files"]

	if validationErrs := validation.ValidateGPTRequest(textQuery, files); len(validationErrs) > 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"error":   "validation failed",
			"details": validationErrs,
		})
		return
	}

	var userID uuid.UUID
	if claims, ok := auth.FromContext(r.Context()); ok {
		var err error
		userID, err = uuid.Parse(claims.UserID)
		if err != nil {
			http.Error(w, "invalid user ID", http.StatusBadRequest)
			return
		}
	}

	request := &models.Request{
		ID:     uuid.New(),
		UserID: userID,
		Status: models.StatusPending,
	}
	if textQuery != "" {
		request.TextQuery = &textQuery
	}

	if err := h.Repo.CreateRequest(r.Context(), request); err != nil {
		slog.Error("failed to create request", "error", err)
		http.Error(w, "failed to create request", http.StatusInternalServerError)
		return
	}

	var fileKeys []string
	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			slog.Error("failed to open file", "filename", fileHeader.Filename, "error", err)
			continue
		}
		defer file.Close()

		contentType := fileHeader.Header.Get("Content-Type")
		if contentType == "" {
			contentType = http.DetectContentType([]byte(fileHeader.Filename))
		}

		uploadResult, err := h.Storage.UploadFile(r.Context(), fileHeader.Filename, file, contentType)
		if err != nil {
			slog.Error("failed to upload file", "filename", fileHeader.Filename, "error", err)
			continue
		}

		fileModel := &models.File{
			ID:               uuid.New(),
			RequestID:        request.ID,
			OriginalFilename: fileHeader.Filename,
			FileType:         contentType,
			FileSize:         fileHeader.Size,
			S3Key:            uploadResult.Key,
			S3URL:            uploadResult.URL,
		}

		if err := h.Repo.CreateFile(r.Context(), fileModel); err != nil {
			slog.Error("failed to create file record", "filename", fileHeader.Filename, "error", err)
			continue
		}

		fileKeys = append(fileKeys, uploadResult.Key)
	}

	if len(fileKeys) == 0 {
		http.Error(w, "no files successfully processed", http.StatusBadRequest)
		return
	}

	payload := gpt.GPTJobPayload{
		RequestID: request.ID,
		TextQuery: textQuery,
		FileKeys:  fileKeys,
		UserID:    userID.String(),
	}
	payloadBytes, _ := json.Marshal(payload)

	j := &job.Job{
		Type:    job.TypeGPTProcess,
		Payload: payloadBytes,
	}

	jobID, err := h.Q.Enqueue(r.Context(), j)
	if err != nil {
		slog.Error("failed to enqueue job", "error", err)
		http.Error(w, "failed to enqueue job", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"request_id":      request.ID,
		"job_id":          jobID,
		"status":          request.Status,
		"files_processed": len(fileKeys),
	})
}

func (h *Handlers) getUserRequests(w http.ResponseWriter, r *http.Request) {
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

func (h *Handlers) getRequest(w http.ResponseWriter, r *http.Request) {
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
		if err.Error() == "no rows in result set" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		slog.Error("failed to get request", "id", id, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	perms := auth.PermsForRoles(claims.Roles)

	if _, hasAdminPerm := perms[auth.PermAdminAll]; !hasAdminPerm {
		if _, hasReadAllPerm := perms[auth.PermJobReadAll]; !hasReadAllPerm {
			userID, err := uuid.Parse(claims.UserID)
			if err != nil || request.UserID != userID {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
		}
	}

	// Check if this is an EKG request with GPT interpretation
	if request.Response != nil && request.Response.Model == "ekg_preprocessor_v1" {
		// Parse EKG response to find gpt_request_id
		var ekgResponseData map[string]interface{}
		if err := json.Unmarshal([]byte(request.Response.Content), &ekgResponseData); err == nil {
			if gptRequestIDStr, ok := ekgResponseData["gpt_request_id"].(string); ok && gptRequestIDStr != "" {
				gptRequestID, err := uuid.Parse(gptRequestIDStr)
				if err == nil {
					// Get GPT request and its response
					gptRequest, err := h.Repo.GetRequestByID(r.Context(), gptRequestID)
					if err == nil {
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

						if hasAccess {
							// Update status and add GPT interpretation to EKG response
							ekgResponseData["gpt_interpretation_status"] = gptRequest.Status
							if gptRequest.Status == "completed" && gptRequest.Response != nil {
								// Extract only the conclusion section from GPT response
								gptContent := gptRequest.Response.Content
								conclusion := extractConclusion(gptContent)
								ekgResponseData["gpt_interpretation"] = conclusion
								// Also keep full interpretation for reference
								ekgResponseData["gpt_full_response"] = gptContent
							} else if gptRequest.Status == "failed" {
								ekgResponseData["gpt_interpretation"] = "GPT analysis failed"
							} else {
								ekgResponseData["gpt_interpretation"] = nil
							}

							// Update the response content with GPT data
							if updatedContent, err := json.Marshal(ekgResponseData); err == nil {
								request.Response.Content = string(updatedContent)
							}
						}
					}
				}
			}
		}
	}

	// Parse response content for EKG requests to make it more accessible
	if request.Response != nil && request.Response.Model == "ekg_preprocessor_v1" {
		var contentData map[string]interface{}
		if err := json.Unmarshal([]byte(request.Response.Content), &contentData); err == nil {
			// Create response with parsed content
			responseWithParsed := map[string]interface{}{
				"id":                 request.Response.ID,
				"request_id":         request.Response.RequestID,
				"content":            request.Response.Content, // Keep original string for compatibility
				"content_parsed":     contentData,              // Parsed object for easy access
				"model":              request.Response.Model,
				"tokens_used":        request.Response.TokensUsed,
				"processing_time_ms": request.Response.ProcessingTimeMs,
				"created_at":         request.Response.CreatedAt,
			}
			request.Response = nil // Clear original to replace with custom structure
			requestCustom := map[string]interface{}{
				"id":         request.ID,
				"user_id":    request.UserID,
				"text_query": request.TextQuery,
				"status":     request.Status,
				"created_at": request.CreatedAt,
				"updated_at": request.UpdatedAt,
				"files":      request.Files,
				"response":   responseWithParsed,
			}

			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(requestCustom); err != nil {
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

func (h *Handlers) getJob(w http.ResponseWriter, r *http.Request) {
	raw := chi.URLParam(r, "id")
	id, err := uuid.Parse(raw)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	j, ok := h.Q.Status(r.Context(), id)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(j); err != nil {
		slog.Warn("encode job", "err", err)
	}
}
