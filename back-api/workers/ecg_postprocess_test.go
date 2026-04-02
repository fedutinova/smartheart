package workers

import (
	"math"
	"testing"

	"github.com/fedutinova/smartheart/back-api/gpt"
	"github.com/fedutinova/smartheart/back-api/models"
)

func ptr(v float64) *float64 { return &v }
func intPtr(v int) *int      { return &v }

func approxEqual(a, b *float64, eps float64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return math.Abs(*a-*b) < eps
}

// --- robustMedian ---

func TestRobustMedian_Empty(t *testing.T) {
	if robustMedian(nil) != nil {
		t.Error("expected nil for empty input")
	}
	if robustMedian([]float64{}) != nil {
		t.Error("expected nil for empty slice")
	}
}

func TestRobustMedian_Single(t *testing.T) {
	got := robustMedian([]float64{5.0})
	if got == nil || *got != 5.0 {
		t.Errorf("expected 5.0, got %v", got)
	}
}

func TestRobustMedian_Even(t *testing.T) {
	got := robustMedian([]float64{2, 4, 6, 8})
	if got == nil || *got != 5.0 {
		t.Errorf("expected 5.0, got %v", got)
	}
}

func TestRobustMedian_Odd(t *testing.T) {
	got := robustMedian([]float64{1, 3, 5})
	if got == nil || *got != 3.0 {
		t.Errorf("expected 3.0, got %v", got)
	}
}

func TestRobustMedian_FiltersOutliers(t *testing.T) {
	// 1,2,3,4,100 — 100 is a clear outlier. After MAD filtering it should be removed.
	got := robustMedian([]float64{1, 2, 3, 4, 100})
	if got == nil {
		t.Fatal("expected non-nil")
	}
	// Without outlier: median of {1,2,3,4} = 2.5
	if *got != 2.5 {
		t.Errorf("expected 2.5 after outlier removal, got %f", *got)
	}
}

// --- madFilter ---

func TestMadFilter_ShortArray(t *testing.T) {
	// Arrays < 3 elements returned as-is.
	in := []float64{1, 2}
	out := madFilter(in, 2.5)
	if len(out) != 2 {
		t.Errorf("expected length 2, got %d", len(out))
	}
}

func TestMadFilter_NoOutliers(t *testing.T) {
	in := []float64{10, 11, 12, 13, 14}
	out := madFilter(in, 2.5)
	if len(out) != len(in) {
		t.Errorf("expected all values retained, got %d/%d", len(out), len(in))
	}
}

func TestMadFilter_RemovesOutliers(t *testing.T) {
	in := []float64{10, 11, 12, 13, 500}
	out := madFilter(in, 2.5)
	for _, v := range out {
		if v == 500 {
			t.Error("outlier 500 should have been removed")
		}
	}
}

// --- robustList ---

func TestRobustList_FiltersSpecialValues(t *testing.T) {
	in := []float64{1, math.NaN(), 3, math.Inf(1), 5, math.Inf(-1)}
	out := robustList(in)
	if len(out) != 3 {
		t.Errorf("expected 3 valid values, got %d", len(out))
	}
}

func TestRobustList_KeepsZero(t *testing.T) {
	out := robustList([]float64{0, 1, 2})
	if len(out) != 3 {
		t.Errorf("zero should be kept, got %d values", len(out))
	}
}

// --- median ---

func TestMedian(t *testing.T) {
	tests := []struct {
		name string
		in   []float64
		want float64
	}{
		{"single", []float64{7}, 7},
		{"odd", []float64{3, 1, 2}, 2},
		{"even", []float64{4, 1, 3, 2}, 2.5},
		{"same values", []float64{5, 5, 5}, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := median(tt.in)
			if got != tt.want {
				t.Errorf("median(%v) = %f, want %f", tt.in, got, tt.want)
			}
		})
	}
}

// --- finalizeFromCounts ---

