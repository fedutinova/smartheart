import cv2
import math
import numpy as np
from PIL import Image, ImageOps

from .constants import DEFAULT_STYLE_REF

PHOTOMETRIC_BLEND_ALPHA = 0.36
TRACE_PRESERVE_ENHANCE = True
TRACE_ENHANCE_STRENGTH = 0.34
TRACE_ENHANCE_MAX_DARKEN_L = 18

def safe_photometric_normalize_bgr_v2(img_bgr):
    img = img_bgr.copy()
    h, w = img.shape[:2]
    lab = cv2.cvtColor(img, cv2.COLOR_BGR2LAB)
    L, A, B = cv2.split(lab)
    sigma = max(25, int(min(h, w) * 0.055))
    bg = cv2.GaussianBlur(L, (0, 0), sigmaX=sigma, sigmaY=sigma)
    Lf = L.astype(np.float32)
    bg_f = np.maximum(bg.astype(np.float32), 1.0)
    corrected = Lf / bg_f * np.percentile(bg_f, 70)
    corrected = np.clip(corrected, 0, 255).astype(np.uint8)
    clahe = cv2.createCLAHE(clipLimit=1.18, tileGridSize=(8,8))
    corrected = clahe.apply(corrected)
    alpha = float(np.clip(PHOTOMETRIC_BLEND_ALPHA, 0.0, 1.0))
    L_base = cv2.addWeighted(corrected, alpha, L, 1.0 - alpha, 0)

    if TRACE_PRESERVE_ENHANCE:
        strength = float(np.clip(TRACE_ENHANCE_STRENGTH, 0.0, 1.0))
        max_darken = float(np.clip(TRACE_ENHANCE_MAX_DARKEN_L, 0.0, 35.0))
        Lb = L_base.astype(np.float32)
        local_sigma = max(7, int(min(h, w) * 0.012))
        local_bg = cv2.GaussianBlur(L_base, (0,0), sigmaX=local_sigma, sigmaY=local_sigma).astype(np.float32)
        dark_resid = np.maximum(0.0, local_bg - Lb)

        k = max(5, int(min(h,w) * 0.006))
        if k % 2 == 0:
            k += 1

        kernel = cv2.getStructuringElement(cv2.MORPH_ELLIPSE, (k,k))
        blackhat = cv2.morphologyEx(L_base, cv2.MORPH_BLACKHAT, kernel).astype(np.float32)

        gx = cv2.Sobel(L_base, cv2.CV_32F, 1, 0, ksize=3)
        gy = cv2.Sobel(L_base, cv2.CV_32F, 0, 1, ksize=3)
        grad = np.sqrt(gx * gx + gy * gy)

        def robust01(a, p_lo=35, p_hi=98):
            lo = float(np.percentile(a, p_lo))
            hi = float(np.percentile(a, p_hi))
            if hi <= lo + 1e-6:
                return np.zeros_like(a, dtype=np.float32)
            out = (a - lo) / (hi - lo)
            return np.clip(out, 0.0, 1.0).astype(np.float32)

        dark_n = robust01(dark_resid, 45, 99)
        bh_n = robust01(blackhat, 50, 99)
        grad_n = robust01(grad, 50, 98.5)

        line_score = 0.52 * dark_n + 0.36 * bh_n + 0.12 * grad_n
        line_score = cv2.GaussianBlur(line_score, (0,0), sigmaX=0.65, sigmaY=0.65)
        soft = np.clip((line_score - 0.20) / 0.75, 0.0, 1.0)
        darken = max_darken * strength * soft
        L_enh = np.clip(Lb - darken, 0, 255).astype(np.uint8)
        L_out = cv2.addWeighted(L_enh, 0.72, L_base, 0.28, 0)
    else:
        L_out = L_base

    lab_out = cv2.merge([L_out, A, B])
    out_bgr = cv2.cvtColor(lab_out, cv2.COLOR_LAB2BGR)
    return out_bgr

