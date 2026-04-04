#!/usr/bin/env bash
# run_h2_test.sh — orchestrates H2 hypothesis load test.
#
# Prerequisites:
#   - Docker services running (docker-compose up -d)
#   - local k6/psql are optional; if absent, dockerized fallbacks are used
#   - DATABASE_URL set (or defaults to local dev)
#
# Usage:
#   ./tests/loadtest/run_h2_test.sh [base|working|elevated] [async|sync]

set -euo pipefail
cd "$(dirname "$0")"

PROFILE="${1:-base}"
VARIANT="${2:-${H2_VARIANT:-async}}"
DATABASE_URL="${DATABASE_URL:-postgres://user:password@localhost:5432/smartheart?sslmode=disable}"
BASE_URL="${BASE_URL:-http://localhost:8080}"
RESULTS_DIR="./results"
RESULTS_FILE="$RESULTS_DIR/h2_results.csv"

mkdir -p "$RESULTS_DIR"

detect_compose() {
    if command -v docker-compose >/dev/null 2>&1; then
        echo "docker-compose"
        return
    fi
    if docker compose version >/dev/null 2>&1; then
        echo "docker compose"
        return
    fi
    echo ""
}

COMPOSE_CMD="${COMPOSE_CMD:-$(detect_compose)}"

run_k6() {
    if command -v k6 >/dev/null 2>&1; then
        K6_PROFILE="$PROFILE" k6 run \
            -e BASE_URL="$BASE_URL" \
            -e K6_PROFILE="$PROFILE" \
            --summary-trend-stats="avg,min,med,max,p(90),p(95)" \
            combined_scenario.js
        return
    fi

    if [ -z "$COMPOSE_CMD" ]; then
        echo "ERROR: neither local k6 nor docker-compose/docker compose is available."
        exit 1
    fi

    docker run --rm --network host \
        -v "$PWD:/scripts" \
        -w /scripts \
        grafana/k6 run \
        -e BASE_URL="$BASE_URL" \
        -e K6_PROFILE="$PROFILE" \
        --summary-trend-stats="avg,min,med,max,p(90),p(95)" \
        combined_scenario.js
}

run_psql_file() {
    local quiet="$1"
    local since="$2"
    local sql_file="$3"

    if command -v psql >/dev/null 2>&1; then
        if [ "$quiet" = "true" ]; then
            psql "$DATABASE_URL" -v since="'$since'" -t -A -F',' -f "$sql_file"
        else
            psql "$DATABASE_URL" -v since="'$since'" -f "$sql_file"
        fi
        return
    fi

    if [ -z "$COMPOSE_CMD" ]; then
        echo "ERROR: neither local psql nor docker-compose/docker compose is available."
        exit 1
    fi

    if [ "$quiet" = "true" ]; then
        $COMPOSE_CMD exec -T postgres psql -U user -d smartheart -v since="'$since'" -t -A -F',' -f - < "$sql_file"
    else
        $COMPOSE_CMD exec -T postgres psql -U user -d smartheart -v since="'$since'" -f - < "$sql_file"
    fi
}

echo "=== H2 Hypothesis Test ==="
echo "Profile: $PROFILE | Base URL: $BASE_URL"
echo "Variant: $VARIANT"
echo ""

run_variant() {
    local variant="$1"
    echo "--- Variant: $variant ---"
    local since
    since=$(date -u +"%Y-%m-%d %H:%M:%S+00")

    echo "Running combined k6 scenario (profile=$PROFILE)..."
    run_k6 || true

    echo ""
    echo "Calculating metrics (since $since)..."
    run_psql_file "false" "$since" "metrics.sql"

    # Append to CSV.
    run_psql_file "true" "$since" "metrics.sql" | \
        while IFS=',' read -r scenario n_all n_success n_under_30s success_rate share_under_30s p95_sec; do
            echo "$variant,$scenario,$n_all,$n_success,$n_under_30s,$success_rate,$share_under_30s,$p95_sec,$(date -u +%Y-%m-%dT%H:%M:%SZ)"
        done >> "$RESULTS_FILE"

    echo ""
}

# --- Header ---
if [ ! -f "$RESULTS_FILE" ]; then
    echo "variant,scenario,n_all,n_success,n_under_30s,success_rate,share_under_30s,p95_sec,run_at" > "$RESULTS_FILE"
fi

# --- Single variant per invocation ---
run_variant "$VARIANT"

echo ""
echo "=== Results saved to $RESULTS_FILE ==="
echo ""
column -t -s',' "$RESULTS_FILE"
echo ""
if [ "$VARIANT" = "async" ]; then
    echo "To run sync baseline, restart the app with ECG_SYNC_MODE=true and re-run:"
    echo "  ./tests/loadtest/run_h2_test.sh $PROFILE sync"
else
    echo "Sync baseline run completed."
fi
