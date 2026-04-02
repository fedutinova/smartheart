package gpt

import (
	"encoding/json"
	"fmt"
	"strings"
)

// BuildECGMeasurementPrompt returns system and user messages for structured ECG measurement.
func BuildECGMeasurementPrompt(paperSpeedMMS float64) (system, user string) {
	system = `Ты эксперт по измерению ЭКГ на бумажных плёнках. Твоя задача: точно посчитать количество МАЛЫХ клеток (1мм) для амплитуд зубцов и интервалов. Возвращай только JSON.`

	schema := ecgSchemaTemplate()
	schemaJSON, _ := json.MarshalIndent(schema, "", "  ")

	user = fmt.Sprintf(`ЗАДАЧА: Измерь ЭКГ по сетке. Верни ТОЛЬКО JSON строго по схеме.

СЕТКА ЭКГ:
- Малая клетка = 1мм (тонкие линии)
- Большая клетка = 5 малых = 5мм (толстые линии)
- Калибровочный импульс (обычно слева): 10мм = 1мВ

КАК ИЗМЕРЯТЬ:
1. Найди изоэлектрическую линию (baseline) — горизонтальный участок между зубцами T и P
2. R_up_sq: количество МАЛЫХ клеток от baseline ВВЕРХ до вершины зубца R (всегда положительное число)
3. S_down_sq: количество МАЛЫХ клеток от baseline ВНИЗ до дна зубца S (всегда отрицательное число)
4. Измеряй 3-5 последовательных комплексов в каждом видимом отведении
5. Разрешены половинки клетки (0.5)
6. Если отведение не видно или не удается измерить — верни null (НЕ 0)

ТИПИЧНЫЕ ЗНАЧЕНИЯ (для самопроверки):
- R в V1: обычно 1-6 малых клеток (маленький зубец)
- S в V1: обычно 8-20 малых клеток (глубокий зубец, отрицательный)
- R в V5-V6: обычно 10-25 малых клеток (высокий зубец)
- S в V5-V6: обычно 0-5 малых клеток (маленький или отсутствует)
- R нарастает от V1 к V4-V5, затем уменьшается к V6
- S уменьшается от V1 к V6
- Если все отведения показывают одинаковую амплитуду — скорее всего ошибка измерения

ИНТЕРВАЛЫ:
- QRS: ширина комплекса QRS в малых клетках (обычно 2-4 клетки)
- RR: расстояние между двумя соседними R-зубцами в малых клетках

EXTRAS:
- SV1_sq: глубина S в V1 (отрицательное число)
- RV5_sq, RV6_sq: высота R в V5 и V6
- RaVL_sq: высота R в aVL
- SV3_sq, SV4_sq: глубина S в V3 и V4
- S_deepest_sq: самый глубокий S среди всех грудных отведений

Скорость плёнки: %.0f мм/с (если видишь другую калибровку — укажи в calibration).
HR_bpm: частота сердечных сокращений, если можно определить.

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
