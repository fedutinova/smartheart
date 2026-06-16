package workers

import (
	"math"

	"github.com/fedutinova/smartheart/back-api/cv"
	"github.com/fedutinova/smartheart/back-api/gpt"
	"github.com/fedutinova/smartheart/back-api/models"
)

const minMeasuredLeadsForRhythm = 4

func hasEnoughECGSignalForRhythm(raw *gpt.RawECGMeasurement) bool {
	return countMeasuredLeads(raw) >= minMeasuredLeadsForRhythm
}

func countMeasuredLeads(raw *gpt.RawECGMeasurement) int {
	if raw == nil {
		return 0
	}
	count := 0
	for _, lead := range allLeads {
		data, ok := raw.Leads[lead]
		if !ok {
			continue
		}
		if hasFiniteSample(data.RUpSq) || hasFiniteSample(data.SDownSq) {
			count++
		}
	}
	return count
}

func hasFiniteSample(vals []float64) bool {
	for _, v := range vals {
		if !math.IsNaN(v) && !math.IsInf(v, 0) {
			return true
		}
	}
	return false
}

// rhythmFromCV maps the cv_service prediction DTO to the persisted model.
// Returns nil if pred is nil so callers can safely propagate the absence of
// a CV result into ECGResponseContent.RhythmResult. explanation may be nil
// when the post-CV LLM call failed — graceful degradation, see the worker.
func rhythmFromCV(pred *cv.RhythmPrediction, explanation *models.ECGRhythmExplanation) *models.ECGRhythmResult {
	if pred == nil {
		return nil
	}
	top3 := make([]models.RhythmClassProb, 0, len(pred.Top3))
	for _, c := range pred.Top3 {
		top3 = append(top3, models.RhythmClassProb{
			Code:    c.Code,
			LabelRU: c.LabelRU,
			Prob:    c.Prob,
		})
	}
	flags := make([]models.RhythmBinaryFlag, 0, len(pred.BinaryFlags))
	for _, f := range pred.BinaryFlags {
		flags = append(flags, models.RhythmBinaryFlag{
			Code:      f.Code,
			LabelRU:   f.LabelRU,
			Prob:      f.Prob,
			Threshold: f.Threshold,
		})
	}
	return &models.ECGRhythmResult{
		PredCode:       pred.PredCode,
		PredLabelRU:    pred.PredLabelRU,
		LayoutLabel:    pred.LayoutLabel,
		PreprocessName: pred.PreprocessName,
		Top3:           top3,
		BinaryFlags:    flags,
		Explanation:    explanation,
	}
}
