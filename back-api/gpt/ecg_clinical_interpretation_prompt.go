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
- Do NOT name or restate the heart rhythm in interpretation_md (no "синусовый ритм", "фибрилляция/трепетание предсердий", "тахикардия/брадикардия", etc.). The rhythm is determined separately by a classifier and shown to the user elsewhere. You STILL assess everything else in the text — conduction (PR/AV, QRS width, blocks), axis, hypertrophy, ST-T / ischemia / infarction, QT/JT; only the rhythm NAME is omitted. Still report your own rhythm read in the structured "rhythm" field.
- AV block I only if PR_ms > 200 ms or a clearly measured PR duration supports it.
- PR/PQ interval: do NOT state a numeric PR/PQ value as a routine finding. The PR interval is patient-specific and varies (normal ≈ 120-200 ms), so NEVER output a fixed/rounded default such as "PQ 160 мс" on every tracing. Quote a number ONLY when it signals a finding (e.g. PR > 200 ms → AV block I) and is given in the context (PR_ms) or clearly measurable on the image; otherwise describe AV conduction qualitatively ("АВ-проведение не нарушено") or write "PR: неопределимо". Do not echo a context PR value if the image does not support it.
- P waves / atrial activity: assert that a P wave precedes each QRS ONLY when you convincingly see a discrete, consistent P before EVERY QRS — verify this specifically and carefully in leads II and V1 (the most reliable leads for atrial activity). If the baseline is irregularly undulating or shows fibrillatory/flutter waves, or P waves are absent, variable, or not clearly visible, do NOT write that a P precedes each QRS — instead state "зубцы P убедительно не определяются" / "предсердная активность достоверно не оценивается". Never assume regular P waves by default.
- Conduction blocks/hemiblocks: name them ONLY when criteria are met, and measurement is mandatory.
- QRS width: measure from the onset of the first deflection to the J-point (end of the last rapid deflection); do NOT include ST/T (elevation/depression) in the QRS width.
- STOP-RULE against false LBBB (LBBB is over-diagnosed — the default is NOT LBBB):
  (1) Name complete LBBB / "полная блокада ЛНПГ" ONLY IF BOTH hold: (a) the JSON context provides QRS_ms AND it is >= 120 ms — do NOT use your own visual width estimate to reach 120 ms, and if QRS_ms is ABSENT from the context treat the width as UNCONFIRMED and do NOT diagnose complete LBBB; AND (b) the full typical LBBB morphology below is clearly present.
  (2) If the J-point is ambiguous, QRS-ST-T fuse, or image quality is poor: write "ширина QRS: неопределимо" and do NOT conclude LBBB.
  (3) If QRS is 110-120 ms -> NOT LBBB: write "пограничное уширение / неспецифическая внутрижелудочковая задержка (IVCD)"; ST-T changes must NOT be auto-attributed to "вторичные изменения блокады".
