package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/fedutinova/smartheart/back-api/apperr"
	"github.com/fedutinova/smartheart/back-api/config"
	"github.com/fedutinova/smartheart/back-api/gpt"
	"github.com/fedutinova/smartheart/back-api/job"
	"github.com/fedutinova/smartheart/back-api/models"
	"github.com/fedutinova/smartheart/back-api/repository"
	"github.com/fedutinova/smartheart/back-api/storage"
	"github.com/google/uuid"
)

// SubmittedJob is the result of enqueueing a job.
type SubmittedJob struct {
	JobID     uuid.UUID
	RequestID uuid.UUID
	Status    string
}

// GPTSubmitResult extends SubmittedJob with file processing details.
type GPTSubmitResult struct {
	SubmittedJob
	FilesProcessed int
	UploadErrors   []string
}

// UploadedFile represents a file ready for processing.
type UploadedFile struct {
	Reader      io.ReadSeeker
	Filename    string
	ContentType string
	Size        int64
}

// SubmissionService handles EKG and GPT job submission business logic.
type SubmissionService interface {
	SubmitEKG(ctx context.Context, userID uuid.UUID, imageURL, notes string) (*SubmittedJob, error)
	SubmitEKGFile(ctx context.Context, userID uuid.UUID, file UploadedFile, notes string) (*SubmittedJob, error)
	SubmitGPT(ctx context.Context, userID uuid.UUID, textQuery string, files []UploadedFile) (*GPTSubmitResult, error)
}

type submissionService struct {
	repo       repository.Store
	queue      job.Queue
	storage    storage.Storage
	dailyLimit int
}

func NewSubmissionService(repo repository.Store, queue job.Queue, storageService storage.Storage, quota ...config.QuotaConfig) SubmissionService {
	s := &submissionService{repo: repo, queue: queue, storage: storageService}
	if len(quota) > 0 {
		s.dailyLimit = quota[0].DailyLimit
	}
	return s
}

const maxNotesLen = 2000

// detectContentType returns the file's content type, sniffing it from the first
// 512 bytes when the UploadedFile does not already carry one.
func detectContentType(file *UploadedFile) (string, error) {
	if file.ContentType != "" {
		return file.ContentType, nil
	}
	buf := make([]byte, 512)
	n, err := io.ReadFull(file.Reader, buf)
	if n == 0 && err != nil {
		return "", apperr.WrapInternal("detect content type", err)
	}
	ct := http.DetectContentType(buf[:n])
	if _, err := file.Reader.Seek(0, io.SeekStart); err != nil {
		return "", apperr.WrapInternal("seek file", err)
	}
	return ct, nil
}

func validateEKGNotes(notes string) error {
	if len(notes) > maxNotesLen {
		return fmt.Errorf("notes too long: %w", apperr.ErrValidation)
	}
	return nil
}

// checkQuota increments the daily usage counter and returns an error if the limit is exceeded.
func (s *submissionService) checkQuota(ctx context.Context, userID uuid.UUID) error {
	if s.dailyLimit <= 0 {
		return nil // unlimited
	}
	count, err := s.repo.IncrementDailyUsage(ctx, userID)
	if err != nil {
		slog.Warn("failed to check quota, allowing request", "user_id", userID, "error", err)
		return nil // fail-open
	}
	if count > s.dailyLimit {
		return fmt.Errorf("daily submission limit (%d) exceeded: %w", s.dailyLimit, apperr.ErrValidation)
	}
	return nil
}

func (s *submissionService) SubmitEKG(ctx context.Context, userID uuid.UUID, imageURL, notes string) (*SubmittedJob, error) {
	if imageURL == "" {
		return nil, fmt.Errorf("image_temp_url is required: %w", apperr.ErrValidation)
	}
	if err := s.checkQuota(ctx, userID); err != nil {
		return nil, err
	}
	if err := validateEKGNotes(notes); err != nil {
		return nil, err
	}

	requestID := uuid.New()
	request := &models.Request{
		ID:     requestID,
		UserID: userID,
		Status: models.StatusPending,
	}
	if notes != "" {
		request.TextQuery = &notes
	}

	if err := s.repo.CreateRequest(ctx, request); err != nil {
		return nil, apperr.WrapInternal("create request", err)
	}

	payload, err := json.Marshal(job.EKGJobPayload{
		ImageTempURL: imageURL,
		Notes:        notes,
		UserID:       userID,
		RequestID:    requestID,
	})
	if err != nil {
		return nil, apperr.WrapInternal("marshal EKG payload", err)
	}

	j := &job.Job{Type: job.TypeEKGAnalyze, Payload: payload}
	jobID, err := s.queue.Enqueue(ctx, j)
	if err != nil {
		return nil, apperr.WrapInternal("enqueue EKG job", err)
	}

	slog.Info("ekg analysis job enqueued", "job_id", jobID, "request_id", requestID, "user_id", userID)

	return &SubmittedJob{
		JobID:     jobID,
		RequestID: requestID,
		Status:    string(j.Status),
	}, nil
}

