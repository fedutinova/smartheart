package workers

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/fedutinova/smartheart/back-api/gpt"
	"github.com/fedutinova/smartheart/back-api/models"
)

const singleSignDetected = "выявлен отдельный признак"

var allLeads = []string{"I", "II", "III", "aVR", "aVL", "aVF", "V1", "V2", "V3", "V4", "V5", "V6"}

var chestLeads = map[string]bool{
	"V1": true, "V2": true, "V3": true, "V4": true, "V5": true, "V6": true,
}

// robustMedian returns the median of the array after MAD-based outlier filtering.
// Returns nil for empty input.
func robustMedian(arr []float64) *float64 {
	if len(arr) == 0 {
		return nil
	}
	filtered := madFilter(arr, 2.5)
	if len(filtered) == 0 {
		filtered = arr
	}
	m := median(filtered)
	return &m
}

func median(arr []float64) float64 {
	s := make([]float64, len(arr))
	copy(s, arr)
	sort.Float64s(s)
	n := len(s)
	if n%2 == 0 {
		return (s[n/2-1] + s[n/2]) / 2
	}
	return s[n/2]
}

func madFilter(arr []float64, threshold float64) []float64 { //nolint:unparam // threshold is parameterized for testability
	if len(arr) < 3 {
		return arr
	}
	med := median(arr)
	deviations := make([]float64, len(arr))
	for i, v := range arr {
		deviations[i] = math.Abs(v - med)
	}
	mad := median(deviations)
	if mad < 0.01 {
		mad = 0.01
	}
	var result []float64
	for i, v := range arr {
		if deviations[i]/mad <= threshold {
			result = append(result, v)
		}
	}
	return result
}

// robustList filters out zeros and returns non-nil values.
func robustList(vals []float64) []float64 {
	var out []float64
	for _, v := range vals {
		if !math.IsNaN(v) && !math.IsInf(v, 0) {
			out = append(out, v)
		}
	}
	return out
}

// finalizeFromCounts converts raw GPT small-square measurements to mm and ms.
// Vertical: 1 small square = 1 mm. Horizontal: msPerSq = 1000 / paperSpeedMMS.
func finalizeFromCounts(raw *gpt.RawECGMeasurement, msPerSq float64) map[string]*float64 {
	result := make(map[string]*float64)

	// Process 12 leads
	for _, lead := range allLeads {
		data, ok := raw.Leads[lead]
		if !ok {
			continue
		}
		rVals := robustList(data.RUpSq)
		sVals := robustList(data.SDownSq)
		result["R_"+lead+"_mm"] = robustMedian(rVals)
		result["S_"+lead+"_mm"] = robustMedian(sVals)
	}

	// Process extras (all are amplitude in small squares = mm)
	for key, vals := range raw.Extras {
		cleaned := robustList(vals)
		// Convert key from e.g. "SV1_sq" to "SV1_mm"
		name := key
		if len(name) > 3 && name[len(name)-3:] == "_sq" {
			name = name[:len(name)-3] + "_mm"
		}
		result[name] = robustMedian(cleaned)
	}

	// Process intervals (in small squares → ms)
	for key, vals := range raw.IntervalsSq {
		cleaned := robustList(vals)
		med := robustMedian(cleaned)
		if med != nil {
			ms := *med * msPerSq
			result[key+"_ms"] = &ms
		}
	}

	// HR
	if raw.HRBpm != nil {
		hr := *raw.HRBpm
		result["HR_bpm"] = &hr
	}

	return result
}

// clampMeasurements clamps values to physiological ranges.
func clampMeasurements(meas map[string]*float64) {
	for key, v := range meas {
		if v == nil {
			continue
		}
		lo, hi := clampRange(key)
		if lo == 0 && hi == 0 {
			continue
		}
		if *v < lo {
			clamped := lo
			meas[key] = &clamped
		} else if *v > hi {
			clamped := hi
			meas[key] = &clamped
		}
	}
}

