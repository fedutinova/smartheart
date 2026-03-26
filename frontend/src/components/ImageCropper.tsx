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
      x: 5,
      y: 5,
      width: 90,
      height: 90,
    };
    setCrop(initialCrop);
    setCompletedCrop({
      unit: 'px',
      x: Math.round(width * 0.05),
      y: Math.round(height * 0.05),
      width: Math.round(width * 0.9),
      height: Math.round(height * 0.9),
    });
  }, []);

  const handleConfirm = async () => {
    if (!completedCrop || !imgRef.current) return;
    setProcessing(true);
    try {
      const blob = await getCroppedBlob(imgRef.current, completedCrop);
      onCropComplete(blob);
    } catch {
      const response = await fetch(imageSrc);
      const blob = await response.blob();
      onCropComplete(blob);
    } finally {
      setProcessing(false);
    }
  };

  return (
    <div className="space-y-3">
      <div className="rounded-xl overflow-hidden border border-gray-200 bg-gray-50">
        <div className="max-h-[60vh] sm:max-h-[500px] overflow-auto touch-manipulation flex items-center justify-center p-2">
          <ReactCrop
            crop={crop}
            onChange={(c) => setCrop(c)}
            onComplete={(c) => setCompletedCrop(c)}
          >
            <img
              ref={imgRef}
              src={imageSrc}
              alt="Обрезка"
              onLoad={onImageLoad}
              className="max-w-full h-auto block"
            />
          </ReactCrop>
        </div>
        <div className="flex border-t border-gray-200">
          <button
            type="button"
            onClick={onCancel}
            className="flex-1 py-3 text-sm font-medium text-gray-600 hover:bg-gray-100 active:bg-gray-200 transition-colors"
          >
            Отмена
          </button>
          <div className="w-px bg-gray-200" />
          <button
            type="button"
            onClick={handleConfirm}
            disabled={processing || !completedCrop}
            className="flex-1 py-3 text-sm font-medium text-rose-600 hover:bg-rose-50 active:bg-rose-100 disabled:opacity-50 transition-colors"
          >
            {processing ? 'Обработка...' : 'Применить'}
          </button>
        </div>
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
