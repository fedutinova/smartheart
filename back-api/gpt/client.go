package gpt

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/fedutinova/smartheart/back-api/storage"
	"github.com/fedutinova/smartheart/back-api/validation"
	"github.com/sashabaranov/go-openai"
)

// Processor is the interface for GPT processing, enabling testability.
type Processor interface {
	ProcessRequest(ctx context.Context, textQuery string, fileKeys []string) (*ProcessResult, error)
	ProcessStructuredECG(ctx context.Context, fileKeys []string, systemPrompt, userPrompt string) (*ProcessResult, error)
}

type Client struct {
	openAI      *openai.Client
	storage     storage.Storage
	model       string                // GPT model name
	imageDetail openai.ImageURLDetail // Detail level for images (Auto, Low, High)
	timeout     time.Duration         // Request timeout
}

// ClientOption configures GPT client
type ClientOption func(*Client)

// WithImageDetail sets image detail level
func WithImageDetail(detail openai.ImageURLDetail) ClientOption {
	return func(c *Client) {
		c.imageDetail = detail
	}
}

// WithTimeout sets request timeout
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.timeout = timeout
	}
}

// WithModel sets the GPT model name
func WithModel(model string) ClientOption {
	return func(c *Client) {
		if model != "" {
			c.model = model
		}
	}
}

type ProcessResult struct {
	Content          string
	Model            string
	TokensUsed       int
	ProcessingTimeMs int
}

func NewClient(apiKey string, storageService storage.Storage, opts ...ClientOption) *Client {
	client := &Client{
		openAI:      openai.NewClient(apiKey),
		storage:     storageService,
		model:       openai.GPT4o,
		imageDetail: openai.ImageURLDetailAuto,
		timeout:     60 * time.Second,
	}
	for _, opt := range opts {
		opt(client)
	}
	return client
}

func (c *Client) ProcessRequest(ctx context.Context, textQuery string, fileKeys []string) (*ProcessResult, error) {
	start := time.Now()

	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	messages := []openai.ChatCompletionMessage{
		{
			Role: openai.ChatMessageRoleSystem,
			Content: "You are an expert assistant for analyzing ECG/EKG (electrocardiogram) images. " +
				"You will receive an image of an ECG recording. " +
				"Your task is to describe what you observe in Russian language.\n\n" +
				"Provide a structured analysis in Russian:\n" +
				"1. Качество изображения: четкость, наличие артефактов, видимость отведений и калибровки\n" +
				"2. Ритм: регулярный/нерегулярный, приблизительная ЧСС если видна разметка\n" +
				"3. Зубцы и интервалы: P, QRS, T — форма, амплитуда, длительность\n" +
				"4. Сегменты: ST-сегмент, PR-интервал, QT-интервал\n" +
				"5. Особенности: отклонения от нормального синусового ритма\n\n" +
				"This is a technical image analysis task for educational purposes. " +
				"Describe what you observe without making diagnostic conclusions. " +
				"If you cannot see certain details or measurements, state that clearly.",
		},
	}

	var content []openai.ChatMessagePart

	// Add images FIRST, then text query (OpenAI recommends this order)
	for _, key := range fileKeys {
		filePart, err := c.createMessagePartFromFile(reqCtx, key)
		if err != nil {
			slog.Error("failed to process file", "key", key, "error", err)
			continue
		}
		if filePart != nil {
			content = append(content, *filePart)
		}
	}

	if textQuery != "" {
		content = append(content, openai.ChatMessagePart{
			Type: openai.ChatMessagePartTypeText,
			Text: textQuery,
		})
	}

	if len(content) == 0 {
		return nil, fmt.Errorf("no valid content to process")
	}

	messages = append(messages, openai.ChatCompletionMessage{
		Role:         openai.ChatMessageRoleUser,
		MultiContent: content,
	})

	slog.Info("sending request to OpenAI",
		"model", c.model,
		"files", len(fileKeys),
		"content_parts", len(content))

	resp, err := c.openAI.CreateChatCompletion(reqCtx, openai.ChatCompletionRequest{
		Model:     c.model,
		Messages:  messages,
		MaxTokens: 2000,
	})
	if err != nil {
		return nil, classifyOpenAIError(err, reqCtx, c.timeout)
	}

	if len(resp.Choices) == 0 {
		slog.Error("openai API returned empty choices",
			"model", resp.Model,
			"tokens_used", resp.Usage.TotalTokens,
			"response_id", resp.ID)
		return nil, fmt.Errorf("no response from OpenAI")
	}

	responseContent := resp.Choices[0].Message.Content

	if IsRefusal(responseContent) {
		slog.Warn("openai returned refusal", "tokens", resp.Usage.TotalTokens, "finish_reason", resp.Choices[0].FinishReason)
	}

	slog.Info("openai response received",
		"model", resp.Model,
		"tokens", resp.Usage.TotalTokens,
		"response_len", len(responseContent))

	processingTime := time.Since(start)

	return &ProcessResult{
		Content:          resp.Choices[0].Message.Content,
		Model:            resp.Model,
		TokensUsed:       resp.Usage.TotalTokens,
		ProcessingTimeMs: int(processingTime.Milliseconds()),
	}, nil
}

