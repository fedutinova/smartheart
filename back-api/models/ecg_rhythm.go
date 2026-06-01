package models

// ECGRhythmResult is the rhythm-classification block produced by cv_service
// and embedded into ECGResponseContent alongside the existing measurement
// result. Nil when CV inference was unavailable or failed.
type ECGRhythmResult struct {
	PredCode       string             `json:"pred_code"`
	PredLabelRU    string             `json:"pred_label_ru"`
	LayoutLabel    string             `json:"layout_label"`
	PreprocessName string             `json:"preprocess_name"`
	Top3           []RhythmClassProb  `json:"top3"`
	BinaryFlags    []RhythmBinaryFlag `json:"binary_flags"`
}

// RhythmClassProb is one entry of the top-K rhythm prediction list.
type RhythmClassProb struct {
	Code    string  `json:"code"`
	LabelRU string  `json:"label_ru"`
	Prob    float64 `json:"prob"`
}

// RhythmBinaryFlag is one auxiliary binary indicator (e.g. ST-T abnormalities).
// cv_service emits flags above its internal probability threshold only.
type RhythmBinaryFlag struct {
	Code    string  `json:"code"`
	LabelRU string  `json:"label_ru"`
	Prob    float64 `json:"prob"`
}
