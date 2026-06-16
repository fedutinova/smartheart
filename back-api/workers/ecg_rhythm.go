package workers

import (
	"fmt"
	"math"
	"strings"

	"github.com/fedutinova/smartheart/back-api/cv"
	"github.com/fedutinova/smartheart/back-api/gpt"
	"github.com/fedutinova/smartheart/back-api/models"
)

const minMeasuredLeadsForRhythm = 4

// rhythmConfidenceThreshold is the classifier top-1 probability below which we
// no longer trust the rhythm badge on its own: at or above it we show the CV
// rhythm (and state the confidence); below it we defer to whether the GPT
// interpretation corroborates the rhythm.
const rhythmConfidenceThreshold = 0.55

// rhythmTop1Confidence returns the classifier's leading-class probability, or 0
// when no Top3 is available.
func rhythmTop1Confidence(r *models.ECGRhythmResult) float64 {
	if r == nil || len(r.Top3) == 0 {
		return 0
	}
	return r.Top3[0].Prob
}

// arbitrateRhythmDisplay decides whether the CV rhythm badge is shown and what
// is appended to the interpretation conclusion, based on classifier confidence
// vs the GPT interpretation's own rhythm read. It records Confidence and may
// set Suppressed on rhythm, and returns the (possibly amended) conclusion.
//
//   - confidence ≥ threshold: show the badge and state the confidence;
//   - confidence < threshold and GPT corroborates the rhythm: show the badge as-is;
//   - confidence < threshold and GPT disagrees: hide the badge and advise a
//     differential. With no GPT rhythm read (parse failed), the badge is kept —
//     we do not suppress on missing data.
func arbitrateRhythmDisplay(rhythm *models.ECGRhythmResult, interp *gpt.ECGInterpretationResult, conclusion string) string {
	if rhythm == nil {
		return conclusion
	}
	conf := rhythmTop1Confidence(rhythm)
	rhythm.Confidence = conf

	var gptRhythm *gpt.ECGInterpretedRhythm
	if interp != nil {
		gptRhythm = interp.Rhythm
	}

	switch {
	case conf >= rhythmConfidenceThreshold:
		return appendParagraph(conclusion, fmt.Sprintf(
			"Классификатор ритма определил «%s» с уверенностью %.0f%%.",
			rhythm.PredLabelRU, conf*100))
	case gptRhythm != nil && !gptRhythm.AgreesWithClassifier:
		rhythm.Suppressed = true
		note := fmt.Sprintf("Оценки ритма расходятся при низкой уверенности: классификатор — «%s»", rhythm.PredLabelRU)
		if lbl := strings.TrimSpace(gptRhythm.LabelRU); lbl != "" {
			note += fmt.Sprintf(", интерпретация по изображению — «%s»", lbl)
		}
		note += ". Требуется дифференциальная диагностика ритма."
		return appendParagraph(conclusion, note)
	default:
		// Low confidence but GPT corroborates, or no GPT rhythm read: keep the
		// badge as-is without extra notes.
		return conclusion
	}
}

// appendParagraph appends note as a new paragraph, handling an empty base.
func appendParagraph(base, note string) string {
	if base == "" {
		return note
	}
	return base + "\n\n" + note
}

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
