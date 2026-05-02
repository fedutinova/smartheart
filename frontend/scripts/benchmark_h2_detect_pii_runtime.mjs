import { execFileSync } from 'node:child_process';
import { mkdtempSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';

import { pipeline } from '@huggingface/transformers';
import { createWorker } from 'tesseract.js';

const ROOT = resolve(process.cwd(), '..');
const manifestPath = resolve(ROOT, 'h2/with-test-data-manifest.json');
const outDir = resolve(ROOT, 'docs/artifacts/source-data/redaction');
const outJsonPath = resolve(outDir, 'h2_detect_pii_runtime_results.json');
const outMdPath = resolve(ROOT, 'docs/artifacts/h2-detect-pii-runtime-report.md');

const MIN_WORD_BOX_WIDTH = 10;
const MIN_WORD_BOX_HEIGHT = 10;
const MIN_WORD_CONFIDENCE = 35;
const MIN_WORD_TEXT_LENGTH = 2;

const PII_PATTERNS = [
  /\b\d{1,2}[.\-\/]\d{1,2}[.\-\/]\d{2,4}\b/g,
  /\b\d{1,2}\s+(янв|фев|мар|апр|май|июн|июл|авг|сен|окт|ноя|дек)\w*\s+\d{4}\b/gi,
  /\b[А-Я]?\d{5,12}\b/g,
  /\b\d{3}-\d{3}-\d{3}\s\d{2}\b/g,
];

function percentile95(values) {
  const sorted = [...values].sort((a, b) => a - b);
  const index = Math.max(0, Math.ceil(sorted.length * 0.95) - 1);
  return sorted[index] ?? 0;
}

function mean(values) {
  if (values.length === 0) return 0;
  return values.reduce((sum, value) => sum + value, 0) / values.length;
}

function loadManifest() {
  return JSON.parse(readFileSync(manifestPath, 'utf-8'));
}

function cropTopThird(inputPath, outputPath) {
  execFileSync('convert', [
    inputPath,
    '-gravity',
    'North',
    '-crop',
    '100%x33%+0+0',
    '+repage',
    '-resize',
    '1000x>',
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

async function main() {
  const manifest = loadManifest();
  const limit = Number(process.env.H2_BENCH_LIMIT || '0');
  const rows = limit > 0 ? manifest.slice(0, limit) : manifest;
  const results = [];
  const tempDir = mkdtempSync(join(tmpdir(), 'h2-detect-pii-'));
  const worker = await createWorker('rus+eng');
  await worker.setParameters({ tessedit_pageseg_mode: '6' });
  const ner = await pipeline(
    'token-classification',
    'onnx-community/bert-base-NER-Russian-ONNX',
    { dtype: 'q8' },
  );

  try {
    mkdirSync(outDir, { recursive: true });

    for (const [index, row] of rows.entries()) {
      const sourcePath = resolve(ROOT, row.image_path);
      const croppedPath = resolve(tempDir, row.image_file);
      console.log(`[${index + 1}/${rows.length}] detectPII ${row.image_file}`);

      const started = Date.now();
      cropTopThird(sourcePath, croppedPath);
      const result = await worker.recognize(croppedPath);
      const words = (result.data.words || []).filter(isUsefulWord);
      const fullText = words.map((word) => word.text).join(' ');
      const nerResults = fullText ? await ner(fullText) : [];

      let regexMatches = 0;
      for (const pattern of PII_PATTERNS) {
        pattern.lastIndex = 0;
        while (pattern.exec(fullText) !== null) {
          regexMatches += 1;
        }
      }

      const elapsedMs = Date.now() - started;
      results.push({
        id: row.id,
        imageFile: row.image_file,
        detectPiiRuntimeMs: elapsedMs,
        rawWordsCount: result.data.words?.length || 0,
        filteredWordsCount: words.length,
        nerEntitiesCount: Array.isArray(nerResults) ? nerResults.length : 0,
        regexMatches,
      });
    }
  } finally {
    await worker.terminate();
    rmSync(tempDir, { recursive: true, force: true });
  }

  const runtimes = results.map((item) => item.detectPiiRuntimeMs);
  const meanRuntime = Number(mean(runtimes).toFixed(2));
  const summary = {
    datasetSize: results.length,
    meanDetectPiiRuntimeMs: meanRuntime,
    p95DetectPiiRuntimeMs: percentile95(runtimes),
    minDetectPiiRuntimeMs: Math.min(...runtimes),
    maxDetectPiiRuntimeMs: Math.max(...runtimes),
    thresholdMs: 3000,
    thresholdMetric: 'mean',
    thresholdPassed: meanRuntime < 3000,
    measurementScope: 'верхняя треть + downsample до 1000px + PSM 6 + фильтрация слов + переиспользуемый OCR worker + NER/regex',
  };

  writeFileSync(outJsonPath, JSON.stringify({ summary, results }, null, 2), 'utf-8');

  const report = `# Отчёт О Времени detectPII Для H2

- Размер набора: ${summary.datasetSize}
- Среднее время detectPII: ${summary.meanDetectPiiRuntimeMs} мс
- P95 времени detectPII: ${summary.p95DetectPiiRuntimeMs} мс (справочно)
- Минимальное время: ${summary.minDetectPiiRuntimeMs} мс
- Максимальное время: ${summary.maxDetectPiiRuntimeMs} мс
- Порог по среднему времени: ${summary.thresholdMs} мс
- Критерий выполнен (mean < ${summary.thresholdMs} мс): ${summary.thresholdPassed ? 'да' : 'нет'}
- Область измерения: ${summary.measurementScope}
`;

  writeFileSync(outMdPath, report, 'utf-8');
  console.log(JSON.stringify(summary, null, 2));
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
