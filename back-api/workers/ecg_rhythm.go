package workers

import (
	"context"
	"fmt"

	"github.com/fedutinova/smartheart/back-api/cv"
	"github.com/fedutinova/smartheart/back-api/gpt"
	"github.com/fedutinova/smartheart/back-api/models"
)

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
		Explanation:    explanation,
	}
}

// buildRhythmExplanation runs the post-CV vision-LLM call that turns the
// rhythm prediction into a four-field medical narrative. Returns (nil, err)
// on failure so the caller can decide whether to soft-degrade — in our
// worker we log a warning and still publish the rhythm result without the
// narrative block.
func buildRhythmExplanation(
	ctx context.Context,
	gptClient gpt.Processor,
	imageKey string,
	filename string,
	layoutLabel string,
	pred *cv.RhythmPrediction,
) (*models.ECGRhythmExplanation, error) {
	if pred == nil {
		return nil, fmt.Errorf("buildRhythmExplanation: nil prediction")
	}

	modelResult := gpt.RhythmExplainContext{
		PredCode:    pred.PredCode,
		PredLabelRU: pred.PredLabelRU,
		Top3:        make([]gpt.RhythmExplainClass, 0, len(pred.Top3)),
		BinaryFlags: make([]gpt.RhythmExplainBinary, 0, len(pred.BinaryFlags)),
	}
	for _, c := range pred.Top3 {
		modelResult.Top3 = append(modelResult.Top3, gpt.RhythmExplainClass{
			Code: c.Code, LabelRU: c.LabelRU, Prob: c.Prob,
		})
	}
	for _, f := range pred.BinaryFlags {
		modelResult.BinaryFlags = append(modelResult.BinaryFlags, gpt.RhythmExplainBinary{
			Code: f.Code, LabelRU: f.LabelRU, Prob: f.Prob,
		})
	}

	systemPrompt, userPrompt, err := gpt.BuildRhythmExplainPrompt(filename, layoutLabel, modelResult)
	if err != nil {
		return nil, fmt.Errorf("build rhythm explain prompt: %w", err)
	}

	res, err := gptClient.ExplainECGRhythm(ctx, imageKey, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("gpt explain rhythm: %w", err)
	}

	parsed, err := gpt.ParseRhythmExplanation(res.Content)
	if err != nil {
		return nil, fmt.Errorf("parse rhythm explanation: %w", err)
	}

	return &models.ECGRhythmExplanation{
		PredictionLine:  parsed.PredictionLine,
		DescriptionText: parsed.DescriptionText,
		ConclusionText:  parsed.ConclusionText,
		NoteText:        parsed.NoteText,
	}, nil
}
