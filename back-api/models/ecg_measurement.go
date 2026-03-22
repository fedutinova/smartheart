package models

// ECGStructuredResult is the top-level structured analysis result.
type ECGStructuredResult struct {
	Measurements map[string]*float64 `json:"measurements"`
	Indices      *LVHIndices         `json:"indices,omitempty"`
	RVH          *RVHData            `json:"rvh,omitempty"`
	Axis         *QRSAxis            `json:"axis_qrs,omitempty"`
	Rhythm       *RhythmTiming       `json:"rhythm,omitempty"`
	Transition   string              `json:"transition_zone_lead,omitempty"`
	Patient      PatientInfo         `json:"patient"`
	Timestamp    string              `json:"timestamp"`
	JobID        string              `json:"job_id"`
}

// LVHIndices holds left ventricular hypertrophy index values (mV).
type LVHIndices struct {
	SokolowLyon     *float64 `json:"sokolow_lyon_mV,omitempty"`
	CornellVoltage  *float64 `json:"cornell_voltage_mV,omitempty"`
	PegueroLoPresti *float64 `json:"peguero_lo_presti_mV,omitempty"`
	Gubner          *float64 `json:"gubner_mV,omitempty"`
	Lewis           *float64 `json:"lewis_mV,omitempty"`
}

// RVHData holds right ventricular hypertrophy markers.
type RVHData struct {
	RV1mV      *float64 `json:"RV1_mV,omitempty"`
	ROverSV1   *float64 `json:"R_over_S_V1,omitempty"`
	RV1PlusSV5 *float64 `json:"RV1_plus_SV5_mV,omitempty"`
	RV1PlusSV6 *float64 `json:"RV1_plus_SV6_mV,omitempty"`
}

// QRSAxis holds electrical axis data.
type QRSAxis struct {
	NetImV         *float64 `json:"net_I_mV,omitempty"`
	NetAVFmV       *float64 `json:"net_aVF_mV,omitempty"`
	AxisDeg        *float64 `json:"axis_deg,omitempty"`
	Classification string   `json:"classification,omitempty"`
}

// RhythmTiming holds basic rhythm measurements.
type RhythmTiming struct {
	QRSms *float64 `json:"QRS_ms,omitempty"`
	RRms  *float64 `json:"RR_ms,omitempty"`
	HRbpm *float64 `json:"HR_bpm,omitempty"`
}

// PatientInfo holds submitted patient demographics.
type PatientInfo struct {
	Sex string `json:"sex,omitempty"`
	Age *int   `json:"age,omitempty"`
}
