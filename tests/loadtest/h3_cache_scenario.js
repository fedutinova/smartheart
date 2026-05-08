// H3 hypothesis test: hybrid pg_trgm + pgvector cache for KB queries.
//
// Dataset split:
//   fill:     canonical seed questions that populate the cache
//   measure:  held-out phrasings of the same topics, unseen during fill
//   negative: lexically/semantically close questions where cache hits are risky
//
// Usage:
//   # Pass 1 — fill cache (clear kb_cache table first):
//   docker run --rm -i --network host \
//     -v $(pwd)/tests/loadtest:/tests \
//     grafana/k6:latest run /tests/h3_cache_scenario.js \
//     -e BASE_URL=http://localhost:8080 -e RUN_NAME=fill
//
//   # Pass 2 — measure:
//   docker run --rm -i --network host \
//     -v $(pwd)/tests/loadtest:/tests \
//     grafana/k6:latest run /tests/h3_cache_scenario.js \
//     -e BASE_URL=http://localhost:8080 -e RUN_NAME=measure

import http from 'k6/http';
import { check } from 'k6';
import { Trend, Rate, Counter } from 'k6/metrics';
import { SharedArray } from 'k6/data';
import { BASE_URL } from './config.js';

const kbDuration  = new Trend('kb_duration', true);
const kbSuccess   = new Rate('kb_success');
const cacheHits   = new Counter('cache_hits');
const cacheMisses = new Counter('cache_misses');

const RUN_NAME = __ENV.RUN_NAME || 'measure';
const DATASET_BY_RUN = {
  fill: './data/questions_h3_fill.json',
  measure: './data/questions_h3_heldout.json',
  negative: './data/questions_h3_negative.json',
};
const datasetPath = __ENV.H3_DATASET || DATASET_BY_RUN[RUN_NAME] || './data/questions_h3_heldout.json';

const questions = new SharedArray('questions_h3', function () {
  return JSON.parse(open(datasetPath));
});

export const options = {
  scenarios: {
    h3: {
      executor: 'shared-iterations',
      vus: 1,
      iterations: questions.length,
      maxDuration: '60m',
    },
  },
};

// Per-request data is emitted via console.log (RESULT:<json>) and
// parsed from the k6 output file after the run. handleSummary runs
// in a separate JS context and cannot access this array.

export function setup() {
  const ts = Date.now();
  const user = {
    username: `h3test_${ts}`,
    email: `h3test_${ts}@loadtest.local`,
    password: 'LoadTest_2026!',
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

  const success  = res.status === 200;
  const elapsed  = res.timings.duration;
  const cacheHdr = res.headers['X-Cache'] || 'MISS';
  const isHit    = cacheHdr === 'HIT';

  kbDuration.add(elapsed);
  kbSuccess.add(success);
  if (isHit) { cacheHits.add(1); } else { cacheMisses.add(1); }

  // Emit structured line — parsed after run to build per-request dataset.
  console.log('RESULT:' + JSON.stringify({
    iter:        __ITER,
    run:         RUN_NAME,
    question_id: __ITER % questions.length,
    question:    q.text,
    group:       q.group,
    duration_ms: Math.round(elapsed * 100) / 100,
    cache_hit:   isHit,
    status:      res.status,
  }));

  check(res, {
    'status 200': (r) => r.status === 200,
    'has answer': (r) => {
      try { return JSON.parse(r.body).answer !== undefined; }
      catch { return false; }
    },
  });
}

export function handleSummary(data) {
  // Per-request data is in the k6 output log (RESULT: lines).
  // A post-run Python script parses those and builds the final JSON.
  // Here we only log k6's native aggregated metrics.
  const d = data.metrics;
  const hits    = d['cache_hits']   ? d['cache_hits'].values.count   : 0;
  const misses  = d['cache_misses'] ? d['cache_misses'].values.count : 0;
  const total   = hits + misses;

  const summary = {
    run_name:          RUN_NAME,
    run_date:          new Date().toISOString().slice(0, 10),
    dataset_path:      datasetPath,
    question_set_size: questions.length,
    n_total:           total,
    n_cache_hits:      hits,
    n_rag_calls:       misses,
    cache_hit_rate:    total > 0 ? Math.round(hits / total * 10000) / 10000 : null,
    kb_duration_p95:   d['kb_duration'] ? d['kb_duration'].values['p(95)'] : null,
    kb_duration_avg:   d['kb_duration'] ? d['kb_duration'].values['avg']   : null,
  };

  console.log('\n=== H3 Summary ===');
  console.log(JSON.stringify(summary, null, 2));

  return {
    stdout: JSON.stringify(summary, null, 2) + '\n',
  };
}
