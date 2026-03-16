package handler

import (
	"context"
	"log/slog"

	"github.com/fedutinova/smartheart/internal/auth"
	"github.com/fedutinova/smartheart/internal/models"
	"github.com/fedutinova/smartheart/internal/repository"
	"github.com/google/uuid"
)

// enrichEKGResponse adds GPT interpretation to an EKG response by looking up
// the linked GPT request. This is a pure function of repo + data, independent
// of any specific handler type.
func enrichEKGResponse(ctx context.Context, repo repository.RequestRepo, request *models.Request, claims *auth.Claims) {
	ekg, err := models.ParseEKGContent(request.Response.Content)
	if err != nil {
		slog.Debug("failed to parse EKG content for enrichment", "request_id", request.ID, "error", err)
		return
	}
	if ekg == nil || ekg.GPTRequestID == "" {
		return
	}

	gptRequestID, err := uuid.Parse(ekg.GPTRequestID)
	if err != nil {
		slog.Warn("invalid GPT request ID in EKG content", "request_id", request.ID, "gpt_request_id", ekg.GPTRequestID, "error", err)
		return
	}

	gptRequest, err := repo.GetRequestByID(ctx, gptRequestID)
	if err != nil {
		slog.Warn("failed to get GPT request for EKG enrichment", "request_id", request.ID, "gpt_request_id", gptRequestID, "error", err)
		return
	}

	if !auth.CanAccessResource(claims, gptRequest.UserID) {
		return
	}

	ekg.GPTInterpretationStatus = gptRequest.Status
	if gptRequest.Status == models.StatusCompleted && gptRequest.Response != nil {
		gptContent := gptRequest.Response.Content
		conclusion := models.ExtractConclusion(gptContent)
		ekg.GPTInterpretation = &conclusion
		ekg.GPTFullResponse = &gptContent
	} else if gptRequest.Status == models.StatusFailed {
		failed := "GPT analysis failed"
		ekg.GPTInterpretation = &failed
	}

	if updatedContent, err := ekg.Marshal(); err == nil {
		request.Response.Content = updatedContent
	}
}
