import numpy as np
from PIL import Image

from .constants import LEAD_NAMES

STANDARD_3X4_MAP = [
    ["I",   "aVR", "V1", "V4"],
    ["II",  "aVL", "V2", "V5"],
    ["III", "aVF", "V3", "V6"],
]

STANDARD_6X2_MAP = [
    ["I",   "V1"],
    ["II",  "V2"],
    ["III", "V3"],
    ["aVR", "V4"],
    ["aVL", "V5"],
    ["aVF", "V6"],
]

STANDARD_12X1_MAP = [
    ["I"], ["II"], ["III"], ["aVR"], ["aVL"], ["aVF"],
    ["V1"], ["V2"], ["V3"], ["V4"], ["V5"], ["V6"],
]

def canonical_layout_name(layout_name):
    s = str(layout_name).lower().strip()
    if "12x1" in s:
        return "12x1"
    if "6x2" in s:
        return "6x2"
    if "3x4" in s and "rhythm" in s:
        return "3x4_rhythm"
    if "3x4" in s:
        return "3x4"
    return "unknown"

def clamp_box(box, w, h):
    x1, y1, x2, y2 = box
    x1 = max(0, min(int(round(x1)), w - 1))
    y1 = max(0, min(int(round(y1)), h - 1))
    x2 = max(x1 + 1, min(int(round(x2)), w))
    y2 = max(y1 + 1, min(int(round(y2)), h))
    return (x1, y1, x2, y2)

def norm_box(box, w, h):
    x1, y1, x2, y2 = box
    return np.array([x1 / w, y1 / h, x2 / w, y2 / h], dtype=np.float32)

def shrink_box(box, w, h, shrink_frac=0.04):
    x1, y1, x2, y2 = box
    bw = x2 - x1
    bh = y2 - y1
    dx = bw * shrink_frac
    dy = bh * shrink_frac
    return clamp_box((x1 + dx, y1 + dy, x2 - dx, y2 - dy), w, h)

def infer_layout_spec(layout_name):
    layout = canonical_layout_name(layout_name)

    if layout == "3x4_rhythm":
        return {
            "layout":"3x4_rhythm","rows":3,"cols":4,"lead_map":STANDARD_3X4_MAP,
            "grid_x0":0.02,"grid_x1":0.98,"grid_y0":0.05,"grid_y1":0.72,
            "has_real_strip":True,"strip_box":(0.02,0.74,0.98,0.98)
        }
    if layout == "3x4":
        return {
            "layout":"3x4","rows":3,"cols":4,"lead_map":STANDARD_3X4_MAP,
            "grid_x0":0.02,"grid_x1":0.98,"grid_y0":0.05,"grid_y1":0.94,
            "has_real_strip":False,"strip_box":(0.03,0.18,0.97,0.90)
        }
    if layout == "6x2":
        return {
            "layout":"6x2","rows":6,"cols":2,"lead_map":STANDARD_6X2_MAP,
            "grid_x0":0.02,"grid_x1":0.98,"grid_y0":0.04,"grid_y1":0.96,
            "has_real_strip":False,"strip_box":(0.03,0.18,0.97,0.92)
        }
    if layout == "12x1":
        return {
            "layout":"12x1","rows":12,"cols":1,"lead_map":STANDARD_12X1_MAP,
            "grid_x0":0.02,"grid_x1":0.98,"grid_y0":0.03,"grid_y1":0.97,
            "has_real_strip":False,"strip_box":(0.03,0.08,0.97,0.30)
        }

    return {
        "layout":"unknown","rows":3,"cols":4,"lead_map":STANDARD_3X4_MAP,
        "grid_x0":0.02,"grid_x1":0.98,"grid_y0":0.05,"grid_y1":0.94,
        "has_real_strip":False,"strip_box":(0.03,0.18,0.97,0.90)
    }

def extract_2x2_tiles(image, overlap_frac=0.04):
    w, h = image.size
    mx = int(round(w * overlap_frac))
    my = int(round(h * overlap_frac))
    cx, cy = w // 2, h // 2

    return {
        "top_left":     image.crop(clamp_box((0, 0, cx + mx, cy + my), w, h)),
        "top_right":    image.crop(clamp_box((cx - mx, 0, w, cy + my), w, h)),
        "bottom_left":  image.crop(clamp_box((0, cy - my, cx + mx, h), w, h)),
        "bottom_right": image.crop(clamp_box((cx - mx, cy - my, w, h), w, h)),
    }

def extract_lead_crops(image, layout_name, has_rhythm_strip):
    w, h = image.size
    spec = infer_layout_spec(layout_name)

    x0 = int(w * spec["grid_x0"])
    x1 = int(w * spec["grid_x1"])
    y0 = int(h * spec["grid_y0"])
    y1 = int(h * spec["grid_y1"])

    grid_w = x1 - x0
    grid_h = y1 - y0
    cell_w = grid_w / spec["cols"]
    cell_h = grid_h / spec["rows"]

    out = {}
    for r in range(spec["rows"]):
        for c in range(spec["cols"]):
            lead_name = spec["lead_map"][r][c]
            bx1 = x0 + c * cell_w
            by1 = y0 + r * cell_h
            bx2 = x0 + (c + 1) * cell_w
            by2 = y0 + (r + 1) * cell_h
            box = shrink_box((bx1, by1, bx2, by2), w, h, shrink_frac=0.05)
            out[lead_name] = {
                "crop": image.crop(box),
                "box_norm": norm_box(box, w, h),
                "is_real": 1.0
            }

    sb = spec["strip_box"]
    strip_box = clamp_box((w * sb[0], h * sb[1], w * sb[2], h * sb[3]), w, h)
    out["RHYTHM_STRIP"] = {
        "crop": image.crop(strip_box),
        "box_norm": norm_box(strip_box, w, h),
        "is_real": float(int(has_rhythm_strip) == 1 and spec["has_real_strip"])
    }

    blank = Image.new("RGB", (160, 160), (255, 255, 255))
    for lead_name in LEAD_NAMES:
        if lead_name not in out:
            out[lead_name] = {
                "crop": blank.copy(),
                "box_norm": np.array([0.0, 0.0, 1.0, 1.0], dtype=np.float32),
                "is_real": 0.0
            }
    return out

def layout_label_to_pair(layout_label):
    s = str(layout_label).strip().lower()
    if s == "3x4_rhythm":
        return ("3x4_rhythm", 1)
    if s == "3x4":
        return ("3x4", 0)
    if s == "6x2_rhythm":
        return ("6x2", 1)
    if s == "6x2":
        return ("6x2", 0)
    if s == "12x1":
        return ("12x1", 0)
    raise ValueError(f"Unknown layout_label: {layout_label}")
