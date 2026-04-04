import http from 'k6/http';
import { check } from 'k6';
import { Trend, Rate } from 'k6/metrics';
import { SharedArray } from 'k6/data';
import { BASE_URL, TARGET_THRESHOLD_MS, TEST_USER, getProfile } from './config.js';

const kbDuration = new Trend('kb_duration', true);
const kbSuccess  = new Rate('kb_success');

const questions = new SharedArray('questions', function () {
  return JSON.parse(open('./data/questions.json'));
});

const profile = getProfile();

export const options = {
  scenarios: {
    kb: {
      executor: 'shared-iterations',
      vus: profile.vus,
      iterations: profile.iterations,
      maxDuration: '30m',
    },
  },
  thresholds: {
    kb_success: ['rate>0.95'],
    kb_duration: [`p(95)<${TARGET_THRESHOLD_MS}`],
  },
};

export function setup() {
  const tokens = [];
  for (let i = 0; i < profile.vus; i++) {
    const ts = Date.now();
    const user = {
      username: `kbvu${i}_${ts}`,
      email: `kbvu${i}_${ts}@loadtest.local`,
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
  return { tokens };
}

export default function (data) {
  const token = data.tokens[(__VU - 1) % data.tokens.length];
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
