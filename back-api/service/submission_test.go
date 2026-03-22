package service

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/fedutinova/smartheart/back-api/apperr"
	jobmocks "github.com/fedutinova/smartheart/back-api/job/mocks"
	"github.com/fedutinova/smartheart/back-api/models"
	repomocks "github.com/fedutinova/smartheart/back-api/repository/mocks"
	"github.com/fedutinova/smartheart/back-api/storage"
	storagemocks "github.com/fedutinova/smartheart/back-api/storage/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newSubmissionService(t *testing.T) (*submissionService, *repomocks.MockStore, *jobmocks.MockQueue, *storagemocks.MockStorage) {
	repo := repomocks.NewMockStore(t)
	queue := jobmocks.NewMockQueue(t)
	store := storagemocks.NewMockStorage(t)
	svc := NewSubmissionService(repo, queue, store).(*submissionService)
	return svc, repo, queue, store
}

// --- SubmitEKG ---

func TestSubmitEKG_Success(t *testing.T) {
	svc, repo, queue, _ := newSubmissionService(t)
	ctx := context.Background()
	userID := uuid.New()
	jobID := uuid.New()

	repo.EXPECT().
		CreateRequest(mock.Anything, mock.Anything).
		Run(func(ctx context.Context, req *models.Request) {
			assert.Equal(t, userID, req.UserID)
			assert.Equal(t, models.StatusPending, req.Status)
		}).
		Return(nil)

	queue.EXPECT().
		Enqueue(mock.Anything, mock.Anything).
		Return(jobID, nil)

	result, err := svc.SubmitEKG(ctx, userID, "https://example.com/ekg.jpg", ECGParams{})
	require.NoError(t, err)
	assert.Equal(t, jobID, result.JobID)
	assert.NotEqual(t, uuid.Nil, result.RequestID)
}

func TestSubmitEKG_EmptyImageURL(t *testing.T) {
	svc, _, _, _ := newSubmissionService(t)
	ctx := context.Background()

	_, err := svc.SubmitEKG(ctx, uuid.New(), "", ECGParams{})
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.ErrValidation)
}

