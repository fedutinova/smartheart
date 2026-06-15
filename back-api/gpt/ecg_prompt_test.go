package gpt

import (
	"fmt"
	"strings"
	"testing"
)

func TestParseECGMeasurementJSON_LongValidJSON(t *testing.T) {
	var leads []string
	for i := 0; i < 80; i++ {
		leads = append(leads, fmt.Sprintf(`"X%d":{"R_up_sq":[1,2,3],"S_down_sq":[-1,-2,-3]}`, i))
	}
	raw := fmt.Sprintf(`{
		"leads":{"I":{"R_up_sq":[4,4.5,4],"S_down_sq":[-2,-2,-2]},%s},
		"extras":{"SV1_sq":[20,21,20]},
		"intervals_sq":{"QRS":[2,2.5,2],"RR":[20,20,21]},
		"HR_bpm":72,
		"calibration":{"mv_pulse_height_small_squares":10,"paper_speed_small_squares_per_sec":25}
	}`, strings.Join(leads, ","))

	if len(raw) <= 2000 {
		t.Fatalf("test JSON must be longer than previous truncation limit, got %d", len(raw))
	}

	parsed, err := ParseECGMeasurementJSON(raw)
	if err != nil {
		t.Fatalf("ParseECGMeasurementJSON failed for long valid JSON: %v", err)
	}
	if parsed == nil || parsed.HRBpm == nil || *parsed.HRBpm != 72 {
		t.Fatalf("parsed result lost HR_bpm: %+v", parsed)
	}
	if _, ok := parsed.Leads["I"]; !ok {
		t.Fatalf("parsed result lost lead I: %+v", parsed.Leads)
	}
}
