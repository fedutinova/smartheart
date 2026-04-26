import type { BandRedactionConfig, ECGClientMeta, RedactionBox } from '@/types';

export interface BandRedactionResult {
  blob: Blob;
  boxes: RedactionBox[];
  clientMeta: ECGClientMeta;
}

export const DEFAULT_BAND_REDACTION_CONFIG: BandRedactionConfig = {
  topRatio: 0.18,
  bottomRatio: 0.1,
  leftRatio: 0.06,
};

function getContext2D(canvas: HTMLCanvasElement): CanvasRenderingContext2D {
  const ctx = canvas.getContext('2d');
  if (!ctx) {
    throw new Error('Браузер не смог подготовить canvas для обезличивания.');
  }
  return ctx;
}

function canvasToBlob(canvas: HTMLCanvasElement, type: string, quality?: number): Promise<Blob> {
  return new Promise((resolve, reject) => {
    canvas.toBlob(
      (blob) => (blob ? resolve(blob) : reject(new Error('Не удалось сформировать redacted blob'))),
      type,
      quality,
    );
  });
}

function blobToImage(blob: Blob): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const img = new Image();
    const objectUrl = URL.createObjectURL(blob);

    img.onload = () => {
      URL.revokeObjectURL(objectUrl);
      resolve(img);
    };
    img.onerror = () => {
      URL.revokeObjectURL(objectUrl);
      reject(new Error('Не удалось загрузить изображение для обезличивания.'));
    };
    img.src = objectUrl;
  });
}

function clampRatio(value: number): number {
  return Math.min(Math.max(value, 0), 0.5);
}

export function buildBandRedactionBoxes(
  width: number,
  height: number,
  config: BandRedactionConfig = DEFAULT_BAND_REDACTION_CONFIG,
): RedactionBox[] {
  const topHeight = Math.round(height * clampRatio(config.topRatio));
  const bottomHeight = Math.round(height * clampRatio(config.bottomRatio));
  const leftWidth = Math.round(width * clampRatio(config.leftRatio));
  const centerHeight = Math.max(height - topHeight - bottomHeight, 0);

  const boxes: RedactionBox[] = [];

  if (topHeight > 0) {
    boxes.push({ x: 0, y: 0, width, height: topHeight });
  }

  if (bottomHeight > 0) {
    boxes.push({ x: 0, y: height - bottomHeight, width, height: bottomHeight });
  }

  if (leftWidth > 0 && centerHeight > 0) {
    boxes.push({ x: 0, y: topHeight, width: leftWidth, height: centerHeight });
  }

  return boxes;
}

function getOutputType(blob: Blob): string {
  if (blob.type === 'image/png' || blob.type === 'image/webp') {
    return blob.type;
  }
  return 'image/jpeg';
}

export async function applyBandRedaction(
  sourceBlob: Blob,
  config: BandRedactionConfig = DEFAULT_BAND_REDACTION_CONFIG,
): Promise<BandRedactionResult> {
  const startedAt = performance.now();
  const image = await blobToImage(sourceBlob);
  const width = image.naturalWidth || image.width;
  const height = image.naturalHeight || image.height;
  const boxes = buildBandRedactionBoxes(width, height, config);

  const canvas = document.createElement('canvas');
  canvas.width = width;
  canvas.height = height;

  const ctx = getContext2D(canvas);
  ctx.drawImage(image, 0, 0, width, height);
  ctx.fillStyle = '#111827';
  for (const box of boxes) {
    ctx.fillRect(box.x, box.y, box.width, box.height);
  }

  const outputType = getOutputType(sourceBlob);
  const blob = await canvasToBlob(canvas, outputType, outputType === 'image/jpeg' ? 0.92 : undefined);
  const maskedArea = boxes.reduce((sum, box) => sum + box.width * box.height, 0);

  return {
    blob,
    boxes,
    clientMeta: {
      redaction_mode: 'band',
      redaction_ms: Math.round(performance.now() - startedAt),
      boxes_count: boxes.length,
      masked_area_ratio: maskedArea / (width * height),
      image_width: width,
      image_height: height,
    },
  };
}

/**
 * OCR-based redaction mode (H2 hypothesis verification)
 *
 * This function:
 * 1. Detects text on the ECG image using Tesseract OCR + NER + regex
 * 2. Identifies PII patterns (names via NER, IDs/dates via regex)
 * 3. Redacts only the minimal bounding boxes around detected PII
 * 4. Returns metrics for hypothesis comparison (masked_area_ratio, redaction_ms)
 */
export async function applyOCRRedaction(
  sourceBlob: Blob,
): Promise<BandRedactionResult> {
  const startedAt = performance.now();

  try {
    // Dynamically import to avoid module loading delay
    const { detectPII } = await import('./piiDetector');

    // Get image dimensions and pixels
    const image = await blobToImage(sourceBlob);
    const width = image.naturalWidth || image.width;
    const height = image.naturalHeight || image.height;

    // Detect PII regions using OCR + NER + regex
    const boxes = await detectPII(sourceBlob);

    // Draw image and apply redaction masks
    const canvas = document.createElement('canvas');
    canvas.width = width;
    canvas.height = height;

    const ctx = getContext2D(canvas);
    ctx.drawImage(image, 0, 0, width, height);
    ctx.fillStyle = '#111827'; // Same dark color as band mode for consistency
    for (const box of boxes) {
      ctx.fillRect(box.x, box.y, box.width, box.height);
    }

    // Convert to blob
    const outputType = getOutputType(sourceBlob);
    const blob = await canvasToBlob(canvas, outputType, outputType === 'image/jpeg' ? 0.92 : undefined);

    // Compute metrics
    const maskedArea = boxes.reduce((sum, box) => sum + box.width * box.height, 0);
    const maskedAreaRatio = maskedArea / (width * height);

    return {
      blob,
      boxes,
      clientMeta: {
        redaction_mode: 'ocr',
        redaction_ms: Math.round(performance.now() - startedAt),
        boxes_count: boxes.length,
        masked_area_ratio: maskedAreaRatio,
        image_width: width,
        image_height: height,
      },
    };
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    throw new Error(`OCR redaction failed: ${message}`);
  }
}
