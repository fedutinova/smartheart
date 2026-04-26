import { useState, useCallback, useEffect, useRef } from 'react';
import type { ECGClientMeta, RedactionBox } from '@/types';
import { applyOCRRedaction } from '@/utils/redaction';

/** Step in the image input workflow: select file → crop → review → ready to upload */
export type ImageStep = 'select' | 'crop' | 'review' | 'ready';

/** Current state of the image input */
interface ImageInputState {
  /** Current workflow step */
  step: ImageStep;
  /** Object URL of the current editable source image */
  previewSrc: string | null;
  /** Current source image before redaction */
  sourceBlob: Blob | null;
  /** Blob of the final processed image */
  croppedBlob: Blob | null;
  /** Object URL of the processed image preview */
  croppedPreview: string | null;
  /** Opaque mask rectangles used in review UI */
  redactionBoxes: RedactionBox[];
  /** Client-side metadata collected during redaction */
  clientMeta: ECGClientMeta | null;
}

/** Action methods for controlling the image input workflow */
interface ImageInputActions {
  /** Load a file, optionally compress it, and set as preview */
  handleFileSelect: (file: File) => Promise<void>;
  /** Accept crop result and re-run redaction */
  handleCropComplete: (blob: Blob) => Promise<void>;
  /** Cancel crop and return to the review step */
  handleCropCancel: () => void;
  /** Rotate current source image 90° clockwise */
  rotateImage: () => Promise<void>;
  /** Confirm auto-applied redaction masks */
  confirmRedaction: () => void;
  /** Return to crop step from review/ready state */
  handleRecrop: () => void;
  /** Clear all state and return to select step */
  reset: () => void;
  /** Set error message */
  setError: (msg: string) => void;
}

/** Return value of useImageInput hook */
export interface UseImageInputReturn extends ImageInputState, ImageInputActions {
  /** Current error message, empty string if no error */
  error: string;
}

/** Maximum pixel dimension for image files (scales down if exceeded) */
const MAX_IMAGE_DIM = 4096;
/** JPEG quality (0-1) for compressed images during file selection */
const COMPRESS_QUALITY = 0.85;
/** JPEG quality (0-1) for rotated images to preserve detail */
const ROTATE_QUALITY = 0.92;

function getContext2D(canvas: HTMLCanvasElement): CanvasRenderingContext2D {
  const ctx = canvas.getContext('2d');
  if (!ctx) throw new Error('Браузер не смог создать canvas-контекст. Попробуйте уменьшить изображение.');
  return ctx;
}

function canvasToBlob(canvas: HTMLCanvasElement, type: string, quality: number): Promise<Blob> {
  return new Promise((resolve, reject) => {
    canvas.toBlob(
      (b) => (b ? resolve(b) : reject(new Error('Не удалось сконвертировать изображение'))),
      type,
      quality,
    );
  });
}

function compressImage(file: File): Promise<Blob> {
  return new Promise((resolve, reject) => {
    const img = new Image();
    const objectUrl = URL.createObjectURL(file);
    img.onload = () => {
      URL.revokeObjectURL(objectUrl);
      let { width, height } = img;
      if (width > MAX_IMAGE_DIM || height > MAX_IMAGE_DIM) {
        const scale = MAX_IMAGE_DIM / Math.max(width, height);
        width = Math.round(width * scale);
        height = Math.round(height * scale);
      }
      const canvas = document.createElement('canvas');
      canvas.width = width;
      canvas.height = height;
      const ctx = getContext2D(canvas);
      ctx.drawImage(img, 0, 0, width, height);
      canvasToBlob(canvas, 'image/jpeg', COMPRESS_QUALITY).then(resolve, reject);
    };
    img.onerror = () => {
      URL.revokeObjectURL(objectUrl);
      reject(new Error('Не удалось загрузить изображение'));
    };
    img.src = objectUrl;
  });
}

/**
 * Manages image input workflow with four states: select → crop → review → ready.
 *
 * State machine:
 * - **select**: Initial state, waiting for file selection
 * - **crop**: User adjusts the current source image
 * - **review**: Automatic OCR-based masks are shown before upload
 * - **ready**: Redacted image is confirmed and ready for upload
 *
 * Features:
 * - Auto-compresses large images (>10MB) to fit MAX_IMAGE_DIM and COMPRESS_QUALITY
 * - Re-applies OCR-based redaction after file select, crop, and rotation
 * - Keeps redaction metadata for the upload request
 * - Handles memory cleanup with useRef to track the latest object URLs
 *
 * @returns State and action methods to control the image workflow
 */