func TestFinalizeFromCounts_LeadAmplitudes(t *testing.T) {
	raw := &gpt.RawECGMeasurement{
		Leads: map[string]gpt.LeadData{
			"I": {RUpSq: []float64{4, 4.5, 4}, SDownSq: []float64{-2, -2, -2}},
		},
		Extras:      map[string][]float64{},
		IntervalsSq: map[string][]float64{},
	}

	result := finalizeFromCounts(raw, 40) // 25 mm/s → msPerSq=40

	// R_I: median of {4, 4.5, 4} = 4 (after MAD, all kept). 1 small sq = 1 mm.
	rI := result["R_I_mm"]
	if rI == nil || *rI != 4.0 {
		t.Errorf("R_I_mm: expected 4.0, got %v", rI)
	}

	// S_I: median of {2, 2, 2} = 2 (abs not applied here, raw values negative but robustList keeps them).
	sI := result["S_I_mm"]
	if sI == nil || *sI != -2.0 {
		t.Errorf("S_I_mm: expected -2.0, got %v", sI)
	}
}

func TestFinalizeFromCounts_Extras(t *testing.T) {
	raw := &gpt.RawECGMeasurement{
		Leads: map[string]gpt.LeadData{},
		Extras: map[string][]float64{
			"SV1_sq": {20, 21, 20},
			"RV5_sq": {25, 25, 26},
		},
		IntervalsSq: map[string][]float64{},
	}

	result := finalizeFromCounts(raw, 40)

	sv1 := result["SV1_mm"]
	if sv1 == nil || *sv1 != 20.0 {
		t.Errorf("SV1_mm: expected 20.0, got %v", sv1)
	}

	rv5 := result["RV5_mm"]
	if rv5 == nil || *rv5 != 25.0 {
		t.Errorf("RV5_mm: expected 25.0, got %v", rv5)
	}
}

func TestFinalizeFromCounts_Intervals(t *testing.T) {
	raw := &gpt.RawECGMeasurement{
		Leads:  map[string]gpt.LeadData{},
		Extras: map[string][]float64{},
		IntervalsSq: map[string][]float64{
			"QRS": {2, 2.5, 2},  // small squares
			"RR":  {20, 20, 21}, // small squares
		},
	}

	msPerSq := 40.0 // 1000/25 = 40 ms per small square
	result := finalizeFromCounts(raw, msPerSq)

	qrs := result["QRS_ms"]
	if qrs == nil {
		t.Fatal("QRS_ms is nil")
	}
	// median of {2, 2.5, 2} = 2, * 40 = 80 ms
	if *qrs != 80.0 {
		t.Errorf("QRS_ms: expected 80.0, got %f", *qrs)
	}

	rr := result["RR_ms"]
	if rr == nil {
		t.Fatal("RR_ms is nil")
	}
	// median of {20, 20, 21} = 20, * 40 = 800 ms
	if *rr != 800.0 {
		t.Errorf("RR_ms: expected 800.0, got %f", *rr)
	}
}

func TestFinalizeFromCounts_HR(t *testing.T) {
	raw := &gpt.RawECGMeasurement{
		Leads:       map[string]gpt.LeadData{},
		Extras:      map[string][]float64{},
		IntervalsSq: map[string][]float64{},
		HRBpm:       ptr(75),
	}

	result := finalizeFromCounts(raw, 40)
	hr := result["HR_bpm"]
	if hr == nil || *hr != 75 {
		t.Errorf("HR_bpm: expected 75, got %v", hr)
	}
}

// --- clampMeasurements ---

func TestClampMeasurements_AmplitudeLimits(t *testing.T) {
	meas := map[string]*float64{
		"R_I_mm":  ptr(100),  // exceeds 80
		"S_V1_mm": ptr(-100), // below -80
	}
	clampMeasurements(meas)

	if *meas["R_I_mm"] != 80 {
		t.Errorf("expected R_I_mm clamped to 80, got %f", *meas["R_I_mm"])
	}
	if *meas["S_V1_mm"] != -80 {
		t.Errorf("expected S_V1_mm clamped to -80, got %f", *meas["S_V1_mm"])
	}
}

