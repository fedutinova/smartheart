// Package cv is the Go client for cv_service — the Python rhythm classifier.
package cv

// Layout labels accepted by cv_service.
const (
	Layout3x4Rhythm = "3x4_rhythm"
	Layout3x4       = "3x4"
	Layout6x2       = "6x2"
	Layout6x2Rhythm = "6x2_rhythm"
	Layout12x1      = "12x1"
)

// Preprocessing modes accepted by cv_service.
const (
	PreprocessSynthmatch = "synthmatch"
	PreprocessRaw        = "raw"
	PreprocessLight      = "light"
)

// DefaultLayout / DefaultPreprocess are applied when the caller omits the value.
const (
	DefaultLayout     = Layout3x4Rhythm
	DefaultPreprocess = PreprocessSynthmatch
)

// IsValidLayout reports whether s is one of the supported layout labels.
func IsValidLayout(s string) bool {
	switch s {
	case Layout3x4Rhythm, Layout3x4, Layout6x2, Layout6x2Rhythm, Layout12x1:
		return true
	}
	return false
}

// IsValidPreprocess reports whether s is one of the supported preprocess modes.
func IsValidPreprocess(s string) bool {
	switch s {
	case PreprocessSynthmatch, PreprocessRaw, PreprocessLight:
		return true
	}
	return false
}

// ClassProb is one entry in the top-K rhythm prediction list.
type ClassProb struct {
	Idx     int     `json:"idx"`
	Code    string  `json:"code"`
	LabelRU string  `json:"label_ru"`
	Prob    float64 `json:"prob"`
}

// BinaryFlag is one auxiliary binary indicator (e.g. "stt_label").
// Only flags above the cv_service threshold (currently 0.55) are returned.
type BinaryFlag struct {
	Code    string  `json:"code"`
	LabelRU string  `json:"label_ru"`
	Prob    float64 `json:"prob"`
}

// RhythmPrediction is the structured response returned by POST /predict.
// Field shape matches cv_service/src/ecg_service/inference.py:predict_pil.
type RhythmPrediction struct {
	PredIdx        int          `json:"pred_idx"`
	PredCode       string       `json:"pred_code"`
	PredLabelRU    string       `json:"pred_label_ru"`
	LayoutLabel    string       `json:"layout_label"`
	PreprocessName string       `json:"preprocess_name"`
	Top3           []ClassProb  `json:"top3"`
	BinaryFlags    []BinaryFlag `json:"binary_flags"`
	// RawProbs is omitted from the Go struct — the backend doesn't need the full
	// vector for now. Adding it later is a non-breaking change.
}
