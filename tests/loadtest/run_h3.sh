#!/usr/bin/env bash
# H3 load-test runner.
#
# Usage:
#   ./tests/loadtest/run_h3.sh [--minimal]
#
# --minimal  runs fill + negative only (25 + 25 requests).
#            Use this to validate the contradiction guard cheaply.
#            If false-hit rate < 0.10, run without the flag for the full suite.
#
# Without --minimal: runs fill + measure + negative (25 + 50 + 25 requests).
#
# Prerequisites: docker compose up (back-api + postgres must be running).

set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
RESULTS_DIR="$SCRIPT_DIR/results"
TS="$(date +%Y%m%d_%H%M%S)"
MINIMAL=false

for arg in "$@"; do
  [[ "$arg" == "--minimal" ]] && MINIMAL=true
done

mkdir -p "$RESULTS_DIR"

log() { echo "[run_h3] $*"; }

# ── step 1: clear cache ──────────────────────────────────────────────────────
log "Truncating kb_cache..."
docker exec smartheart_postgres \
  psql -U user -d smartheart -c "TRUNCATE kb_cache RESTART IDENTITY CASCADE;" \
  > /dev/null
log "kb_cache cleared."

# ── k6 helper ────────────────────────────────────────────────────────────────
run_k6() {
  local run_name="$1"
  log "Starting k6 run: $run_name"
  docker run --rm -i --network host \
    -v "$SCRIPT_DIR":/tests \
    grafana/k6:latest run /tests/h3_cache_scenario.js \
    -e BASE_URL="$BASE_URL" \
    -e RUN_NAME="$run_name" \
    2>&1 | tee "$RESULTS_DIR/${TS}_${run_name}_k6.log"

  # Extract handleSummary JSON from log (last occurrence of the summary block).
  grep -A 999 '=== H3 Summary ===' "$RESULTS_DIR/${TS}_${run_name}_k6.log" \
    | tail -n +2 \
    | python3 -c "
import sys, json
buf = sys.stdin.read().strip()
# handleSummary prints JSON; grab everything between first { and last }
start = buf.find('{')
end   = buf.rfind('}')
if start != -1 and end != -1:
    print(buf[start:end+1])
" > "$RESULTS_DIR/${TS}_${run_name}_summary.json" || true

  log "Done: $run_name → ${TS}_${run_name}_k6.log"
}

# ── step 2: fill ─────────────────────────────────────────────────────────────
run_k6 fill

# ── step 3: measure (skipped in minimal mode) ────────────────────────────────
if [[ "$MINIMAL" == "false" ]]; then
  run_k6 measure
fi

# ── step 4: negative ─────────────────────────────────────────────────────────
run_k6 negative

# ── step 5: quick hit-rate summary ───────────────────────────────────────────
log "── Results ──────────────────────────────────────────────────────"
for run in fill measure negative; do
  f="$RESULTS_DIR/${TS}_${run}_summary.json"
  [[ -f "$f" ]] || continue
  rate=$(python3 -c "import json; d=json.load(open('$f')); print(d.get('cache_hit_rate','?'))" 2>/dev/null || echo "?")
  log "  $run: hit_rate = $rate"
done

if [[ "$MINIMAL" == "true" ]]; then
  log ""
  log "Minimal run complete."
  log "Check negative hit rate above."
  log "Target: < 0.10 → if OK, rerun without --minimal for the full suite."
fi