func TestClampMeasurements_IntervalLimits(t *testing.T) {
	meas := map[string]*float64{
		"QRS_ms": ptr(10),   // below 60
		"RR_ms":  ptr(5000), // above 3000
		"HR_bpm": ptr(300),  // above 220
	}
	clampMeasurements(meas)

	if *meas["QRS_ms"] != 60 {
		t.Errorf("QRS_ms: expected 60, got %f", *meas["QRS_ms"])
	}
	if *meas["RR_ms"] != 3000 {
		t.Errorf("RR_ms: expected 3000, got %f", *meas["RR_ms"])
	}
	if *meas["HR_bpm"] != 220 {
		t.Errorf("HR_bpm: expected 220, got %f", *meas["HR_bpm"])
	}
}

func TestClampMeasurements_NilSkipped(t *testing.T) {
	meas := map[string]*float64{
		"R_I_mm": nil,
		"QRS_ms": nil,
	}
	clampMeasurements(meas)
	if meas["R_I_mm"] != nil {
		t.Error("nil values should remain nil")
	}
}

func TestClampMeasurements_InRangeUnchanged(t *testing.T) {
	meas := map[string]*float64{
		"R_I_mm": ptr(15),
		"QRS_ms": ptr(100),
		"HR_bpm": ptr(72),
	}
	clampMeasurements(meas)

	if *meas["R_I_mm"] != 15 {
		t.Errorf("in-range value changed: %f", *meas["R_I_mm"])
	}
	if *meas["QRS_ms"] != 100 {
		t.Errorf("in-range value changed: %f", *meas["QRS_ms"])
	}
}

// --- classifyAxis ---

func TestClassifyAxis(t *testing.T) {
	tests := []struct {
		deg  float64
		want string
	}{
		{0, "нормальная"},
		{60, "нормальная"},
		{90, "нормальная"},
		{-30, "нормальная"},
		{91, "правограмма"},
		{180, "правограмма"},
		{-31, "левограмма"},
		{-90, "левограмма"},
		{-91, "резкое отклонение"},
		{181, "резкое отклонение"},
		{-180, "резкое отклонение"},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := classifyAxis(tt.deg)
			if got != tt.want {
				t.Errorf("classifyAxis(%v) = %q, want %q", tt.deg, got, tt.want)
			}
		})
	}
}

// --- qrsNet ---

func TestQrsNet(t *testing.T) {
	// Both present: R - |S|
	got := qrsNet(ptr(10), ptr(-6))
	if got == nil || *got != 4.0 {
		t.Errorf("expected 4.0 (10 - |−6|), got %v", got)
	}

	// R only
	got = qrsNet(ptr(10), nil)
	if got == nil || *got != 10.0 {
		t.Errorf("expected 10.0, got %v", got)
	}

	// S only
	got = qrsNet(nil, ptr(-5))
	if got == nil || *got != -5.0 {
		t.Errorf("expected -5.0 (0 - |−5|), got %v", got)
	}

	// Both nil
	got = qrsNet(nil, nil)
	if got != nil {
		t.Error("expected nil when both R and S are nil")
	}
}

// --- findTransitionZone ---

func TestFindTransitionZone(t *testing.T) {
	// R/S ratio ~1.0 at V3
	meas := map[string]*float64{
		"R_V1_mm": ptr(2),
		"S_V1_mm": ptr(-15),
		"R_V2_mm": ptr(5),
		"S_V2_mm": ptr(-10),
		"R_V3_mm": ptr(10),
		"S_V3_mm": ptr(-10), // ratio = 10/10 = 1.0 → transition
	}
	got := findTransitionZone(meas)
	if got != "V3" {
		t.Errorf("expected V3, got %q", got)
	}
}

