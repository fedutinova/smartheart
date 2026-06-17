package gpt

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/fedutinova/smartheart/back-api/models"
)

const ecgClinicalInterpretationSystemPrompt = `You are a cardiologist-electrophysiologist interpreting an ECG image.

Output MUST be a single JSON object, no prose and no code fences, shaped exactly:
{"interpretation_md": "<Russian Markdown>", "rhythm": {"code": "<RHYTHM_CODE>", "label_ru": "<short Russian rhythm name>", "agrees_with_classifier": <true|false>}}
The interpretation_md value MUST be Russian Markdown.
Use AHA/ACC/HRS and ESC-style ECG criteria. Describe ECG patterns, not clinical diagnoses.
Use the image first; use JSON measurements only when plausible. Never invent missing values.
The JSON context contains ONLY values that passed validation; anything not present was unmeasurable — treat it as unknown, do not guess it.
Do not mention JSON, internal fields, "digital/automatic context", processing errors, or invalid values such as 0/300 in the user-facing text.

Hard rules:
- Do NOT name or restate the heart rhythm in interpretation_md (no "синусовый ритм", "фибрилляция/трепетание предсердий", "тахикардия/брадикардия", etc.). The rhythm is determined separately by a classifier and shown to the user elsewhere; in the text assess only conduction (PR/AV, QRS width, blocks) and morphology. Still report your rhythm read in the structured "rhythm" field.
- AV block I only if PR_ms > 200 ms or a clearly measured PR duration supports it.
- Complete BBB only if QRS_ms >= 120 ms and morphology criteria are visible in specific leads.
- Do not explain ST-T changes by BBB unless BBB criteria are met.
- Check ST elevation/depression, contiguous leads, reciprocal changes, and pathologic Q waves separately.
- When STEMI criteria are met (ST elevation at the J-point in >=2 anatomically contiguous leads — typically >=1 mm, or >=2 mm in V2-V3 men / >=1.5 mm V2-V3 women — especially with reciprocal depression), you MUST explicitly name the pattern in interpretation_md as "паттерн инфаркта миокарда с подъёмом ST (STEMI)" and state the territory (передний/нижний/боковой/перегородочный/задний) with the leads that support it. STEMI/инфаркт/ишемия are ECG PATTERN terms — naming them when criteria are visible is required, NOT a forbidden clinical diagnosis. Do NOT soften a clear STEMI to a vague "подъём ST" or "изменения ST-T".
- If ST elevation is present but does not meet full STEMI thresholds or contiguity, say so explicitly (e.g. "подъём ST, не достигающий критериев STEMI") rather than omitting it.
- If you suspect LBBB, double-check you are not mistaken: confirm true LBBB morphology (QRS >= 120 ms, broad/notched R in I/aVL/V5-V6, no septal q, dominant S/QS in V1-V2) before calling it. Discordant ST-T changes are EXPECTED in LBBB and must not be read as ischemia/infarction on their own. Conversely, do not let a presumed LBBB mask a real infarction — if QRS is not truly wide or morphology does not fit LBBB, the ST/T changes may be infarct-related and should be assessed as such.
- If LBBB is suspected while assessing infarction pattern, mention Sgarbossa/modified Sgarbossa only if applied.
- If uncertain, write "данных недостаточно" or "неопределимо"; use "вероятно" only with visible criteria.
- No treatment, dosing, or emergency instructions. Naming ECG pattern terms (including STEMI/инфаркт/ишемия) when their criteria are visible is allowed and expected; what is forbidden is therapy and management advice, not the pattern name.`

// validPos reports whether p is a usable positive measurement (non-nil, finite,
// strictly positive). Intervals, rates and amplitude indices are only meaningful
// when positive, so a 0 or negative value is treated as "not measured".
func validPos(p *float64) bool {
	return p != nil && !math.IsNaN(*p) && !math.IsInf(*p, 0) && *p > 0
}

// putPos inserts key only when the value is a valid positive number, so the slim
// context never carries zeros or sentinel/implausible readings.
func putPos(m map[string]any, key string, p *float64) {
	if validPos(p) {
		m[key] = *p
	}
}