func clampRange(key string) (lo, hi float64) {
	n := len(key)
	if n > 3 && key[n-3:] == "_mm" {
		return -80, 80
	}
	switch key {
	case "QRS_ms":
		return 60, 180
	case "RR_ms":
		return 200, 3000
	case "HR_bpm":
		return 30, 220
	}
	return 0, 0
}

// computeStructuredResult builds the final ECGStructuredResult from mm measurements.
func computeStructuredResult(
	measMM map[string]*float64,
	sex string, age *int,
	mmPerMvLimb, mmPerMvChest float64,
	timestamp, jobID string,
) *models.ECGStructuredResult {
	// Helper: get value
	get := func(key string) *float64 { return measMM[key] }

	// Helper: mm to mV
	toMV := func(mm *float64, lead string) *float64 {
		if mm == nil {
			return nil
		}
		scale := mmPerMvLimb
		if chestLeads[lead] {
			scale = mmPerMvChest
		}
		if scale <= 0 {
			scale = 10
		}
		v := *mm / scale
		return &v
	}

	// Helper: abs
	absF := func(v *float64) *float64 {
		if v == nil {
			return nil
		}
		a := math.Abs(*v)
		return &a
	}

	// Helper: add two pointers
	addP := func(a, b *float64) *float64 {
		if a == nil || b == nil {
			return nil
		}
		v := *a + *b
		return &v
	}

	// Helper: max of two pointers
	maxP := func(a, b *float64) *float64 {
		if a == nil {
			return b
		}
		if b == nil {
			return a
		}
		v := math.Max(*a, *b)
		return &v
	}

	// Convert key measurements to mV
	sv1 := toMV(absF(get("SV1_mm")), "V1")
	rv5 := toMV(get("RV5_mm"), "V5")
	rv6 := toMV(get("RV6_mm"), "V6")
	ravl := toMV(get("RaVL_mm"), "aVL")
	sv3 := toMV(absF(get("SV3_mm")), "V3")
	sv4 := toMV(absF(get("SV4_mm")), "V4")
	sDeepest := toMV(absF(get("S_deepest_mm")), "V1") // use limb by default but deepest is usually chest
	sv5 := toMV(absF(get("SV5_mm")), "V5")
	sv6 := toMV(absF(get("SV6_mm")), "V6")
	rv1 := toMV(get("R_V1_mm"), "V1")

	// LVH indices
	indices := &models.LVHIndices{
		SokolowLyon:     addP(sv1, maxP(rv5, rv6)),
		CornellVoltage:  addP(ravl, sv3),
		PegueroLoPresti: addP(sDeepest, sv4),
	}

	// Gubner: R_I + |S_III| in mV
	ri := toMV(get("R_I_mm"), "I")
	siii := toMV(absF(get("S_III_mm")), "III")
	indices.Gubner = addP(ri, siii)

	// Lewis: (R_I + |S_III|) - (|S_I| + R_III)
	si := toMV(absF(get("S_I_mm")), "I")
	riii := toMV(get("R_III_mm"), "III")
	left := addP(ri, siii)
	right := addP(si, riii)
	if left != nil && right != nil {
		v := *left - *right
		indices.Lewis = &v
	}

	// RVH
	rvh := &models.RVHData{
		RV1mV:      rv1,
		RV1PlusSV5: addP(rv1, sv5),
		RV1PlusSV6: addP(rv1, sv6),
	}
	if rv1 != nil {
		sv1Val := toMV(absF(get("S_V1_mm")), "V1")
		if sv1Val != nil && *sv1Val != 0 {
			ratio := *rv1 / *sv1Val
			rvh.ROverSV1 = &ratio
		}
	}

	// QRS Axis
	var axis *models.QRSAxis
	netI := qrsNet(get("R_I_mm"), get("S_I_mm"))
	netAVF := qrsNet(get("R_aVF_mm"), get("S_aVF_mm"))
	netImV := toMV(netI, "I")
	netAVFmV := toMV(netAVF, "aVF")
	if netImV != nil && netAVFmV != nil {
		deg := math.Atan2(*netAVFmV, *netImV) * 180 / math.Pi
		cls := classifyAxis(deg)
		axis = &models.QRSAxis{
			NetImV:         netImV,
			NetAVFmV:       netAVFmV,
			AxisDeg:        &deg,
			Classification: cls,
		}
	}

	// Rhythm
	rhythm := &models.RhythmTiming{
		QRSms: get("QRS_ms"),
		RRms:  get("RR_ms"),
		HRbpm: get("HR_bpm"),
	}
	// Compute HR from RR if missing
	if rhythm.HRbpm == nil && rhythm.RRms != nil && *rhythm.RRms > 0 {
		hr := 60000.0 / *rhythm.RRms
		rhythm.HRbpm = &hr
	}

	// Transition zone
	transition := findTransitionZone(measMM)

	interp := buildInterpretation(indices, rvh, axis, rhythm, sex)

	return &models.ECGStructuredResult{
		Measurements:   measMM,
		Indices:        indices,
		RVH:            rvh,
		Axis:           axis,
		Rhythm:         rhythm,
		Transition:     transition,
		Interpretation: interp,
		Patient:        models.PatientInfo{Sex: sex, Age: age},
		Timestamp:      timestamp,
		JobID:          jobID,
	}
}