func TestSubmitEKG_CreateRequestFails(t *testing.T) {
	svc, repo, _, _ := newSubmissionService(t)
	ctx := context.Background()

	repo.EXPECT().
		CreateRequest(mock.Anything, mock.Anything).
		Return(errors.New("db error"))

	_, err := svc.SubmitEKG(ctx, uuid.New(), "https://example.com/ekg.jpg", ECGParams{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create request")
}

func TestSubmitEKG_EnqueueFails(t *testing.T) {
	svc, repo, queue, _ := newSubmissionService(t)
	ctx := context.Background()

	repo.EXPECT().
		CreateRequest(mock.Anything, mock.Anything).
		Return(nil)

	queue.EXPECT().
		Enqueue(mock.Anything, mock.Anything).
		Return(uuid.Nil, errors.New("queue full"))

	_, err := svc.SubmitEKG(ctx, uuid.New(), "https://example.com/ekg.jpg", ECGParams{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "enqueue EKG job")
}

// --- SubmitEKGFile ---

func TestSubmitEKGFile_Success(t *testing.T) {
	svc, repo, queue, store := newSubmissionService(t)
	ctx := context.Background()
	userID := uuid.New()
	jobID := uuid.New()

	store.EXPECT().
		UploadFile(mock.Anything, "ekg.jpg", mock.Anything, "image/jpeg").
		Return(&storage.UploadResult{Key: "uploads/ekg.jpg", URL: "https://s3/uploads/ekg.jpg"}, nil)

	repo.EXPECT().
		CreateRequest(mock.Anything, mock.Anything).
		Return(nil)

	repo.EXPECT().
		CreateFile(mock.Anything, mock.Anything).
		Run(func(ctx context.Context, f *models.File) {
			assert.Equal(t, "uploads/ekg.jpg", f.S3Key)
			assert.Equal(t, "image/jpeg", f.FileType)
		}).
		Return(nil)

	queue.EXPECT().
		Enqueue(mock.Anything, mock.Anything).
		Return(jobID, nil)

	file := UploadedFile{
		Reader:      bytes.NewReader([]byte("image data")),
		Filename:    "ekg.jpg",
		ContentType: "image/jpeg",
		Size:        10,
	}

	result, err := svc.SubmitEKGFile(ctx, userID, file, ECGParams{})
	require.NoError(t, err)
	assert.Equal(t, jobID, result.JobID)
	assert.NotEqual(t, uuid.Nil, result.RequestID)
}

func TestSubmitEKGFile_UploadFails(t *testing.T) {
	svc, _, _, store := newSubmissionService(t)
	ctx := context.Background()

	store.EXPECT().
		UploadFile(mock.Anything, "ekg.jpg", mock.Anything, "image/jpeg").
		Return(nil, errors.New("storage down"))

	file := UploadedFile{
		Reader:      bytes.NewReader([]byte("data")),
		Filename:    "ekg.jpg",
		ContentType: "image/jpeg",
		Size:        4,
	}

	_, err := svc.SubmitEKGFile(ctx, uuid.New(), file, ECGParams{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "upload EKG image")
}

func TestSubmitEKGFile_CreateRequestFails(t *testing.T) {
	svc, repo, _, store := newSubmissionService(t)
	ctx := context.Background()

	store.EXPECT().
		UploadFile(mock.Anything, "ekg.jpg", mock.Anything, "image/jpeg").
		Return(&storage.UploadResult{Key: "uploads/ekg.jpg", URL: "https://s3/uploads/ekg.jpg"}, nil)

	repo.EXPECT().
		CreateRequest(mock.Anything, mock.Anything).
		Return(errors.New("db error"))

	file := UploadedFile{
		Reader:      bytes.NewReader([]byte("data")),
		Filename:    "ekg.jpg",
		ContentType: "image/jpeg",
		Size:        4,
	}

	_, err := svc.SubmitEKGFile(ctx, uuid.New(), file, ECGParams{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create request")
}

// --- SubmitGPT ---

func TestSubmitGPT_Success(t *testing.T) {
	svc, repo, queue, store := newSubmissionService(t)
	ctx := context.Background()
	userID := uuid.New()
	jobID := uuid.New()

	repo.EXPECT().
		CreateRequest(mock.Anything, mock.Anything).
		Return(nil)

	store.EXPECT().
		UploadFile(mock.Anything, "test.pdf", mock.Anything, "application/pdf").
		Return(&storage.UploadResult{Key: "files/test.pdf", URL: "https://s3/files/test.pdf"}, nil)

	repo.EXPECT().
		CreateFile(mock.Anything, mock.Anything).
		Return(nil)

	queue.EXPECT().
		Enqueue(mock.Anything, mock.Anything).
		Return(jobID, nil)

	files := []UploadedFile{
		{
			Reader:      bytes.NewReader([]byte("pdf content")),
			Filename:    "test.pdf",
			ContentType: "application/pdf",
			Size:        11,
		},
	}

	result, err := svc.SubmitGPT(ctx, userID, "analyze this", files)
	require.NoError(t, err)
	assert.Equal(t, jobID, result.JobID)
	assert.Equal(t, 1, result.FilesProcessed)
	assert.Empty(t, result.UploadErrors)
}

func TestSubmitGPT_NoFiles(t *testing.T) {
	svc, repo, _, _ := newSubmissionService(t)
	ctx := context.Background()

	repo.EXPECT().
		CreateRequest(mock.Anything, mock.Anything).
		Return(nil)

	repo.EXPECT().
		UpdateRequestStatus(mock.Anything, mock.Anything, models.StatusFailed).
		Return(nil)

	result, err := svc.SubmitGPT(ctx, uuid.New(), "query", nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.ErrValidation)
	assert.NotNil(t, result)
}

func TestSubmitGPT_AllUploadsFail(t *testing.T) {
	svc, repo, _, store := newSubmissionService(t)
	ctx := context.Background()

	repo.EXPECT().
		CreateRequest(mock.Anything, mock.Anything).
		Return(nil)

	store.EXPECT().
		UploadFile(mock.Anything, "bad.pdf", mock.Anything, "application/pdf").
		Return(nil, errors.New("storage error"))

	repo.EXPECT().
		UpdateRequestStatus(mock.Anything, mock.Anything, models.StatusFailed).
		Return(nil)

	files := []UploadedFile{
		{
			Reader:      bytes.NewReader([]byte("content")),
			Filename:    "bad.pdf",
			ContentType: "application/pdf",
			Size:        7,
		},
	}

	result, err := svc.SubmitGPT(ctx, uuid.New(), "query", files)
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.ErrValidation)
	assert.Len(t, result.UploadErrors, 1)
}

func TestSubmitGPT_PartialUploadFailure(t *testing.T) {
	svc, repo, queue, store := newSubmissionService(t)
	ctx := context.Background()
	userID := uuid.New()

	repo.EXPECT().
		CreateRequest(mock.Anything, mock.Anything).
		Return(nil)

	// First file succeeds
	store.EXPECT().
		UploadFile(mock.Anything, "good.pdf", mock.Anything, "application/pdf").
		Return(&storage.UploadResult{Key: "files/good.pdf", URL: "https://s3/good.pdf"}, nil)

	repo.EXPECT().
		CreateFile(mock.Anything, mock.Anything).
		Return(nil)

	// Second file fails
	store.EXPECT().
		UploadFile(mock.Anything, "bad.pdf", mock.Anything, "application/pdf").
		Return(nil, errors.New("upload failed"))

	queue.EXPECT().
		Enqueue(mock.Anything, mock.Anything).
		Return(uuid.New(), nil)

	files := []UploadedFile{
		{Reader: bytes.NewReader([]byte("ok")), Filename: "good.pdf", ContentType: "application/pdf", Size: 2},
		{Reader: bytes.NewReader([]byte("bad")), Filename: "bad.pdf", ContentType: "application/pdf", Size: 3},
	}

	result, err := svc.SubmitGPT(ctx, userID, "query", files)
	require.NoError(t, err)
	assert.Equal(t, 1, result.FilesProcessed)
	assert.Len(t, result.UploadErrors, 1)
	assert.Contains(t, result.UploadErrors[0], "bad.pdf")
}

func TestSubmitGPT_ContentTypeDetection(t *testing.T) {
	svc, repo, queue, store := newSubmissionService(t)
	ctx := context.Background()

	repo.EXPECT().
		CreateRequest(mock.Anything, mock.Anything).
		Return(nil)

	// When ContentType is empty, processFile should detect it
	store.EXPECT().
		UploadFile(mock.Anything, "image.bin", mock.Anything, mock.Anything).
		Return(&storage.UploadResult{Key: "files/image.bin", URL: "https://s3/image.bin"}, nil)

	repo.EXPECT().
		CreateFile(mock.Anything, mock.Anything).
		Return(nil)

	queue.EXPECT().
		Enqueue(mock.Anything, mock.Anything).
		Return(uuid.New(), nil)

	// PNG header bytes for content type detection
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	files := []UploadedFile{
		{Reader: bytes.NewReader(pngHeader), Filename: "image.bin", ContentType: "", Size: int64(len(pngHeader))},
	}

	result, err := svc.SubmitGPT(ctx, uuid.New(), "query", files)
	require.NoError(t, err)
	assert.Equal(t, 1, result.FilesProcessed)
}

func TestSubmitGPT_CreateRequestFails(t *testing.T) {
	svc, repo, _, _ := newSubmissionService(t)
	ctx := context.Background()

	repo.EXPECT().
		CreateRequest(mock.Anything, mock.Anything).
		Return(errors.New("db error"))

	files := []UploadedFile{
		{Reader: bytes.NewReader([]byte("x")), Filename: "f.pdf", ContentType: "application/pdf", Size: 1},
	}

	_, err := svc.SubmitGPT(ctx, uuid.New(), "query", files)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create request")
}
