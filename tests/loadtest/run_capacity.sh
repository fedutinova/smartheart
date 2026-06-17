#!/usr/bin/env bash
# Capacity load-test runner for the ECG pipeline.
#
# Drives the full server pipeline (auth → queue → 4 workers → DB → SSE) with the
# prod-calibrated GPT mock, so the queue saturates with REAL GPT wall times and
# we measure our own capacity without spending OpenAI quota. See
# docs/loadtest-plan.md.
#
# Usage:
#   ./tests/loadtest/run_capacity.sh [profile ...]
#   profiles: base working elevated stress   (default: all four)
#
# Prereqs: docker + docker compose. The script (re)creates the app container with
# the calibrated GPT_MOCK env and brings up its dependencies.
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
RESULTS_DIR="$SCRIPT_DIR/results"
TS="$(date +%Y%m%d_%H%M%S)"

# Prod-calibrated GPT latency (docs/loadtest-plan.md, n=81: p50 9.6s / p95 13.5s).
# Override by exporting these before running.
export GPT_MOCK=true
export GPT_MOCK_MEASURE_DELAY="${GPT_MOCK_MEASURE_DELAY:-5.3s}"
export GPT_MOCK_INTERPRET_DELAY="${GPT_MOCK_INTERPRET_DELAY:-4.3s}"
export GPT_MOCK_JITTER="${GPT_MOCK_JITTER:-0.4}"
# Poll long enough to record true e2e even when the queue is saturated.
POLL_TIMEOUT_MS="${POLL_TIMEOUT_MS:-180000}"

PROFILES=("$@")
[[ ${#PROFILES[@]} -eq 0 ]] && PROFILES=(base working elevated stress)

mkdir -p "$RESULTS_DIR"
log() { echo "[run_capacity] $*"; }

log "Recreating app with calibrated GPT mock (measure=$GPT_MOCK_MEASURE_DELAY interpret=$GPT_MOCK_INTERPRET_DELAY jitter=$GPT_MOCK_JITTER)..."
( cd "$ROOT_DIR" && docker compose up -d --force-recreate app )

log "Waiting for $BASE_URL/health ..."
for _ in $(seq 1 30); do
  curl -fsS "$BASE_URL/health" >/dev/null 2>&1 && break
  sleep 2
done

for prof in "${PROFILES[@]}"; do
  log "=== profile: $prof ==="
  docker run --rm -i --network host \
    -v "$SCRIPT_DIR":/tests \
    grafana/k6:latest run /tests/ecg_scenario.js \
    -e BASE_URL="$BASE_URL" \
    -e K6_PROFILE="$prof" \
    -e POLL_TIMEOUT_MS="$POLL_TIMEOUT_MS" \
    --summary-export "/tests/results/${TS}_capacity_${prof}_summary.json" \
    2>&1 | tee "$RESULTS_DIR/${TS}_capacity_${prof}_k6.log"
done

log "── Results (${TS}) ───────────────────────────────────────────────"
for prof in "${PROFILES[@]}"; do
  f="$RESULTS_DIR/${TS}_capacity_${prof}_summary.json"
  [[ -f "$f" ]] || continue
  python3 - "$prof" "$f" <<'PY' || true
import json, sys
prof, path = sys.argv[1], sys.argv[2]
d = json.load(open(path)).get("metrics", {})
succ = d.get("ecg_success", {}).get("value")
p95  = d.get("ecg_duration", {}).get("p(95)")
print(f"  {prof:9s} success={succ}  e2e_p95_ms={p95}")
PY
done
log "Capacity ≈ workers / GPT_wall_time. Watch where success drops and p95 climbs — that's the ceiling."