// thresholdCheck evaluates a single value against a threshold and appends the result item.
// Returns true if the threshold was exceeded.
type thresholdCheck struct {
	value     *float64
	label     string
	threshold float64
	op        string // ">=" or ">"
	unit      string // e.g. "мВ" or "" for dimensionless
	threshFmt string // full threshold label for display
	group     string
}

func evalChecks(checks []thresholdCheck, items *[]models.InterpretationItem) int {
	posCount := 0
	for _, c := range checks {
		if c.value == nil {
			continue
		}
		var exceeded bool
		if c.op == ">=" {
			exceeded = *c.value >= c.threshold
		} else {
			exceeded = *c.value > c.threshold
		}
		status := models.StatusNegative
		if exceeded {
			status = models.StatusPositive
			posCount++
		}
		display := fmt.Sprintf("%.2f", *c.value)
		if c.unit != "" {
			display += " " + c.unit
		}
		*items = append(*items, models.InterpretationItem{
			Label: c.label, Value: display, Threshold: c.threshFmt, Status: status, Group: c.group,
		})
	}
	return posCount
}

func lvhTextPart(s models.InterpretationItem, items []models.InterpretationItem) string {
	if s.Status != models.StatusPositive {
		return "Убедительных признаков ГЛЖ не выявлено."
	}
	var criteria []string
	for _, it := range items {
		if it.Group == "lvh" && it.Status == models.StatusPositive {
			criteria = append(criteria, fmt.Sprintf("%s %s", it.Label, it.Value))
		}
	}
	if len(criteria) > 0 {
		return fmt.Sprintf("Признаки ГЛЖ: %s (%s).", s.Value, strings.Join(criteria, ", "))
	}
	return fmt.Sprintf("Признаки ГЛЖ: %s.", s.Value)
}

func lvhSummary(posCount int) models.InterpretationItem {
	value := "не обнаружены"
	status := models.StatusNegative
	if posCount >= 2 {
		value = "обнаружены"
		status = models.StatusPositive
	} else if posCount == 1 {
		value = singleSignDetected
		status = models.StatusPositive
	}
	return models.InterpretationItem{Label: "Признаки ГЛЖ", Value: value, Status: status}
}