// buildSlimInterpretationContext distills the full structured result and rhythm
// classifier output into a compact, validated context for the interpretation
// LLM. Only plausible values are included: missing intervals, zero amplitudes,
// and implausible rates are omitted entirely so the model cannot latch onto a
// HR=300 or PR=0 artifact. This also keeps the prompt (and token cost) small.
func buildSlimInterpretationContext(
	structured *models.ECGStructuredResult,
	rhythm *models.ECGRhythmResult,
) map[string]any {
	ctx := map[string]any{}

	if structured == nil {
		return ctx
	}

	// Patient demographics drive sex/age-specific thresholds.
	if structured.Patient.Sex != "" || structured.Patient.Age != nil {
		patient := map[string]any{}
		if structured.Patient.Sex != "" {
			patient["sex"] = structured.Patient.Sex
		}
		if structured.Patient.Age != nil {
			patient["age"] = *structured.Patient.Age
		}
		ctx["patient"] = patient
	}

	// Validated rhythm/interval timings (positive values only).
	if r := structured.Rhythm; r != nil {
		intervals := map[string]any{}
		putPos(intervals, "PR_ms", r.PRms)
		putPos(intervals, "QRS_ms", r.QRSms)
		putPos(intervals, "RR_ms", r.RRms)
		putPos(intervals, "QT_ms", r.QTms)
		putPos(intervals, "JT_ms", r.JTms)
		putPos(intervals, "HR_bpm", r.HRbpm)
		putPos(intervals, "QTc_bazett_ms", r.QTcBazettMs)
		putPos(intervals, "QTc_fridericia_ms", r.QTcFridericiaMs)
		if len(intervals) > 0 {
			ctx["intervals"] = intervals
		}
	}

	// Electrical axis (degree can be negative, so it is not gated by validPos).
	if a := structured.Axis; a != nil && a.AxisDeg != nil && !math.IsNaN(*a.AxisDeg) {
		axis := map[string]any{"axis_deg": *a.AxisDeg}
		if a.Classification != "" {
			axis["classification"] = a.Classification
		}
		ctx["axis"] = axis
	}

	// LVH voltage indices (mV), valid positives only.
	if i := structured.Indices; i != nil {
		indices := map[string]any{}
		putPos(indices, "sokolow_lyon_mV", i.SokolowLyon)
		putPos(indices, "cornell_voltage_mV", i.CornellVoltage)
		putPos(indices, "peguero_lo_presti_mV", i.PegueroLoPresti)
		putPos(indices, "gubner_mV", i.Gubner)
		putPos(indices, "lewis_mV", i.Lewis)
		if len(indices) > 0 {
			ctx["lvh_indices"] = indices
		}
	}

	// RVH markers (mV / ratio), valid positives only.
	if v := structured.RVH; v != nil {
		rvh := map[string]any{}
		putPos(rvh, "RV1_mV", v.RV1mV)
		putPos(rvh, "R_over_S_V1", v.ROverSV1)
		putPos(rvh, "RV1_plus_SV5_mV", v.RV1PlusSV5)
		putPos(rvh, "RV1_plus_SV6_mV", v.RV1PlusSV6)
		if len(rvh) > 0 {
			ctx["rvh"] = rvh
		}
	}

	if structured.Transition != "" {
		ctx["transition_zone_lead"] = structured.Transition
	}

	// Rhythm classifier prediction plus quality/auxiliary flags.
	if rhythm != nil {
		classifier := map[string]any{}
		if rhythm.PredCode != "" {
			classifier["pred_code"] = rhythm.PredCode
		}
		if rhythm.PredLabelRU != "" {
			classifier["pred_label_ru"] = rhythm.PredLabelRU
		}
		if len(rhythm.Top3) > 0 {
			classifier["top3"] = rhythm.Top3
		}
		if len(rhythm.BinaryFlags) > 0 {
			classifier["quality_flags"] = rhythm.BinaryFlags
		}
		if len(classifier) > 0 {
			ctx["rhythm_classifier"] = classifier
		}
	}

	return ctx
}

