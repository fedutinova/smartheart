package workers

import (
	"encoding/json"
	"testing"

	"github.com/fedutinova/smartheart/back-api/cv"
	"github.com/fedutinova/smartheart/back-api/models"
)

func TestRhythmFromCV_Nil(t *testing.T) {
	if got := rhythmFromCV(nil, nil); got != nil {
		t.Errorf("expected nil for nil input, got %+v", got)
	}
}

func TestRhythmFromCV_MapsAllFields(t *testing.T) {
	pred := &cv.RhythmPrediction{
		PredIdx:        1,
		PredCode:       "AFIB",
		PredLabelRU:    "фибрилляция предсердий",
		LayoutLabel:    "3x4_rhythm",
		PreprocessName: "synthmatch",
		Top3: []cv.ClassProb{
			{Idx: 1, Code: "AFIB", LabelRU: "фибрилляция предсердий", Prob: 0.81},
			{Idx: 0, Code: "SINUS_GROUP", LabelRU: "синусовый ритм", Prob: 0.12},
			{Idx: 2, Code: "AFLT", LabelRU: "трепетание предсердий", Prob: 0.04},
		},
		BinaryFlags: []cv.BinaryFlag{
			{Code: "stt_label", LabelRU: "возможны изменения ST-T", Prob: 0.71},
		},
	}

	explanation := &models.ECGRhythmExplanation{
		PredictionLine:  "Ритм: фибрилляция предсердий.",
		DescriptionText: "Нерегулярные RR, нет чётких P.",
		ConclusionText:  "Картина соответствует фибрилляции предсердий.",
		NoteText:        "Не заменяет очного врача.",
	}
	got := rhythmFromCV(pred, explanation)
	if got == nil {
		t.Fatal("expected non-nil")
	}
	if got.Explanation == nil || got.Explanation.PredictionLine == "" {
		t.Errorf("explanation not propagated: %+v", got.Explanation)
	}
	if got.PredCode != "AFIB" || got.PredLabelRU != "фибрилляция предсердий" {
		t.Errorf("pred mapping wrong: %+v", got)
	}
	if got.LayoutLabel != "3x4_rhythm" || got.PreprocessName != "synthmatch" {
		t.Errorf("layout/preprocess mapping wrong: %+v", got)
	}
	if len(got.Top3) != 3 {
		t.Fatalf("Top3 length = %d, want 3", len(got.Top3))
	}
	if got.Top3[0].Code != "AFIB" || got.Top3[0].Prob != 0.81 {
		t.Errorf("Top3[0] = %+v", got.Top3[0])
	}
	// PredIdx is intentionally dropped — frontend doesn't need raw index.
	if len(got.BinaryFlags) != 1 || got.BinaryFlags[0].Code != "stt_label" {
		t.Errorf("BinaryFlags = %+v", got.BinaryFlags)
	}
}

func TestRhythmFromCV_EmptySlices(t *testing.T) {
	pred := &cv.RhythmPrediction{
		PredCode: "SINUS_GROUP",
	}
	got := rhythmFromCV(pred, nil)
	if got == nil {
		t.Fatal("expected non-nil")
	}
	if got.Explanation != nil {
		t.Errorf("expected nil explanation when not provided, got %+v", got.Explanation)
	}
	if got.Top3 == nil || got.BinaryFlags == nil {
		t.Error("expected non-nil empty slices for stable JSON marshalling")
	}
	if len(got.Top3) != 0 || len(got.BinaryFlags) != 0 {
		t.Errorf("expected empty slices, got top3=%d flags=%d", len(got.Top3), len(got.BinaryFlags))
	}
}

// Persisted-shape test: ensures ECGResponseContent serialises the new
// rhythm_result block and that legacy consumers (which ignore unknown fields)
// can still parse the structured_result block.
func TestECGResponseContent_WithRhythmResult_RoundTrip(t *testing.T) {
	original := &models.ECGResponseContent{
		AnalysisType: models.ECGModelStructured,
		Timestamp:    "2026-06-01T12:00:00Z",
		JobID:        "abc",
		RhythmResult: &models.ECGRhythmResult{
			PredCode:       "AFIB",
			PredLabelRU:    "фибрилляция предсердий",
			LayoutLabel:    "3x4_rhythm",
			PreprocessName: "synthmatch",
			Top3: []models.RhythmClassProb{
				{Code: "AFIB", LabelRU: "фибрилляция предсердий", Prob: 0.81},
			},
			BinaryFlags: []models.RhythmBinaryFlag{
				{Code: "stt_label", LabelRU: "возможны изменения ST-T", Prob: 0.71},
			},
		},
	}
	raw, err := original.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	parsed, err := models.ParseECGContent(raw)
	if err != nil {
		t.Fatalf("ParseECGContent: %v", err)
	}
	if parsed == nil {
		t.Fatal("ParseECGContent returned nil")
	}
	if parsed.RhythmResult == nil {
		t.Fatal("RhythmResult was dropped during round-trip")
	}
	if parsed.RhythmResult.PredCode != "AFIB" {
		t.Errorf("PredCode lost: %+v", parsed.RhythmResult)
	}
}

// Verify that the absence of rhythm_result in the JSON yields nil RhythmResult,
// which is what legacy responses look like and what graceful-degradation produces.
func TestECGResponseContent_NoRhythmResult_OmittedFromJSON(t *testing.T) {
	original := &models.ECGResponseContent{
		AnalysisType: models.ECGModelStructured,
		Timestamp:    "2026-06-01T12:00:00Z",
		JobID:        "abc",
		// RhythmResult intentionally nil.
	}
	raw, err := original.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var asMap map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &asMap); err != nil {
		t.Fatalf("Unmarshal to map: %v", err)
	}
	if _, present := asMap["rhythm_result"]; present {
		t.Errorf("rhythm_result must be omitted when nil, got JSON: %s", raw)
	}
}
