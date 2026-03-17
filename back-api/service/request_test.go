package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/fedutinova/smartheart/back-api/apperr"
	"github.com/fedutinova/smartheart/back-api/auth"
	"github.com/fedutinova/smartheart/back-api/job"
	jobmocks "github.com/fedutinova/smartheart/back-api/job/mocks"
	"github.com/fedutinova/smartheart/back-api/models"
	repomocks "github.com/fedutinova/smartheart/back-api/repository/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newRequestService(t *testing.T) (*requestService, *repomocks.MockStore, *jobmocks.MockQueue) {
	repo := repomocks.NewMockStore(t)
	queue := jobmocks.NewMockQueue(t)
	svc := NewRequestService(repo, queue).(*requestService)
	return svc, repo, queue
}

func userClaims(userID uuid.UUID) *auth.Claims {
	return &auth.Claims{
		UserID: userID.String(),
		Roles:  []string{auth.RoleUser},
	}
}

// --- GetUserRequests ---

func TestGetUserRequests_Success(t *testing.T) {
	svc, repo, _ := newRequestService(t)
	ctx := context.Background()
	userID := uuid.New()

	repo.EXPECT().
		GetRequestsByUserID(mock.Anything, userID, 50, 0).
		Return([]models.Request{{ID: uuid.New(), UserID: userID}}, nil)

	repo.EXPECT().
		CountRequestsByUserID(mock.Anything, userID).
		Return(1, nil)

	page, err := svc.GetUserRequests(ctx, userID, 50, 0)
	require.NoError(t, err)
	assert.Len(t, page.Data, 1)
	assert.Equal(t, 1, page.Total)
	assert.Equal(t, 50, page.Limit)
	assert.Equal(t, 0, page.Offset)
}

func TestGetUserRequests_DefaultLimit(t *testing.T) {
	svc, repo, _ := newRequestService(t)
	ctx := context.Background()
	userID := uuid.New()

	// Limit <= 0 should default to 50
	repo.EXPECT().
		GetRequestsByUserID(mock.Anything, userID, 50, 0).
		Return(nil, nil)

	repo.EXPECT().
		CountRequestsByUserID(mock.Anything, userID).
		Return(0, nil)

	page, err := svc.GetUserRequests(ctx, userID, 0, 0)
	require.NoError(t, err)
	assert.Equal(t, 50, page.Limit)
	assert.Empty(t, page.Data) // nil is converted to empty slice
}

func TestGetUserRequests_LimitTooHigh(t *testing.T) {
	svc, repo, _ := newRequestService(t)
	ctx := context.Background()
	userID := uuid.New()

	// Limit > 200 should default to 50
	repo.EXPECT().
		GetRequestsByUserID(mock.Anything, userID, 50, 0).
		Return(nil, nil)

	repo.EXPECT().
		CountRequestsByUserID(mock.Anything, userID).
		Return(0, nil)

	page, err := svc.GetUserRequests(ctx, userID, 300, 0)
	require.NoError(t, err)
	assert.Equal(t, 50, page.Limit)
}

func TestGetUserRequests_NegativeOffset(t *testing.T) {
	svc, repo, _ := newRequestService(t)
	ctx := context.Background()
	userID := uuid.New()

	// Negative offset should default to 0
	repo.EXPECT().
		GetRequestsByUserID(mock.Anything, userID, 50, 0).
		Return(nil, nil)

	repo.EXPECT().
		CountRequestsByUserID(mock.Anything, userID).
		Return(0, nil)

	page, err := svc.GetUserRequests(ctx, userID, 50, -5)
	require.NoError(t, err)
	assert.Equal(t, 0, page.Offset)
}

func TestGetUserRequests_RepoError(t *testing.T) {
	svc, repo, _ := newRequestService(t)
	ctx := context.Background()
	userID := uuid.New()

	repo.EXPECT().
		GetRequestsByUserID(mock.Anything, userID, 50, 0).
		Return(nil, errors.New("db error"))

	_, err := svc.GetUserRequests(ctx, userID, 50, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get user requests")
}

func TestGetUserRequests_CountError(t *testing.T) {
	svc, repo, _ := newRequestService(t)
	ctx := context.Background()
	userID := uuid.New()

	repo.EXPECT().
		GetRequestsByUserID(mock.Anything, userID, 50, 0).
		Return([]models.Request{}, nil)

	repo.EXPECT().
		CountRequestsByUserID(mock.Anything, userID).
		Return(0, errors.New("count error"))

	_, err := svc.GetUserRequests(ctx, userID, 50, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "count user requests")
}

// --- GetRequest ---

func TestGetRequest_Success(t *testing.T) {
	svc, repo, _ := newRequestService(t)
	ctx := context.Background()
	userID := uuid.New()
	requestID := uuid.New()

	repo.EXPECT().
		GetRequestByID(mock.Anything, requestID).
		Return(&models.Request{ID: requestID, UserID: userID}, nil)

	req, err := svc.GetRequest(ctx, requestID, userClaims(userID))
	require.NoError(t, err)
	assert.Equal(t, requestID, req.ID)
}

func TestGetRequest_NotFound(t *testing.T) {
	svc, repo, _ := newRequestService(t)
	ctx := context.Background()

	repo.EXPECT().
		GetRequestByID(mock.Anything, mock.Anything).
		Return(nil, apperr.ErrNotFound)

	_, err := svc.GetRequest(ctx, uuid.New(), userClaims(uuid.New()))
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.ErrNotFound)
}

