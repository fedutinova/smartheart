import { useState, useCallback, useEffect } from 'react';

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

function compressImage(file: File): Promise<string> {
  const MAX_PIXELS = 4096;
  return new Promise((resolve, reject) => {
    const img = new Image();
    const objectUrl = URL.createObjectURL(file);
    img.onload = () => {
      URL.revokeObjectURL(objectUrl);
      let { width, height } = img;
      if (width > MAX_PIXELS || height > MAX_PIXELS) {
        const scale = MAX_PIXELS / Math.max(width, height);
        width = Math.round(width * scale);
        height = Math.round(height * scale);
      }
      const canvas = document.createElement('canvas');
      canvas.width = width;
      canvas.height = height;
      const ctx = canvas.getContext('2d')!;
      ctx.drawImage(img, 0, 0, width, height);
      resolve(canvas.toDataURL('image/jpeg', 0.85));
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

  // Cleanup Object URLs on unmount
  useEffect(() => {
    return () => {
      if (previewSrc) URL.revokeObjectURL(previewSrc);
      if (croppedPreview && croppedPreview !== previewSrc) URL.revokeObjectURL(croppedPreview);
    };
    // Only run on unmount — intentionally omit deps
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleFileSelect = useCallback(async (file: File) => {
    if (!file.type.startsWith('image/') && file.type !== 'application/pdf') {
      setError('Поддерживаются только изображения и PDF');
      return;
    }
    if (file.size > 10 * 1024 * 1024) {
      if (file.type.startsWith('image/')) {
        try {
          const compressedUrl = await compressImage(file);
          setError('');
          setPreviewSrc(compressedUrl);
          const blob = await srcToBlob(compressedUrl);
          setCroppedBlob(blob);
          setCroppedPreview(compressedUrl);
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
    setCroppedBlob(blob);
    const url = URL.createObjectURL(blob);
    setCroppedPreview(url);
    setStep('ready');
  }, []);

  const handleCropCancel = useCallback(() => {
    if (previewSrc) {
      setCroppedPreview(previewSrc);
      srcToBlob(previewSrc).then(setCroppedBlob);
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
    const ctx = canvas.getContext('2d')!;
    ctx.translate(canvas.width / 2, canvas.height / 2);
    ctx.rotate(Math.PI / 2);
    ctx.drawImage(img, -img.width / 2, -img.height / 2);
    const blob = await new Promise<Blob>((resolve) =>
      canvas.toBlob((b) => resolve(b!), 'image/jpeg', 0.92),
    );
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
