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

	"github.com/fedutinova/smartheart/internal/storage"
	"github.com/sashabaranov/go-openai"
)

type Client struct {
	openAI      *openai.Client
	storage     storage.Storage
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
		imageDetail: openai.ImageURLDetailAuto, // Default to Auto for cost/performance balance
		timeout:     60 * time.Second,          // Default timeout
	}
	for _, opt := range opts {
		opt(client)
	}
	return client
}

func (c *Client) ProcessRequest(ctx context.Context, textQuery string, fileKeys []string) (*ProcessResult, error) {
	start := time.Now()

	// Add timeout to context
	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	messages := []openai.ChatCompletionMessage{
		{
			Role: openai.ChatMessageRoleSystem,
			Content: "You are a helpful image analysis assistant that analyzes waveform graphs and technical data visualizations. " +
				"You will receive an image showing a waveform graph with lines and patterns. " +
				"Your task is to describe what you see in the image in Russian language. " +
				"Analyze the image as you would analyze any technical graph or chart.\n\n" +
				"Provide a structured analysis in Russian:\n" +
				"1. Описание качества изображения: четкость, контрастность, видимость всех элементов\n" +
				"2. Описание формы графика: какие линии видны, их направление, паттерны\n" +
				"3. Измерения на графике: если есть разметка, опишите видимые значения\n" +
				"4. Особенности визуализации: любые заметные особенности или изменения в паттерне\n" +
				"5. Технические характеристики: опишите технические параметры, если они видны или указаны\n\n" +
				"This is a technical image analysis task for educational purposes. " +
				"Simply describe what you observe in the image without making any diagnostic conclusions. " +
				"If you cannot see certain details or measurements, state that clearly.",
		},
	}

	var content []openai.ChatMessagePart

	// Add images FIRST, then text query
	// OpenAI recommends this order for better image understanding
	for _, key := range fileKeys {
		filePart, err := c.createMessagePartFromFile(reqCtx, key)
		if err != nil {
			slog.Error("failed to process file", "key", key, "error", err)
			continue
		}
		if filePart != nil {
			content = append(content, *filePart)
			if filePart.Type == openai.ChatMessagePartTypeImageURL {
				// Log only non-sensitive metadata
				slog.Info("added image to GPT request",
					"key", key,
					"detail", filePart.ImageURL.Detail)
			} else {
				slog.Warn("added non-image file to GPT request",
					"key", key,
					"type", filePart.Type)
			}
		} else {
			slog.Error("filePart is nil after processing", "key", key)
		}
	}

	// Add text query AFTER images
	if textQuery != "" {
		content = append(content, openai.ChatMessagePart{
			Type: openai.ChatMessagePartTypeText,
			Text: textQuery,
		})
	}

	if len(content) == 0 {
		return nil, fmt.Errorf("no valid content to process")
	}

	imagesCount := 0
	for _, part := range content {
		if part.Type == openai.ChatMessagePartTypeImageURL {
			imagesCount++
		}
	}

	slog.Info("GPT request prepared",
		"text_query_length", len(textQuery),
		"files_count", len(fileKeys),
		"images_count", imagesCount,
		"content_parts", len(content),
		"has_image", imagesCount > 0)

	// Verify that we have at least one image part
	if imagesCount == 0 && len(fileKeys) > 0 {
		slog.Error("WARNING: fileKeys provided but no images in content!",
			"file_keys", fileKeys,
			"content_parts_count", len(content))
	}

	messages = append(messages, openai.ChatCompletionMessage{
		Role:         openai.ChatMessageRoleUser,
		MultiContent: content,
	})

	slog.Info("sending request to OpenAI",
		"model", openai.GPT4o,
		"messages_count", len(messages),
		"content_parts_in_user_message", len(content))

	resp, err := c.openAI.CreateChatCompletion(reqCtx, openai.ChatCompletionRequest{
		// Using gpt-4o for better image analysis capabilities
		Model:     openai.GPT4o,
		Messages:  messages,
		MaxTokens: 2000, // Increased for detailed medical analysis
	})
	if err != nil {
		// Detailed error logging
		errorStr := err.Error()
		errorDetails := map[string]interface{}{
			"error":         errorStr,
			"model":         openai.GPT4o,
			"messages":      len(messages),
			"content_parts": len(content),
		}

		// Check for specific error types
		if strings.Contains(errorStr, "insufficient_quota") || strings.Contains(errorStr, "quota") {
			slog.Error("OpenAI API error: Insufficient quota/tokens",
				"error_details", errorDetails,
				"hint", "Check your OpenAI account balance and usage limits")
			return nil, fmt.Errorf("OpenAI API quota exceeded: %w", err)
		}
		if strings.Contains(errorStr, "invalid_api_key") || strings.Contains(errorStr, "authentication") {
			slog.Error("OpenAI API error: Invalid API key",
				"error_details", errorDetails,
				"hint", "Check OPENAI_API_KEY environment variable")
			return nil, fmt.Errorf("OpenAI API authentication failed: %w", err)
		}
		if strings.Contains(errorStr, "rate_limit") {
			slog.Error("OpenAI API error: Rate limit exceeded",
				"error_details", errorDetails,
				"hint", "Too many requests, please retry later")
			return nil, fmt.Errorf("OpenAI API rate limit exceeded: %w", err)
		}
		if strings.Contains(errorStr, "content_filter") || strings.Contains(errorStr, "safety") {
			slog.Error("OpenAI API error: Content filtered/safety",
				"error_details", errorDetails,
				"hint", "Request was filtered by content moderation")
			return nil, fmt.Errorf("OpenAI API content filtered: %w", err)
		}
		if reqCtx.Err() == context.DeadlineExceeded {
			slog.Error("OpenAI API error: Timeout",
				"error_details", errorDetails,
				"timeout", c.timeout)
			return nil, fmt.Errorf("OpenAI API request timeout: %w", err)
		}

		slog.Error("OpenAI API error: Unknown error",
			"error_details", errorDetails,
			"full_error", err)
		return nil, fmt.Errorf("OpenAI API error: %w", err)
	}

	// Check response validity BEFORE accessing Choices[0]
	if len(resp.Choices) == 0 {
		slog.Error("OpenAI API returned empty choices",
			"model", resp.Model,
			"tokens_used", resp.Usage.TotalTokens,
			"response_id", resp.ID)
		return nil, fmt.Errorf("no response from OpenAI")
	}

	responseContent := resp.Choices[0].Message.Content

	// Check for refusal messages (in English and Russian)
	refusalPatterns := []string{
		"i'm sorry",
		"i cannot",
		"can't assist",
		"unable to",
		"not able",
		"не могу",
		"извините",
		"не в состоянии",
	}
	isRefusal := false
	for _, pattern := range refusalPatterns {
		if strings.Contains(strings.ToLower(responseContent), pattern) {
			isRefusal = true
			break
		}
	}

	if isRefusal {
		slog.Warn("OpenAI returned refusal message",
			"response_length", len(responseContent),
			"response_preview", func() string {
				if len(responseContent) > 200 {
					return responseContent[:200] + "..."
				}
				return responseContent
			}(),
			"hint", "This might be due to content moderation or safety filters",
			"tokens_used", resp.Usage.TotalTokens,
			"finish_reason", resp.Choices[0].FinishReason)
	}

	slog.Info("received response from OpenAI",
		"model", resp.Model,
		"tokens_prompt", resp.Usage.PromptTokens,
		"tokens_completion", resp.Usage.CompletionTokens,
		"tokens_total", resp.Usage.TotalTokens,
		"response_length", len(responseContent),
		"finish_reason", resp.Choices[0].FinishReason)

	processingTime := time.Since(start)

	return &ProcessResult{
		Content:          resp.Choices[0].Message.Content,
		Model:            resp.Model,
		TokensUsed:       resp.Usage.TotalTokens,
		ProcessingTimeMs: int(processingTime.Milliseconds()),
	}, nil
}

