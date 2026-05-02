import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';

import { createWorker } from 'tesseract.js';

const ROOT = resolve(process.cwd(), '..');
const manifestPath = resolve(ROOT, 'h2/with-test-data-manifest.json');
const outDir = resolve(ROOT, 'docs/artifacts/source-data/redaction');
const outJsonPath = resolve(outDir, 'h2_tesseract_runtime_results.json');
const outMdPath = resolve(ROOT, 'docs/artifacts/h2-tesseract-runtime-report.md');

function percentile95(values) {
  const sorted = [...values].sort((a, b) => a - b);
  const index = Math.max(0, Math.ceil(sorted.length * 0.95) - 1);
  return sorted[index] ?? 0;
}

function mean(values) {
  if (values.length === 0) {
    return 0;
  }
  return values.reduce((sum, value) => sum + value, 0) / values.length;
}

function loadManifest() {
  return JSON.parse(readFileSync(manifestPath, 'utf-8'));
}

async function recognizeOne(worker, imagePath) {
  const image = readFileSync(imagePath);
  const started = Date.now();

  const result = await worker.recognize(image);
  return {
    elapsedMs: Date.now() - started,
    wordsCount: result.data.words?.length || 0,
  };
}

async function main() {
  const manifest = loadManifest();
  const limit = Number(process.env.H2_BENCH_LIMIT || '0');
  const rows = limit > 0 ? manifest.slice(0, limit) : manifest;
  const results = [];
  const worker = await createWorker('rus+eng');

  try {
    mkdirSync(outDir, { recursive: true });

    for (const [index, row] of rows.entries()) {
      const imagePath = resolve(ROOT, row.image_path);
      console.log(`[${index + 1}/${rows.length}] OCR ${row.image_file}`);
      const { elapsedMs, wordsCount } = await recognizeOne(worker, imagePath);
      results.push({
        id: row.id,
        imageFile: row.image_file,
        imagePath: row.image_path,
        ocrRuntimeMs: elapsedMs,
        wordsCount,
      });
    }
  } finally {
    await worker.terminate();
  }

  const runtimes = results.map((item) => item.ocrRuntimeMs);
  const summary = {
    datasetSize: results.length,
    meanOcrRuntimeMs: Number(mean(runtimes).toFixed(2)),
    p95OcrRuntimeMs: percentile95(runtimes),
    minOcrRuntimeMs: Math.min(...runtimes),
    maxOcrRuntimeMs: Math.max(...runtimes),
    thresholdMs: 3000,
    thresholdPassed: percentile95(runtimes) < 3000,
    measurementScope: 'время OCR через tesseract.js с переиспользуемым воркером и полным изображением',
  };

  writeFileSync(outJsonPath, JSON.stringify({ summary, results }, null, 2), 'utf-8');

  const report = `# Отчёт О Времени OCR Для H2

- Размер набора: ${summary.datasetSize}
- Среднее время OCR: ${summary.meanOcrRuntimeMs} мс
- P95 времени OCR: ${summary.p95OcrRuntimeMs} мс
- Минимальное время: ${summary.minOcrRuntimeMs} мс
- Максимальное время: ${summary.maxOcrRuntimeMs} мс
- Порог: ${summary.thresholdMs} мс
- Критерий выполнен: ${summary.thresholdPassed ? 'да' : 'нет'}
- Область измерения: ${summary.measurementScope}
`;

  writeFileSync(outMdPath, report, 'utf-8');

  console.log(JSON.stringify(summary, null, 2));
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