export function useImageInput(): UseImageInputReturn {
  const [step, setStep] = useState<ImageStep>('select');
  const [previewSrc, setPreviewSrc] = useState<string | null>(null);
  const [sourceBlob, setSourceBlob] = useState<Blob | null>(null);
  const [croppedBlob, setCroppedBlob] = useState<Blob | null>(null);
  const [croppedPreview, setCroppedPreview] = useState<string | null>(null);
  const [redactionBoxes, setRedactionBoxes] = useState<RedactionBox[]>([]);
  const [clientMeta, setClientMeta] = useState<ECGClientMeta | null>(null);
  const [error, setError] = useState('');

  const previewSrcRef = useRef(previewSrc);
  const croppedPreviewRef = useRef(croppedPreview);
  const sourceBlobRef = useRef(sourceBlob);
  previewSrcRef.current = previewSrc;
  croppedPreviewRef.current = croppedPreview;
  sourceBlobRef.current = sourceBlob;

  useEffect(() => {
    return () => {
      if (previewSrcRef.current) URL.revokeObjectURL(previewSrcRef.current);
      if (croppedPreviewRef.current && croppedPreviewRef.current !== previewSrcRef.current) {
        URL.revokeObjectURL(croppedPreviewRef.current);
      }
    };
  }, []);

  const revokeDerivedPreview = useCallback(() => {
    if (croppedPreviewRef.current && croppedPreviewRef.current !== previewSrcRef.current) {
      URL.revokeObjectURL(croppedPreviewRef.current);
    }
  }, []);

  const resetDerivedState = useCallback(() => {
    revokeDerivedPreview();
    setCroppedBlob(null);
    setCroppedPreview(null);
    setRedactionBoxes([]);
    setClientMeta(null);
  }, [revokeDerivedPreview]);

  const setSourcePreview = useCallback((blob: Blob) => {
    if (previewSrcRef.current) {
      URL.revokeObjectURL(previewSrcRef.current);
    }
    setPreviewSrc(URL.createObjectURL(blob));
  }, []);

  const applyRedaction = useCallback(async (blob: Blob) => {
    if (!blob.type.startsWith('image/')) {
      revokeDerivedPreview();
      setCroppedBlob(blob);
      setCroppedPreview(previewSrcRef.current);
      setRedactionBoxes([]);
      setClientMeta(null);
      setStep('ready');
      return;
    }

    const result = await applyOCRRedaction(blob);
    revokeDerivedPreview();
    setCroppedBlob(result.blob);
    setCroppedPreview(URL.createObjectURL(result.blob));
    setRedactionBoxes(result.boxes);
    setClientMeta(result.clientMeta);
    setStep('review');
  }, [revokeDerivedPreview]);

  const updateSourceAndRedaction = useCallback(async (blob: Blob) => {
    setSourceBlob(blob);
    setSourcePreview(blob);
    await applyRedaction(blob);
  }, [applyRedaction, setSourcePreview]);

  const handleFileSelect = useCallback(async (file: File) => {
    if (previewSrcRef.current) URL.revokeObjectURL(previewSrcRef.current);
    revokeDerivedPreview();
    setPreviewSrc(null);
    setSourceBlob(null);
    setStep('select');
    setError('');
    resetDerivedState();

    if (!file.type.startsWith('image/') && file.type !== 'application/pdf') {
      setError('Поддерживаются только изображения и PDF');
      return;
    }

    if (file.size > 10 * 1024 * 1024) {
      if (file.type.startsWith('image/')) {
        try {
          const blob = await compressImage(file);
          await updateSourceAndRedaction(blob);
        } catch {
          setError('Не удалось сжать изображение');
        }
        return;
      }
      setError('Файл слишком большой (макс. 10MB)');
      return;
    }

    const blob = new Blob([file], { type: file.type });
    await updateSourceAndRedaction(blob);
  }, [resetDerivedState, revokeDerivedPreview, updateSourceAndRedaction]);

  const handleCropComplete = useCallback(async (blob: Blob) => {
    setError('');
    await updateSourceAndRedaction(blob);
  }, [updateSourceAndRedaction]);

  const handleCropCancel = useCallback(() => {
    setError('');
    setStep('review');
  }, []);

  const rotateImage = useCallback(async () => {
    const currentSourceBlob = sourceBlobRef.current;
    if (!currentSourceBlob || !currentSourceBlob.type.startsWith('image/')) return;

    const img = new Image();
    const url = URL.createObjectURL(currentSourceBlob);
    img.src = url;
    await new Promise<void>((resolve) => { img.onload = () => resolve(); });
    URL.revokeObjectURL(url);

    const canvas = document.createElement('canvas');
    canvas.width = img.height;
    canvas.height = img.width;
    const ctx = getContext2D(canvas);
    ctx.translate(canvas.width / 2, canvas.height / 2);
    ctx.rotate(Math.PI / 2);
    ctx.drawImage(img, -img.width / 2, -img.height / 2);

    const blob = await canvasToBlob(canvas, 'image/jpeg', ROTATE_QUALITY);
    await updateSourceAndRedaction(blob);
  }, [updateSourceAndRedaction]);

  const confirmRedaction = useCallback(() => {
    setStep('ready');
  }, []);

  const handleRecrop = useCallback(() => {
    resetDerivedState();
    setStep('crop');
  }, [resetDerivedState]);

  const reset = useCallback(() => {
    if (previewSrcRef.current) URL.revokeObjectURL(previewSrcRef.current);
    revokeDerivedPreview();
    setPreviewSrc(null);
    setSourceBlob(null);
    setCroppedBlob(null);
    setCroppedPreview(null);
    setRedactionBoxes([]);
    setClientMeta(null);
    setStep('select');
    setError('');
  }, [revokeDerivedPreview]);

  return {
    step,
    previewSrc,
    sourceBlob,
    croppedBlob,
    croppedPreview,
    redactionBoxes,
    clientMeta,
    error,
    handleFileSelect,
    handleCropComplete,
    handleCropCancel,
    rotateImage,
    confirmRedaction,
    handleRecrop,
    reset,
    setError,
  };
}
