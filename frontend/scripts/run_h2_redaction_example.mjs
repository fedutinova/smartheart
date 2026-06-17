import { execFileSync } from 'node:child_process';
import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { basename, join, resolve } from 'node:path';

import { pipeline } from '@huggingface/transformers';
import { createWorker } from 'tesseract.js';

const ROOT = resolve(process.cwd(), '..');
const DEFAULT_IMAGE = 'h2/with-test-data/00049_hr_1R.png';
const OUT_DIR = resolve(ROOT, 'docs/artifacts/h2-example-images');

const OCR_REGION_RATIO = 1 / 3;
const OCR_MAX_WIDTH = 1000;
const MIN_WORD_BOX_WIDTH = 10;
const MIN_WORD_BOX_HEIGHT = 10;
const MIN_WORD_CONFIDENCE = 35;
const MIN_WORD_TEXT_LENGTH = 2;

const PII_PATTERNS = [
  { pattern: /[А-ЯЁ][а-яё]+(?:\s+[А-ЯЁ][а-яё]+){1,2}/g, label: 'fio' },
  { pattern: /[А-ЯЁ][а-яё]+\s+[А-ЯЁ]\.\s?[А-ЯЁ]\./g, label: 'fio_initials_after' },
  { pattern: /[А-ЯЁ]\.\s?[А-ЯЁ]\.\s+[А-ЯЁ][а-яё]+/g, label: 'fio_initials_before' },
  { pattern: /\b\d{1,2}[.\-/]\d{1,2}[.\-/]\d{2}(?:\s?\d{2})?\b/g, label: 'date' },
  {
    pattern: /\b\d{1,2}\s+(янв|фев|мар|апр|май|июн|июл|авг|сен|окт|ноя|дек)\w*\s+\d{4}\b/gi,
    label: 'date_words',
  },
  { pattern: /\+?7?\s?\(?\d{3}\)?\s?\d{3}[-\s]?\d{2}[-\s]?\d{2}/g, label: 'phone' },
  { pattern: /\b[А-Я]?\d{5,12}\b/g, label: 'patient_id' },
  { pattern: /\b\d{3}-\d{3}-\d{3}\s\d{2}\b/g, label: 'snils' },
];

function imageSize(path) {
  const output = execFileSync('magick', ['identify', '-format', '%w %h', path], { encoding: 'utf-8' });
  const [width, height] = output.trim().split(/\s+/).map(Number);
  return { width, height };
}

function cropTopRegion(inputPath, outputPath, imageHeight) {
  const ocrRegionHeight = Math.max(1, Math.round(imageHeight * OCR_REGION_RATIO));
  execFileSync('magick', [
    inputPath,
    '-crop',
    `100%x${ocrRegionHeight}+0+0`,
    '+repage',
    '-resize',
    `${OCR_MAX_WIDTH}x>`,
    outputPath,
  ]);
}

function isUsefulWord(word) {
  const text = String(word.text || '').trim();
  const width = Math.max(0, Number(word.bbox?.x1 || 0) - Number(word.bbox?.x0 || 0));
  const height = Math.max(0, Number(word.bbox?.y1 || 0) - Number(word.bbox?.y0 || 0));
  const confidence = Number(word.confidence ?? word.conf ?? 100);
  const hasDigits = /\d/.test(text);

  if (!text) return false;
  if (width < MIN_WORD_BOX_WIDTH || height < MIN_WORD_BOX_HEIGHT) return false;
  if (text.length < MIN_WORD_TEXT_LENGTH && !hasDigits) return false;
  if (confidence < MIN_WORD_CONFIDENCE && !hasDigits) return false;
  if (!/[A-Za-zА-Яа-яЁё0-9]/.test(text)) return false;

  return true;
}

function buildWordMap(words, inverseScale) {
  let cursor = 0;
  return words.map((word) => {
    const text = String(word.text || '').trim();
    const entry = {
      text,
      bbox: {
        x0: word.bbox.x0 * inverseScale,
        y0: word.bbox.y0 * inverseScale,
        x1: word.bbox.x1 * inverseScale,
        y1: word.bbox.y1 * inverseScale,
      },
      charStart: cursor,
      charEnd: cursor + text.length,
      confidence: Number(word.confidence ?? word.conf ?? 0),
    };
    cursor += text.length + 1;
    return entry;
  });
}

function mergeSpans(spans) {
  spans.sort((a, b) => a.start - b.start);
  const merged = [];

  for (const span of spans) {
    const last = merged[merged.length - 1];
    if (last && span.start <= last.end) {
      last.end = Math.max(last.end, span.end);
      last.labels = [...new Set([...last.labels, ...span.labels])];
    } else {
      merged.push({ ...span, labels: [...span.labels] });
    }
  }

  return merged;
}