def detect_page_bbox_safe_v2(img_bgr, min_area_ratio=0.35, min_w_ratio=0.72, min_h_ratio=0.72, max_crop_ratio=0.16):
    gray = cv2.cvtColor(img_bgr, cv2.COLOR_BGR2GRAY)
    h, w = gray.shape[:2]
    blur = cv2.GaussianBlur(gray, (0,0), sigmaX=5, sigmaY=5)
    thr = max(150, int(np.percentile(blur, 58)))
    mask = (blur >= thr).astype(np.uint8) * 255

    k1 = max(9, int(min(h,w) * 0.025))
    if k1 % 2 == 0:
        k1 += 1
    kernel1 = cv2.getStructuringElement(cv2.MORPH_ELLIPSE, (k1, k1))
    mask = cv2.morphologyEx(mask, cv2.MORPH_CLOSE, kernel1, iterations=2)

    k2 = max(5, int(min(h,w) * 0.010))
    if k2 % 2 == 0:
        k2 += 1
    kernel2 = cv2.getStructuringElement(cv2.MORPH_ELLIPSE, (k2, k2))
    mask = cv2.morphologyEx(mask, cv2.MORPH_OPEN, kernel2, iterations=1)

    cnts, _ = cv2.findContours(mask, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_SIMPLE)
    if not cnts:
        return (0, 0, w, h)

    cnt = max(cnts, key=cv2.contourArea)
    area = cv2.contourArea(cnt)
    if area < min_area_ratio * (h * w):
        return (0, 0, w, h)

    x, y, bw, bh = cv2.boundingRect(cnt)
    if bw < min_w_ratio * w or bh < min_h_ratio * h:
        return (0, 0, w, h)

    left_crop, right_crop = x, w - (x + bw)
    top_crop, bottom_crop = y, h - (y + bh)
    if (
        left_crop > max_crop_ratio * w or right_crop > max_crop_ratio * w or
        top_crop > max_crop_ratio * h or bottom_crop > max_crop_ratio * h
    ):
        return (0, 0, w, h)

    pad_x = int(0.03 * bw)
    pad_y = int(0.03 * bh)
    x0 = max(0, x - pad_x)
    y0 = max(0, y - pad_y)
    x1 = min(w, x + bw + pad_x)
    y1 = min(h, y + bh + pad_y)

    if (x1 - x0) < 0.78 * w or (y1 - y0) < 0.78 * h:
        return (0, 0, w, h)

    return (x0, y0, x1, y1)

def crop_bbox_v2(img_bgr, bbox):
    x0, y0, x1, y1 = bbox
    return img_bgr[y0:y1, x0:x1].copy()

def estimate_small_deskew_angle_v2(gray):
    h, w = gray.shape[:2]
    edges = cv2.Canny(gray, 60, 160)
    lines = cv2.HoughLinesP(
        edges, rho=1, theta=np.pi / 180,
        threshold=max(60, int(0.08 * min(h, w))),
        minLineLength=max(80, int(0.18 * min(h, w))),
        maxLineGap=max(10, int(0.01 * min(h, w)))
    )
    if lines is None:
        return 0.0

    angles = []
    for ln in lines[:, 0]:
        x1, y1, x2, y2 = ln
        dx = x2 - x1
        dy = y2 - y1
        if dx == 0 and dy == 0:
            continue
        ang = math.degrees(math.atan2(dy, dx))
        while ang <= -90:
            ang += 180
        while ang > 90:
            ang -= 180

        if abs(ang) <= 12:
            angles.append(ang)
        elif abs(abs(ang) - 90) <= 12:
            angles.append(ang - 90 if ang > 0 else ang + 90)

    if len(angles) < 8:
        return 0.0

    angle = float(np.median(angles))
    angle = float(np.clip(angle, -5.0, 5.0))
    if abs(angle) < 0.25:
        return 0.0
    return angle

