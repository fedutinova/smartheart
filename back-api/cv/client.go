package cv

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"time"

	"github.com/fedutinova/smartheart/back-api/apperr"
)

// Client is the interface ECGWorker depends on. Implemented by *HTTPClient
// against a real cv_service deployment and by mockery-generated mocks in tests.
//
//go:generate mockery --name=Client --outpkg=mocks --output=mocks --filename=mock_client.go --with-expecter
type Client interface {
	// Predict runs rhythm classification on a single ECG image.
	//
	// imageBytes must be a decodable image (JPEG/PNG/etc); content-type is
	// derived from contentType and forwarded as multipart Content-Type so the
	// service can pick the right PIL decoder.
	//
	// Empty layoutLabel / preprocessName fall back to DefaultLayout / DefaultPreprocess.
	Predict(
		ctx context.Context,
		imageBytes []byte,
		contentType string,
		filename string,
		layoutLabel string,
		preprocessName string,
	) (*RhythmPrediction, error)

	// Health is a cheap GET /health probe used at startup. It does NOT load the
	// model; cv_service lazily initialises the predictor on the first /predict.
	Health(ctx context.Context) error
}

// HTTPClient is the real, blocking HTTP implementation of Client.
type HTTPClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewHTTPClient builds an HTTPClient. baseURL is required and must include the
// scheme (e.g. "http://cv:8000"). timeout caps every individual request; pass
// 0 to use the package default (60s).
func NewHTTPClient(baseURL string, timeout time.Duration) (*HTTPClient, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("new cv client: %w", apperr.ErrValidation)
	}
	if _, err := url.Parse(baseURL); err != nil {
		return nil, fmt.Errorf("new cv client: parse base url: %w", apperr.ErrValidation)
	}
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &HTTPClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}, nil
}

// Health implements Client.
func (c *HTTPClient) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", http.NoBody)
	if err != nil {
		return apperr.WrapInternal("cv health: build request", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return apperr.WrapInternal("cv health: do request", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return apperr.WrapInternal(
			"cv health: unexpected status",
			fmt.Errorf("status=%d body=%s", resp.StatusCode, string(body)),
		)
	}
	return nil
}

// Predict implements Client.
func (c *HTTPClient) Predict(
	ctx context.Context,
	imageBytes []byte,
	contentType string,
	filename string,
	layoutLabel string,
	preprocessName string,
) (*RhythmPrediction, error) {
	if len(imageBytes) == 0 {
		return nil, fmt.Errorf("cv predict: empty image: %w", apperr.ErrValidation)
	}
	if layoutLabel == "" {
		layoutLabel = DefaultLayout
	}
	if preprocessName == "" {
		preprocessName = DefaultPreprocess
	}
	if !IsValidLayout(layoutLabel) {
		return nil, fmt.Errorf("cv predict: invalid layout_label=%q: %w", layoutLabel, apperr.ErrValidation)
	}
	if !IsValidPreprocess(preprocessName) {
		return nil, fmt.Errorf("cv predict: invalid preprocess_name=%q: %w", preprocessName, apperr.ErrValidation)
	}
	if filename == "" {
		filename = "ecg.jpg"
	}
	if contentType == "" {
		contentType = "image/jpeg"
	}

	body, multipartContentType, err := buildPredictForm(imageBytes, contentType, filename, layoutLabel, preprocessName)
	if err != nil {
		return nil, apperr.WrapInternal("cv predict: build form", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/predict", body)
	if err != nil {
		return nil, apperr.WrapInternal("cv predict: build request", err)
	}
	req.Header.Set("Content-Type", multipartContentType)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, apperr.WrapInternal("cv predict: do request", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MiB cap on response
	if err != nil {
		return nil, apperr.WrapInternal("cv predict: read response", err)
	}

	if resp.StatusCode != http.StatusOK {
		// cv_service returns 400 for invalid image / bad params, 500 for inference failures.
		if resp.StatusCode == http.StatusBadRequest {
			return nil, fmt.Errorf("cv predict: bad request: %s: %w", trimErr(respBody), apperr.ErrValidation)
		}
		return nil, apperr.WrapInternal(
			"cv predict: upstream error",
			fmt.Errorf("status=%d body=%s", resp.StatusCode, trimErr(respBody)),
		)
	}

	var pred RhythmPrediction
	if err := json.Unmarshal(respBody, &pred); err != nil {
		return nil, apperr.WrapInternal("cv predict: decode response", err)
	}
	if pred.PredCode == "" {
		return nil, apperr.WrapInternal("cv predict: empty response", errors.New("pred_code missing"))
	}
	return &pred, nil
}

func buildPredictForm(
	imageBytes []byte,
	contentType, filename, layoutLabel, preprocessName string,
) (io.Reader, string, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	imgHeader := textproto.MIMEHeader{}
	imgHeader.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="file"; filename=%q`, filename))
	imgHeader.Set("Content-Type", contentType)
	imgPart, err := mw.CreatePart(imgHeader)
	if err != nil {
		return nil, "", fmt.Errorf("create file part: %w", err)
	}
	if _, err := imgPart.Write(imageBytes); err != nil {
		return nil, "", fmt.Errorf("write file part: %w", err)
	}

	for _, kv := range [][2]string{
		{"layout_label", layoutLabel},
		{"preprocess_name", preprocessName},
	} {
		if err := mw.WriteField(kv[0], kv[1]); err != nil {
			return nil, "", fmt.Errorf("write field %s: %w", kv[0], err)
		}
	}
	if err := mw.Close(); err != nil {
		return nil, "", fmt.Errorf("close multipart: %w", err)
	}
	return &buf, mw.FormDataContentType(), nil
}

// trimErr keeps cv_service error bodies short and safe for logs.
func trimErr(b []byte) string {
	s := strings.TrimSpace(string(b))
	const maxLen = 256
	if len(s) > maxLen {
		s = s[:maxLen] + "…"
	}
	return s
}
