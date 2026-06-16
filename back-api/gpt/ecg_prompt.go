package gpt

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// BuildECGMeasurementPrompt returns system and user messages for structured ECG measurement.
func BuildECGMeasurementPrompt(paperSpeedMMS float64) (system, user string) {
	system = `You are an ECG measurement engine.
Return ONLY valid JSON matching the schema. No prose, no diagnosis.
Measure only visible ECG grid values in small squares. If the image is not an ECG, a lead is not visible, quality is poor, or a value is uncertain, use null or [].
Never use 0 as "missing". Use 0 only if a clearly visible ECG deflection is truly isoelectric.
Check calibration/speed when visible; otherwise use the supplied speed.`

	schema := ecgSchemaTemplate()
	schemaJSON, _ := json.Marshal(schema)

	user = fmt.Sprintf(`Task: measure the ECG grid and return JSON only.

Grid: 1 small square = 1 mm; 5 small = 1 large; calibration pulse usually 10 mm = 1 mV.
Baseline: isoelectric TP segment.

Leads:
- R_up_sq: small squares from baseline up to R peak, positive number.
- S_down_sq: small squares from baseline down to S nadir, negative number.
- Prefer 3-5 complexes per visible lead; halves allowed (0.5).
- Invisible/uncertain lead or value: null/[]; do NOT output 0 for missing.

Intervals in small squares:
- PR: P onset to QRS onset.
- QRS: QRS width; normal usually 2-4 sq. If uncertain, [].
- RR: consecutive R-to-R; for irregular rhythm return >=3 values.
- QT: QRS onset to T end; exclude U wave.
- JT: J point to T end; useful if QRS is wide.
- If T/U merge or T end unclear, leave QT/JT empty.

Extras when visible: SV1_sq, RV5_sq, RV6_sq, RaVL_sq, SV3_sq, SV4_sq, SV5_sq, SV6_sq, S_deepest_sq.
Expected paper speed: %.0f mm/s; set calibration if visibly different.
HR_bpm only if reliable.

Schema:
%s

Return one JSON object.`, paperSpeedMMS, string(schemaJSON))

	return system, user
}

func ecgSchemaTemplate() map[string]any {
	leadEntry := map[string]any{"R_up_sq": []any{}, "S_down_sq": []any{}}
	leads := make(map[string]any)
	for _, name := range []string{"I", "II", "III", "aVR", "aVL", "aVF", "V1", "V2", "V3", "V4", "V5", "V6"} {
		leads[name] = leadEntry
	}
	return map[string]any{
		"leads": leads,
		"extras": map[string]any{
			"SV1_sq": []any{}, "RV5_sq": []any{}, "RV6_sq": []any{},
			"RaVL_sq": []any{}, "SV3_sq": []any{}, "SV4_sq": []any{},
			"S_deepest_sq": []any{}, "SV5_sq": []any{}, "SV6_sq": []any{},
		},
		"intervals_sq": map[string]any{
			"PR": []any{}, "QRS": []any{}, "RR": []any{}, "QT": []any{}, "JT": []any{},
		},
		"HR_bpm": nil,
		"calibration": map[string]any{
			"mv_pulse_height_small_squares":     nil,
			"paper_speed_small_squares_per_sec": nil,
		},
	}
}

// RawECGMeasurement is the JSON structure returned by GPT.
type RawECGMeasurement struct {
	Leads       map[string]LeadData  `json:"leads"`
	Extras      map[string][]float64 `json:"extras"`
	IntervalsSq map[string][]float64 `json:"intervals_sq"`
	HRBpm       *float64             `json:"HR_bpm"`
	Calibration RawCalibration       `json:"calibration"`
}

// flexFloat64Slice accepts both a single number and an array of numbers from JSON.
// GPT may return either `"R_up_sq": 3.5` or `"R_up_sq": [3.5]`.
type flexFloat64Slice []float64

func (f *flexFloat64Slice) UnmarshalJSON(data []byte) error {
	// null → nil slice.
	if string(data) == "null" {
		*f = nil
		return nil
	}
	// Try array first.
	var arr []float64
	if err := json.Unmarshal(data, &arr); err == nil {
		*f = arr
		return nil
	}
	// Fallback: single number.
	var v float64
	if err := json.Unmarshal(data, &v); err == nil {
		*f = []float64{v}
		return nil
	}
	// Fallback: GPT sometimes returns a stringified number like "3.5".
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		if sv, err := strconv.ParseFloat(s, 64); err == nil {
			*f = []float64{sv}
			return nil
		}
	}
	return fmt.Errorf("flexFloat64Slice: cannot unmarshal %s", string(data))
}

// LeadData holds R and S measurements in small squares.
type LeadData struct {
	RUpSq   flexFloat64Slice `json:"R_up_sq"`
	SDownSq flexFloat64Slice `json:"S_down_sq"`
}

// RawCalibration holds optional calibration detected by GPT.
type RawCalibration struct {
	MvPulseHeight *float64 `json:"mv_pulse_height_small_squares"`
	PaperSpeed    *float64 `json:"paper_speed_small_squares_per_sec"`
}

// ParseECGMeasurementJSON parses GPT's JSON response, stripping markdown fences.
func ParseECGMeasurementJSON(raw string) (*RawECGMeasurement, error) {
	text := strings.TrimSpace(raw)

	// Strip markdown code fences
	if strings.HasPrefix(text, "```") {
		lines := strings.Split(text, "\n")
		start, end := 0, len(lines)
		for i, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "```") {
				if start != 0 {
					end = i
					break
				}
				start = i + 1
			}
		}
		text = strings.Join(lines[start:end], "\n")
	}

	text = strings.TrimSpace(text)

	var result RawECGMeasurement
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return nil, fmt.Errorf("parse ECG JSON (first 2000 chars: %s...): %w", truncateForLog(text, 2000), err)
	}
	return &result, nil
}

func truncateForLog(s string, limit int) string {
	if limit <= 0 || len(s) <= limit {
		return s
	}
	return s[:limit]
}