def rotate_bound_bgr_v2(img_bgr, angle_deg, border_value=(255,255,255)):
    h, w = img_bgr.shape[:2]
    center = (w / 2.0, h / 2.0)
    M = cv2.getRotationMatrix2D(center, angle_deg, 1.0)
    cos = abs(M[0,0])
    sin = abs(M[0,1])
    nW = int((h * sin) + (w * cos))
    nH = int((h * cos) + (w * sin))
    M[0,2] += (nW / 2) - center[0]
    M[1,2] += (nH / 2) - center[1]
    return cv2.warpAffine(
        img_bgr, M, (nW, nH),
        flags=cv2.INTER_LINEAR,
        borderMode=cv2.BORDER_CONSTANT,
        borderValue=border_value
    )

def final_scan_render_v2(img_bgr, whiten_strength=0.22):
    img_bgr = safe_photometric_normalize_bgr_v2(img_bgr)
    gray = cv2.cvtColor(img_bgr, cv2.COLOR_BGR2GRAY).astype(np.float32)
    p1 = np.percentile(gray, 1.0)
    p99 = np.percentile(gray, 99.4)
    if p99 > p1 + 1:
        gray = (gray - p1) * 255.0 / (p99 - p1)
    gray = np.clip(gray, 0, 255)
    bg_mask = np.clip((gray - 168.0) / 72.0, 0.0, 1.0)
    gray = gray * (1.0 - whiten_strength * bg_mask) + 255.0 * (whiten_strength * bg_mask)
    gray_u8 = np.clip(gray, 0, 255).astype(np.uint8)
    base = cv2.GaussianBlur(gray_u8, (0,0), sigmaX=0.35, sigmaY=0.35)
    return cv2.cvtColor(base, cv2.COLOR_GRAY2BGR)

def build_dark_structure_mask_v2(gray_u8):
    g = gray_u8.astype(np.float32)
    local_bg = cv2.GaussianBlur(gray_u8, (0,0), sigmaX=2.2, sigmaY=2.2).astype(np.float32)
    dark_resid = np.maximum(0.0, local_bg - g)

    kernel = cv2.getStructuringElement(cv2.MORPH_ELLIPSE, (5,5))
    blackhat = cv2.morphologyEx(gray_u8, cv2.MORPH_BLACKHAT, kernel).astype(np.float32)

    gx = cv2.Sobel(gray_u8, cv2.CV_32F, 1, 0, ksize=3)
    gy = cv2.Sobel(gray_u8, cv2.CV_32F, 0, 1, ksize=3)
    grad = np.sqrt(gx * gx + gy * gy)

    def robust01(a, p_lo=45, p_hi=99.2):
        lo = float(np.percentile(a, p_lo))
        hi = float(np.percentile(a, p_hi))
        if hi <= lo + 1e-6:
            return np.zeros_like(a, dtype=np.float32)
        out = (a - lo) / (hi - lo)
        return np.clip(out, 0.0, 1.0).astype(np.float32)

    dark_n = robust01(dark_resid, 45, 99.2)
    bh_n = robust01(blackhat, 45, 99.2)
    grad_n = robust01(grad, 55, 99.0)

    line_mask = 0.58 * dark_n + 0.27 * bh_n + 0.15 * grad_n
    line_mask = cv2.GaussianBlur(line_mask, (0,0), sigmaX=0.8, sigmaY=0.8)
    return np.clip(line_mask, 0.0, 1.0)

