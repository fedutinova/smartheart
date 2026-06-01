package cv_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/fedutinova/smartheart/back-api/apperr"
	"github.com/fedutinova/smartheart/back-api/cv"
)

// fakePNG is a 1×1 transparent PNG, just enough bytes for cv_service's PIL
// decoder to accept the file. Inference quality is irrelevant for these tests.
var fakePNG = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
	0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4,
	0x89, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x44, 0x41,
	0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00,
	0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE,
	0x42, 0x60, 0x82,
}

func TestNewHTTPClient_EmptyURL(t *testing.T) {
	_, err := cv.NewHTTPClient("", 0)
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
	if !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

func TestHTTPClient_Predict_Happy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/predict" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			t.Errorf("expected multipart, got %s", r.Header.Get("Content-Type"))
		}
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Fatalf("parse multipart: %v", err)
		}
		if got := r.FormValue("layout_label"); got != "3x4_rhythm" {
			t.Errorf("layout_label=%q", got)
		}
		if got := r.FormValue("preprocess_name"); got != "synthmatch" {
			t.Errorf("preprocess_name=%q", got)
		}
		fh := r.MultipartForm.File["file"]
		if len(fh) != 1 {
			t.Fatalf("expected 1 file part, got %d", len(fh))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"pred_idx": 1,
			"pred_code": "AFIB",
			"pred_label_ru": "фибрилляция предсердий",
			"layout_label": "3x4_rhythm",
			"preprocess_name": "synthmatch",
			"top3": [
				{"idx":1,"code":"AFIB","label_ru":"фибрилляция предсердий","prob":0.81},
				{"idx":0,"code":"SINUS_GROUP","label_ru":"синусовый ритм","prob":0.12},
				{"idx":2,"code":"AFLT","label_ru":"трепетание предсердий","prob":0.04}
			],
			"binary_flags": [
				{"code":"stt_label","label_ru":"возможны изменения ST-T","prob":0.71}
			],
			"raw_probs": [0.12, 0.81, 0.04]
		}`))
	}))
	defer srv.Close()

	c, err := cv.NewHTTPClient(srv.URL, 5*time.Second)
	if err != nil {
		t.Fatalf("NewHTTPClient: %v", err)
	}
	pred, err := c.Predict(context.Background(), fakePNG, "image/png", "ecg.png", "", "")
	if err != nil {
		t.Fatalf("Predict: %v", err)
	}
	if pred.PredCode != "AFIB" {
		t.Errorf("PredCode=%q", pred.PredCode)
	}
	if len(pred.Top3) != 3 {
		t.Errorf("len(Top3)=%d", len(pred.Top3))
	}
	if len(pred.BinaryFlags) != 1 || pred.BinaryFlags[0].Code != "stt_label" {
		t.Errorf("BinaryFlags=%+v", pred.BinaryFlags)
	}
}

func TestHTTPClient_Predict_BadRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"detail":"Invalid image file"}`))
	}))
	defer srv.Close()

	c, _ := cv.NewHTTPClient(srv.URL, 5*time.Second)
	_, err := c.Predict(context.Background(), fakePNG, "image/png", "x.png", "", "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, apperr.ErrValidation) {
		t.Errorf("expected ErrValidation, got %v", err)
	}
}

func TestHTTPClient_Predict_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"detail":"Inference failed"}`))
	}))
	defer srv.Close()

	c, _ := cv.NewHTTPClient(srv.URL, 5*time.Second)
	_, err := c.Predict(context.Background(), fakePNG, "image/png", "x.png", "", "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, apperr.ErrInternal) {
		t.Errorf("expected ErrInternal, got %v", err)
	}
}

func TestHTTPClient_Predict_InvalidLayout(t *testing.T) {
	c, _ := cv.NewHTTPClient("http://example.invalid", time.Second)
	_, err := c.Predict(context.Background(), fakePNG, "image/png", "x.png", "not-a-layout", "")
	if !errors.Is(err, apperr.ErrValidation) {
		t.Errorf("expected ErrValidation, got %v", err)
	}
}

func TestHTTPClient_Predict_EmptyImage(t *testing.T) {
	c, _ := cv.NewHTTPClient("http://example.invalid", time.Second)
	_, err := c.Predict(context.Background(), nil, "image/png", "x.png", "", "")
	if !errors.Is(err, apperr.ErrValidation) {
		t.Errorf("expected ErrValidation, got %v", err)
	}
}

func TestHTTPClient_Health_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	c, _ := cv.NewHTTPClient(srv.URL, 5*time.Second)
	if err := c.Health(context.Background()); err != nil {
		t.Fatalf("Health: %v", err)
	}
}

// TestE2E_LiveCV exercises a real cv_service deployment to verify the Go ↔
// Python contract. Skipped unless CV_URL_E2E is set (e.g. http://localhost:8001).
// Requires an ECG image at CV_E2E_IMAGE or falls back to a known h2 fixture.
func TestE2E_LiveCV(t *testing.T) {
	url := os.Getenv("CV_URL_E2E")
	if url == "" {
		t.Skip("CV_URL_E2E not set; skipping end-to-end CV test")
	}

	imgPath := os.Getenv("CV_E2E_IMAGE")
	if imgPath == "" {
		imgPath = "../../h2/with-test-data/00001_hr_1R.png"
	}
	f, err := os.Open(imgPath)
	if err != nil {
		t.Fatalf("open test image %s: %v", imgPath, err)
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("read test image: %v", err)
	}

	c, err := cv.NewHTTPClient(url, 60*time.Second)
	if err != nil {
		t.Fatalf("NewHTTPClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	if err := c.Health(ctx); err != nil {
		t.Fatalf("Health: %v", err)
	}

	pred, err := c.Predict(ctx, data, "image/png", "ecg.png", "", "")
	if err != nil {
		t.Fatalf("Predict: %v", err)
	}
	if pred.PredCode == "" {
		t.Errorf("expected non-empty PredCode, got %+v", pred)
	}
	if len(pred.Top3) == 0 {
		t.Errorf("expected non-empty Top3, got %+v", pred)
	}
	t.Logf("LIVE CV result: pred=%s label=%q top3=%d flags=%d",
		pred.PredCode, pred.PredLabelRU, len(pred.Top3), len(pred.BinaryFlags))
}
