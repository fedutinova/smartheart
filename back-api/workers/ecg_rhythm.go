package workers

import (
	"github.com/fedutinova/smartheart/back-api/cv"
	"github.com/fedutinova/smartheart/back-api/models"
)

// rhythmFromCV maps the cv_service prediction DTO to the persisted model.
// Returns nil if pred is nil so callers can safely propagate the absence of
// a CV result into ECGResponseContent.RhythmResult.
func rhythmFromCV(pred *cv.RhythmPrediction) *models.ECGRhythmResult {
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
			Code:    f.Code,
			LabelRU: f.LabelRU,
			Prob:    f.Prob,
		})
	}
	return &models.ECGRhythmResult{
		PredCode:       pred.PredCode,
		PredLabelRU:    pred.PredLabelRU,
		LayoutLabel:    pred.LayoutLabel,
		PreprocessName: pred.PreprocessName,
		Top3:           top3,
		BinaryFlags:    flags,
	}
}