def match_gray_to_synth_style_v2(gray_u8, ref, white_boost=0.10, protect_strength=0.92):
    ref = ref or DEFAULT_STYLE_REF
    g = gray_u8.astype(np.float32)

    src_lo = float(np.percentile(g, 1))
    src_hi = float(np.percentile(g, 99))
    dst_lo = float(ref["p01"])
    dst_hi = float(ref["p99"])

    if src_hi > src_lo + 1:
        g = (g - src_lo) * (dst_hi - dst_lo) / (src_hi - src_lo) + dst_lo

    cur_mean = float(g.mean())
    cur_std = float(g.std())
    tgt_mean = float(ref["mean"])
    tgt_std = float(ref["std"])

    if cur_std > 1e-6:
        g = (g - cur_mean) * (tgt_std / cur_std) + tgt_mean

    g = np.clip(g, 0, 255)
    g_u8 = g.astype(np.uint8)

    line_mask = build_dark_structure_mask_v2(g_u8)
    bg_mask = np.clip((g - ref["p50"]) / max(ref["p95"] - ref["p50"], 1.0), 0.0, 1.0)
    effective_bg = bg_mask * (1.0 - protect_strength * line_mask)
    g2 = g * (1.0 - white_boost * effective_bg) + 255.0 * (white_boost * effective_bg)
    return np.clip(g2, 0, 255).astype(np.uint8)

def canonical_scan_domain_real_v2(pil_img, add_white_border=0, second_crop_after_rotate=False, whiten_strength=0.22):
    rgb = np.array(ImageOps.exif_transpose(pil_img).convert("RGB"))
    img_bgr = cv2.cvtColor(rgb, cv2.COLOR_RGB2BGR)

    bbox1 = detect_page_bbox_safe_v2(img_bgr)
    img_bgr = crop_bbox_v2(img_bgr, bbox1)

    img_bgr = safe_photometric_normalize_bgr_v2(img_bgr)
    gray = cv2.cvtColor(img_bgr, cv2.COLOR_BGR2GRAY)

    angle = estimate_small_deskew_angle_v2(gray)
    if abs(angle) > 0.01:
        img_bgr = rotate_bound_bgr_v2(img_bgr, angle, border_value=(255,255,255))

    if second_crop_after_rotate:
        bbox2 = detect_page_bbox_safe_v2(img_bgr)
        img_bgr = crop_bbox_v2(img_bgr, bbox2)

    img_bgr = final_scan_render_v2(img_bgr, whiten_strength=whiten_strength)

    if add_white_border > 0:
        img_bgr = cv2.copyMakeBorder(
            img_bgr,
            add_white_border, add_white_border, add_white_border, add_white_border,
            cv2.BORDER_CONSTANT, value=(255,255,255)
        )

    rgb_out = cv2.cvtColor(img_bgr, cv2.COLOR_BGR2RGB)
    return Image.fromarray(rgb_out)

def canonical_scan_domain_real_to_synth_v2(
    pil_img,
    style_ref=None,
    whiten_strength=0.22,
    second_crop_after_rotate=False,
    white_boost=0.10,
    protect_strength=0.92,
    add_white_border=0
):
    base = canonical_scan_domain_real_v2(
        pil_img,
        whiten_strength=whiten_strength,
        second_crop_after_rotate=second_crop_after_rotate,
        add_white_border=0
    )

    style_ref = style_ref or DEFAULT_STYLE_REF
    gray = np.array(base.convert("L")).astype(np.uint8)
    gray2 = match_gray_to_synth_style_v2(
        gray, style_ref, white_boost=white_boost, protect_strength=protect_strength
    )
    out_bgr = cv2.cvtColor(gray2, cv2.COLOR_GRAY2BGR)

    if add_white_border > 0:
        out_bgr = cv2.copyMakeBorder(
            out_bgr,
            add_white_border, add_white_border, add_white_border, add_white_border,
            cv2.BORDER_CONSTANT, value=(255,255,255)
        )

    rgb_out = cv2.cvtColor(out_bgr, cv2.COLOR_BGR2RGB)
    return Image.fromarray(rgb_out)

def page_preproc_v2_border0(pil_img, style_ref=None):
    return canonical_scan_domain_real_to_synth_v2(
        pil_img,
        style_ref=style_ref,
        whiten_strength=0.22,
        second_crop_after_rotate=False,
        white_boost=0.10,
        protect_strength=0.92,
        add_white_border=0
    )