func (c *Client) createMessagePartFromFile(ctx context.Context, key string) (*openai.ChatMessagePart, error) {
	reader, contentType, err := c.storage.GetFile(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get file from storage: %w", err)
	}
	defer reader.Close()

	const maxFileSize = 20 * 1024 * 1024 // 20 MB
	data, err := io.ReadAll(io.LimitReader(reader, maxFileSize+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read file data: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("file is empty: %s", key)
	}
	if len(data) > maxFileSize {
		return nil, fmt.Errorf("file too large: %s (%d bytes, max %d)", key, len(data), maxFileSize)
	}

	// Detect content type from file header if not provided or generic
	if contentType == "" || contentType == "application/octet-stream" {
		sniffLen := min(512, len(data))
		detected := http.DetectContentType(data[:sniffLen])
		if detected != "application/octet-stream" {
			contentType = detected
			slog.Debug("detected content type", "key", key, "detected_type", contentType)
		}
	}

	if isImageType(contentType) {
		return c.buildImagePart(ctx, key, data, contentType)
	}

	if isTextType(contentType) {
		return &openai.ChatMessagePart{
			Type: openai.ChatMessagePartTypeText,
			Text: fmt.Sprintf("File content (%s):\n%s", key, string(data)),
		}, nil
	}

	return &openai.ChatMessagePart{
		Type: openai.ChatMessagePartTypeText,
		Text: fmt.Sprintf("File: %s (type: %s, size: %d bytes) - Content not directly readable", key, contentType, len(data)),
	}, nil
}

// buildImagePart creates an image message part, preferring presigned URL over base64.
func (c *Client) buildImagePart(ctx context.Context, key string, data []byte, contentType string) (*openai.ChatMessagePart, error) {
	// Try presigned URL first — avoids base64 overhead
	presignedURL, err := c.storage.GetPresignedURL(ctx, key, 10*time.Minute)
	if err == nil && !isLocalhostURL(presignedURL) {
		slog.Info("using presigned URL for image", "key", key, "content_type", contentType, "detail", c.imageDetail)
		return &openai.ChatMessagePart{
			Type: openai.ChatMessagePartTypeImageURL,
			ImageURL: &openai.ChatMessageImageURL{
				URL:    presignedURL,
				Detail: c.imageDetail,
			},
		}, nil
	}

	// Fall back to base64 encoding
	const maxBase64Size = 20 * 1024 * 1024
	estimatedBase64Size := (len(data) * 4) / 3
	if estimatedBase64Size > maxBase64Size {
		return nil, fmt.Errorf("image too large for base64 encoding: %d bytes (estimated base64: %d)", len(data), estimatedBase64Size)
	}

	encodedData := base64.StdEncoding.EncodeToString(data)
	imageURL := fmt.Sprintf("data:%s;base64,%s", contentType, encodedData)

	slog.Info("using base64 encoding for image",
		"key", key,
		"content_type", contentType,
		"original_size", len(data),
		"detail", c.imageDetail)

	return &openai.ChatMessagePart{
		Type: openai.ChatMessagePartTypeImageURL,
		ImageURL: &openai.ChatMessageImageURL{
			URL:    imageURL,
			Detail: c.imageDetail,
		},
	}, nil
}

// ProcessStructuredECG calls GPT with temperature=0 and custom prompts for structured ECG measurement.
func (c *Client) ProcessStructuredECG(ctx context.Context, fileKeys []string, systemPrompt, userPrompt string) (*ProcessResult, error) {
	start := time.Now()

	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
	}

	var content []openai.ChatMessagePart
	for _, key := range fileKeys {
		filePart, err := c.createMessagePartFromFile(reqCtx, key)
		if err != nil {
			slog.Error("failed to process file for structured ECG", "key", key, "error", err)
			continue
		}
		if filePart != nil {
			content = append(content, *filePart)
		}
	}

	content = append(content, openai.ChatMessagePart{
		Type: openai.ChatMessagePartTypeText,
		Text: userPrompt,
	})

	messages = append(messages, openai.ChatCompletionMessage{
		Role:         openai.ChatMessageRoleUser,
		MultiContent: content,
	})

	slog.Info("sending structured ECG request to OpenAI",
		"model", c.model, "files", len(fileKeys))

	temp := float32(0.0)
	resp, err := c.openAI.CreateChatCompletion(reqCtx, openai.ChatCompletionRequest{
		Model:       c.model,
		Messages:    messages,
		MaxTokens:   4000,
		Temperature: temp,
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
	})
	if err != nil {
		return nil, classifyOpenAIError(err, reqCtx, c.timeout)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from OpenAI")
	}

	responseContent := resp.Choices[0].Message.Content
	if IsRefusal(responseContent) {
		slog.Warn("openai returned refusal for structured ECG", "tokens", resp.Usage.TotalTokens)
	}

	slog.Info("structured ECG response received",
		"model", resp.Model, "tokens", resp.Usage.TotalTokens, "response_len", len(responseContent))

	return &ProcessResult{
		Content:          responseContent,
		Model:            resp.Model,
		TokensUsed:       resp.Usage.TotalTokens,
		ProcessingTimeMs: int(time.Since(start).Milliseconds()),
	}, nil
}

