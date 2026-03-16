package testutil

import (
	"context"
	"errors"

	"github.com/fedutinova/smartheart/internal/models"
	"github.com/fedutinova/smartheart/internal/repository"
	"github.com/google/uuid"
)

// Compile-time interface check.
var _ repository.RequestRepo = (*MockRequestRepo)(nil)

// MockRequestRepo is a test double for repository.RequestRepo.
// Set the function fields to override default behaviour.
type MockRequestRepo struct {
	CreateRequestFn                  func(ctx context.Context, req *models.Request) error
	GetRequestByIDFn                 func(ctx context.Context, id uuid.UUID) (*models.Request, error)
	GetRequestsByUserIDFn            func(ctx context.Context, userID uuid.UUID, limit, offset int) ([]models.Request, error)
	CountRequestsByUserIDFn          func(ctx context.Context, userID uuid.UUID) (int, error)
	GetRecentRequestsWithResponsesFn func(ctx context.Context, userID uuid.UUID, limit int) ([]models.Request, error)
	UpdateRequestStatusFn            func(ctx context.Context, id uuid.UUID, status string) error
	CreateFileFn                     func(ctx context.Context, file *models.File) error
	GetFilesByRequestIDFn            func(ctx context.Context, requestID uuid.UUID) ([]models.File, error)
	CreateResponseFn                 func(ctx context.Context, resp *models.Response) error
	GetResponseByReqIDFn             func(ctx context.Context, requestID uuid.UUID) (*models.Response, error)
}

func (m *MockRequestRepo) CreateRequest(ctx context.Context, req *models.Request) error {
	if m.CreateRequestFn != nil {
		return m.CreateRequestFn(ctx, req)
	}
	return nil
}

func (m *MockRequestRepo) GetRequestByID(ctx context.Context, id uuid.UUID) (*models.Request, error) {
	if m.GetRequestByIDFn != nil {
		return m.GetRequestByIDFn(ctx, id)
	}
	return nil, errors.New("not found")
}

func (m *MockRequestRepo) GetRequestsByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]models.Request, error) {
	if m.GetRequestsByUserIDFn != nil {
		return m.GetRequestsByUserIDFn(ctx, userID, limit, offset)
	}
	return nil, nil
}

func (m *MockRequestRepo) CountRequestsByUserID(ctx context.Context, userID uuid.UUID) (int, error) {
	if m.CountRequestsByUserIDFn != nil {
		return m.CountRequestsByUserIDFn(ctx, userID)
	}
	return 0, nil
}

func (m *MockRequestRepo) GetRecentRequestsWithResponses(ctx context.Context, userID uuid.UUID, limit int) ([]models.Request, error) {
	if m.GetRecentRequestsWithResponsesFn != nil {
		return m.GetRecentRequestsWithResponsesFn(ctx, userID, limit)
	}
	return nil, nil
}

func (m *MockRequestRepo) UpdateRequestStatus(ctx context.Context, id uuid.UUID, status string) error {
	if m.UpdateRequestStatusFn != nil {
		return m.UpdateRequestStatusFn(ctx, id, status)
	}
	return nil
}

func (m *MockRequestRepo) CreateFile(ctx context.Context, file *models.File) error {
	if m.CreateFileFn != nil {
		return m.CreateFileFn(ctx, file)
	}
	return nil
}

func (m *MockRequestRepo) GetFilesByRequestID(ctx context.Context, requestID uuid.UUID) ([]models.File, error) {
	if m.GetFilesByRequestIDFn != nil {
		return m.GetFilesByRequestIDFn(ctx, requestID)
	}
	return nil, nil
}

func (m *MockRequestRepo) CreateResponse(ctx context.Context, resp *models.Response) error {
	if m.CreateResponseFn != nil {
		return m.CreateResponseFn(ctx, resp)
	}
	return nil
}

func (m *MockRequestRepo) GetResponseByRequestID(ctx context.Context, requestID uuid.UUID) (*models.Response, error) {
	if m.GetResponseByReqIDFn != nil {
		return m.GetResponseByReqIDFn(ctx, requestID)
	}
	return nil, errors.New("not found")
}
