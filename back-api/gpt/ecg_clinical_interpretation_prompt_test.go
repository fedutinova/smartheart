package gpt

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/fedutinova/smartheart/back-api/models"
)

func fptr(v float64) *float64 { return &v }

func TestBuildSlimContext_DropsZeroAndInvalid(t *testing.T) {
	structured := &models.ECGStructuredResult{
		Patient: models.PatientInfo{Sex: "male"},
		Rhythm: &models.RhythmTiming{
			PRms:   fptr(0),   // invalid → dropped
			QRSms:  fptr(90),  // valid
			HRbpm:  fptr(72),  // valid
			QTms:   nil,       // missing → dropped
			RRms:   fptr(833), // valid
		},
		Indices: &models.LVHIndices{
			SokolowLyon:    fptr(2.1),
			CornellVoltage: fptr(0), // invalid → dropped
		},
		Axis: &models.QRSAxis{AxisDeg: fptr(-15), Classification: "нормальная"},
	}

	ctx := buildSlimInterpretationContext(structured, nil)

	intervals, ok := ctx["intervals"].(map[string]any)
	if !ok {
		t.Fatalf("intervals missing: %+v", ctx)
	}
	if _, exists := intervals["PR_ms"]; exists {
		t.Error("PR_ms=0 should be dropped from slim context")
	}
	if _, exists := intervals["QT_ms"]; exists {
		t.Error("nil QT_ms should be absent from slim context")
	}
	if intervals["QRS_ms"] != 90.0 || intervals["HR_bpm"] != 72.0 {
		t.Errorf("valid intervals missing/wrong: %+v", intervals)
	}

	indices, ok := ctx["lvh_indices"].(map[string]any)
	if !ok {
		t.Fatalf("lvh_indices missing")
	}
	if _, exists := indices["cornell_voltage_mV"]; exists {
		t.Error("cornell_voltage 0 should be dropped")
	}

	axis, ok := ctx["axis"].(map[string]any)
	if !ok || axis["axis_deg"] != -15.0 {
		t.Errorf("axis (with negative deg) should be kept: %+v", ctx["axis"])
	}
}

func TestBuildSlimContext_NoMeasurementDumpOrInterpretation(t *testing.T) {
	// The full measurement map and the deterministic interpretation block must
	// NOT leak into the GPT context (token bloat + risk of discussing artifacts).
	structured := &models.ECGStructuredResult{
		Measurements: map[string]*float64{"R_I_mm": fptr(12)},
		Interpretation: &models.ECGInterpretation{
			TextSummary: "should-not-appear",
		},
		Rhythm: &models.RhythmTiming{QRSms: fptr(90)},
	}

	_, user, err := BuildECGClinicalInterpretationPrompt(structured, nil, "", 25, 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, leaked := range []string{"measurements", "should-not-appear", "text_summary", "R_I_mm"} {
		if strings.Contains(user, leaked) {
			t.Errorf("slim prompt unexpectedly contains %q", leaked)
		}
	}
}

func TestBuildSlimContext_RhythmClassifierIncluded(t *testing.T) {
	rhythm := &models.ECGRhythmResult{
		PredCode:    "AFIB",
		PredLabelRU: "Фибрилляция предсердий",
		BinaryFlags: []models.RhythmBinaryFlag{{Code: "STT", LabelRU: "ST-T", Prob: 0.8}},
	}
	ctx := buildSlimInterpretationContext(&models.ECGStructuredResult{}, rhythm)

	classifier, ok := ctx["rhythm_classifier"].(map[string]any)
	if !ok {
		t.Fatalf("rhythm_classifier missing: %+v", ctx)
	}
	if classifier["pred_code"] != "AFIB" {
		t.Errorf("pred_code wrong: %+v", classifier)
	}
	if _, ok := classifier["quality_flags"]; !ok {
		t.Error("binary flags should be exposed as quality_flags")
	}

	// Round-trips as JSON without error.
	if _, err := json.Marshal(ctx); err != nil {
		t.Fatalf("slim context not JSON-serializable: %v", err)
	}
}