// isLocalhostURL checks whether a URL points to a local address that OpenAI cannot reach.
func isLocalhostURL(u string) bool {
	return strings.Contains(u, "localhost") || strings.Contains(u, "127.0.0.1") || strings.Contains(u, "::1")
}

// classifyOpenAIError wraps an OpenAI API error with a descriptive message based on its type.
func classifyOpenAIError(err error, reqCtx context.Context, timeout time.Duration) error {
	errStr := err.Error()

	type errClass struct {
		keywords []string
		message  string
		hint     string
	}

	classes := []errClass{
		{[]string{"insufficient_quota", "quota"}, "OpenAI API quota exceeded", "Check your OpenAI account balance and usage limits"},
		{[]string{"invalid_api_key", "authentication"}, "OpenAI API authentication failed", "Check OPENAI_API_KEY environment variable"},
		{[]string{"rate_limit"}, "OpenAI API rate limit exceeded", "Too many requests, please retry later"},
		{[]string{"content_filter", "safety"}, "OpenAI API content filtered", "Request was filtered by content moderation"},
	}

	for _, c := range classes {
		for _, kw := range c.keywords {
			if strings.Contains(errStr, kw) {
				slog.Error(c.message, "error", err, "hint", c.hint)
				return fmt.Errorf("%s: %w", c.message, err)
			}
		}
	}

	if reqCtx.Err() == context.DeadlineExceeded {
		slog.Error("openai API request timeout", "error", err, "timeout", timeout)
		return fmt.Errorf("OpenAI API request timeout: %w", err)
	}

	slog.Error("openai API error", "error", err)
	return fmt.Errorf("OpenAI API error: %w", err)
}

func isImageType(contentType string) bool {
	return validation.IsImageType(contentType)
}

var textTypes = map[string]bool{
	"text/plain":       true,
	"application/json": true,
	"text/csv":         true,
}

func isTextType(contentType string) bool {
	return textTypes[contentType]
}
