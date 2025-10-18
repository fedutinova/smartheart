package gpt

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/sashabaranov/go-openai"
)

type Client struct {
	openAI   *openai.Client
	s3Client *s3.Client
	bucket   string
}

type ProcessResult struct {
	Content          string
	Model            string
	TokensUsed       int
	ProcessingTimeMs int
}

func NewClient(apiKey, s3Bucket string, s3Client *s3.Client) *Client {
	return &Client{
		openAI:   openai.NewClient(apiKey),
		s3Client: s3Client,
		bucket:   s3Bucket,
	}
}

func (c *Client) ProcessRequest(ctx context.Context, textQuery string, fileKeys []string) (*ProcessResult, error) {
	start := time.Now()

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: "You are a helpful AI assistant that can analyze images and documents. Provide detailed and accurate responses based on the content provided.",
		},
	}

	var content []openai.ChatMessagePart
	if textQuery != "" {
		content = append(content, openai.ChatMessagePart{
			Type: openai.ChatMessagePartTypeText,
			Text: textQuery,
		})
	}

	for _, key := range fileKeys {
		filePart, err := c.createMessagePartFromS3File(ctx, key)
		if err != nil {
			slog.Error("failed to process file", "key", key, "error", err)
			continue
		}
		if filePart != nil {
			content = append(content, *filePart)
		}
	}

	if len(content) == 0 {
		return nil, fmt.Errorf("no valid content to process")
	}

	messages = append(messages, openai.ChatCompletionMessage{
		Role:         openai.ChatMessageRoleUser,
		MultiContent: content,
	})

	resp, err := c.openAI.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		// gpt-4o-mini-2024-07-18
		Model:     openai.GPT4oMini20240718,
		Messages:  messages,
		MaxTokens: 1000,
	})
	if err != nil {
		return nil, fmt.Errorf("OpenAI API error: %w", err)
	}

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

func (c *Client) createMessagePartFromS3File(ctx context.Context, key string) (*openai.ChatMessagePart, error) {
	result, err := c.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &c.bucket,
		Key:    &key,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get file from S3: %w", err)
	}
	defer result.Body.Close()

	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read file data: %w", err)
	}

	contentType := "application/octet-stream"
	if result.ContentType != nil {
		contentType = *result.ContentType
	}

	if isImageType(contentType) {
		encodedData := base64.StdEncoding.EncodeToString(data)
		imageURL := fmt.Sprintf("data:%s;base64,%s", contentType, encodedData)

		return &openai.ChatMessagePart{
			Type: openai.ChatMessagePartTypeImageURL,
			ImageURL: &openai.ChatMessageImageURL{
				URL:    imageURL,
				Detail: openai.ImageURLDetailAuto,
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
