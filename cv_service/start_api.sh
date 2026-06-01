#!/usr/bin/env bash
set -e
export PYTHONPATH="$(pwd)/src:${PYTHONPATH}"
export ECG_CHECKPOINT_PATH="${ECG_CHECKPOINT_PATH:-$(pwd)/models/stage_oldv9_residual_best.pt}"
export ECG_STYLE_REF_PATH="${ECG_STYLE_REF_PATH:-$(pwd)/config/synth_style_ref.json}"
export ECG_DEFAULT_PREPROCESS="${ECG_DEFAULT_PREPROCESS:-synthmatch}"
uvicorn ecg_service.api_fastapi:app --host 0.0.0.0 --port 8000
