package service

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"

	"github.com/fedutinova/smartheart/back-api/apperr"
	"github.com/fedutinova/smartheart/back-api/auth"
	"github.com/fedutinova/smartheart/back-api/job"
	"github.com/fedutinova/smartheart/back-api/models"
	"github.com/fedutinova/smartheart/back-api/repository"
)

// RequestPage is a paginated list of requests.
type RequestPage struct {
	Data   []models.Request
	Total  int
	Limit  int
	Offset int
}

// RequestService handles request retrieval and enrichment.
type RequestService interface {
	GetUserRequests(ctx context.Context, userID uuid.UUID, limit, offset int) (*RequestPage, error)
	GetRequest(ctx context.Context, requestID uuid.UUID, claims *auth.Claims) (*models.Request, error)
	GetJobStatus(ctx context.Context, jobID uuid.UUID, claims *auth.Claims) (*job.Job, error)
}

type requestService struct {
	repo  repository.Store
	queue job.Queue
}

func NewRequestService(repo repository.Store, queue job.Queue) RequestService {
	return &requestService{repo: repo, queue: queue}
}

func (s *requestService) GetUserRequests(ctx context.Context, userID uuid.UUID, limit, offset int) (*RequestPage, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset = max(offset, 0)

	requests, err := s.repo.GetRequestsByUserID(ctx, userID, limit, offset)
	if err != nil {
		return nil, apperr.WrapInternal("get user requests", err)
	}

	total, err := s.repo.CountRequestsByUserID(ctx, userID)
	if err != nil {
		return nil, apperr.WrapInternal("count user requests", err)
	}

	if requests == nil {
		requests = []models.Request{}
	}

	return &RequestPage{
		Data:   requests,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}, nil
}

func (s *requestService) GetRequest(ctx context.Context, requestID uuid.UUID, claims *auth.Claims) (*models.Request, error) {
	request, err := s.repo.GetRequestByID(ctx, requestID)
	if err != nil {
		if apperr.IsNotFound(err) {
			return nil, err
		}
		return nil, apperr.WrapInternal("get request", err)
	}

	if !auth.CanAccessResource(claims, request.UserID) {
		return nil, apperr.ErrForbidden
	}

	// Enrich old EKG responses with GPT interpretation (not needed for structured)
	if request.Response != nil && request.Response.Model == models.EKGModelDirect {
		enrichEKGResponse(ctx, s.repo, request, claims)
	}

	return request, nil
}

func (s *requestService) GetJobStatus(ctx context.Context, jobID uuid.UUID, claims *auth.Claims) (*job.Job, error) {
	j, ok := s.queue.Status(ctx, jobID)
	if !ok {
		return nil, apperr.ErrJobNotFound
	}

	var payload struct {
		UserID uuid.UUID `json:"user_id"`
	}
	if err := json.Unmarshal(j.Payload, &payload); err != nil {
		return nil, apperr.ErrForbidden
	}
	if !auth.CanAccessResource(claims, payload.UserID) {
		return nil, apperr.ErrForbidden
	}

	return j, nil
}

// enrichEKGResponse adds GPT interpretation to an EKG response.
// Moved from handler/enrich.go to the service layer.
func enrichEKGResponse(ctx context.Context, repo repository.RequestRepo, request *models.Request, claims *auth.Claims) {
	ekg, err := models.ParseEKGContent(request.Response.Content)
	if err != nil {
		slog.DebugContext(ctx, "Failed to parse EKG content for enrichment", "request_id", request.ID, "error", err)
		return
	}
	if ekg == nil || ekg.GPTRequestID == "" {
		return
	}

	gptRequestID, err := uuid.Parse(ekg.GPTRequestID)
	if err != nil {
		slog.WarnContext(ctx, "Invalid GPT request ID in EKG content", "request_id", request.ID, "gpt_request_id", ekg.GPTRequestID, "error", err)
		return
	}

	gptRequest, err := repo.GetRequestByID(ctx, gptRequestID)
	if err != nil {
		slog.WarnContext(ctx, "Failed to get GPT request for EKG enrichment", "request_id", request.ID, "gpt_request_id", gptRequestID, "error", err)
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
