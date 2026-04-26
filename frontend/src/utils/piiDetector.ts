import { pipeline } from '@huggingface/transformers';
import { createWorker } from 'tesseract.js';
import type { RedactionBox } from '@/types';

/** Character offset span for PII match */
interface CharSpan {
  start: number;
  end: number;
  confidence?: number;
}

/** Word with pixel location and character tracking */
interface WordEntry {
  text: string;
  bbox: { x0: number; y0: number; x1: number; y1: number };
  charStart: number;
  charEnd: number;
}

/** PII patterns for dates, IDs, and СНИЛС */
const PII_PATTERNS = [
  { pattern: /\b\d{1,2}[.\-\/]\d{1,2}[.\-\/]\d{2,4}\b/g, label: 'date' },
  {
    pattern: /\b\d{1,2}\s+(янв|фев|мар|апр|май|июн|июл|авг|сен|окт|ноя|дек)\w*\s+\d{4}\b/gi,
    label: 'date_words',
  },
  { pattern: /\b[А-Я]?\d{5,12}\b/g, label: 'patient_id' },
  { pattern: /\b\d{3}-\d{3}-\d{3}\s\d{2}\b/g, label: 'snils' },
];

// Lazy-load NER pipeline once and reuse across calls
let nerPipeline: any = null;

async function getNERPipeline(): Promise<any> {
  if (!nerPipeline) {
    try {
      nerPipeline = await pipeline(
        'token-classification' as const,
        'onnx-community/bert-base-NER-Russian-ONNX',
        { dtype: 'q8' } as any
      );
    } catch (err) {
      console.error('Failed to load NER model, falling back to regex-only detection', err);
      nerPipeline = null; // Mark as failed so regex-only fallback continues
      throw err;
    }
  }
  return nerPipeline;
}

/**
 * Extract PII text regions from an image using Tesseract OCR + NER + Regex.
 * Returns pixel-space bounding boxes for text that matches PII patterns.
 */
export async function detectPII(imageBlob: Blob): Promise<RedactionBox[]> {
  const worker = await createWorker('rus+eng');
  const piiSpans: CharSpan[] = [];

  try {
    // 1. OCR: extract text + word positions
    const result = await worker.recognize(imageBlob);
    const words = result.data.words || [];

    if (words.length === 0) {
      return [];
    }

    // 2. Build word map with character offsets
    let cursor = 0;
    const wordMap: WordEntry[] = words.map((w: any) => {
      const entry: WordEntry = {
        text: w.text,
        bbox: w.bbox,
        charStart: cursor,
        charEnd: cursor + w.text.length,
      };
      cursor += w.text.length + 1; // +1 for space separator
      return entry;
    });

    // 3. Full text for NER/regex
    const fullText = wordMap.map((w) => w.text).join(' ');

    // 4. NER-based PII detection (Russian names)
    try {
      const nerPipeline = await getNERPipeline();
      const nerResults = await nerPipeline(fullText);

      // Filter for person-related entities
      const personEntityGroups = ['PER', 'LAST_NAME', 'FIRST_NAME', 'MIDDLE_NAME'];
      for (const entity of nerResults) {
        if (personEntityGroups.includes(entity.entity_group)) {
          piiSpans.push({
            start: entity.start,
            end: entity.end,
            confidence: entity.score,
          });
        }
      }
    } catch (err) {
      console.warn('NER detection failed, continuing with regex-only', err);
      // Fall back to regex-only mode
    }

    // 5. Regex-based detection (dates, IDs, СНИЛС)
    for (const { pattern } of PII_PATTERNS) {
      let match;
      while ((match = pattern.exec(fullText)) !== null) {
        piiSpans.push({
          start: match.index,
          end: match.index + match[0].length,
        });
      }
    }

    if (piiSpans.length === 0) {
      return [];
    }

    // 6. Merge overlapping spans (simple approach: sort & merge)
    piiSpans.sort((a, b) => a.start - b.start);
    const mergedSpans: CharSpan[] = [];
    for (const span of piiSpans) {
      const last = mergedSpans[mergedSpans.length - 1];
      if (last && span.start <= last.end) {
        // Overlapping: extend
        last.end = Math.max(last.end, span.end);
        if (span.confidence && (!last.confidence || span.confidence > last.confidence)) {
          last.confidence = span.confidence;
        }
      } else {
        mergedSpans.push(span);
      }
    }

    // 7. Map char spans to word bboxes
    const redactionBoxes: RedactionBox[] = [];
    for (const span of mergedSpans) {
      // Find all words that overlap this span
      const overlappingWords = wordMap.filter(
        (w) => w.charStart < span.end && w.charEnd > span.start
      );

      if (overlappingWords.length === 0) continue;

      // Merge bboxes
      const minX = Math.min(...overlappingWords.map((w) => w.bbox.x0));
      const minY = Math.min(...overlappingWords.map((w) => w.bbox.y0));
      const maxX = Math.max(...overlappingWords.map((w) => w.bbox.x1));
      const maxY = Math.max(...overlappingWords.map((w) => w.bbox.y1));

      redactionBoxes.push({
        x: Math.round(minX),
        y: Math.round(minY),
        width: Math.round(maxX - minX),
        height: Math.round(maxY - minY),
      });
    }

    return redactionBoxes;
  } finally {
    await worker.terminate();
  }
}
