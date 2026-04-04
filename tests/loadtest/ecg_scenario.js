import http from 'k6/http';
import { check, sleep } from 'k6';
import { Trend, Rate } from 'k6/metrics';
import { BASE_URL, POLL_INTERVAL_MS, POLL_TIMEOUT_MS, TARGET_THRESHOLD_MS, TEST_USER, getProfile } from './config.js';

const ecgDuration = new Trend('ecg_duration', true);
const ecgSuccess  = new Rate('ecg_success');

const profile = getProfile();

export const options = {
  scenarios: {
    ecg: {
      executor: 'shared-iterations',
      vus: profile.vus,
      iterations: profile.iterations,
      maxDuration: '30m',
    },
  },
  thresholds: {
    ecg_success: ['rate>0.95'],
    ecg_duration: [`p(95)<${TARGET_THRESHOLD_MS}`],
  },
};

const ecgImage = open('./data/test_ekg.jpg', 'b');

export function setup() {
  const tokens = [];
  for (let i = 0; i < profile.vus; i++) {
    const ts = Date.now();
    const user = {
      username: `ecgvu${i}_${ts}`,
      email: `ecgvu${i}_${ts}@loadtest.local`,
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
  const headers = { Authorization: `Bearer ${token}` };
  const fd = http.file(ecgImage, 'test_ecg.jpg', 'image/jpeg');

  const submitRes = http.post(`${BASE_URL}/v1/ecg/analyze`, { image: fd }, {
    headers, timeout: '30s',
  });

  if (submitRes.status !== 200) {
    ecgSuccess.add(0);
    return;
  }

  const { request_id } = JSON.parse(submitRes.body);
  if (!request_id) { ecgSuccess.add(0); return; }

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
