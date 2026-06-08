import os
from io import BytesIO
from fastapi import FastAPI, UploadFile, File, Form, HTTPException
from PIL import Image

from .inference import ECGPredictor

APP_TITLE = "ECG Rhythm Service"
app = FastAPI(title=APP_TITLE)

_PREDICTOR = None

def get_predictor():
    global _PREDICTOR
    if _PREDICTOR is None:
        root = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
        ckpt = os.getenv("ECG_CHECKPOINT_PATH", os.path.join(root, "models", "stage_oldv9_residual_best.pt"))
        style_ref = os.getenv("ECG_STYLE_REF_PATH", os.path.join(root, "config", "synth_style_ref.json"))
        default_preprocess = os.getenv("ECG_DEFAULT_PREPROCESS", "synthmatch")
        binary_thresholds = os.getenv("ECG_BINARY_THRESHOLDS_PATH", os.path.join(root, "config", "binary_thresholds.json"))
        binary_threshold_mode = os.getenv("ECG_BINARY_THRESHOLD_MODE", "thresholds_f1")
        super_prior_alpha = float(os.getenv("ECG_SUPER_PRIOR_ALPHA", "0.3"))
        clinical_guards = os.getenv("ECG_CLINICAL_GUARDS", "1").strip().lower() not in ("0", "false", "no", "")
        _PREDICTOR = ECGPredictor(
            checkpoint_path=ckpt,
            style_ref_path=style_ref,
            default_preprocess=default_preprocess,
            binary_thresholds_path=binary_thresholds,
            binary_threshold_mode=binary_threshold_mode,
            super_prior_alpha=super_prior_alpha,
            clinical_guards=clinical_guards
        )
    return _PREDICTOR

@app.get("/health")
def health():
    predictor = get_predictor()
    return {
        "status": "ok",
        "device": str(predictor.device),
        "checkpoint_path": predictor.checkpoint_path,
        "checkpoint_loaded": bool(predictor.checkpoint_path and os.path.exists(predictor.checkpoint_path)),
        "default_preprocess": predictor.default_preprocess,
        "binary_threshold_mode": predictor.binary_threshold_mode,
        "binary_thresholds": predictor.binary_thresholds,
        "super_prior_alpha": predictor.super_prior_alpha,
        "clinical_guards": predictor.clinical_guards,
    }

@app.post("/predict")
async def predict(
    file: UploadFile = File(...),
    layout_label: str = Form("3x4_rhythm"),
    preprocess_name: str = Form("synthmatch"),
):
    predictor = get_predictor()

    try:
        data = await file.read()
        pil_img = Image.open(BytesIO(data)).convert("RGB")
    except Exception as e:
        raise HTTPException(status_code=400, detail=f"Invalid image file: {e}")

    try:
        return predictor.predict_pil(
            pil_img=pil_img,
            layout_label=layout_label,
            preprocess_name=preprocess_name,
        )
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))
    except RuntimeError as e:
        raise HTTPException(status_code=500, detail=str(e))
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Inference failed: {e}")
