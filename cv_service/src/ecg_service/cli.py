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
    parser.add_argument("--explain", action="store_true")
    args = parser.parse_args()

    predictor = ECGPredictor(
        checkpoint_path=args.checkpoint,
        style_ref_path=args.style_ref,
        default_preprocess=args.preprocess
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
