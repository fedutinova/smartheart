// H3 hypothesis test: semantic cache for KB queries.
// Sends all questions sequentially — first pass fills the cache, second pass tests hits.
// Usage: k6 run tests/loadtest/h3_cache_scenario.js

import http from 'k6/http';
import { check } from 'k6';
import { Trend, Rate, Counter } from 'k6/metrics';
import { SharedArray } from 'k6/data';
import { BASE_URL, TEST_USER } from './config.js';

const kbDuration   = new Trend('kb_duration', true);
const kbSuccess    = new Rate('kb_success');
const cacheHits    = new Counter('cache_hits');
const cacheMisses  = new Counter('cache_misses');

const questions = new SharedArray('questions_h3', function () {
  return JSON.parse(open('./data/questions_h3.json'));
});

export const options = {
  scenarios: {
    h3: {
      executor: 'shared-iterations',
      vus: 1,       // Sequential to control cache fill order.
      iterations: questions.length,
      maxDuration: '30m',
    },
  },
};

export function setup() {
  const ts = Date.now();
  const user = {
    username: `h3test_${ts}`,
    email: `h3test_${ts}@loadtest.local`,
    password: TEST_USER.password,
  };
  http.post(`${BASE_URL}/v1/auth/register`, JSON.stringify(user), {
    headers: { 'Content-Type': 'application/json' },
  });
  const loginRes = http.post(`${BASE_URL}/v1/auth/login`, JSON.stringify({
    email: user.email, password: user.password,
  }), { headers: { 'Content-Type': 'application/json' } });
  const body = JSON.parse(loginRes.body);
  if (!body.access_token) throw new Error(`Login failed: ${loginRes.body}`);
  return { token: body.access_token };
}

export default function (data) {
  const q = questions[__ITER % questions.length];

  const res = http.post(`${BASE_URL}/v1/rag/query`, JSON.stringify({
    question: q.text,
    n_results: 5,
  }), {
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${data.token}`,
    },
    timeout: '120s',
  });

  const success = res.status === 200;
  const elapsed = res.timings.duration;
  const cacheHeader = res.headers['X-Cache'] || 'UNKNOWN';

  kbDuration.add(elapsed);
  kbSuccess.add(success);

  if (cacheHeader === 'HIT') {
    cacheHits.add(1);
  } else {
    cacheMisses.add(1);
  }

  check(res, {
    'status 200': (r) => r.status === 200,
    'has answer': (r) => {
      try { return JSON.parse(r.body).answer !== undefined; }
      catch { return false; }
    },
  });
}
