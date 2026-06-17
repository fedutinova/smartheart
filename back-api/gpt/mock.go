package gpt

import (
	"context"
	"crypto/sha256"
	"math/rand"
	"sync/atomic"
	"time"
)

// MockProcessor simulates GPT responses with prod-calibrated latency, so load
// tests can measure the real queue/worker behaviour without spending OpenAI
// quota. Activated via GPT_MOCK=true.
type MockProcessor struct {
	// Delay is the baseline simulated latency and the fallback when a per-call
	// delay below is zero (also used by ProcessRequest).
	Delay time.Duration
	// MeasureDelay / InterpretDelay model the two sequential ECG GPT calls with
	// their real wall times (calibrate from responses.processing_time_ms in prod).
	MeasureDelay   time.Duration
	InterpretDelay time.Duration
	// Jitter varies each simulated delay by ±Jitter (fraction 0..1) so the queue
	// saturates under realistic, non-uniform latency instead of a flat constant.
	Jitter float64

	concurrent int64 // current number of in-flight calls
	MaxConc    int64 // high-water mark — max observed concurrent calls
}

// sampleDelay returns base (or Delay when base is zero) varied by ±Jitter.
func (m *MockProcessor) sampleDelay(base time.Duration) time.Duration {
	if base <= 0 {
		base = m.Delay
	}
	if m.Jitter <= 0 || base <= 0 {
		return base
	}
	factor := 1 + m.Jitter*(2*rand.Float64()-1) //nolint:gosec // load-test jitter, not security-sensitive
	if factor < 0 {
		factor = 0
	}
	return time.Duration(float64(base) * factor)
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
	d := m.sampleDelay(0)
	if err := simulateWork(ctx, d); err != nil {
		return nil, err
	}
	return &ProcessResult{
		Content:          "Mock GPT response for load testing.",
		Model:            "mock",
		TokensUsed:       100,
		ProcessingTimeMs: int(d.Milliseconds()),
	}, nil
}

func (m *MockProcessor) ProcessStructuredECG(ctx context.Context, _ []string, _, _ string) (*ProcessResult, error) {
	done := m.trackConcurrency()
	defer done()
	d := m.sampleDelay(m.MeasureDelay)
	if err := simulateWork(ctx, d); err != nil {
		return nil, err
	}
	return &ProcessResult{
		Content:          mockECGResponse,
		Model:            "mock",
		TokensUsed:       200,
		ProcessingTimeMs: int(d.Milliseconds()),
	}, nil
}

func (m *MockProcessor) InterpretStructuredECG(ctx context.Context, _ []string, _, _ string) (*ProcessResult, error) {
	done := m.trackConcurrency()
	defer done()
	d := m.sampleDelay(m.InterpretDelay)
	if err := simulateWork(ctx, d); err != nil {
		return nil, err
	}
	// Mirror the real interpretation prompt: a single JSON object with the
	// Markdown "## Итог" section plus a structured rhythm read (incl. the
	// agrees-with-classifier flag), so GPT_MOCK output matches prod shape.
	return &ProcessResult{
		Content:          `{"interpretation_md":"## Итог\n- Тестовая интерпретация для режима GPT_MOCK без клинических выводов.\n- Данные демонстрационные; реальная оценка ритма и ЭКГ-паттернов не выполняется.\n- Уверенность низкая: это фиктивный ответ mock-сервиса.","rhythm":{"code":"UNCLEAR","label_ru":"ритм не оценивается (mock)","agrees_with_classifier":true}}`,
		Model:            "mock",
		TokensUsed:       120,
		ProcessingTimeMs: int(d.Milliseconds()),
	}, nil
}