func (c *Client) createMessagePartFromFile(ctx context.Context, key string) (*openai.ChatMessagePart, error) {
	// First, check if we need to detect content type
	reader, contentType, err := c.storage.GetFile(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get file from storage: %w", err)
	}

	// Detect content type if not provided or generic
	if contentType == "" || contentType == "application/octet-stream" {
		buffer := make([]byte, 512)
		n, readErr := reader.Read(buffer)
		reader.Close() // Close after reading header

		if readErr != nil && readErr != io.EOF {
			return nil, fmt.Errorf("failed to read file header: %w", readErr)
		}

		detected := http.DetectContentType(buffer[:n])
		if detected != "application/octet-stream" {
			contentType = detected
			slog.Debug("detected content type", "key", key, "detected_type", contentType)
		}
	} else {
		reader.Close() // Close if we don't need it
	}

	// For images, check if we can use presigned URL or need base64
	if isImageType(contentType) {
		// Try to get presigned URL first
		presignedURL, err := c.storage.GetPresignedURL(ctx, key, 10*time.Minute)
		if err != nil {
			slog.Warn("failed to get presigned URL, falling back to base64", "key", key, "error", err)
			// Fall through to base64 encoding
		} else {
			// Check if URL is publicly accessible (not localhost)
			// OpenAI cannot access localhost URLs
			if !strings.Contains(presignedURL, "localhost") && !strings.Contains(presignedURL, "127.0.0.1") && !strings.Contains(presignedURL, "::1") {
				slog.Info("using presigned URL for image",
					"key", key,
					"content_type", contentType,
					"detail", c.imageDetail)

				return &openai.ChatMessagePart{
					Type: openai.ChatMessagePartTypeImageURL,
					ImageURL: &openai.ChatMessageImageURL{
						URL:    presignedURL,
						Detail: c.imageDetail,
					},
				}, nil
			}
			slog.Info("presigned URL is localhost, using base64 instead",
				"key", key,
				"url", presignedURL)
			// Fall through to base64 encoding for localhost
		}

		// For local storage or when presigned URL is not available/public,
		// use base64 encoding (less efficient but works everywhere)
		reader, _, err := c.storage.GetFile(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("failed to re-open file for base64 encoding: %w", err)
		}
		defer reader.Close()

		data, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("failed to read file data: %w", err)
		}

		// Check size limit (OpenAI limit is ~20MB for base64)
		const maxBase64Size = 20 * 1024 * 1024     // 20MB
		estimatedBase64Size := (len(data) * 4) / 3 // Approximate base64 size
		if estimatedBase64Size > maxBase64Size {
			return nil, fmt.Errorf("image too large for base64 encoding: %d bytes (estimated base64: %d)", len(data), estimatedBase64Size)
		}

		encodedData := base64.StdEncoding.EncodeToString(data)

		imageURL := fmt.Sprintf("data:%s;base64,%s", contentType, encodedData)

		slog.Info("using base64 encoding for image",
			"key", key,
			"content_type", contentType,
			"original_size", len(data),
			"base64_size", len(encodedData),
			"detail", c.imageDetail)

		return &openai.ChatMessagePart{
			Type: openai.ChatMessagePartTypeImageURL,
			ImageURL: &openai.ChatMessageImageURL{
				URL:    imageURL,
				Detail: c.imageDetail,
			},
		}, nil
	}

	// For non-images, read the full data
	reader, _, err = c.storage.GetFile(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to re-open file for reading: %w", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read file data: %w", err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("file is empty: %s", key)
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

func isImageType(contentType string) bool {
	imageTypes := map[string]bool{
		"image/jpeg": true,
		"image/jpg":  true,
		"image/png":  true,
		"image/gif":  true,
		"image/webp": true,
	}
	return imageTypes[contentType]
}

func isTextType(contentType string) bool {
	textTypes := map[string]bool{
		"text/plain":       true,
		"application/json": true,
		"text/csv":         true,
	}
	return textTypes[contentType]
}