func (s *submissionService) SubmitEKGFile(ctx context.Context, userID uuid.UUID, file UploadedFile, notes string) (*SubmittedJob, error) {
	if err := s.checkQuota(ctx, userID); err != nil {
		return nil, err
	}
	if err := validateEKGNotes(notes); err != nil {
		return nil, err
	}

	contentType, err := detectContentType(&file)
	if err != nil {
		return nil, err
	}

	// Upload to storage
	uploadResult, err := s.storage.UploadFile(ctx, file.Filename, file.Reader, contentType)
	if err != nil {
		return nil, apperr.WrapInternal("upload EKG image", err)
	}

	requestID := uuid.New()
	request := &models.Request{
		ID:     requestID,
		UserID: userID,
		Status: models.StatusPending,
	}
	if notes != "" {
		request.TextQuery = &notes
	}

	if err := s.repo.CreateRequest(ctx, request); err != nil {
		return nil, apperr.WrapInternal("create request", err)
	}

	fileModel := &models.File{
		ID:               uuid.New(),
		RequestID:        requestID,
		OriginalFilename: file.Filename,
		FileType:         contentType,
		FileSize:         file.Size,
		S3Key:            uploadResult.Key,
		S3URL:            uploadResult.URL,
	}
	if err := s.repo.CreateFile(ctx, fileModel); err != nil {
		return nil, apperr.WrapInternal("create file record", err)
	}

	payload, err := json.Marshal(job.EKGJobPayload{
		ImageFileKey: uploadResult.Key,
		Notes:        notes,
		UserID:       userID,
		RequestID:    requestID,
	})
	if err != nil {
		return nil, apperr.WrapInternal("marshal EKG payload", err)
	}

	j := &job.Job{Type: job.TypeEKGAnalyze, Payload: payload}
	jobID, err := s.queue.Enqueue(ctx, j)
	if err != nil {
		return nil, apperr.WrapInternal("enqueue EKG job", err)
	}

	slog.Info("ekg file analysis job enqueued", "job_id", jobID, "request_id", requestID, "user_id", userID, "file_key", uploadResult.Key)

	return &SubmittedJob{
		JobID:     jobID,
		RequestID: requestID,
		Status:    string(j.Status),
	}, nil
}

func (s *submissionService) SubmitGPT(ctx context.Context, userID uuid.UUID, textQuery string, files []UploadedFile) (*GPTSubmitResult, error) {
	if err := s.checkQuota(ctx, userID); err != nil {
		return nil, err
	}

	request := &models.Request{
		ID:     uuid.New(),
		UserID: userID,
		Status: models.StatusPending,
	}
	if textQuery != "" {
		request.TextQuery = &textQuery
	}

	if err := s.repo.CreateRequest(ctx, request); err != nil {
		return nil, apperr.WrapInternal("create request", err)
	}

	var fileKeys []string
	var uploadErrors []string
	for _, f := range files {
		key, err := s.processFile(ctx, request.ID, f)
		if err != nil {
			slog.Error("failed to process file", "filename", f.Filename, "error", err)
			uploadErrors = append(uploadErrors, fmt.Sprintf("%s: %s", f.Filename, err.Error()))
			continue
		}
		fileKeys = append(fileKeys, key)
	}

	if len(fileKeys) == 0 {
		if err := s.repo.UpdateRequestStatus(ctx, request.ID, models.StatusFailed); err != nil {
			slog.Error("failed to mark request as failed", "request_id", request.ID, "error", err)
		}
		return &GPTSubmitResult{
			UploadErrors: uploadErrors,
		}, fmt.Errorf("no files successfully processed: %w", apperr.ErrValidation)
	}

	payload := gpt.JobPayload{
		RequestID: request.ID,
		TextQuery: textQuery,
		FileKeys:  fileKeys,
		UserID:    userID,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, apperr.WrapInternal("marshal GPT payload", err)
	}

	j := &job.Job{Type: job.TypeGPTProcess, Payload: payloadBytes}
	jobID, err := s.queue.Enqueue(ctx, j)
	if err != nil {
		return nil, apperr.WrapInternal("enqueue GPT job", err)
	}

	return &GPTSubmitResult{
		SubmittedJob: SubmittedJob{
			JobID:     jobID,
			RequestID: request.ID,
			Status:    request.Status,
		},
		FilesProcessed: len(fileKeys),
		UploadErrors:   uploadErrors,
	}, nil
}

func (s *submissionService) processFile(ctx context.Context, requestID uuid.UUID, f UploadedFile) (string, error) {
	contentType, err := detectContentType(&f)
	if err != nil {
		return "", err
	}

	uploadResult, err := s.storage.UploadFile(ctx, f.Filename, f.Reader, contentType)
	if err != nil {
		return "", apperr.WrapInternal("upload file", err)
	}

	fileModel := &models.File{
		ID:               uuid.New(),
		RequestID:        requestID,
		OriginalFilename: f.Filename,
		FileType:         contentType,
		FileSize:         f.Size,
		S3Key:            uploadResult.Key,
		S3URL:            uploadResult.URL,
	}
	if err := s.repo.CreateFile(ctx, fileModel); err != nil {
		return "", apperr.WrapInternal("create file record", err)
	}

	return uploadResult.Key, nil
}
