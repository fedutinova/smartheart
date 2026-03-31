package gpt

import (
	"encoding/json"
	"fmt"
	"strings"
)

// BuildECGMeasurementPrompt returns system and user messages for structured ECG measurement.
func BuildECGMeasurementPrompt(paperSpeedMMS float64) (system, user string) {
	system = "Измеряй ЭКГ по сетке. Возвращай только JSON. " +
		"Вертикаль/горизонталь — в малых клетках (0.5 допускается). Null — если невозможно."

	schema := ecgSchemaTemplate()
	schemaJSON, _ := json.MarshalIndent(schema, "", "  ")

	user = fmt.Sprintf(`Ты измеряешь ЭКГ на бумаге. Верни ТОЛЬКО JSON строго по схеме.
Измеряй 3–5 последовательных комплексов в каждом видимом отведении.
Вертикаль: количество МАЛЫХ клеток (small squares): вверх — положительные, вниз — отрицательные.
Горизонталь (интервалы): количество малых клеток по X.
Если измерение невозможно — верни null (НЕ 0).
Скорость плёнки по умолчанию: %.0f мм/с (если увидишь явную калибровку — заполни calibration).
HR_bpm — одно число, если возможно.
Разрешены половинки клетки (0.5). Избегай null для заметных отведений.
СХЕМА:
%s
Верни один JSON.`, paperSpeedMMS, string(schemaJSON))

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
			"QRS": []any{}, "RR": []any{},
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

// LeadData holds R and S measurements in small squares.
type LeadData struct {
	RUpSq   []float64 `json:"R_up_sq"`
	SDownSq []float64 `json:"S_down_sq"`
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
		return nil, fmt.Errorf("parse ECG JSON: %w", err)
	}
	return &result, nil
}