function spansToBoxes(spans, wordMap) {
  const boxes = [];

  for (const span of spans) {
    const overlappingWords = wordMap.filter(
      (word) => word.charStart < span.end && word.charEnd > span.start,
    );

    if (overlappingWords.length === 0) continue;

    const minX = Math.min(...overlappingWords.map((word) => word.bbox.x0));
    const minY = Math.min(...overlappingWords.map((word) => word.bbox.y0));
    const maxX = Math.max(...overlappingWords.map((word) => word.bbox.x1));
    const maxY = Math.max(...overlappingWords.map((word) => word.bbox.y1));

    boxes.push({
      x: Math.round(minX),
      y: Math.round(minY),
      width: Math.round(maxX - minX),
      height: Math.round(maxY - minY),
      text: overlappingWords.map((word) => word.text).join(' '),
      labels: span.labels,
    });
  }

  return boxes;
}

function drawRedactedImage(inputPath, outputPath, previewPath, boxes) {
  const drawArgs = boxes.map((box) => {
    const x2 = box.x + box.width;
    const y2 = box.y + box.height;
    return `rectangle ${box.x},${box.y} ${x2},${y2}`;
  });

  if (drawArgs.length === 0) {
    execFileSync('magick', [inputPath, '-quality', '92', outputPath]);
  } else {
    execFileSync('magick', [
      inputPath,
      '-fill',
      '#111827',
      '-draw',
      drawArgs.join(' '),
      '-quality',
      '92',
      outputPath,
    ]);
  }

  execFileSync('magick', [outputPath, '-resize', '1200x>', '-quality', '92', previewPath]);
}

async function main() {
  const inputArg = process.argv[2] || DEFAULT_IMAGE;
  const inputPath = resolve(ROOT, inputArg);
  const sourceName = basename(inputPath).replace(/\.[^.]+$/, '');
  const outputPath = resolve(OUT_DIR, `${sourceName}_h2_ocr_redacted.jpg`);
  const previewPath = resolve(OUT_DIR, `${sourceName}_h2_ocr_redacted_preview.jpg`);
  const jsonPath = resolve(OUT_DIR, `${sourceName}_h2_ocr_redaction_result.json`);
  const tempDir = mkdtempSync(join(tmpdir(), 'h2-redaction-example-'));
  const croppedPath = resolve(tempDir, `${sourceName}_top.png`);
  const startedAt = Date.now();

  mkdirSync(OUT_DIR, { recursive: true });

  const { width, height } = imageSize(inputPath);
  cropTopRegion(inputPath, croppedPath, height);
  const croppedSize = imageSize(croppedPath);
  const inverseScale = width / croppedSize.width;

  const worker = await createWorker('rus+eng');
  await worker.setParameters({ tessedit_pageseg_mode: '6' });

  let ner = null;
  let nerError = null;
  try {
    ner = await pipeline('token-classification', 'onnx-community/bert-base-NER-Russian-ONNX', {
      dtype: 'q8',
    });
  } catch (error) {
    nerError = error instanceof Error ? error.message : String(error);
  }

  try {
    const ocrResult = await worker.recognize(croppedPath);
    const words = (ocrResult.data.words || []).filter(isUsefulWord);
    const wordMap = buildWordMap(words, inverseScale);
    const fullText = wordMap.map((word) => word.text).join(' ');
    const spans = [];

    if (ner && fullText) {
      const nerResults = await ner(fullText);
      const personEntityGroups = ['PER', 'LAST_NAME', 'FIRST_NAME', 'MIDDLE_NAME'];
      for (const entity of nerResults) {
        if (personEntityGroups.includes(entity.entity_group)) {
          spans.push({
            start: entity.start,
            end: entity.end,
            labels: [`ner:${entity.entity_group}`],
          });
        }
      }
    }

    for (const { pattern, label } of PII_PATTERNS) {
      pattern.lastIndex = 0;
      let match;
      while ((match = pattern.exec(fullText)) !== null) {
        spans.push({
          start: match.index,
          end: match.index + match[0].length,
          labels: [label],
        });
      }
    }

    const mergedSpans = mergeSpans(spans);
    const boxes = spansToBoxes(mergedSpans, wordMap);
    drawRedactedImage(inputPath, outputPath, previewPath, boxes);

    const maskedArea = boxes.reduce((sum, box) => sum + box.width * box.height, 0);
    const result = {
      input: inputArg,
      output: outputPath.replace(`${ROOT}/`, ''),
      preview: previewPath.replace(`${ROOT}/`, ''),
      image_width: width,
      image_height: height,
      ocr_region: 'upper third',
      ocr_region_height: Math.round(height * OCR_REGION_RATIO),
      ocr_width_after_resize: croppedSize.width,
      boxes_count: boxes.length,
      masked_area_ratio: Number((maskedArea / (width * height)).toFixed(6)),
      redaction_ms: Date.now() - startedAt,
      raw_words_count: ocrResult.data.words?.length || 0,
      filtered_words_count: words.length,
      ner_loaded: Boolean(ner),
      ner_error: nerError,
      full_text: fullText,
      boxes,
    };

    writeFileSync(jsonPath, JSON.stringify(result, null, 2), 'utf-8');
    console.log(JSON.stringify({ ...result, json: jsonPath.replace(`${ROOT}/`, '') }, null, 2));
  } finally {
    await worker.terminate();
    rmSync(tempDir, { recursive: true, force: true });
  }
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
