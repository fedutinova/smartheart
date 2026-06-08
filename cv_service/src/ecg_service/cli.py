import json
import argparse
from PIL import Image

from .inference import ECGPredictor
from .llm_service import BothubECGExplainer, build_combined_explanation

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--image", required=True)
    parser.add_argument("--layout", default="3x4_rhythm")
    parser.add_argument("--preprocess", default="synthmatch", choices=["raw", "light", "synthmatch"])
    parser.add_argument("--checkpoint", default=None)
    parser.add_argument("--style-ref", default=None)
    parser.add_argument("--binary-thresholds", default=None,
                        help="Path to binary_thresholds.json (per-class calibrated thresholds)")
    parser.add_argument("--threshold-mode", default="thresholds_f1",
                        choices=["thresholds_f1", "thresholds_clinical"])
    parser.add_argument("--super-prior-alpha", type=float, default=0.3,
                        help="Strength of the super-class hierarchical prior over rhythm classes; 0 disables")
    parser.add_argument("--clinical-guards", action=argparse.BooleanOptionalAction, default=True,
                        help="Suppress clinically impossible binary findings (e.g. BBB in a ventricular rhythm)")
    parser.add_argument("--localized-binary-head", action="store_true",
                        help="Feed inferior/anteroseptal lead pools to the findings head; needs a checkpoint retrained with this flag")
    parser.add_argument("--explain", action="store_true")
    args = parser.parse_args()

    predictor = ECGPredictor(
        checkpoint_path=args.checkpoint,
        style_ref_path=args.style_ref,
        default_preprocess=args.preprocess,
        binary_thresholds_path=args.binary_thresholds,
        binary_threshold_mode=args.threshold_mode,
        super_prior_alpha=args.super_prior_alpha,
        clinical_guards=args.clinical_guards,
        localized_binary_head=args.localized_binary_head
    )

    if args.explain:
        img = Image.open(args.image).convert("RGB")
        explainer = BothubECGExplainer()
        result = build_combined_explanation(
            predictor=predictor,
            pil_img=img,
            layout_label=args.layout,
            preprocess_name=args.preprocess,
            file_name=args.image,
            explainer=explainer
        )
    else:
        result = predictor.predict_path(
            image_path=args.image,
            layout_label=args.layout,
            preprocess_name=args.preprocess
        )

    print(json.dumps(result, ensure_ascii=False, indent=2))

if __name__ == "__main__":
    main()
