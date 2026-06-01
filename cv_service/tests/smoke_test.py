import argparse
import torch

from ecg_service.model import build_model
from ecg_service.inference import ECGPredictor

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--checkpoint", default=None)
    parser.add_argument("--style-ref", default=None)
    args = parser.parse_args()

    device = "cuda" if torch.cuda.is_available() else "cpu"
    print("[SMOKE] device:", device)

    model = build_model(device=device)
    model.eval()

    x_full = torch.randn(1, 3, 384, 384, device=device)
    x_tiles = torch.randn(1, 4, 3, 224, 224, device=device)
    x_leads = torch.randn(1, 13, 3, 160, 160, device=device)
    lead_boxes = torch.randn(1, 13, 4, device=device)
    lead_id_idx = torch.arange(13, device=device).unsqueeze(0)
    lead_real_mask = torch.ones(1, 13, device=device)
    layout_idx = torch.tensor([0], dtype=torch.long, device=device)
    has_rhythm_strip = torch.tensor([1.0], device=device)

    with torch.no_grad():
        out = model(
            x_full=x_full,
            x_tiles=x_tiles,
            x_leads=x_leads,
            lead_boxes=lead_boxes,
            lead_id_idx=lead_id_idx,
            lead_real_mask=lead_real_mask,
            layout_idx=layout_idx,
            has_rhythm_strip=has_rhythm_strip
        )

    assert "rhythm_logits" in out
    assert out["rhythm_logits"].shape[0] == 1
    print("[SMOKE] dummy forward: OK")

    predictor = ECGPredictor(
        checkpoint_path=args.checkpoint,
        style_ref_path=args.style_ref,
        default_preprocess="synthmatch"
    )
    print("[SMOKE] predictor init: OK")
    if predictor.checkpoint_info is not None:
        print("[SMOKE] loaded keys:", len(predictor.checkpoint_info["loaded_keys"]))
        print("[SMOKE] skipped keys:", len(predictor.checkpoint_info["skipped_keys"]))
    else:
        print("[SMOKE] checkpoint not provided or not found")

    print("[SMOKE] ALL TESTS PASSED")

if __name__ == "__main__":
    main()
