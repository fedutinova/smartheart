import { useState, useCallback, useEffect, useRef } from 'react';

export type ImageStep = 'select' | 'crop' | 'ready';

interface ImageInputState {
  step: ImageStep;
  previewSrc: string | null;
  croppedBlob: Blob | null;
  croppedPreview: string | null;
}

interface ImageInputActions {
  handleFileSelect: (file: File) => Promise<void>;
  handleCropComplete: (blob: Blob) => void;
  handleCropCancel: () => void;
  rotateImage: () => Promise<void>;
  handleRecrop: () => void;
  reset: () => void;
  setError: (msg: string) => void;
}

export interface UseImageInputReturn extends ImageInputState, ImageInputActions {
  error: string;
}

const MAX_IMAGE_DIM = 4096;
const COMPRESS_QUALITY = 0.85;
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

async function srcToBlob(src: string): Promise<Blob> {
  const res = await fetch(src);
  return res.blob();
}

export function useImageInput(): UseImageInputReturn {
  const [step, setStep] = useState<ImageStep>('select');
  const [previewSrc, setPreviewSrc] = useState<string | null>(null);
  const [croppedBlob, setCroppedBlob] = useState<Blob | null>(null);
  const [croppedPreview, setCroppedPreview] = useState<string | null>(null);
  const [error, setError] = useState('');

  // Track current object URLs in refs so the unmount cleanup always
  // revokes the latest values, not the stale ones from the first render.
  const previewSrcRef = useRef(previewSrc);
  const croppedPreviewRef = useRef(croppedPreview);
  previewSrcRef.current = previewSrc;
  croppedPreviewRef.current = croppedPreview;

  useEffect(() => {
    return () => {
      if (previewSrcRef.current) URL.revokeObjectURL(previewSrcRef.current);
      if (croppedPreviewRef.current && croppedPreviewRef.current !== previewSrcRef.current) {
        URL.revokeObjectURL(croppedPreviewRef.current);
      }
    };
  }, []);

  const handleFileSelect = useCallback(async (file: File) => {
    // Revoke previous object URLs to avoid memory leaks on re-select
    if (previewSrcRef.current) URL.revokeObjectURL(previewSrcRef.current);
    if (croppedPreviewRef.current && croppedPreviewRef.current !== previewSrcRef.current) {
      URL.revokeObjectURL(croppedPreviewRef.current);
    }

    if (!file.type.startsWith('image/') && file.type !== 'application/pdf') {
      setError('Поддерживаются только изображения и PDF');
      return;
    }
    if (file.size > 10 * 1024 * 1024) {
      if (file.type.startsWith('image/')) {
        try {
          const blob = await compressImage(file);
          const url = URL.createObjectURL(blob);
          setError('');
          setPreviewSrc(url);
          setCroppedBlob(blob);
          setCroppedPreview(url);
          setStep('ready');
        } catch {
          setError('Не удалось сжать изображение');
        }
        return;
      }
      setError('Файл слишком большой (макс. 10MB)');
      return;
    }
    setError('');
    const url = URL.createObjectURL(file);
    setPreviewSrc(url);
    const blob = new Blob([file], { type: file.type });
    setCroppedBlob(blob);
    setCroppedPreview(url);
    setStep('ready');
  }, []);

  const handleCropComplete = useCallback((blob: Blob) => {
    if (croppedPreviewRef.current && croppedPreviewRef.current !== previewSrcRef.current) {
      URL.revokeObjectURL(croppedPreviewRef.current);
    }
    setCroppedBlob(blob);
    const url = URL.createObjectURL(blob);
    setCroppedPreview(url);
    setStep('ready');
  }, []);

  const handleCropCancel = useCallback(() => {
    if (previewSrc) {
      setCroppedPreview(previewSrc);
      srcToBlob(previewSrc).then(setCroppedBlob).catch(() => {});
    }
    setStep('ready');
  }, [previewSrc]);

  const rotateImage = useCallback(async () => {
    if (!croppedBlob) return;
    const img = new Image();
    const url = URL.createObjectURL(croppedBlob);
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
    if (croppedPreview) URL.revokeObjectURL(croppedPreview);
    setCroppedBlob(blob);
    setCroppedPreview(URL.createObjectURL(blob));
  }, [croppedBlob, croppedPreview]);

  const handleRecrop = useCallback(() => {
    if (croppedPreview && croppedPreview !== previewSrc) URL.revokeObjectURL(croppedPreview);
    setCroppedBlob(null);
    setCroppedPreview(null);
    setStep('crop');
  }, [croppedPreview, previewSrc]);

  const reset = useCallback(() => {
    if (previewSrc) URL.revokeObjectURL(previewSrc);
    if (croppedPreview) URL.revokeObjectURL(croppedPreview);
    setPreviewSrc(null);
    setCroppedBlob(null);
    setCroppedPreview(null);
    setStep('select');
    setError('');
  }, [previewSrc, croppedPreview]);

  return {
    step, previewSrc, croppedBlob, croppedPreview, error,
    handleFileSelect, handleCropComplete, handleCropCancel,
    rotateImage, handleRecrop, reset, setError,
  };
}
