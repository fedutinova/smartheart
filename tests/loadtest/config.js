// Shared configuration for H2 load tests.

export const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
export const POLL_INTERVAL_MS = 2000;
export const POLL_TIMEOUT_MS = 35000;
export const TARGET_THRESHOLD_MS = 30000; // H2 target: result under 30s

// Test user credentials (created during setup).
const ts = Date.now();
export const TEST_USER = {
  username: `h2test_${ts}`,
  email: `h2test_${ts}@loadtest.local`,
  password: 'LoadTest_H2_2026!',
};

// Load profiles selected via K6_PROFILE env var.
// "working" is the primary profile — concurrent load to show async benefit.
const PROFILES = {
  base:     { vus: 3,  iterations: 50  },
  working:  { vus: 5,  iterations: 100 },
  elevated: { vus: 10, iterations: 200 },
  stress:   { vus: 20, iterations: 200 },
};

export function getProfile() {
  const name = __ENV.K6_PROFILE || 'base';
  return PROFILES[name] || PROFILES.base;
}
