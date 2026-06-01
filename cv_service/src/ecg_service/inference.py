import io
import json
import numpy as np
from PIL import Image, ImageOps

import torch
import torch.nn.functional as F
from torchvision import transforms

from .constants import (
    IMAGENET_MEAN, IMAGENET_STD,
    PAGE2X2_IMG_SIZE, PAGE2X2_TILE_SIZE, PAGE2X2_OVERLAP_FRAC, LEAD_IMG_SIZE,
    ACTIVE_RHYTHM_CLASS_NAMES, RHYTHM_FRIENDLY_RU,
    BINARY_TARGETS, BINARY_FRIENDLY_RU,
    LAYOUT_FAMILY_TO_IDX, LEAD_NAMES, LEAD_TO_IDX, DEFAULT_STYLE_REF
)
from .layouts import extract_2x2_tiles, extract_lead_crops, layout_label_to_pair
from .preprocessing import page_preproc_v2_border0
from .model import build_model, load_checkpoint_flex

_full_tfms_single = transforms.Compose([
    transforms.Resize((PAGE2X2_IMG_SIZE, PAGE2X2_IMG_SIZE)),
    transforms.ToTensor(),
    transforms.Normalize(IMAGENET_MEAN, IMAGENET_STD),
])

_tile_tfms_single = transforms.Compose([
    transforms.Resize((PAGE2X2_TILE_SIZE, PAGE2X2_TILE_SIZE)),
    transforms.ToTensor(),
    transforms.Normalize(IMAGENET_MEAN, IMAGENET_STD),
])

_lead_tfms_single = transforms.Compose([
    transforms.Resize((LEAD_IMG_SIZE, LEAD_IMG_SIZE)),
    transforms.ToTensor(),
    transforms.Normalize(IMAGENET_MEAN, IMAGENET_STD),
])

def load_style_ref(style_ref_path=None):
    if style_ref_path:
        try:
            with open(style_ref_path, "r", encoding="utf-8") as f:
                return json.load(f)
        except (OSError, json.JSONDecodeError):
            pass
    return DEFAULT_STYLE_REF.copy()

def refine_rhythm_probs(outputs):
    probs = F.softmax(outputs["rhythm_logits"], dim=1)

    name_to_idx = {name: i for i, name in enumerate(ACTIVE_RHYTHM_CLASS_NAMES)}
    atrial_present = [name for name in ["AFIB", "AFLT", "SVTAC"] if name in name_to_idx]
    atrial_aux_to_idx = {"AFIB": 0, "AFLT": 1, "SVTAC": 2}

    if len(atrial_present) >= 2:
        atrial_indices = [name_to_idx[name] for name in atrial_present]
        atrial_aux_probs = F.softmax(outputs["atrial_aux_logits"], dim=1)
        aux_cols = [atrial_aux_to_idx[name] for name in atrial_present]
        atrial_aux_subset = atrial_aux_probs[:, aux_cols]

        atrial_mass = probs[:, atrial_indices].sum(dim=1, keepdim=True)
        refined_atrial = 0.78 * probs[:, atrial_indices] + 0.22 * (atrial_mass * atrial_aux_subset)

        for j, idx in enumerate(atrial_indices):
            probs[:, idx] = refined_atrial[:, j]

    if "PACE" in name_to_idx:
        pace_idx = name_to_idx["PACE"]
        pace_prob = torch.sigmoid(outputs["pace_aux_logits"])
        probs[:, pace_idx] = 0.94 * probs[:, pace_idx] + 0.06 * pace_prob

    probs = probs / probs.sum(dim=1, keepdim=True).clamp_min(1e-8)
    return probs

def build_single_hybrid_batch(pil_img, layout_name, has_rhythm_strip, device):
    image = pil_img.convert("RGB")
    layout_idx = LAYOUT_FAMILY_TO_IDX.get(layout_name, LAYOUT_FAMILY_TO_IDX["unknown"])

    x_full = _full_tfms_single(image).unsqueeze(0).to(device)

    tiles = extract_2x2_tiles(image, overlap_frac=PAGE2X2_OVERLAP_FRAC)
    x_tiles = torch.stack(
        [_tile_tfms_single(tiles[k]) for k in ["top_left", "top_right", "bottom_left", "bottom_right"]],
        dim=0
    ).unsqueeze(0).to(device)

    lead_pack = extract_lead_crops(image=image, layout_name=layout_name, has_rhythm_strip=has_rhythm_strip)

    lead_imgs, lead_boxes, lead_id_idx, lead_real_mask = [], [], [], []
    for lead_name in LEAD_NAMES:
        lead_imgs.append(_lead_tfms_single(lead_pack[lead_name]["crop"]))
        lead_boxes.append(lead_pack[lead_name]["box_norm"])
        lead_id_idx.append(LEAD_TO_IDX[lead_name])
        lead_real_mask.append(float(lead_pack[lead_name]["is_real"]))

    x_leads = torch.stack(lead_imgs, dim=0).unsqueeze(0).to(device)
    lead_boxes = torch.tensor(np.stack(lead_boxes), dtype=torch.float32).unsqueeze(0).to(device)
    lead_id_idx = torch.tensor(lead_id_idx, dtype=torch.long).unsqueeze(0).to(device)
    lead_real_mask = torch.tensor(lead_real_mask, dtype=torch.float32).unsqueeze(0).to(device)

    batch = {
        "x_full": x_full,
        "x_tiles": x_tiles,
        "x_leads": x_leads,
        "lead_boxes": lead_boxes,
        "lead_id_idx": lead_id_idx,
        "lead_real_mask": lead_real_mask,
        "layout_idx": torch.tensor([layout_idx], dtype=torch.long, device=device),
        "has_rhythm_strip": torch.tensor([float(has_rhythm_strip)], dtype=torch.float32, device=device),
    }
    return batch

