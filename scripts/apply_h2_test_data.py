#!/usr/bin/env python3
"""
Prepare and apply synthetic identifier text onto H2 ECG photos.

Usage:
  python3 scripts/apply_h2_test_data.py create-template
  python3 scripts/apply_h2_test_data.py apply
  python3 scripts/apply_h2_test_data.py make-transparent
  python3 scripts/apply_h2_test_data.py export-manifest
"""

from __future__ import annotations

import csv
import hashlib
import json
import random
import sys
from pathlib import Path

from PIL import Image, ImageDraw, ImageFont


ROOT = Path(__file__).resolve().parents[1]
H2_DIR = ROOT / "h2"
CSV_PATH = H2_DIR / "test-identifiers-template.csv"
OUT_DIR = H2_DIR / "with-test-data"
TRANSPARENT_OUT_DIR = H2_DIR / "transparent-overlays"
MANIFEST_CSV_PATH = H2_DIR / "with-test-data-manifest.csv"
MANIFEST_JSON_PATH = H2_DIR / "with-test-data-manifest.json"
FONT_PATH = "/usr/share/fonts/truetype/noto/NotoSans-Regular.ttf"

NAMES = [
    "Иван Петров",
    "Мария Сидорова",
    "Алексей Иванов",
    "Елена Федорова",
    "Дмитрий Смирнов",
    "Ольга Кузнецова",
    "Никита Павлов",
    "Анна Васильева",
    "Сергей Орлов",
    "Татьяна Морозова",
]


def rng_for(name: str) -> random.Random:
    seed = int(hashlib.sha256(name.encode("utf-8")).hexdigest()[:16], 16)
    return random.Random(seed)


def create_template() -> None:
    images = sorted(H2_DIR.glob("*_hr_1R.png"))
    if not images:
        raise SystemExit("No H2 images found.")

    with CSV_PATH.open("w", newline="", encoding="utf-8") as f:
        writer = csv.DictWriter(
            f,
            fieldnames=[
                "image_file",
                "full_name",
                "birth_date",
                "patient_id",
                "record_no",
                "sex",
                "ecg_date",
                "left_x",
                "left_y",
                "right_x",
                "right_y",
                "font_size",
            ],
        )
        writer.writeheader()
        for idx, image in enumerate(images, start=1):
            yy = 1980 + (idx % 30)
            mm = 1 + (idx % 12)
            dd = 1 + (idx % 27)
            ecg_dd = 1 + ((idx * 3) % 27)
            ecg_mm = 1 + ((idx * 5) % 12)
            writer.writerow(
                {
                    "image_file": image.name,
                    "full_name": NAMES[(idx - 1) % len(NAMES)],
                    "birth_date": f"{dd:02d}.{mm:02d}.{yy}",
                    "patient_id": f"{70000000 + idx:08d}",
                    "record_no": f"EKG-{2026}{idx:04d}",
                    "sex": "Ж" if idx % 2 == 0 else "М",
                    "ecg_date": f"{ecg_dd:02d}.{ecg_mm:02d}.2026",
                    "left_x": "",
                    "left_y": "",
                    "right_x": "",
                    "right_y": "",
                    "font_size": "",
                }
            )

    print(f"Created template: {CSV_PATH}")


def load_font(size: int) -> ImageFont.FreeTypeFont | ImageFont.ImageFont:
    try:
        return ImageFont.truetype(FONT_PATH, size=size)
    except OSError:
        return ImageFont.load_default()


def draw_block(
    draw: ImageDraw.ImageDraw,
    x: int,
    y: int,
    lines: list[str],
    font: ImageFont.ImageFont,
    fill: tuple[int, int, int, int],
) -> None:
    line_gap = 10
    current_y = y
    for line in lines:
        draw.text((x, current_y), line, font=font, fill=fill)
        bbox = draw.textbbox((x, current_y), line, font=font)
        current_y = bbox[3] + line_gap


