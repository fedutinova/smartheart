/// <reference types="node" />

import { mkdirSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

import { applyOCRRedaction } from '../redaction';
import { H2_TEST_CASES, loadTestCase } from './h2-dataset';

interface RuntimeCaseResult {
  id: string;
  imagePath: string;
  redactionMs: number;
  maskedAreaRatio: number;
  boxesCount: number;
}

function percentile95(values: number[]): number {
  const sorted = [...values].sort((a, b) => a - b);
  const index = Math.max(0, Math.ceil(0.95 * sorted.length) - 1);
  return sorted[index] ?? 0;
}

function mean(values: number[]): number {
  if (values.length === 0) {
    return 0;
  }
  return values.reduce((sum, value) => sum + value, 0) / values.length;
}

function outputPath(relativeToRepoRoot: string): string {
  return resolve(process.cwd(), '..', relativeToRepoRoot);
}

describe('H2 OCR runtime benchmark', () => {
  it(
    'measures OCR redaction time across the H2 dataset',
    async () => {
      const limit = Number(process.env.H2_BENCH_LIMIT || '0');
      const cases = limit > 0 ? H2_TEST_CASES.slice(0, limit) : H2_TEST_CASES;
      const results: RuntimeCaseResult[] = [];

      for (const testCase of cases) {
        const blob = await loadTestCase(testCase.id);
        const result = await applyOCRRedaction(blob);

        results.push({
          id: testCase.id,
          imagePath: testCase.imagePath,
          redactionMs: result.clientMeta.redaction_ms,
          maskedAreaRatio: result.clientMeta.masked_area_ratio,
          boxesCount: result.clientMeta.boxes_count,
        });
      }

      const redactionTimes = results.map((item) => item.redactionMs);
      const summary = {
        datasetSize: results.length,
        meanRedactionMs: Number(mean(redactionTimes).toFixed(2)),
        p95RedactionMs: percentile95(redactionTimes),
        maxRedactionMs: Math.max(...redactionTimes),
        minRedactionMs: Math.min(...redactionTimes),
        thresholdMs: 3000,
        thresholdPassed: percentile95(redactionTimes) < 3000,
      };

      const outDir = outputPath('docs/artifacts/source-data/redaction');
      mkdirSync(outDir, { recursive: true });

      writeFileSync(
        resolve(outDir, 'h2_ocr_runtime_results.json'),
        JSON.stringify({ summary, results }, null, 2),
        'utf-8',
      );

      const report = `# H2 OCR Runtime Benchmark

- Размер набора: ${summary.datasetSize}
- Среднее время OCR-маскировки: ${summary.meanRedactionMs} мс
- P95 времени OCR-маскировки: ${summary.p95RedactionMs} мс
- Минимальное время: ${summary.minRedactionMs} мс
- Максимальное время: ${summary.maxRedactionMs} мс
- Порог: ${summary.thresholdMs} мс
- Критерий выполнен: ${summary.thresholdPassed ? 'да' : 'нет'}
`;

      writeFileSync(outputPath('docs/artifacts/h2-ocr-runtime-benchmark.md'), report, 'utf-8');

      expect(results.length).toBeGreaterThan(0);
    },
    60 * 60 * 1000,
  );
});