func TestFindTransitionZone_NoTransition(t *testing.T) {
	meas := map[string]*float64{
		"R_V1_mm": ptr(2),
		"S_V1_mm": ptr(-15),
		"R_V6_mm": ptr(20),
		"S_V6_mm": ptr(-2),
	}
	got := findTransitionZone(meas)
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestFindTransitionZone_ZeroSWave(t *testing.T) {
	meas := map[string]*float64{
		"R_V3_mm": ptr(10),
		"S_V3_mm": ptr(0), // sAbs=0, should skip (division by zero guard)
	}
	got := findTransitionZone(meas)
	if got != "" {
		t.Errorf("expected empty when S=0, got %q", got)
	}
}

// --- computeStructuredResult (integration) ---

func TestComputeStructuredResult_SokolowLyon(t *testing.T) {
	// Sokolow-Lyon = |S_V1| + max(R_V5, R_V6)
	// SV1 = 20mm → 2.0 mV at 10 mm/mV
	// RV5 = 25mm → 2.5 mV
	// RV6 = 22mm → 2.2 mV
	// Expected: 2.0 + max(2.5, 2.2) = 4.5 mV
	meas := map[string]*float64{
		"SV1_mm": ptr(-20), // negative (S wave)
		"RV5_mm": ptr(25),
		"RV6_mm": ptr(22),
	}

	result := computeStructuredResult(meas, "male", intPtr(45), 10, 10, "2025-01-01T00:00:00Z", "test-job")

	if result.Indices == nil {
		t.Fatal("Indices is nil")
	}
	if !approxEqual(result.Indices.SokolowLyon, ptr(4.5), 0.01) {
		t.Errorf("Sokolow-Lyon: expected ~4.5, got %v", result.Indices.SokolowLyon)
	}
}

func TestComputeStructuredResult_CornellVoltage(t *testing.T) {
	// Cornell = R_aVL + |S_V3|
	// RaVL = 12mm → 1.2 mV (limb, 10 mm/mV)
	// SV3 = -18mm → 1.8 mV (chest, 10 mm/mV)
	// Expected: 1.2 + 1.8 = 3.0 mV
	meas := map[string]*float64{
		"RaVL_mm": ptr(12),
		"SV3_mm":  ptr(-18),
	}

	result := computeStructuredResult(meas, "female", intPtr(55), 10, 10, "2025-01-01T00:00:00Z", "test-job")

	if !approxEqual(result.Indices.CornellVoltage, ptr(3.0), 0.01) {
		t.Errorf("Cornell: expected ~3.0, got %v", result.Indices.CornellVoltage)
	}
}

func TestComputeStructuredResult_Gubner(t *testing.T) {
	// Gubner = R_I + |S_III|
	// R_I = 8mm → 0.8 mV (limb), S_III = -6mm → 0.6 mV (limb)
	// Expected: 0.8 + 0.6 = 1.4 mV
	meas := map[string]*float64{
		"R_I_mm":   ptr(8),
		"S_III_mm": ptr(-6),
	}

	result := computeStructuredResult(meas, "male", intPtr(50), 10, 10, "2025-01-01T00:00:00Z", "test-job")

	if !approxEqual(result.Indices.Gubner, ptr(1.4), 0.01) {
		t.Errorf("Gubner: expected ~1.4, got %v", result.Indices.Gubner)
	}
}

func TestComputeStructuredResult_Lewis(t *testing.T) {
	// Lewis = (R_I + |S_III|) − (|S_I| + R_III)
	// R_I = 10mm → 1.0 mV, S_III = -8mm → 0.8 mV
	// S_I = -2mm → 0.2 mV, R_III = 3mm → 0.3 mV
	// Expected: (1.0 + 0.8) − (0.2 + 0.3) = 1.3 mV
	meas := map[string]*float64{
		"R_I_mm":   ptr(10),
		"S_I_mm":   ptr(-2),
		"R_III_mm": ptr(3),
		"S_III_mm": ptr(-8),
	}

	result := computeStructuredResult(meas, "male", intPtr(50), 10, 10, "2025-01-01T00:00:00Z", "test-job")

	if !approxEqual(result.Indices.Lewis, ptr(1.3), 0.01) {
		t.Errorf("Lewis: expected ~1.3, got %v", result.Indices.Lewis)
	}
}

func TestComputeStructuredResult_RVH(t *testing.T) {
	// R_V1 = 8mm → 0.8 mV (chest), S_V1 = -4mm → 0.4 mV
	// SV5 = -3mm → 0.3 mV, SV6 = -2mm → 0.2 mV
	// RV1+SV5 = 0.8 + 0.3 = 1.1
	// RV1+SV6 = 0.8 + 0.2 = 1.0
	// R/S V1 = 0.8 / 0.4 = 2.0
	meas := map[string]*float64{
		"R_V1_mm": ptr(8),
		"S_V1_mm": ptr(-4),
		"SV5_mm":  ptr(-3),
		"SV6_mm":  ptr(-2),
	}

	result := computeStructuredResult(meas, "male", intPtr(40), 10, 10, "2025-01-01T00:00:00Z", "test-job")

	if result.RVH == nil {
		t.Fatal("RVH is nil")
	}
	if !approxEqual(result.RVH.RV1mV, ptr(0.8), 0.01) {
		t.Errorf("RV1: expected ~0.8, got %v", result.RVH.RV1mV)
	}
	if !approxEqual(result.RVH.ROverSV1, ptr(2.0), 0.01) {
		t.Errorf("R/S V1: expected ~2.0, got %v", result.RVH.ROverSV1)
	}
	if !approxEqual(result.RVH.RV1PlusSV5, ptr(1.1), 0.01) {
		t.Errorf("RV1+SV5: expected ~1.1, got %v", result.RVH.RV1PlusSV5)
	}
}

func TestComputeStructuredResult_QRSAxis(t *testing.T) {
	// Net I = R_I - |S_I| = 10 - 2 = 8mm → 0.8 mV
	// Net aVF = R_aVF - |S_aVF| = 6 - 1 = 5mm → 0.5 mV
	// Axis = atan2(0.5, 0.8) ≈ 32 degrees → нормальная
	meas := map[string]*float64{
		"R_I_mm":   ptr(10),
		"S_I_mm":   ptr(-2),
		"R_aVF_mm": ptr(6),
		"S_aVF_mm": ptr(-1),
	}

	result := computeStructuredResult(meas, "male", intPtr(30), 10, 10, "2025-01-01T00:00:00Z", "test-job")

	if result.Axis == nil {
		t.Fatal("Axis is nil")
	}
	expectedDeg := math.Atan2(0.5, 0.8) * 180 / math.Pi // ~32 degrees
	if !approxEqual(result.Axis.AxisDeg, &expectedDeg, 0.5) {
		t.Errorf("Axis: expected ~%.1f, got %v", expectedDeg, result.Axis.AxisDeg)
	}
	if result.Axis.Classification != "нормальная" {
		t.Errorf("Axis classification: expected нормальная, got %q", result.Axis.Classification)
	}
}

func TestComputeStructuredResult_LeftAxisDeviation(t *testing.T) {
	// Net I positive, Net aVF strongly negative → left axis deviation
	meas := map[string]*float64{
		"R_I_mm":   ptr(12),
		"S_I_mm":   ptr(-1),
		"R_aVF_mm": ptr(1),
		"S_aVF_mm": ptr(-10),
	}

	result := computeStructuredResult(meas, "male", intPtr(70), 10, 10, "2025-01-01T00:00:00Z", "test-job")

	if result.Axis == nil {
		t.Fatal("Axis is nil")
	}
	if result.Axis.Classification != "левограмма" {
		t.Errorf("expected левограмма, got %q", result.Axis.Classification)
	}
}

func TestComputeStructuredResult_RightAxisDeviation(t *testing.T) {
	// Net I negative, Net aVF positive → right axis deviation
	meas := map[string]*float64{
		"R_I_mm":   ptr(1),
		"S_I_mm":   ptr(-10),
		"R_aVF_mm": ptr(12),
		"S_aVF_mm": ptr(-1),
	}

	result := computeStructuredResult(meas, "male", intPtr(50), 10, 10, "2025-01-01T00:00:00Z", "test-job")

	if result.Axis == nil {
		t.Fatal("Axis is nil")
	}
	if result.Axis.Classification != "правограмма" {
		t.Errorf("expected правограмма, got %q", result.Axis.Classification)
	}
}

func TestComputeStructuredResult_Rhythm(t *testing.T) {
	meas := map[string]*float64{
		"QRS_ms": ptr(100),
		"RR_ms":  ptr(800),
		"HR_bpm": ptr(75),
	}

	result := computeStructuredResult(meas, "", nil, 10, 10, "2025-01-01T00:00:00Z", "test-job")

	if result.Rhythm == nil {
		t.Fatal("Rhythm is nil")
	}
	if !approxEqual(result.Rhythm.QRSms, ptr(100), 0.01) {
		t.Errorf("QRS_ms: expected 100, got %v", result.Rhythm.QRSms)
	}
	if !approxEqual(result.Rhythm.HRbpm, ptr(75), 0.01) {
		t.Errorf("HR_bpm: expected 75, got %v", result.Rhythm.HRbpm)
	}
}

func TestComputeStructuredResult_HRFromRR(t *testing.T) {
	// When HR is nil but RR is present, HR should be computed: 60000 / RR
	meas := map[string]*float64{
		"RR_ms": ptr(800), // → 75 bpm
	}

	result := computeStructuredResult(meas, "", nil, 10, 10, "2025-01-01T00:00:00Z", "test-job")

	if result.Rhythm == nil {
		t.Fatal("Rhythm is nil")
	}
	if !approxEqual(result.Rhythm.HRbpm, ptr(75), 0.01) {
		t.Errorf("HR computed from RR: expected 75, got %v", result.Rhythm.HRbpm)
	}
}

func TestComputeStructuredResult_PatientInfo(t *testing.T) {
	meas := map[string]*float64{}
	result := computeStructuredResult(meas, "female", intPtr(65), 10, 10, "2025-01-01T00:00:00Z", "test-job")

	if result.Patient.Sex != "female" {
		t.Errorf("expected female, got %q", result.Patient.Sex)
	}
	if result.Patient.Age == nil || *result.Patient.Age != 65 {
		t.Errorf("expected age 65, got %v", result.Patient.Age)
	}
	if result.Timestamp != "2025-01-01T00:00:00Z" {
		t.Errorf("timestamp: %s", result.Timestamp)
	}
	if result.JobID != "test-job" {
		t.Errorf("jobID: %s", result.JobID)
	}
}

func TestComputeStructuredResult_DifferentCalibration(t *testing.T) {
	// Limb: 10 mm/mV, Chest: 5 mm/mV (half sensitivity)
	// R_V5 = 25mm at 5 mm/mV = 5.0 mV
	// SV1 = -20mm at 5 mm/mV = 4.0 mV (abs)
	// Sokolow-Lyon = 4.0 + 5.0 = 9.0 mV
	meas := map[string]*float64{
		"SV1_mm": ptr(-20),
		"RV5_mm": ptr(25),
		"RV6_mm": ptr(10),
	}

	result := computeStructuredResult(meas, "male", intPtr(45), 10, 5, "2025-01-01T00:00:00Z", "test-job")

	if !approxEqual(result.Indices.SokolowLyon, ptr(9.0), 0.01) {
		t.Errorf("Sokolow-Lyon at 5 mm/mV chest: expected ~9.0, got %v", result.Indices.SokolowLyon)
	}
}

func TestComputeStructuredResult_TransitionZone(t *testing.T) {
	meas := map[string]*float64{
		"R_V1_mm": ptr(2),
		"S_V1_mm": ptr(-15),
		"R_V2_mm": ptr(5),
		"S_V2_mm": ptr(-12),
		"R_V3_mm": ptr(10),
		"S_V3_mm": ptr(-11), // R/|S| = 10/11 ≈ 0.91 → within [0.8, 1.2]
		"R_V4_mm": ptr(15),
		"S_V4_mm": ptr(-5),
	}

	result := computeStructuredResult(meas, "male", intPtr(45), 10, 10, "2025-01-01T00:00:00Z", "test-job")

	if result.Transition != "V3" {
		t.Errorf("expected transition at V3, got %q", result.Transition)
	}
}

// --- buildInterpretation ---

func TestBuildInterpretation_NilWhenNoData(t *testing.T) {
	result := computeStructuredResult(map[string]*float64{}, "", nil, 10, 10, "t", "j")
	if result.Interpretation != nil {
		t.Error("expected nil interpretation when no data")
	}
}

func TestBuildInterpretation_LVHPositive(t *testing.T) {
	// Sokolow-Lyon = 2.0 + 2.5 = 4.5 mV >= 3.5 → positive
	meas := map[string]*float64{
		"SV1_mm": ptr(-20), "RV5_mm": ptr(25), "RV6_mm": ptr(22),
	}
	result := computeStructuredResult(meas, "male", intPtr(50), 10, 10, "t", "j")
	if result.Interpretation == nil {
		t.Fatal("interpretation is nil")
	}
	// Check summary has LVH positive
	for _, s := range result.Interpretation.Summary {
		if s.Label == "Признаки ГЛЖ" {
			if s.Status != "positive" {
				t.Errorf("expected LVH positive, got %s", s.Status)
			}
			if s.Value != "выявлен отдельный признак" {
				t.Errorf("expected single sign, got %s", s.Value)
			}
			return
		}
	}
	t.Error("LVH summary item not found")
}

func TestBuildInterpretation_QRSAbnormal(t *testing.T) {
	meas := map[string]*float64{
		"QRS_ms": ptr(130),                    // > 100 ms → abnormal
		"SV1_mm": ptr(-10), "RV5_mm": ptr(10), // need at least one index for items
	}
	result := computeStructuredResult(meas, "male", nil, 10, 10, "t", "j")
	if result.Interpretation == nil {
		t.Fatal("interpretation is nil")
	}
	for _, it := range result.Interpretation.Items {
		if it.Label == "QRS" {
			if it.Status != "abnormal" {
				t.Errorf("expected QRS abnormal, got %s", it.Status)
			}
			return
		}
	}
	t.Error("QRS item not found")
}

func TestBuildInterpretation_FemaleCornellThreshold(t *testing.T) {
	// Cornell female threshold = 2.0 mV; male = 2.8 mV
	// RaVL=12mm=1.2mV + SV3=10mm=1.0mV = 2.2 mV
	// Male: 2.2 < 2.8 → negative. Female: 2.2 > 2.0 → positive.
	meas := map[string]*float64{
		"RaVL_mm": ptr(12), "SV3_mm": ptr(-10),
	}
	male := computeStructuredResult(meas, "male", nil, 10, 10, "t", "j")
	female := computeStructuredResult(meas, "female", nil, 10, 10, "t", "j")

	findCornell := func(interp *models.ECGInterpretation) string {
		if interp == nil {
			return ""
		}
		for _, it := range interp.Items {
			if it.Label == "Корнелл (RaVL+SV3)" {
				return string(it.Status)
			}
		}
		return ""
	}

	if findCornell(male.Interpretation) != "negative" {
		t.Errorf("male Cornell 2.2mV should be negative (thr 2.8), got %s", findCornell(male.Interpretation))
	}
	if findCornell(female.Interpretation) != "positive" {
		t.Errorf("female Cornell 2.2mV should be positive (thr 2.0), got %s", findCornell(female.Interpretation))
	}
}

// --- Benchmark ---

func BenchmarkComputeStructuredResult(b *testing.B) {
	meas := map[string]*float64{
		"R_I_mm": ptr(8), "S_I_mm": ptr(-2),
		"R_II_mm": ptr(10), "S_II_mm": ptr(-1),
		"R_III_mm": ptr(3), "S_III_mm": ptr(-6),
		"R_aVR_mm": ptr(1), "S_aVR_mm": ptr(-9),
		"R_aVL_mm": ptr(5), "S_aVL_mm": ptr(-1),
		"R_aVF_mm": ptr(7), "S_aVF_mm": ptr(-2),
		"R_V1_mm": ptr(2), "S_V1_mm": ptr(-15),
		"R_V2_mm": ptr(5), "S_V2_mm": ptr(-12),
		"R_V3_mm": ptr(10), "S_V3_mm": ptr(-10),
		"R_V4_mm": ptr(15), "S_V4_mm": ptr(-5),
		"R_V5_mm": ptr(18), "S_V5_mm": ptr(-3),
		"R_V6_mm": ptr(16), "S_V6_mm": ptr(-2),
		"SV1_mm": ptr(-20), "RV5_mm": ptr(25), "RV6_mm": ptr(22),
		"RaVL_mm": ptr(12), "SV3_mm": ptr(-18), "SV4_mm": ptr(-15),
		"S_deepest_mm": ptr(-22), "SV5_mm": ptr(-3), "SV6_mm": ptr(-2),
		"QRS_ms": ptr(90), "RR_ms": ptr(800), "HR_bpm": ptr(75),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		computeStructuredResult(meas, "male", intPtr(50), 10, 10, "2025-01-01T00:00:00Z", "bench")
	}
}
