package models

import "fmt"

type RedactionMode string

const (
	RedactionModeBand RedactionMode = "band"
	RedactionModeOCR  RedactionMode = "ocr"
)

func (m RedactionMode) Valid() bool {
	switch m {
	case RedactionModeBand, RedactionModeOCR:
		return true
	default:
		return false
	}
}

type RequestClientMeta struct {
	RedactionMode   RedactionMode `json:"redaction_mode,omitempty"`
	RedactionMs     int           `json:"redaction_ms,omitempty"`
	BoxesCount      int           `json:"boxes_count,omitempty"`
	MaskedAreaRatio float64       `json:"masked_area_ratio,omitempty"`
	ImageWidth      int           `json:"image_width,omitempty"`
	ImageHeight     int           `json:"image_height,omitempty"`
}

func (m *RequestClientMeta) Validate() error {
	if m == nil {
		return nil
	}
	if !m.RedactionMode.Valid() {
		return fmt.Errorf("invalid redaction_mode: %q", m.RedactionMode)
	}
	if m.RedactionMs < 0 {
		return fmt.Errorf("redaction_ms must be >= 0")
	}
	if m.BoxesCount < 0 {
		return fmt.Errorf("boxes_count must be >= 0")
	}
	if m.ImageWidth < 0 || m.ImageHeight < 0 {
		return fmt.Errorf("image dimensions must be >= 0")
	}
	if m.MaskedAreaRatio < 0 || m.MaskedAreaRatio > 1 {
		return fmt.Errorf("masked_area_ratio must be between 0 and 1")
	}
	return nil
}
