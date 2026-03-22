import { useState, useRef, useCallback } from 'react';
import ReactCrop, { type Crop, type PixelCrop } from 'react-image-crop';
import 'react-image-crop/dist/ReactCrop.css';

interface ImageCropperProps {
  imageSrc: string;
  onCropComplete: (blob: Blob) => void;
  onCancel: () => void;
}

export function ImageCropper({ imageSrc, onCropComplete, onCancel }: ImageCropperProps) {
  const [crop, setCrop] = useState<Crop>();
  const [completedCrop, setCompletedCrop] = useState<PixelCrop>();
  const [processing, setProcessing] = useState(false);
  const imgRef = useRef<HTMLImageElement>(null);

  const onImageLoad = useCallback((e: React.SyntheticEvent<HTMLImageElement>) => {
    const { width, height } = e.currentTarget;
    const initialCrop: Crop = {
      unit: '%',
      x: 10,
      y: 10,
      width: 80,
      height: 80,
    };
    setCrop(initialCrop);
    // Set initial completed crop in pixels
    setCompletedCrop({
      unit: 'px',
      x: Math.round(width * 0.1),
      y: Math.round(height * 0.1),
      width: Math.round(width * 0.8),
      height: Math.round(height * 0.8),
    });
  }, []);

  const handleConfirm = async () => {
    if (!completedCrop || !imgRef.current) return;
    setProcessing(true);
    try {
      const blob = await getCroppedBlob(imgRef.current, completedCrop);
      onCropComplete(blob);
    } catch {
      // fallback: send the original image
      const response = await fetch(imageSrc);
      const blob = await response.blob();
      onCropComplete(blob);
    } finally {
      setProcessing(false);
    }
  };

  return (
    <div className="space-y-4">
      <div className="max-h-[60vh] sm:max-h-[500px] overflow-auto border border-gray-200 rounded-lg bg-gray-50 p-1 sm:p-2 touch-manipulation">
        <ReactCrop
          crop={crop}
          onChange={(c) => setCrop(c)}
          onComplete={(c) => setCompletedCrop(c)}
        >
          <img
            ref={imgRef}
            src={imageSrc}
            alt="Изображение для обрезки"
            onLoad={onImageLoad}
            className="max-w-full h-auto"
          />
        </ReactCrop>
      </div>

      <p className="text-xs text-gray-500">
        Перетаскивайте углы и края рамки для изменения области обрезки. На телефоне — используйте палец.
      </p>

      <div className="flex items-center justify-between">
        <button
          type="button"
          onClick={onCancel}
          className="text-gray-600 hover:text-gray-800"
        >
          Отмена
        </button>
        <button
          type="button"
          onClick={handleConfirm}
          disabled={processing || !completedCrop}
          className="px-4 py-2 bg-rose-600 text-white rounded-md hover:bg-rose-700 disabled:opacity-50"
        >
          {processing ? 'Обработка...' : 'Подтвердить обрезку'}
        </button>
      </div>
    </div>
  );
}

async function getCroppedBlob(image: HTMLImageElement, crop: PixelCrop): Promise<Blob> {
  const canvas = document.createElement('canvas');
  const scaleX = image.naturalWidth / image.width;
  const scaleY = image.naturalHeight / image.height;

  canvas.width = crop.width * scaleX;
  canvas.height = crop.height * scaleY;

  const ctx = canvas.getContext('2d');
  if (!ctx) throw new Error('Canvas context not available');

  ctx.drawImage(
    image,
    crop.x * scaleX,
    crop.y * scaleY,
    crop.width * scaleX,
    crop.height * scaleY,
    0,
    0,
    canvas.width,
    canvas.height,
  );

  return new Promise((resolve, reject) => {
    canvas.toBlob(
      (blob) => {
        if (blob) resolve(blob);
        else reject(new Error('Canvas toBlob failed'));
      },
      'image/jpeg',
      0.92,
    );
  });
}