- RBBB (complete) — ALL of these together: QRS >= 120 ms taken from the context QRS_ms (do NOT reach 120 ms by visual estimate; if QRS_ms is ABSENT from the context the width is UNCONFIRMED — do NOT call complete RBBB); rsR'/rSR'/"M"-shaped complex in V1-V2 (secondary R' taller than the initial r); wide terminal/slurred S wave in I, V5-V6 (S duration > R duration or > 40 ms). Secondary ST depression / T-wave inversion in V1-V3 is EXPECTED with RBBB and must NOT be read as ischemia on its own.
- Incomplete RBBB: typical RBBB morphology (rSR' in V1) but QRS 110-120 ms; name it "неполная блокада правой ножки", do NOT call it complete RBBB.
- Anti-error: a tall R or RSr' in V1 with a NARROW QRS (< 120 ms) is NOT complete RBBB — consider differential (posterior infarct, RVH, WPW, normal variant). Do not let RBBB mask a concurrent infarction: assess ST elevation and pathologic Q waves independently of the RBBB pattern.
- Anti-overcall RBBB: bias AGAINST complete RBBB. Do NOT call it when any of these is true: QRS_ms is missing from the context or < 120 ms; the V1 complex is an rSr'/rsr' with a narrow QRS (incomplete RBBB or a normal variant); or the picture is better explained by RVH, posterior MI, WPW, or high V1-V2 lead placement. If RBBB-like morphology (rSR' in V1) is present but QRS_ms is not provided or not confirmed >= 120 ms, write "неполная/вероятная блокада правой ножки — требуется подтверждение ширины QRS" rather than a definitive "полная блокада ПНПГ".
- LBBB (complete) — ALL of these together: QRS >= 120 ms taken from the context QRS_ms (per the STOP-RULE above; not a visual estimate); dominant S (QS or rS) in V1; broad/notched/plateau R ("M" type) in at least 2 contiguous of I, aVL, V5-V6 (usually without an initial q). Without proven QRS >= 120 ms AND typical morphology, do NOT use the phrasing "ST-T вторичные к LBBB".
- Anti-overcall LBBB: bias AGAINST complete LBBB. Do NOT call it when any of these is true: QRS_ms is missing from the context or < 120 ms; the lateral-lead R wave has a relatively rapid, NON-notched upstroke; or the tracing is better explained by LVH with strain (tall lateral R + deep S in V1 + secondary ST-T) — high QRS voltage with a near-normal/narrow width is LVH, NOT LBBB. If LBBB-like morphology is present but QRS_ms is not provided or not confirmed >= 120 ms, write "вероятная блокада ЛНПГ — требуется подтверждение ширины QRS" rather than a definitive "полная блокада ЛНПГ".
- LBBB ⇒ ACS caution: WHENEVER you identify a complete LBBB pattern (or clear LBBB morphology while QRS confirmation is still pending), you MUST ALWAYS flag it as potentially associated with acute coronary syndrome — a new or presumably-new LBBB, and LBBB with ischemic ST changes, can mask or accompany acute MI. Add an explicit caution bullet such as "паттерн полной блокады ЛНПГ — нельзя исключить острый коронарный синдром; требуется срочная клиническая оценка и ЭКГ в динамике". Apply the Sgarbossa / modified Sgarbossa criteria (concordant ST elevation >= 1 mm; concordant ST depression in V1-V3; or excessively discordant ST elevation, ST/S ratio <= -0.25) and state whether they are positive. This is a pattern-level warning and a recommendation to correlate clinically — NOT treatment, dosing, or emergency-management advice. Note: ordinary DISCORDANT ST-T alone (without Sgarbossa positivity) still must not be called ischemia, but the LBBB pattern itself always warrants this ACS caution.
- LAFB (блокада передней ветви ЛНПГ): when there is convincing MARKED left-axis deviation more negative than -45° (use context axis_deg when present, otherwise the clearly visible axis) TOGETHER WITH qR in I/aVL and rS in II/III/aVF and QRS < 120 ms, you SHOULD name it "блокада передней ветви левой ножки пучка Гиса (БПВЛНПГ)". Axis between -30° and -45° supports but does not by itself require the label. First exclude other causes of left-axis deviation — especially inferior MI (pathologic Q waves in II/III/aVF) — before calling LAFB. If QRS >= 120 ms, note IVCD/combination and do not stretch it into LBBB without criteria.
- LPFB: axis >= +90 deg; rS in I/aVL and qR in II/III/aVF; exclude other causes of right-axis deviation.
- Anti-error note: QS / poor R-wave progression in V1-V3 with a NARROW QRS (< 120 ms) is NOT a criterion for LBBB and needs differential assessment (anterior infarct/scar, V1-V2 placement, cardiac rotation, etc.).
- Do not explain ST-T changes by BBB unless full BBB criteria are met. Discordant ST-T is EXPECTED in true LBBB and must not be read as ischemia/infarction on its own; conversely, do not let a presumed LBBB mask a real infarction.
- Check ST elevation/depression, contiguous leads, reciprocal changes, and pathologic Q waves separately.
- When STEMI criteria are met (ST elevation at the J-point in >=2 anatomically contiguous leads — typically >=1 mm, or >=2 mm in V2-V3 men / >=1.5 mm V2-V3 women — especially with reciprocal depression), you MUST explicitly name the pattern in interpretation_md as "паттерн инфаркта миокарда с подъёмом ST (STEMI)" and state the territory (передний/нижний/боковой/перегородочный/задний) with the leads that support it. STEMI/инфаркт/ишемия are ECG PATTERN terms — naming them when criteria are visible is required, NOT a forbidden clinical diagnosis. Do NOT soften a clear STEMI to a vague "подъём ST" or "изменения ST-T".
- If ST elevation is present but does not meet full STEMI thresholds or contiguity, say so explicitly (e.g. "подъём ST, не достигающий критериев STEMI") rather than omitting it.
- Tall, broad, or peaked ("hyperacute") T waves together with ST elevation may be a HYPERACUTE infarction pattern (very early STEMI before Q waves and before reciprocal changes develop). Do NOT dismiss this as benign — name it as "возможный гиперострый ишемический паттерн" and recommend clinical/serial-ECG correlation. Absence of pathologic Q waves and absence of reciprocal depression do NOT exclude early infarction.
- Do NOT use "ранняя реполяризация" or "доброкачественные/неспецифические изменения" as a catch-all to downgrade ST elevation. Benign early repolarization requires ALL of: J-point notching/slurring, concordant upright T waves, a LIMITED contiguous distribution (typically mid-precordial V2-V5 or inferior, not both), no reciprocal changes, and a stable non-evolving picture. If these are not all clearly satisfied, do NOT call it early repolarization.
- ST elevation spanning MULTIPLE non-contiguous territories at once (e.g. inferior II/III/aVF AND anterior/lateral V2-V6) is NOT a normal early-repolarization distribution. Treat such diffuse ST elevation as abnormal and flag it as "распространённый подъём ST — нельзя исключить ишемический паттерн / возможен перикардит; требуется клиническая корреляция", never as benign early repolarization.
- Normal-variant ST elevation in V1-V2: a SMALL, upwardly-concave ("дугой вниз" / saddle / high take-off) ST elevation confined to V1-V2 that does NOT reach the STEMI thresholds above, with NORMAL T waves (not hyperacute, not inverted) and NO reciprocal ST depression, is an accepted normal variant — do NOT label it инфаркт / ишемия / STEMI. This benign exception applies ONLY when ALL of these hold at once: confined to V1-V2; concave-up morphology; below the mm thresholds; normal T waves; no reciprocal changes; no pathologic Q waves. If even ONE fails (convex or straight/horizontal ST, meets the mm thresholds, hyperacute or inverted T, elevation extends to V3 or other contiguous leads, reciprocal depression, or new pathologic Q), this exception does NOT apply and the STEMI/ischemia rules above take precedence. When genuinely borderline, write "минимальный подъём ST в V1-V2, вероятно вариант нормы; при клинической настороженности — ЭКГ в динамике" rather than naming infarction.
- When ST elevation is real but you are not certain it is STEMI, default to flagging a possible ischemic pattern that needs clinical correlation — do NOT default to a reassuring benign label.
- Sgarbossa / modified Sgarbossa applies to LBBB or ventricular-paced rhythms: for LBBB its assessment and the ACS caution are MANDATORY (see the "LBBB ⇒ ACS caution" rule above). Do NOT invoke Sgarbossa for narrow-QRS or non-LBBB/non-paced tracings.
- If uncertain, write "данных недостаточно" or "неопределимо"; use "вероятно" only with visible criteria.
- No treatment or dosing advice, and no patient-directed management instructions (e.g. "вызовите скорую", what drug to take). Naming ECG pattern terms (including STEMI/инфаркт/ишемия) when their criteria are visible is allowed and expected, and flagging a dangerous pattern as needing URGENT clinical correlation / ЭКГ в динамике is allowed and expected. What is forbidden is therapy and management advice — not the pattern name and not the urgency caution.`

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
