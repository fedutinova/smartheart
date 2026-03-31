package validation

import (
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"
)

const (
	MaxFileSize   = 10 << 20 // 10mb
	MaxFiles      = 5
	MaxTextLength = 4000
)

var AllowedMimeTypes = map[string]bool{
	"image/jpeg":       true,
	"image/jpg":        true,
	"image/png":        true,
	"image/gif":        true,
	"image/webp":       true,
	"image/bmp":        true,
	"image/tiff":       true,
	"application/pdf":  true,
	"text/plain":       true,
	"application/json": true,
	"text/csv":         true,
}

// ImageMimeTypes is the subset of AllowedMimeTypes that are image formats.
var ImageMimeTypes = map[string]bool{
	"image/jpeg": true,
	"image/jpg":  true,
	"image/png":  true,
	"image/gif":  true,
	"image/webp": true,
	"image/bmp":  true,
	"image/tiff": true,
}

// IsImageType reports whether the content type is an image format.
func IsImageType(contentType string) bool {
	return ImageMimeTypes[contentType]
}

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	var messages []string
	for _, err := range e {
		messages = append(messages, err.Error())
	}
	return strings.Join(messages, "; ")
}

func ValidateGPTRequest(textQuery string, files []*multipart.FileHeader) ValidationErrors {
	var errors ValidationErrors

	if len(files) == 0 {
		errors = append(errors, ValidationError{
			Field:   "files",
			Message: "at least one file must be provided",
		})
		return errors
	}

	if textQuery != "" && len(textQuery) > MaxTextLength {
		errors = append(errors, ValidationError{
			Field:   "text_query",
			Message: fmt.Sprintf("text query exceeds maximum length of %d characters", MaxTextLength),
		})
	}

	if len(files) > MaxFiles {
		errors = append(errors, ValidationError{
			Field:   "files",
			Message: fmt.Sprintf("maximum %d files allowed, got %d", MaxFiles, len(files)),
		})
	}

	for i, file := range files {
		if file.Size > MaxFileSize {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("files[%d]", i),
				Message: fmt.Sprintf("file %s exceeds maximum size of %d bytes", file.Filename, MaxFileSize),
			})
			continue
		}

		if file.Size == 0 {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("files[%d]", i),
				Message: fmt.Sprintf("file %s is empty", file.Filename),
			})
			continue
		}

		contentType := file.Header.Get("Content-Type")
		if contentType == "" || contentType == "application/octet-stream" {
			if f, err := file.Open(); err == nil {
				buf := make([]byte, 512)
				n, _ := f.Read(buf)
				contentType = http.DetectContentType(buf[:n])
				_ = f.Close()
			}
		}

		if !AllowedMimeTypes[contentType] {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("files[%d]", i),
				Message: fmt.Sprintf("file %s has unsupported content type: %s", file.Filename, contentType),
			})
		}
	}

	return errors
}
