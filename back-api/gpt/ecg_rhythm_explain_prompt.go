package gpt

import (
	"encoding/json"
	"fmt"
	"strings"
)

// rhythmExplainSystemPrompt is a faithful port of the bundled llm_service.py
// prompt that ECG-team produced. It instructs the vision LLM to read the CV
// rhythm prediction plus the original image and emit a four-field medical
// explanation in Russian.
const rhythmExplainSystemPrompt = `Ты — модуль автоматизированной интерпретации ЭКГ по фото для врача.
Нужно вернуть строго JSON:
{
  "prediction_line": "...",
  "description_text": "...",
  "conclusion_text": "...",
  "note_text": "..."
}

У каждого поля СВОЯ роль. Не повторяй одну и ту же информацию в разных полях —
название ритма и находки не должны дублироваться из поля в поле:
- prediction_line — короткий заголовок: только название ведущего ритма, одной строкой, без находок и пояснений;
- description_text — что видно на ЭКГ: частота, морфология, значимые признаки; это наблюдения, НЕ повторяй здесь название ритма как готовый вывод;
- conclusion_text — итог одним предложением: ведущий ритм вместе с ключевыми находками; это единственное место, где они сводятся вместе;
- note_text — только оговорки, ограничения снимка и рекомендация клинической корреляции; без диагноза и без повтора находок.

Правила:
- основной ритм бери из model_result;
- смотри и на изображение;
- значимые дополнительные признаки упомяни кратко и только один раз (в description_text);
- не выдумывай лишнего;
- пиши кратко, по-медицински, естественно, на русском;
- не используй технические термины вроде logits/confidence/model says.`

// RhythmExplainContext is the minimal slice of the CV result that we hand to
// the LLM. Mirrors the "model_result" field the bundle prompt expects — same
// keys, so the model treats it identically.
type RhythmExplainContext struct {
	PredCode    string                 `json:"pred_code"`
	PredLabelRU string                 `json:"pred_label_ru"`
	Top3        []RhythmExplainClass   `json:"top3,omitempty"`
	BinaryFlags []RhythmExplainBinary  `json:"binary_flags,omitempty"`
}

// RhythmExplainClass mirrors cv.ClassProb in the slim form the prompt needs.
type RhythmExplainClass struct {
	Code    string  `json:"code"`
	LabelRU string  `json:"label_ru"`
	Prob    float64 `json:"prob"`
}

// RhythmExplainBinary mirrors cv.BinaryFlag.
type RhythmExplainBinary struct {
	Code    string  `json:"code"`
	LabelRU string  `json:"label_ru"`
	Prob    float64 `json:"prob"`
}

// BuildRhythmExplainPrompt returns the (system, user) pair for ExplainECGRhythm.
// The user message is a JSON payload identical in shape to what the original
// bundle's llm_service.py emitted.
func BuildRhythmExplainPrompt(fileName, layoutLabel string, modelResult RhythmExplainContext) (system, user string, err error) {
	payload := map[string]any{
		"file_name":    fileName,
		"layout_label": layoutLabel,
		"model_result": modelResult,
		"instruction":  "Сделай краткое медицинское заключение по фото ЭКГ.",
	}
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", "", fmt.Errorf("marshal rhythm-explain payload: %w", err)
	}
	return rhythmExplainSystemPrompt, string(b), nil
}

// RhythmExplanation is the parsed JSON the LLM returns. Field names match the
// frontend type ECGRhythmExplanation.
type RhythmExplanation struct {
	PredictionLine  string `json:"prediction_line"`
	DescriptionText string `json:"description_text"`
	ConclusionText  string `json:"conclusion_text"`
	NoteText        string `json:"note_text"`
}

// ParseRhythmExplanation parses the LLM JSON, stripping ``` fences if present.
func ParseRhythmExplanation(raw string) (*RhythmExplanation, error) {
	text := strings.TrimSpace(raw)
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
		text = strings.TrimSpace(strings.Join(lines[start:end], "\n"))
	}
	var r RhythmExplanation
	if err := json.Unmarshal([]byte(text), &r); err != nil {
		return nil, fmt.Errorf("parse rhythm explanation JSON: %w", err)
	}
	return &r, nil
}
