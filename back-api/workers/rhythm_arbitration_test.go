package workers

import (
	"strings"
	"testing"

	"github.com/fedutinova/smartheart/back-api/gpt"
	"github.com/fedutinova/smartheart/back-api/models"
)

func rhythmWithConf(label string, conf float64) *models.ECGRhythmResult {
	return &models.ECGRhythmResult{
		PredLabelRU: label,
		Top3:        []models.RhythmClassProb{{LabelRU: label, Prob: conf}},
	}
}

func interp(code, labelRU string, agrees bool) *gpt.ECGInterpretationResult {
	return &gpt.ECGInterpretationResult{
		InterpretationMD: "## Итог\n- текст",
		Rhythm:           &gpt.ECGInterpretedRhythm{Code: code, LabelRU: labelRU, AgreesWithClassifier: agrees},
	}
}

func TestArbitrate_HighConfidence_ShowsBadgeAndStatesConfidence(t *testing.T) {
	r := rhythmWithConf("синусовый ритм", 0.93)
	out := arbitrateRhythmDisplay(r, interp("SINUS_GROUP", "синусовый ритм", true), "## Итог\n- вывод")

	if r.Suppressed {
		t.Error("high-confidence rhythm must not be suppressed")
	}
	if r.Confidence != 0.93 {
		t.Errorf("confidence not recorded: %v", r.Confidence)
	}
	if !strings.Contains(out, "93%") || !strings.Contains(out, "синусовый ритм") {
		t.Errorf("conclusion should state confidence: %q", out)
	}
}

func TestArbitrate_LowConfidence_Agrees_ShowsBadgeNoNote(t *testing.T) {
	r := rhythmWithConf("желудочковая тахикардия", 0.43)
	base := "## Итог\n- вывод"
	// GPT says "wide-complex tachycardia" but flags it as compatible (agrees).
	out := arbitrateRhythmDisplay(r, interp("VTAC", "ширококомплексная тахикардия", true), base)

	if r.Suppressed {
		t.Error("low confidence but agreeing rhythm should still show the badge")
	}
	if out != base {
		t.Errorf("no note expected when rhythms agree, got %q", out)
	}
}

func TestArbitrate_LowConfidence_Disagrees_SuppressesAndAdvisesDifferential(t *testing.T) {
	r := rhythmWithConf("трепетание предсердий", 0.48)
	out := arbitrateRhythmDisplay(r, interp("AFIB", "фибрилляция предсердий", false), "## Итог\n- вывод")

	if !r.Suppressed {
		t.Error("low confidence + disagreement must suppress the badge")
	}
	if !strings.Contains(out, "дифференциальная диагностика") {
		t.Errorf("conclusion should advise differential: %q", out)
	}
	// Both competing rhythm names must be stated.
	if !strings.Contains(out, "трепетание предсердий") || !strings.Contains(out, "фибрилляция предсердий") {
		t.Errorf("conclusion should name both rhythms: %q", out)
	}
}

func TestArbitrate_LowConfidence_NoGPTRhythm_KeepsBadge(t *testing.T) {
	r := rhythmWithConf("наджелудочковая тахикардия", 0.50)
	// parse failed → interp nil: do not suppress on missing data.
	out := arbitrateRhythmDisplay(r, nil, "## Итог\n- вывод")

	if r.Suppressed {
		t.Error("must not suppress when there is no GPT rhythm read")
	}
	if out != "## Итог\n- вывод" {
		t.Errorf("no note expected, got %q", out)
	}
}

func TestArbitrate_NilRhythm_Noop(t *testing.T) {
	if out := arbitrateRhythmDisplay(nil, interp("AFIB", "фибрилляция предсердий", false), "x"); out != "x" {
		t.Errorf("nil rhythm should return conclusion unchanged, got %q", out)
	}
}
