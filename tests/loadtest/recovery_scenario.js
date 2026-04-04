// H2 Property 1: Recovery after connection abort.
//
// Scenario:
//   1. Submit ECG analysis (POST /v1/ecg/analyze)
//   2. In async mode: get request_id, then do NOT poll — simulate disconnect
//   3. In sync mode: abort after 2s via timeout (before GPT finishes)
//   4. Wait for processing to complete (sleep)
//   5. Check if result is available (GET /v1/requests/{id})
//
// Usage:
//   k6 run -e BASE_URL=http://localhost:8080 -e MODE=async tests/loadtest/recovery_scenario.js
//   k6 run -e BASE_URL=http://localhost:8080 -e MODE=sync  tests/loadtest/recovery_scenario.js

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Counter } from 'k6/metrics';
import { BASE_URL, TEST_USER } from './config.js';

const recoveryRate = new Rate('recovery_rate');
const abortedCount = new Counter('aborted_count');
const recoveredCount = new Counter('recovered_count');

const MODE = __ENV.MODE || 'async';
const ITERATIONS = parseInt(__ENV.ITERATIONS || '20');
const VUS = parseInt(__ENV.VUS || '10');

export const options = {
  scenarios: {
    recovery: {
      executor: 'shared-iterations',
      vus: VUS,
      iterations: ITERATIONS,
      maxDuration: '20m',
    },
  },
  thresholds: {
    // async should recover >= 95%; sync expected to be ~0%
    recovery_rate: MODE === 'async' ? ['rate>=0.95'] : [],
  },
};

const ecgImage = open('./data/test_ekg.jpg', 'b');

export function setup() {
  const tokens = [];
  for (let i = 0; i < VUS; i++) {
    const ts = Date.now();
    const user = {
      username: `recvu${i}_${ts}`,
      email: `recvu${i}_${ts}@loadtest.local`,
      password: TEST_USER.password,
    };
    http.post(`${BASE_URL}/v1/auth/register`, JSON.stringify(user), {
      headers: { 'Content-Type': 'application/json' },
    });
    const loginRes = http.post(`${BASE_URL}/v1/auth/login`, JSON.stringify({
      email: user.email, password: user.password,
    }), { headers: { 'Content-Type': 'application/json' } });
    const body = JSON.parse(loginRes.body);
    if (!body.access_token) throw new Error(`Login failed for VU ${i}: ${loginRes.body}`);
    tokens.push(body.access_token);
  }

  // Reset concurrency counter before run.
  http.post(`${BASE_URL}/debug/h2/reset`);

  return { tokens };
}

export default function (data) {
  const token = data.tokens[(__VU - 1) % data.tokens.length];
  const headers = { Authorization: `Bearer ${token}` };
  const fd = http.file(ecgImage, 'test_ecg.jpg', 'image/jpeg');

  let requestId = null;

  if (MODE === 'async') {
    // Async: submit returns immediately with request_id.
    const submitRes = http.post(`${BASE_URL}/v1/ecg/analyze`, { image: fd }, {
      headers,
      timeout: '10s',
    });
    if (submitRes.status === 200) {
      try {
        requestId = JSON.parse(submitRes.body).request_id;
      } catch (_) {}
    }
    // "Disconnect": we simply do NOT poll. The task is in the queue.
  } else {
    // Sync: submit with short timeout to simulate abort mid-processing.
    // GPT_MOCK_DELAY=15s, so 2s timeout will abort before completion.
    const submitRes = http.post(`${BASE_URL}/v1/ecg/analyze`, { image: fd }, {
      headers,
      timeout: '2s',
    });
    // In sync mode, if we got a response it means processing finished fast enough.
    if (submitRes.status === 200) {
      try {
        requestId = JSON.parse(submitRes.body).request_id;
      } catch (_) {}
    }
    // If timed out, requestId stays null — we can't recover what we don't know.
  }

  abortedCount.add(1);

  if (!requestId) {
    // No request_id means we can't even check — count as not recovered.
    recoveryRate.add(0);
    return;
  }

  // Wait for all queued jobs to finish. With QUEUE_WORKERS=4 and
  // GPT_MOCK_DELAY=15s, worst-case queue drain is ceil(ITERATIONS/4)*15s.
  // We wait that + buffer. Each VU sleeps the full duration once.
  const worstCaseSec = Math.ceil(ITERATIONS / 4) * 17;
  sleep(worstCaseSec);

  // Try to recover the result.
  const pollRes = http.get(`${BASE_URL}/v1/requests/${requestId}`, { headers });

  let recovered = false;
  if (pollRes.status === 200) {
    try {
      const req = JSON.parse(pollRes.body);
      recovered = req.status === 'completed';
    } catch (_) {}
  }

  if (recovered) {
    recoveredCount.add(1);
  }
  recoveryRate.add(recovered);

  check(null, {
    'result recovered after disconnect': () => recovered,
  });
}

export function teardown(data) {
  // Fetch concurrency stats.
  const res = http.get(`${BASE_URL}/debug/h2`);
  if (res.status === 200) {
    console.log(`[H2] max_concurrent_gpt: ${JSON.parse(res.body).max_concurrent_gpt}`);
  }
}
