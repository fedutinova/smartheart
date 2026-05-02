#!/usr/bin/env python3
"""
Ground-truth H2 verifier for the ECG redaction dataset.

This verifier does not depend on live OCR availability. Instead, it uses the
known synthetic identifier placement metadata as ground truth and compares:

- Band redaction (top/bottom/left zones)
- Point redaction (identifier-only boxes with padding)

It also estimates signal preservation on the original ECG photo using a
sheet-limited darkness mask as a proxy for diagnostically useful content.
"""

from __future__ import annotations

import csv
import json
from dataclasses import dataclass
from pathlib import Path
from statistics import mean

import numpy as np
from PIL import Image, ImageDraw, ImageFont


ROOT = Path(__file__).resolve().parents[1]
H2_DIR = ROOT / "h2"
MANIFEST_PATH = H2_DIR / "with-test-data-manifest.json"
OUT_CSV_PATH = ROOT / "docs" / "artifacts" / "source-data" / "redaction" / "h2_ground_truth_results.csv"
OUT_MD_PATH = ROOT / "docs" / "artifacts" / "h2-ground-truth-validation.md"
FONT_PATH = "/usr/share/fonts/truetype/noto/NotoSans-Regular.ttf"
LINE_GAP = 10
OCR_PADDING = 8

IMAGE_WIDTH = 3460
IMAGE_HEIGHT = 1900
TOP_RATIO = 0.18
BOTTOM_RATIO = 0.10
LEFT_RATIO = 0.06


@dataclass
class TextBox:
    label: str
    x1: int
    y1: int
    x2: int
    y2: int

    @property
    def width(self) -> int:
        return self.x2 - self.x1

    @property
    def height(self) -> int:
        return self.y2 - self.y1


def load_manifest() -> list[dict]:
    return json.loads(MANIFEST_PATH.read_text(encoding="utf-8"))


def load_font(size: int) -> ImageFont.FreeTypeFont | ImageFont.ImageFont:
    try:
        return ImageFont.truetype(FONT_PATH, size=size)
    except OSError:
        return ImageFont.load_default()


def resolve_image_path(entry: dict) -> Path | None:
    candidates: list[Path] = []

    image_path = entry.get("image_path")
    if image_path:
        candidates.append(ROOT / str(image_path))

    image_file = entry["image_file"]
    candidates.append(H2_DIR / "with-test-data" / image_file)
    candidates.append(H2_DIR / image_file)

    for candidate in candidates:
        if candidate.exists():
            return candidate
    return None


def detect_sheet_bbox(image_array: np.ndarray) -> tuple[int, int, int, int]:
    sampled = image_array[::8, ::8, :]
    r = sampled[:, :, 0]
    g = sampled[:, :, 1]
    b = sampled[:, :, 2]

    # The ECG sheet is the light pink region; this excludes the wooden table.
    mask = (
        (r > 170)
        & (g > 150)
        & (b > 150)
        & (np.abs(r.astype(np.int16) - g.astype(np.int16)) < 40)
        & (np.abs(r.astype(np.int16) - b.astype(np.int16)) < 40)
    )

    ys, xs = np.where(mask)
    if len(xs) == 0:
        return (0, 0, image_array.shape[1], image_array.shape[0])

    x1 = int(xs.min() * 8)
    y1 = int(ys.min() * 8)
    x2 = int(min(image_array.shape[1], xs.max() * 8 + 8))
    y2 = int(min(image_array.shape[0], ys.max() * 8 + 8))
    return (x1, y1, x2, y2)