class ECGPredictor:
    def __init__(self, checkpoint_path=None, style_ref_path=None, device=None, default_preprocess="synthmatch"):
        self.device = torch.device(device or ("cuda" if torch.cuda.is_available() else "cpu"))
        self.style_ref = load_style_ref(style_ref_path)
        self.model = build_model(device=self.device)
        self.checkpoint_path = checkpoint_path
        self.checkpoint_info = None
        self.default_preprocess = default_preprocess

        if checkpoint_path:
            try:
                self.checkpoint_info = load_checkpoint_flex(self.model, checkpoint_path, map_location=self.device)
            except Exception as e:
                print(f"[WARN] checkpoint load failed: {e}")

        self.model.eval()

    def preprocess_image(self, pil_img, preprocess_name=None):
        preprocess_name = (preprocess_name or self.default_preprocess).strip().lower()
        base = ImageOps.exif_transpose(pil_img).convert("RGB")

        if preprocess_name == "raw":
            return base
        if preprocess_name == "light":
            return ImageOps.autocontrast(base)
        if preprocess_name == "synthmatch":
            return page_preproc_v2_border0(base, style_ref=self.style_ref)

        raise ValueError(f"Unknown preprocess_name: {preprocess_name}")

    @torch.no_grad()
    def predict_pil(self, pil_img, layout_label="3x4_rhythm", preprocess_name=None):
        preprocess_name = preprocess_name or self.default_preprocess
        layout_name, has_strip = layout_label_to_pair(layout_label)
        proc_img = self.preprocess_image(pil_img, preprocess_name=preprocess_name)
        batch = build_single_hybrid_batch(
            proc_img,
            layout_name=layout_name,
            has_rhythm_strip=has_strip,
            device=self.device,
        )

        outputs = self.model(
            x_full=batch["x_full"],
            x_tiles=batch["x_tiles"],
            x_leads=batch["x_leads"],
            lead_boxes=batch["lead_boxes"],
            lead_id_idx=batch["lead_id_idx"],
            lead_real_mask=batch["lead_real_mask"],
            layout_idx=batch["layout_idx"],
            has_rhythm_strip=batch["has_rhythm_strip"]
        )

        rhythm_probs = refine_rhythm_probs(outputs)[0].detach().cpu().numpy().astype(np.float32)
        rhythm_probs = rhythm_probs / max(float(rhythm_probs.sum()), 1e-12)

        order = np.argsort(rhythm_probs)[::-1]
        pred_idx = int(order[0])
        pred_code = ACTIVE_RHYTHM_CLASS_NAMES[pred_idx]

        binary_probs = torch.sigmoid(outputs["binary_logits"])[0].detach().cpu().numpy().astype(np.float32)
        binary_rows = []
        for code, p in zip(BINARY_TARGETS, binary_probs):
            if float(p) >= 0.55:
                binary_rows.append({
                    "code": code,
                    "label_ru": BINARY_FRIENDLY_RU.get(code, code),
                    "prob": float(p)
                })
        binary_rows = sorted(binary_rows, key=lambda x: x["prob"], reverse=True)

        return {
            "pred_idx": pred_idx,
            "pred_code": pred_code,
            "pred_label_ru": RHYTHM_FRIENDLY_RU.get(pred_code, pred_code),
            "layout_label": layout_label,
            "preprocess_name": preprocess_name,
            "top3": [
                {
                    "idx": int(i),
                    "code": ACTIVE_RHYTHM_CLASS_NAMES[int(i)],
                    "label_ru": RHYTHM_FRIENDLY_RU.get(
                        ACTIVE_RHYTHM_CLASS_NAMES[int(i)],
                        ACTIVE_RHYTHM_CLASS_NAMES[int(i)],
                    ),
                    "prob": float(rhythm_probs[int(i)])
                }
                for i in order[:3]
            ],
            "binary_flags": binary_rows,
            "raw_probs": rhythm_probs.tolist()
        }

    def predict_bytes(self, image_bytes, layout_label="3x4_rhythm", preprocess_name=None):
        img = Image.open(io.BytesIO(image_bytes)).convert("RGB")
        return self.predict_pil(img, layout_label=layout_label, preprocess_name=preprocess_name)

    def predict_path(self, image_path, layout_label="3x4_rhythm", preprocess_name=None):
        img = Image.open(image_path).convert("RGB")
        return self.predict_pil(img, layout_label=layout_label, preprocess_name=preprocess_name)
