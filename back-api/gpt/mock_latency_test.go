package gpt

import (
	"context"
	"testing"
	"time"
)

func TestMock_PerCallDelayReportedInProcessingTime(t *testing.T) {
	m := &MockProcessor{
		Delay:          5 * time.Millisecond,
		MeasureDelay:   20 * time.Millisecond,
		InterpretDelay: 40 * time.Millisecond,
		// Jitter 0 → deterministic, ProcessingTimeMs equals the per-call delay.
	}
	ctx := context.Background()

	meas, err := m.ProcessStructuredECG(ctx, nil, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if meas.ProcessingTimeMs != 20 {
		t.Errorf("measurement should report MeasureDelay (20ms), got %d", meas.ProcessingTimeMs)
	}

	interp, err := m.InterpretStructuredECG(ctx, nil, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if interp.ProcessingTimeMs != 40 {
		t.Errorf("interpretation should report InterpretDelay (40ms), got %d", interp.ProcessingTimeMs)
	}

	gen, err := m.ProcessRequest(ctx, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if gen.ProcessingTimeMs != 5 {
		t.Errorf("ProcessRequest should fall back to Delay (5ms), got %d", gen.ProcessingTimeMs)
	}
}

func TestMock_SampleDelayFallbackAndJitterBounds(t *testing.T) {
	m := &MockProcessor{Delay: 100 * time.Millisecond}

	// base 0 → falls back to Delay; jitter 0 → exact.
	if got := m.sampleDelay(0); got != 100*time.Millisecond {
		t.Errorf("expected fallback to Delay 100ms, got %v", got)
	}

	// With jitter, every sample stays within ±jitter of the base.
	m.Jitter = 0.5
	base := 200 * time.Millisecond
	lo := time.Duration(float64(base) * 0.5)
	hi := time.Duration(float64(base) * 1.5)
	for range 1000 {
		d := m.sampleDelay(base)
		if d < lo || d > hi {
			t.Fatalf("jittered delay %v out of bounds [%v, %v]", d, lo, hi)
		}
	}
}