func rvhSummary(posCount int, axis *models.QRSAxis) models.InterpretationItem {
	axisRight := axis != nil && axis.AxisDeg != nil && *axis.AxisDeg > 90
	axisNormal := axis != nil && axis.AxisDeg != nil && *axis.AxisDeg >= -30 && *axis.AxisDeg <= 90

	value := "не обнаружены"
	status := models.StatusNegative
	switch {
	case posCount >= 2 && axisRight:
		value = "обнаружены"
		status = models.StatusPositive
	case posCount >= 2:
		value = "выявлены признаки, ось не отклонена вправо"
		status = models.StatusPositive
	case posCount == 1 && axisNormal:
		value = "выявлен отдельный признак при нормальной оси, требуется проверка"
		status = models.StatusNegative
	case posCount == 1:
		value = singleSignDetected
		status = models.StatusPositive
	}
	return models.InterpretationItem{Label: "Признаки ГПЖ", Value: value, Status: status}
}

// buildInterpretation generates a structured conclusion from computed indices.
func buildInterpretation(
	indices *models.LVHIndices,
	rvh *models.RVHData,
	axis *models.QRSAxis,
	rhythm *models.RhythmTiming,
	sex string,
) *models.ECGInterpretation {
	male := sex != "female"
	thrCornell := 2.8
	if !male {
		thrCornell = 2.0
	}
	thrPeguero := 2.8
	if !male {
		thrPeguero = 2.3
	}
	sexLabel := "муж"
	if !male {
		sexLabel = "жен"
	}

	normStatus := func(ok bool) models.InterpretationStatus {
		if ok {
			return models.StatusNormal
		}
		return models.StatusAbnormal
	}

	var items []models.InterpretationItem

	// LVH indices
	thrSok := 3.5
	var lvhChecks []thresholdCheck
	if indices != nil {
		lvhChecks = []thresholdCheck{
			{indices.SokolowLyon, "Соколов-Лайон", thrSok, ">=", "мВ", fmt.Sprintf(">= %.1f мВ", thrSok), "lvh"},
			{indices.CornellVoltage, "Корнелл (RaVL+SV3)", thrCornell, ">", "мВ", fmt.Sprintf("> %.1f мВ (%s)", thrCornell, sexLabel), "lvh"},
			{indices.PegueroLoPresti, "Пегеро-Ло Прести", thrPeguero, ">=", "мВ", fmt.Sprintf(">= %.1f мВ (%s)", thrPeguero, sexLabel), "lvh"},
		}
	}
	lvhPosCount := evalChecks(lvhChecks, &items)

	// RVH markers
	var rvhChecks []thresholdCheck
	if rvh != nil {
		rvhChecks = []thresholdCheck{
			{rvh.RV1mV, "R в V1", 0.7, ">=", "мВ", ">= 0.70 мВ", "rvh"},
			{rvh.ROverSV1, "R/S в V1", 1.0, ">", "", "> 1.0", "rvh"},
			{rvh.RV1PlusSV5, "RV1+|SV5|", 1.05, ">", "мВ", "> 1.05 мВ", "rvh"},
			{rvh.RV1PlusSV6, "RV1+|SV6|", 1.05, ">", "мВ", "> 1.05 мВ", "rvh"},
		}
	}
	rvhPosCount := evalChecks(rvhChecks, &items)

	// QRS
	if rhythm != nil && rhythm.QRSms != nil {
		items = append(items, models.InterpretationItem{
			Label: "QRS", Value: fmt.Sprintf("%.0f мс", *rhythm.QRSms),
			Threshold: "60-100 мс", Status: normStatus(*rhythm.QRSms >= 60 && *rhythm.QRSms <= 100), Group: "rhythm",
		})
	}

	// HR
	if rhythm != nil && rhythm.HRbpm != nil {
		items = append(items, models.InterpretationItem{
			Label: "ЧСС", Value: fmt.Sprintf("%.0f уд/мин", *rhythm.HRbpm),
			Threshold: "60-100", Status: normStatus(*rhythm.HRbpm >= 60 && *rhythm.HRbpm <= 100), Group: "rhythm",
		})
	}

	// Summary
	var summary []models.InterpretationItem

	if axis != nil && axis.AxisDeg != nil {
		axisOk := *axis.AxisDeg >= -30 && *axis.AxisDeg <= 90
		summary = append(summary, models.InterpretationItem{
			Label: "ЭОС", Value: fmt.Sprintf("%.0f° (%s)", *axis.AxisDeg, axis.Classification),
			Threshold: "-30°...+90°", Status: normStatus(axisOk),
		})
	}

	summary = append(summary, lvhSummary(lvhPosCount), rvhSummary(rvhPosCount, axis))

	if len(items) == 0 {
		return nil
	}

	textSummary := buildTextSummary(summary, items, rhythm)

	return &models.ECGInterpretation{
		Items:       items,
		Summary:     summary,
		TextSummary: textSummary,
	}
}