func TestGetRequest_Forbidden(t *testing.T) {
	svc, repo, _ := newRequestService(t)
	ctx := context.Background()
	ownerID := uuid.New()
	otherUserID := uuid.New()

	repo.EXPECT().
		GetRequestByID(mock.Anything, mock.Anything).
		Return(&models.Request{ID: uuid.New(), UserID: ownerID}, nil)

	_, err := svc.GetRequest(ctx, uuid.New(), userClaims(otherUserID))
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.ErrForbidden)
}

func TestGetRequest_WithEKGEnrichment(t *testing.T) {
	svc, repo, _ := newRequestService(t)
	ctx := context.Background()
	userID := uuid.New()
	requestID := uuid.New()
	gptRequestID := uuid.New()

	ekgContent := &models.EKGResponseContent{
		AnalysisType: models.EKGModelDirect,
		Timestamp:    "2026-01-01T00:00:00Z",
		GPTRequestID: gptRequestID.String(),
	}
	ekgJSON, _ := ekgContent.Marshal()

	repo.EXPECT().
		GetRequestByID(mock.Anything, requestID).
		Return(&models.Request{
			ID:     requestID,
			UserID: userID,
			Response: &models.Response{
				Model:   models.EKGModelDirect,
				Content: ekgJSON,
			},
		}, nil)

	gptResponse := "### Заключение\nAll good"
	repo.EXPECT().
		GetRequestByID(mock.Anything, gptRequestID).
		Return(&models.Request{
			ID:     gptRequestID,
			UserID: userID,
			Status: models.StatusCompleted,
			Response: &models.Response{
				Content: gptResponse,
			},
		}, nil)

	req, err := svc.GetRequest(ctx, requestID, userClaims(userID))
	require.NoError(t, err)

	// Parse enriched content
	var enriched models.EKGResponseContent
	require.NoError(t, json.Unmarshal([]byte(req.Response.Content), &enriched))
	assert.Equal(t, models.StatusCompleted, enriched.GPTInterpretationStatus)
	assert.NotNil(t, enriched.GPTInterpretation)
	assert.Contains(t, *enriched.GPTInterpretation, "All good")
}

// --- GetJobStatus ---

func TestGetJobStatus_Success(t *testing.T) {
	svc, _, queue := newRequestService(t)
	ctx := context.Background()
	userID := uuid.New()
	jobID := uuid.New()

	payload, _ := json.Marshal(map[string]string{"user_id": userID.String()})

	queue.EXPECT().
		Status(mock.Anything, jobID).
		Return(&job.Job{
			ID:      jobID,
			Type:    job.TypeEKGAnalyze,
			Status:  job.StatusQueued,
			Payload: payload,
		}, true)

	j, err := svc.GetJobStatus(ctx, jobID, userClaims(userID))
	require.NoError(t, err)
	assert.Equal(t, jobID, j.ID)
	assert.Equal(t, job.StatusQueued, j.Status)
}

func TestGetJobStatus_NotFound(t *testing.T) {
	svc, _, queue := newRequestService(t)
	ctx := context.Background()

	queue.EXPECT().
		Status(mock.Anything, mock.Anything).
		Return(nil, false)

	_, err := svc.GetJobStatus(ctx, uuid.New(), userClaims(uuid.New()))
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.ErrJobNotFound)
}

func TestGetJobStatus_Forbidden(t *testing.T) {
	svc, _, queue := newRequestService(t)
	ctx := context.Background()
	ownerID := uuid.New()
	otherUserID := uuid.New()
	jobID := uuid.New()

	payload, _ := json.Marshal(map[string]string{"user_id": ownerID.String()})

	queue.EXPECT().
		Status(mock.Anything, jobID).
		Return(&job.Job{
			ID:      jobID,
			Payload: payload,
		}, true)

	_, err := svc.GetJobStatus(ctx, jobID, userClaims(otherUserID))
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.ErrForbidden)
}

func TestGetJobStatus_InvalidPayload(t *testing.T) {
	svc, _, queue := newRequestService(t)
	ctx := context.Background()
	jobID := uuid.New()

	queue.EXPECT().
		Status(mock.Anything, jobID).
		Return(&job.Job{
			ID:      jobID,
			Payload: []byte("{invalid json"),
		}, true)

	_, err := svc.GetJobStatus(ctx, jobID, userClaims(uuid.New()))
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.ErrForbidden)
}
