package gpt

import (
	"context"
	"crypto/sha256"
	"sync/atomic"
	"time"
)

// MockProcessor simulates GPT responses with a fixed delay.
// Activated via GPT_MOCK=true for load testing without OpenAI API calls.
type MockProcessor struct {
	Delay      time.Duration
	concurrent int64 // current number of in-flight calls
	MaxConc    int64 // high-water mark — max observed concurrent calls
}

// ConcurrentMax returns the peak number of concurrent GPT calls observed.
func (m *MockProcessor) ConcurrentMax() int64 {
	return atomic.LoadInt64(&m.MaxConc)
}

// ResetConcurrentMax resets the high-water mark to zero.
func (m *MockProcessor) ResetConcurrentMax() {
	atomic.StoreInt64(&m.MaxConc, 0)
}

func (m *MockProcessor) trackConcurrency() func() {
	cur := atomic.AddInt64(&m.concurrent, 1)
	for {
		old := atomic.LoadInt64(&m.MaxConc)
		if cur <= old || atomic.CompareAndSwapInt64(&m.MaxConc, old, cur) {
			break
		}
	}
	return func() { atomic.AddInt64(&m.concurrent, -1) }
}

// Static ECG response — valid JSON matching RawECGMeasurement schema.
const mockECGResponse = `{
  "leads": {
    "I":   {"R_up_sq": [3.0], "S_down_sq": [1.0]},
    "II":  {"R_up_sq": [5.0], "S_down_sq": [0.5]},
    "III": {"R_up_sq": [2.0], "S_down_sq": [1.5]},
    "aVR": {"R_up_sq": [0.5], "S_down_sq": [4.0]},
    "aVL": {"R_up_sq": [2.5], "S_down_sq": [1.0]},
    "aVF": {"R_up_sq": [3.5], "S_down_sq": [0.5]},
    "V1":  {"R_up_sq": [1.0], "S_down_sq": [6.0]},
    "V2":  {"R_up_sq": [2.0], "S_down_sq": [5.0]},
    "V3":  {"R_up_sq": [4.0], "S_down_sq": [3.0]},
    "V4":  {"R_up_sq": [6.0], "S_down_sq": [1.5]},
    "V5":  {"R_up_sq": [7.0], "S_down_sq": [0.5]},
    "V6":  {"R_up_sq": [6.0], "S_down_sq": [0.5]}
  },
  "intervals_sq": {
    "QRS": [2.0, 2.5, 2.0],
    "RR":  [20.0, 19.5, 20.5]
  },
  "HR_bpm": 75,
  "calibration": {
    "mv_pulse_height_small_squares": 10,
    "paper_speed_small_squares_per_sec": 25
  }
}`

// simulateWork sleeps for most of the duration (I/O wait, like real GPT calls)
// then burns CPU briefly to make the goroutine visible in profiling.
func simulateWork(ctx context.Context, d time.Duration) error {
	// 90% sleep (I/O simulation) + 10% CPU burst.
	sleepDur := d * 9 / 10
	burstDur := d - sleepDur

	t := time.NewTimer(sleepDur)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
	}

	// Short CPU burst.
	deadline := time.Now().Add(burstDur)
	h := sha256.New()
	data := []byte("mock-workload")
	for time.Now().Before(deadline) {
		h.Reset()
		h.Write(data)
		data = h.Sum(data[:0])
	}
	return nil
}

func (m *MockProcessor) ProcessRequest(ctx context.Context, _ string, _ []string) (*ProcessResult, error) {
	done := m.trackConcurrency()
	defer done()
	if err := simulateWork(ctx, m.Delay); err != nil {
		return nil, err
	}
	return &ProcessResult{
		Content:          "Mock GPT response for load testing.",
		Model:            "mock",
		TokensUsed:       100,
		ProcessingTimeMs: int(m.Delay.Milliseconds()),
	}, nil
}

func (m *MockProcessor) ProcessStructuredECG(ctx context.Context, _ []string, _, _ string) (*ProcessResult, error) {
	done := m.trackConcurrency()
	defer done()
	if err := simulateWork(ctx, m.Delay); err != nil {
		return nil, err
	}
	return &ProcessResult{
		Content:          mockECGResponse,
		Model:            "mock",
		TokensUsed:       200,
		ProcessingTimeMs: int(m.Delay.Milliseconds()),
	}, nil
}