// buildTextSummary generates a human-readable Russian text from interpretation data.
func buildTextSummary(
	summary []models.InterpretationItem,
	items []models.InterpretationItem,
	rhythm *models.RhythmTiming,
) string {
	var parts []string

	// Rhythm
	if rhythm != nil && rhythm.HRbpm != nil {
		hr := *rhythm.HRbpm
		rhythmType := "Ритм по данным автоматического анализа"
		if hr < 60 {
			rhythmType = "Брадикардия по данным автоматического анализа"
		} else if hr > 100 {
			rhythmType = "Тахикардия по данным автоматического анализа"
		}
		parts = append(parts, fmt.Sprintf("%s, ЧСС %.0f уд/мин.", rhythmType, hr))
	}

	// QRS
	for _, it := range items {
		if it.Group == "rhythm" && it.Label == "QRS" {
			if it.Status == models.StatusNormal {
				parts = append(parts, fmt.Sprintf("Длительность QRS %s, в пределах нормы.", it.Value))
			} else {
				parts = append(parts, fmt.Sprintf("Длительность QRS %s, расширен.", it.Value))
			}
			break
		}
	}

	// Axis
	for _, s := range summary {
		if s.Label == "ЭОС" {
			if s.Status == models.StatusNormal {
				parts = append(parts, fmt.Sprintf("ЭОС %s.", s.Value))
			} else {
				parts = append(parts, fmt.Sprintf("ЭОС отклонена: %s.", s.Value))
			}
			break
		}
	}

	// LVH
	for _, s := range summary {
		if s.Label == "Признаки ГЛЖ" {
			parts = append(parts, lvhTextPart(s, items))
			break
		}
	}

	// RVH
	for _, s := range summary {
		if s.Label == "Признаки ГПЖ" {
			switch {
			case s.Status == models.StatusPositive:
				parts = append(parts, fmt.Sprintf("Признаки ГПЖ: %s.", s.Value))
			case strings.Contains(s.Value, "требуется проверка"):
				parts = append(parts, fmt.Sprintf("ГПЖ: %s.", s.Value))
			default:
				parts = append(parts, "Убедительных признаков ГПЖ не выявлено.")
			}
			break
		}
	}

	parts = append(parts, "Результат автоматической обработки, не является медицинским заключением.")

	return strings.Join(parts, " ")
}

func qrsNet(r, s *float64) *float64 {
	rVal := 0.0
	sVal := 0.0
	if r != nil {
		rVal = *r
	}
	if s != nil {
		sVal = math.Abs(*s)
	}
	if r == nil && s == nil {
		return nil
	}
	v := rVal - sVal
	return &v
}

func classifyAxis(deg float64) string {
	switch {
	case deg >= -30 && deg <= 90:
		return "нормальная"
	case deg > 90 && deg <= 180:
		return "правограмма"
	case deg < -30 && deg >= -90:
		return "левограмма"
	default:
		return "резкое отклонение"
	}
}

func findTransitionZone(meas map[string]*float64) string {
	chestOrder := []string{"V1", "V2", "V3", "V4", "V5", "V6"}
	for _, lead := range chestOrder {
		r := meas["R_"+lead+"_mm"]
		s := meas["S_"+lead+"_mm"]
		if r != nil && s != nil {
			sAbs := math.Abs(*s)
			if sAbs > 0 && *r/sAbs >= 0.8 && *r/sAbs <= 1.2 {
				return lead
			}
		}
	}
	return ""
}
