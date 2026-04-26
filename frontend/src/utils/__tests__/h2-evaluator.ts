import { createWorker } from 'tesseract.js';

/** Result of evaluating a redacted image for remaining PII */
export interface EvaluationResult {
  /** PII patterns found in the redacted image (should be empty) */
  residualIdentifiers: string[];
  /** Leak rate: residual / expected identifiers */
  leakRate: number;
  /** All words extracted via OCR for debugging */
  ocrWords: string[];
  /** Confidence scores for found identifiers */
  confidences: number[];
}

/** PII patterns to check for in redacted images */
const PII_PATTERNS = [
  { pattern: /\b\d{1,2}[.\-\/]\d{1,2}[.\-\/]\d{2,4}\b/g, label: 'date' },
  {
    pattern: /\b\d{1,2}\s+(янв|фев|мар|апр|май|июн|июл|авг|сен|окт|ноя|дек)\w*\s+\d{4}\b/gi,
    label: 'date_words',
  },
  { pattern: /\b[А-Я]?\d{5,12}\b/g, label: 'patient_id' },
  { pattern: /\b\d{3}-\d{3}-\d{3}\s\d{2}\b/g, label: 'snils' },
];

/**
 * Evaluate a redacted image for remaining PII (leak_rate metric for H2).
 *
 * This is the INDEPENDENT evaluator: it runs Tesseract on the OUTPUT
 * (redacted image), not the input, using only regex-based detection
 * (no NER model to avoid circular dependencies with the redaction logic).
 *
 * @param redactedBlob The image after redaction
 * @param expectedIdentifiers Original PII that should have been redacted
 * @returns Evaluation result including leak_rate metric
 */
export async function evaluateRedaction(
  redactedBlob: Blob,
  expectedIdentifiers: string[],
): Promise<EvaluationResult> {
  const worker = await createWorker('rus+eng');
  const residualIdentifiers: string[] = [];
  const confidences: number[] = [];

  try {
    // 1. OCR the redacted image
    const result = await worker.recognize(redactedBlob);
    const words = (result.data.words || []) as any[];
    const ocrWords = words.map((w: any) => w.text);

    if (ocrWords.length === 0) {
      return {
        residualIdentifiers: [],
        leakRate: 0,
        ocrWords: [],
        confidences: [],
      };
    }

    // 2. Build full text from OCR words
    const fullText = ocrWords.join(' ');

    // 3. Regex-based PII detection (no NER to maintain independence)
    const foundMatches: Array<{ text: string; confidence: number }> = [];
    for (const { pattern } of PII_PATTERNS) {
      let match;
      while ((match = pattern.exec(fullText)) !== null) {
        foundMatches.push({ text: match[0], confidence: 1.0 });
      }
    }

    // 4. Fuzzy-match found patterns against expected identifiers
    // (to account for slight OCR variations like '1' vs 'l', 'O' vs '0')
    for (const found of foundMatches) {
      for (const expected of expectedIdentifiers) {
        if (isFuzzyMatch(found.text, expected)) {
          if (!residualIdentifiers.includes(found.text)) {
            residualIdentifiers.push(found.text);
            confidences.push(found.confidence);
          }
          break;
        }
      }
    }

    // 5. Also check for capitalized words that might be names
    // (simple heuristic: if a word starts with capital Cyrillic and appears near a date/ID)
    const capitalizedWords = ocrWords.filter((w: any) => /^[А-ЯЁ]/.test(w) && w.length > 2);
    for (const word of capitalizedWords) {
      for (const expected of expectedIdentifiers) {
        if (isFuzzyMatch(word, expected)) {
          if (!residualIdentifiers.includes(word)) {
            residualIdentifiers.push(word);
            confidences.push(0.8); // Lower confidence for heuristic matches
          }
          break;
        }
      }
    }

    return {
      residualIdentifiers,
      leakRate: expectedIdentifiers.length > 0
        ? residualIdentifiers.length / expectedIdentifiers.length
        : 0,
      ocrWords,
      confidences,
    };
  } finally {
    await worker.terminate();
  }
}

/**
 * Fuzzy match check: allows for minor OCR errors.
 * Considers strings matching if:
 * - Exact match
 * - Normalized match (lowercase, no whitespace)
 * - Similar length (within 2 chars) and high character overlap (>80%)
 */
function isFuzzyMatch(ocr: string, expected: string): boolean {
  // Exact match
  if (ocr === expected) return true;

  // Normalized match
  const ocrNorm = ocr.toLowerCase().replace(/\s+/g, '');
  const expectedNorm = expected.toLowerCase().replace(/\s+/g, '');
  if (ocrNorm === expectedNorm) return true;

  // Allow minor length difference (OCR errors on dates/IDs)
  if (Math.abs(ocr.length - expected.length) > 2) return false;

  // Character overlap heuristic
  const overlap = countOverlap(ocrNorm, expectedNorm);
  const maxLen = Math.max(ocrNorm.length, expectedNorm.length);
  return overlap / maxLen > 0.8;
}

/**
 * Count overlapping characters between two strings.
 */
function countOverlap(a: string, b: string): number {
  const aChars = new Map<string, number>();
  for (const ch of a) {
    aChars.set(ch, (aChars.get(ch) ?? 0) + 1);
  }

  let overlap = 0;
  for (const ch of b) {
    if (aChars.has(ch) && aChars.get(ch)! > 0) {
      overlap++;
      aChars.set(ch, aChars.get(ch)! - 1);
    }
  }
  return overlap;
}