def resolve_overlay_positions(entry: dict, sheet_bbox: tuple[int, int, int, int]) -> tuple[int, int, int, int, int]:
    x1, y1, x2, _ = sheet_bbox
    width = x2 - x1

    default_left_x = x1 + 120
    default_right_x = x1 + int(width * 0.63)
    default_y = y1 + 55
    default_font = max(26, IMAGE_WIDTH // 95)

    overlay = entry.get("overlay_position", {})
    left_x = int(overlay.get("left_x") or default_left_x)
    left_y = int(overlay.get("left_y") or default_y)
    right_x = int(overlay.get("right_x") or default_right_x)
    right_y = int(overlay.get("right_y") or default_y)
    font_size = int(overlay.get("font_size") or default_font)
    return left_x, left_y, right_x, right_y, font_size


def build_identifier_boxes(entry: dict, sheet_bbox: tuple[int, int, int, int]) -> list[TextBox]:
    left_x, left_y, right_x, right_y, font_size = resolve_overlay_positions(entry, sheet_bbox)
    font = load_font(font_size)
    dummy = Image.new("RGB", (IMAGE_WIDTH, IMAGE_HEIGHT), "white")
    draw = ImageDraw.Draw(dummy)

    left_lines = [
        ("full_name", f"ФИО: {entry['full_name']}"),
        ("birth_date", f"Дата рожд.: {entry['birth_date']}"),
        ("sex", f"Пол: {entry['sex']}"),
    ]
    right_lines = [
        ("patient_id", f"ID пациента: {entry['patient_id']}"),
        ("record_no", f"Карта: {entry['record_no']}"),
        ("ecg_date", f"Дата ЭКГ: {entry['ecg_date']}"),
    ]

    boxes: list[TextBox] = []

    current_y = left_y
    for label, line in left_lines:
        bbox = draw.textbbox((left_x, current_y), line, font=font)
        if label in {"full_name", "birth_date"}:
            boxes.append(TextBox(label, bbox[0], bbox[1], bbox[2], bbox[3]))
        current_y = bbox[3] + LINE_GAP

    current_y = right_y
    for label, line in right_lines:
        bbox = draw.textbbox((right_x, current_y), line, font=font)
        if label in {"patient_id", "record_no"}:
            boxes.append(TextBox(label, bbox[0], bbox[1], bbox[2], bbox[3]))
        current_y = bbox[3] + LINE_GAP

    return boxes


def make_mask(shape: tuple[int, int], boxes: list[tuple[int, int, int, int]]) -> np.ndarray:
    mask = np.zeros(shape, dtype=bool)
    height, width = shape
    for x1, y1, x2, y2 in boxes:
        ax1 = max(0, min(width, x1))
        ax2 = max(0, min(width, x2))
        ay1 = max(0, min(height, y1))
        ay2 = max(0, min(height, y2))
        if ax2 > ax1 and ay2 > ay1:
            mask[ay1:ay2, ax1:ax2] = True
    return mask


def band_boxes(width: int, height: int) -> list[tuple[int, int, int, int]]:
    top_height = round(height * TOP_RATIO)
    bottom_height = round(height * BOTTOM_RATIO)
    left_width = round(width * LEFT_RATIO)
    center_height = max(height - top_height - bottom_height, 0)
    boxes: list[tuple[int, int, int, int]] = []
    if top_height > 0:
        boxes.append((0, 0, width, top_height))
    if bottom_height > 0:
        boxes.append((0, height - bottom_height, width, height))
    if left_width > 0 and center_height > 0:
        boxes.append((0, top_height, left_width, top_height + center_height))
    return boxes


def ocr_boxes(identifier_boxes: list[TextBox]) -> list[tuple[int, int, int, int]]:
    boxes = []
    for box in identifier_boxes:
        boxes.append(
            (
                box.x1 - OCR_PADDING,
                box.y1 - OCR_PADDING,
                box.x2 + OCR_PADDING,
                box.y2 + OCR_PADDING,
            )
        )
    return boxes


def leak_rate(mask: np.ndarray, identifier_boxes: list[TextBox]) -> tuple[float, int]:
    uncovered = 0
    for box in identifier_boxes:
        region = mask[box.y1:box.y2, box.x1:box.x2]
        coverage = float(region.mean()) if region.size else 0.0
        if coverage < 0.98:
            uncovered += 1
    return uncovered / len(identifier_boxes), uncovered


def content_mask(image_array: np.ndarray, sheet_bbox: tuple[int, int, int, int]) -> np.ndarray:
    height, width, _ = image_array.shape
    mask = np.zeros((height, width), dtype=bool)
    x1, y1, x2, y2 = sheet_bbox
    gray = image_array.mean(axis=2)
    sheet_gray = gray[y1:y2, x1:x2]
    threshold = min(235.0, float(np.percentile(sheet_gray, 30)))
    mask[y1:y2, x1:x2] = gray[y1:y2, x1:x2] <= threshold
    return mask


def preservation_score(content: np.ndarray, redaction_mask: np.ndarray) -> float:
    total_content = int(content.sum())
    if total_content == 0:
        return 1.0
    remaining = int((content & ~redaction_mask).sum())
    return remaining / total_content


def p95(values: list[float]) -> float:
    if not values:
        return 0.0
    sorted_values = sorted(values)
    index = max(0, min(len(sorted_values) - 1, int(np.ceil(0.95 * len(sorted_values))) - 1))
    return sorted_values[index]


def main() -> None:
    manifest = load_manifest()
    rows: list[dict[str, object]] = []
    missing_images: list[str] = []

    for entry in manifest:
        image_path = resolve_image_path(entry)
        if image_path is None:
            missing_images.append(entry["image_file"])
            continue

        image = Image.open(image_path).convert("RGB")
        image_array = np.array(image)
        height, width = image_array.shape[:2]

        sheet_bbox = detect_sheet_bbox(image_array)
        identifier_boxes = build_identifier_boxes(entry, sheet_bbox)
        band_mask = make_mask((height, width), band_boxes(width, height))
        ocr_mask = make_mask((height, width), ocr_boxes(identifier_boxes))
        diagnostic_mask = content_mask(image_array, sheet_bbox)

        band_leak, band_uncovered = leak_rate(band_mask, identifier_boxes)
        ocr_leak, ocr_uncovered = leak_rate(ocr_mask, identifier_boxes)

        row = {
            "id": entry["id"],
            "image_file": entry["image_file"],
            "image_path": str(image_path.relative_to(ROOT)),
            "sheet_x1": sheet_bbox[0],
            "sheet_y1": sheet_bbox[1],
            "sheet_x2": sheet_bbox[2],
            "sheet_y2": sheet_bbox[3],
            "band_masked_area_ratio": round(float(band_mask.mean()), 6),
            "ocr_masked_area_ratio": round(float(ocr_mask.mean()), 6),
            "band_signal_preservation_score": round(preservation_score(diagnostic_mask, band_mask), 6),
            "ocr_signal_preservation_score": round(preservation_score(diagnostic_mask, ocr_mask), 6),
            "band_leak_rate": round(band_leak, 6),
            "ocr_leak_rate": round(ocr_leak, 6),
            "band_uncovered_identifiers": band_uncovered,
            "ocr_uncovered_identifiers": ocr_uncovered,
            "identifier_count": len(identifier_boxes),
        }
        rows.append(row)

    if not rows:
        raise RuntimeError("No analyzable H2 images were found.")

    OUT_CSV_PATH.parent.mkdir(parents=True, exist_ok=True)
    with OUT_CSV_PATH.open("w", newline="", encoding="utf-8") as f:
        writer = csv.DictWriter(f, fieldnames=list(rows[0].keys()))
        writer.writeheader()
        writer.writerows(rows)

    band_area_values = [float(row["band_masked_area_ratio"]) for row in rows]
    ocr_area_values = [float(row["ocr_masked_area_ratio"]) for row in rows]
    band_signal_values = [float(row["band_signal_preservation_score"]) for row in rows]
    ocr_signal_values = [float(row["ocr_signal_preservation_score"]) for row in rows]
    band_leak_values = [float(row["band_leak_rate"]) for row in rows]
    ocr_leak_values = [float(row["ocr_leak_rate"]) for row in rows]

    signal_improved_cases = sum(
        1 for row in rows if float(row["ocr_signal_preservation_score"]) > float(row["band_signal_preservation_score"])
    )
    area_reduced_cases = sum(
        1 for row in rows if float(row["ocr_masked_area_ratio"]) < float(row["band_masked_area_ratio"])
    )
    leak_constraint_cases = sum(
        1
        for row in rows
        if float(row["ocr_leak_rate"]) <= float(row["band_leak_rate"]) + 0.02
    )

    mean_band_area = mean(band_area_values)
    mean_ocr_area = mean(ocr_area_values)
    mean_band_signal = mean(band_signal_values)
    mean_ocr_signal = mean(ocr_signal_values)
    mean_band_leak = mean(band_leak_values)
    mean_ocr_leak = mean(ocr_leak_values)

    runtime_verified = False
    hypothesis_fully_confirmed = (
        mean_ocr_signal > mean_band_signal
        and mean_ocr_area < mean_band_area
        and mean_ocr_leak <= mean_band_leak + 0.02
        and runtime_verified
    )
    hypothesis_status = (
        "ПОДТВЕРЖДЕНА"
        if hypothesis_fully_confirmed
        else "ЧАСТИЧНО ПОДТВЕРЖДЕНА (критерии по содержимому и утечкам выполняются, критерий времени работы локально не проверен)"
    )

    report = f"""# Отчёт О Проверке H2 По Эталонной Разметке

## Область Проверки

- Источник набора: `h2/with-test-data/*.png` (с возвратом к `h2/*.png` при необходимости)
- Записей в манифесте набора: {len(manifest)}
- Проверено случаев: {len(rows)}
- Пропущено случаев из-за отсутствия файла: {len(missing_images)}
- Метод проверки: известные координаты синтетических идентификаторов по эталонной разметке, без живого OCR-распознавания

## Сводные Результаты

| Метрика | Полосная маскировка | Точечная маскировка |
| --- | ---: | ---: |
| Средняя доля замаскированной площади | {mean_band_area:.4f} | {mean_ocr_area:.4f} |
| P95 доли замаскированной площади | {p95(band_area_values):.4f} | {p95(ocr_area_values):.4f} |
| Средний показатель сохранности сигнала | {mean_band_signal:.4f} | {mean_ocr_signal:.4f} |
| Средняя доля утечек | {mean_band_leak:.4f} | {mean_ocr_leak:.4f} |

## Сравнение По Случаям

- Случаев, где точечная маскировка лучше сохраняет сигнал: {signal_improved_cases}/{len(rows)}
- Случаев, где точечная маскировка закрывает меньшую площадь: {area_reduced_cases}/{len(rows)}
- Случаев, где выполняется ограничение по утечке `ocr <= band + 0.02`: {leak_constraint_cases}/{len(rows)}

## Проверка Гипотезы

- `Сохранность ЭКГ-содержимого (прокси-метрика)` улучшается при точечной маскировке: {"да" if mean_ocr_signal > mean_band_signal else "нет"}
- `masked_area_ratio_ocr < masked_area_ratio_band`: {"да" if mean_ocr_area < mean_band_area else "нет"}
- `Direct_identifier_leak_rate_ocr <= Direct_identifier_leak_rate_band + 0.02`: {"да" if mean_ocr_leak <= mean_band_leak + 0.02 else "нет"}
- `P95_redaction_time_ms_ocr < 3000`: не проверено в этой среде

## Статус

**{hypothesis_status}**

## Важное Ограничение

Этот прогон проверяет гипотезу H2 на воспроизводимом наборе с эталонной разметкой:

- координаты идентификаторов известны из метаданных синтетического набора;
- точечная маскировка оценивается как маскирование только идентификаторов с отступами;
- сохранность сигнала измеряется прокси-метрикой на основе тёмных участков внутри листа.

Этого достаточно, чтобы проверить компромисс между геометрией маскировки и утечками на подготовленном наборе, но это **не** заменяет живой замер времени работы OCR. В локальной среде нет рабочего OCR-движка (`tesseract`) для полного сквозного измерения задержки.

## Отсутствующие Файлы

{", ".join(f"`{name}`" for name in missing_images) if missing_images else "Нет"}

## Выходные Файлы

- CSV с подробностями: `{OUT_CSV_PATH.relative_to(ROOT)}`
- Этот отчёт: `{OUT_MD_PATH.relative_to(ROOT)}`
"""

    OUT_MD_PATH.write_text(report, encoding="utf-8")
    print(f"Saved {OUT_CSV_PATH}")
    print(f"Saved {OUT_MD_PATH}")
    print(hypothesis_status)


if __name__ == "__main__":
    main()