def render_row(row: dict[str, str], transparent_only: bool) -> None:
    image_path = H2_DIR / row["image_file"]
    if not image_path.exists():
        print(f"Skip missing file: {image_path}")
        return

    image = Image.open(image_path).convert("RGBA")
    width, height = image.size

    overlay = Image.new("RGBA", image.size, (255, 255, 255, 0))
    draw = ImageDraw.Draw(overlay)

    font_size = int(row["font_size"]) if row.get("font_size") else max(26, width // 95)
    font = load_font(font_size)
    fill = (70, 70, 70, 190)
    local_rng = rng_for(row["image_file"])

    default_left_x = int(width * 0.22) + local_rng.randint(-18, 18)
    default_right_x = int(width * 0.64) + local_rng.randint(-22, 22)
    default_top_y = int(height * 0.18) + local_rng.randint(-10, 10)

    left_x = int(row["left_x"]) if row.get("left_x") else default_left_x
    right_x = int(row["right_x"]) if row.get("right_x") else default_right_x
    left_y = int(row["left_y"]) if row.get("left_y") else default_top_y
    right_y = int(row["right_y"]) if row.get("right_y") else default_top_y

    left_lines = [
        f"ФИО: {row['full_name']}",
        f"Дата рожд.: {row['birth_date']}",
        f"Пол: {row['sex']}",
    ]
    right_lines = [
        f"ID пациента: {row['patient_id']}",
        f"Карта: {row['record_no']}",
        f"Дата ЭКГ: {row['ecg_date']}",
    ]

    draw_block(draw, left_x, left_y, left_lines, font, fill)
    draw_block(draw, right_x, right_y, right_lines, font, fill)

    if transparent_only:
        TRANSPARENT_OUT_DIR.mkdir(parents=True, exist_ok=True)
        out_path = TRANSPARENT_OUT_DIR / row["image_file"]
        overlay.save(out_path)
    else:
        result = Image.alpha_composite(image, overlay).convert("RGB")
        OUT_DIR.mkdir(parents=True, exist_ok=True)
        out_path = OUT_DIR / row["image_file"]
        result.save(out_path, quality=95)

    print(f"Saved {out_path}")


def apply_all() -> None:
    if not CSV_PATH.exists():
        raise SystemExit(
            f"Template not found: {CSV_PATH}. Run create-template first."
        )

    with CSV_PATH.open("r", newline="", encoding="utf-8") as f:
        reader = csv.DictReader(f)
        rows = list(reader)

    if not rows:
        raise SystemExit("Template CSV is empty.")

    for row in rows:
        render_row(row, transparent_only=False)


def make_transparent_all() -> None:
    if not CSV_PATH.exists():
        raise SystemExit(
            f"Template not found: {CSV_PATH}. Run create-template first."
        )

    with CSV_PATH.open("r", newline="", encoding="utf-8") as f:
        reader = csv.DictReader(f)
        rows = list(reader)

    if not rows:
        raise SystemExit("Template CSV is empty.")

    for row in rows:
        render_row(row, transparent_only=True)


def export_manifest() -> None:
    if not CSV_PATH.exists():
        raise SystemExit(
            f"Template not found: {CSV_PATH}. Run create-template first."
        )

    with CSV_PATH.open("r", newline="", encoding="utf-8") as f:
        rows = list(csv.DictReader(f))

    manifest_rows = []
    for idx, row in enumerate(rows, start=1):
        image_file = row["image_file"]
        manifest_rows.append(
            {
                "id": f"h2_{idx:03d}",
                "image_file": image_file,
                "image_path": f"h2/with-test-data/{image_file}",
                "full_name": row["full_name"],
                "birth_date": row["birth_date"],
                "patient_id": row["patient_id"],
                "record_no": row["record_no"],
                "sex": row["sex"],
                "ecg_date": row["ecg_date"],
                "expected_identifiers": [
                    row["full_name"],
                    row["birth_date"],
                    row["patient_id"],
                    row["record_no"],
                ],
                "overlay_position": {
                    "left_x": row.get("left_x", ""),
                    "left_y": row.get("left_y", ""),
                    "right_x": row.get("right_x", ""),
                    "right_y": row.get("right_y", ""),
                    "font_size": row.get("font_size", ""),
                },
            }
        )

    with MANIFEST_CSV_PATH.open("w", newline="", encoding="utf-8") as f:
        writer = csv.DictWriter(
            f,
            fieldnames=[
                "id",
                "image_file",
                "image_path",
                "full_name",
                "birth_date",
                "patient_id",
                "record_no",
                "sex",
                "ecg_date",
                "expected_identifiers",
            ],
        )
        writer.writeheader()
        for row in manifest_rows:
            writer.writerow(
                {
                    **{k: row[k] for k in writer.fieldnames if k != "expected_identifiers"},
                    "expected_identifiers": " | ".join(row["expected_identifiers"]),
                }
            )

    with MANIFEST_JSON_PATH.open("w", encoding="utf-8") as f:
        json.dump(manifest_rows, f, ensure_ascii=False, indent=2)
        f.write("\n")

    print(f"Saved {MANIFEST_CSV_PATH}")
    print(f"Saved {MANIFEST_JSON_PATH}")


def main() -> None:
    if len(sys.argv) != 2 or sys.argv[1] not in {
        "create-template",
        "apply",
        "make-transparent",
        "export-manifest",
    }:
        raise SystemExit(
            "Usage: python3 scripts/apply_h2_test_data.py create-template|apply|make-transparent|export-manifest"
        )

    if sys.argv[1] == "create-template":
        create_template()
    elif sys.argv[1] == "apply":
        apply_all()
    elif sys.argv[1] == "make-transparent":
        make_transparent_all()
    else:
        export_manifest()


if __name__ == "__main__":
    main()