// BuildECGClinicalInterpretationPrompt returns prompts for a patient-facing
// ECG interpretation. The LLM receives both the original image and a slim,
// validated measurement context so clinical pattern wording stays with GPT,
// while deterministic code only supplies trustworthy numerical context.
func BuildECGClinicalInterpretationPrompt(
	structured *models.ECGStructuredResult,
	rhythm *models.ECGRhythmResult,
	notes string,
	paperSpeedMMS float64,
	mmPerMvLimb float64,
	mmPerMvChest float64,
) (system, user string, err error) {
	payload := map[string]any{
		"calibration": map[string]any{
			"paper_speed_mm_s": paperSpeedMMS,
			"mm_per_mv_limb":   mmPerMvLimb,
			"mm_per_mv_chest":  mmPerMvChest,
		},
		"context": buildSlimInterpretationContext(structured, rhythm),
	}
	if notes != "" {
		payload["notes"] = notes
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", "", fmt.Errorf("marshal ECG interpretation payload: %w", err)
	}

	user = fmt.Sprintf(`Analyze the ECG image and compact JSON context. Return ONE JSON object, nothing else.

"interpretation_md" — exactly one Russian Markdown section:
## Итог
- 3-6 short, self-contained bullets.
- Each bullet: neutral ECG pattern + brief visible/measurement criterion, or "данных недостаточно".
- Cover conduction (PR/AV, QRS width, blocks), axis/BBB, hypertrophy, ST-T/infarction pattern, QT/JT only when assessable.
- If STEMI criteria are visible, one bullet MUST explicitly name "паттерн инфаркта миокарда с подъёмом ST (STEMI)" with the territory and supporting leads — never downgrade it to a generic "подъём ST".
- Do NOT name or restate the heart rhythm (no "синусовый ритм", "фибрилляция/трепетание", "тахи-/брадикардия") — it is provided separately by the classifier.
- Do not expose internal measurement problems or JSON field names.

"rhythm" — your OWN rhythm read from the image:
- "code": one of SINUS_GROUP, AFIB, AFLT, SVTAC, VTAC, VFIB_VFLT, PACE, or UNCLEAR if you cannot tell.
- "label_ru": short Russian name of the rhythm you see.
- "agrees_with_classifier": true if your read is the SAME rhythm as context.rhythm_classifier.pred_code, OR a clinically compatible broader/narrower form of it (a cautious umbrella term such as "ширококомплексная тахикардия" agrees with "желудочковая тахикардия"; "наджелудочковый ритм" agrees with a specific supraventricular code). Set false only if it is genuinely a different rhythm. If context has no rhythm_classifier prediction, set false.

Context JSON:
%s`, string(b))

	return ecgClinicalInterpretationSystemPrompt, user, nil
}

// ECGInterpretedRhythm is GPT's own rhythm read, returned alongside the
// interpretation so the worker can arbitrate against the CV classifier when CV
// confidence is low. AgreesWithClassifier is GPT's clinical-compatibility
// judgement (it sees the classifier prediction in context), so an umbrella
// term like "ширококомплексная тахикардия" can still agree with "VTAC".
type ECGInterpretedRhythm struct {
	Code                 string `json:"code"`
	LabelRU              string `json:"label_ru"`
	AgreesWithClassifier bool   `json:"agrees_with_classifier"`
}

// ECGInterpretationResult is the parsed structured output of
// InterpretStructuredECG: the patient-facing Markdown plus GPT's structured
// rhythm read.
type ECGInterpretationResult struct {
	InterpretationMD string                `json:"interpretation_md"`
	Rhythm           *ECGInterpretedRhythm `json:"rhythm,omitempty"`
}

// ParseECGInterpretation parses the JSON the interpretation model returns,
// tolerating ``` fences. Returns an error (so callers can fall back to the raw
// text) when the payload is not the expected JSON object.
func ParseECGInterpretation(raw string) (*ECGInterpretationResult, error) {
	text := stripCodeFences(strings.TrimSpace(raw))
	var r ECGInterpretationResult
	if err := json.Unmarshal([]byte(text), &r); err != nil {
		return nil, fmt.Errorf("parse ECG interpretation JSON: %w", err)
	}
	if strings.TrimSpace(r.InterpretationMD) == "" {
		return nil, fmt.Errorf("ECG interpretation JSON missing interpretation_md")
	}
	return &r, nil
}

// stripCodeFences removes a leading ```/```json fence line and a trailing ```
// fence, if present, so JSON wrapped in a Markdown code block still parses.
func stripCodeFences(s string) string {
	if !strings.HasPrefix(s, "```") {
		return s
	}
	if i := strings.IndexByte(s, '\n'); i != -1 {
		s = s[i+1:]
	}
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "```"))
}
