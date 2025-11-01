package gpt

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/fedutinova/smartheart/internal/storage"
	"github.com/sashabaranov/go-openai"
)

type Client struct {
	openAI  *openai.Client
	storage storage.Storage
}

type ProcessResult struct {
	Content          string
	Model            string
	TokensUsed       int
	ProcessingTimeMs int
}

func NewClient(apiKey string, storageService storage.Storage) *Client {
	return &Client{
		openAI:  openai.NewClient(apiKey),
		storage: storageService,
	}
}

func (c *Client) ProcessRequest(ctx context.Context, textQuery string, fileKeys []string) (*ProcessResult, error) {
	start := time.Now()

	messages := []openai.ChatCompletionMessage{
		{
			Role: openai.ChatMessageRoleSystem,
			Content: "You are an experienced cardiologist analyzing EKG (ECG) images. Your task is to provide medical interpretation of electrocardiogram images. " +
				"You will receive an image of an EKG tracing. Please analyze it carefully and provide: " +
				"1. Assessment of image quality (clarity, contrast, completeness of all leads)\n" +
				"2. Heart rate calculation (bpm)\n" +
				"3. Rhythm source and type (sinus rhythm, nodal, atrial, fibrillation, etc.)\n" +
				"4. Electrical axis of the heart (EOS) in degrees\n" +
				"5. Measurement of PR, QRS, QTc intervals\n" +
				"6. Description of any pathologies (hypertrophy, blocks, ST-T ischemic changes, extrasystoles, etc.)\n" +
				"7. Final diagnostic conclusion\n\n" +
				"This is a medical image analysis task for educational and informational purposes. " +
				"You are analyzing a real EKG image that has been provided. Please proceed with the analysis.",
		},
	}

	var content []openai.ChatMessagePart

	// Add images FIRST, then text query
	// OpenAI recommends this order for better image understanding
	for _, key := range fileKeys {
		filePart, err := c.createMessagePartFromFile(ctx, key)
		if err != nil {
			slog.Error("failed to process file", "key", key, "error", err)
			continue
		}
		if filePart != nil {
			content = append(content, *filePart)
			if filePart.Type == openai.ChatMessagePartTypeImageURL {
				slog.Info("added image to GPT request",
					"key", key,
					"image_url_length", len(filePart.ImageURL.URL))
			} else {
				slog.Debug("added file to GPT request", "key", key, "type", filePart.Type)
			}
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
		"content_parts", len(content))

	messages = append(messages, openai.ChatCompletionMessage{
		Role:         openai.ChatMessageRoleUser,
		MultiContent: content,
	})

	slog.Info("sending request to OpenAI",
		"model", openai.GPT4o,
		"messages_count", len(messages),
		"content_parts_in_user_message", len(content))

	resp, err := c.openAI.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		// Using gpt-4o for better image analysis capabilities
		Model:     openai.GPT4o,
		Messages:  messages,
		MaxTokens: 2000, // Increased for detailed medical analysis
	})
	if err != nil {
		slog.Error("OpenAI API error", "error", err, "model", openai.GPT4o)
		return nil, fmt.Errorf("OpenAI API error: %w", err)
	}

	responseContent := resp.Choices[0].Message.Content
	responsePreview := responseContent
	if len(responseContent) > 200 {
		responsePreview = responseContent[:200] + "..."
	}
	slog.Info("received response from OpenAI",
		"model", resp.Model,
		"tokens_used", resp.Usage.TotalTokens,
		"response_length", len(responseContent),
		"response_preview", responsePreview)

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from OpenAI")
	}

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

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read file data: %w", err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("file is empty: %s", key)
	}

	slog.Info("processing file for GPT",
		"key", key,
		"content_type", contentType,
		"size_bytes", len(data),
		"is_image", isImageType(contentType))

	if isImageType(contentType) {
		encodedData := base64.StdEncoding.EncodeToString(data)
		encodedSize := len(encodedData)

		// OpenAI has a limit on base64 size (usually around 20MB)
		if encodedSize > 20*1024*1024 {
			slog.Warn("image is too large for base64 encoding",
				"key", key,
				"encoded_size", encodedSize,
				"original_size", len(data))
			return nil, fmt.Errorf("image too large: %d bytes (encoded)", encodedSize)
		}

		imageURL := fmt.Sprintf("data:%s;base64,%s", contentType, encodedData)

		slog.Info("created image URL for GPT",
			"key", key,
			"url_length", len(imageURL),
			"content_type", contentType,
			"original_size_bytes", len(data),
			"base64_size", encodedSize,
			"url_preview", imageURL[:min(100, len(imageURL))]+"...") // First 100 chars for debugging

		return &openai.ChatMessagePart{
			Type: openai.ChatMessagePartTypeImageURL,
			ImageURL: &openai.ChatMessageImageURL{
				URL:    imageURL,
				Detail: openai.ImageURLDetailHigh, // High detail for precise medical image analysis
			},
		}, nil
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
