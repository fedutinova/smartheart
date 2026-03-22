package workers

import (
	"math"
	"sort"

	"github.com/fedutinova/smartheart/back-api/gpt"
	"github.com/fedutinova/smartheart/back-api/models"
)

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

func madFilter(arr []float64, threshold float64) []float64 {
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

	return &models.ECGStructuredResult{
		Measurements: measMM,
		Indices:      indices,
		RVH:          rvh,
		Axis:         axis,
		Rhythm:       rhythm,
		Transition:   transition,
		Patient:      models.PatientInfo{Sex: sex, Age: age},
		Timestamp:    timestamp,
		JobID:        jobID,
	}
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
