// Combined ECG + KB load test — runs both scenarios in parallel.
// Each VU registers and logs in as its own user to avoid per-user rate limits.
// Usage: K6_PROFILE=base k6 run tests/loadtest/combined_scenario.js

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Trend, Rate } from 'k6/metrics';
import { SharedArray } from 'k6/data';
import { BASE_URL, POLL_INTERVAL_MS, POLL_TIMEOUT_MS, TARGET_THRESHOLD_MS, TEST_USER, getProfile } from './config.js';

// -- Metrics --
const ecgDuration = new Trend('ecg_duration', true);
const ecgSuccess  = new Rate('ecg_success');
const kbDuration  = new Trend('kb_duration', true);
const kbSuccess   = new Rate('kb_success');

// -- Data --
const ecgImage = open('./data/test_ekg.jpg', 'b');
const questions = new SharedArray('questions', function () {
  return JSON.parse(open('./data/questions.json'));
});

const profile = getProfile();
const maxVUs = profile.vus;

export const options = {
  scenarios: {
    ecg: {
      executor: 'shared-iterations',
      vus: Math.max(1, Math.floor(profile.vus / 2)),
      iterations: Math.floor(profile.iterations / 2),
      maxDuration: '30m',
      exec: 'ecgScenario',
    },
    kb: {
      executor: 'shared-iterations',
      vus: Math.max(1, Math.ceil(profile.vus / 2)),
      iterations: Math.ceil(profile.iterations / 2),
      maxDuration: '30m',
      exec: 'kbScenario',
    },
  },
  thresholds: {
    ecg_success: ['rate>0.95'],
    ecg_duration: [`p(95)<${TARGET_THRESHOLD_MS}`],
    kb_success: ['rate>0.95'],
    kb_duration: [`p(95)<${TARGET_THRESHOLD_MS}`],
  },
};

// Create one user per VU slot in setup, so each VU gets its own token.
export function setup() {
  const tokens = [];
  for (let i = 0; i < maxVUs; i++) {
    const ts = Date.now();
    const user = {
      username: `h2vu${i}_${ts}`,
      email: `h2vu${i}_${ts}@loadtest.local`,
      password: TEST_USER.password,
    };

    http.post(`${BASE_URL}/v1/auth/register`, JSON.stringify(user), {
      headers: { 'Content-Type': 'application/json' },
    });

    const loginRes = http.post(`${BASE_URL}/v1/auth/login`, JSON.stringify({
      email: user.email,
      password: user.password,
    }), { headers: { 'Content-Type': 'application/json' } });

    const body = JSON.parse(loginRes.body);
    if (!body.access_token) {
      throw new Error(`Login failed for VU ${i}: ${loginRes.body}`);
    }
    tokens.push(body.access_token);
  }
  return { tokens };
}

function getToken(data) {
  // Each VU picks its own token by VU index (mod token count for safety).
  return data.tokens[(__VU - 1) % data.tokens.length];
}

export function ecgScenario(data) {
  const token = getToken(data);
  const headers = { Authorization: `Bearer ${token}` };
  const fd = http.file(ecgImage, 'test_ecg.jpg', 'image/jpeg');

  const submitRes = http.post(`${BASE_URL}/v1/ecg/analyze`, { image: fd }, {
    headers,
    timeout: '30s',
  });

  if (submitRes.status !== 200) {
    ecgSuccess.add(0);
    return;
  }

  const { request_id } = JSON.parse(submitRes.body);
  if (!request_id) {
    ecgSuccess.add(0);
    return;
  }

  const start = Date.now();
  let status = 'pending';

  while (Date.now() - start < POLL_TIMEOUT_MS) {
    sleep(POLL_INTERVAL_MS / 1000);
    const pollRes = http.get(`${BASE_URL}/v1/requests/${request_id}`, { headers });
    if (pollRes.status === 200) {
      const req = JSON.parse(pollRes.body);
      status = req.status;
      if (status === 'completed' || status === 'failed') break;
    }
  }

  const elapsed = Date.now() - start;
  const success = status === 'completed';
  ecgDuration.add(elapsed);
  ecgSuccess.add(success);

  check(null, {
    'ecg completed': () => success,
    'ecg under 30s': () => success && elapsed <= TARGET_THRESHOLD_MS,
  });
}

export function kbScenario(data) {
  const token = getToken(data);
  const question = questions[Math.floor(Math.random() * questions.length)];

  const res = http.post(`${BASE_URL}/v1/rag/query`, JSON.stringify({
    question,
    n_results: 5,
  }), {
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    timeout: '130s',
  });

  const success = res.status === 200;
  const elapsed = res.timings.duration;
  kbDuration.add(elapsed);
  kbSuccess.add(success);

  check(res, {
    'kb status 200': (r) => r.status === 200,
    'kb under 30s': () => success && elapsed <= TARGET_THRESHOLD_MS,
  });
}
