import { useState, useRef, useCallback, useEffect, forwardRef, useImperativeHandle } from 'react';
import ReactCrop, { type Crop, type PixelCrop } from 'react-image-crop';
import 'react-image-crop/dist/ReactCrop.css';

type CropMode = 'none' | 'rect' | 'perspective';
type ActiveControl = 'brightness' | 'contrast' | null;
type Point = [number, number]; // normalized 0-1 relative to displayed image

interface CropModeHandle {
  getCroppedBlob(): Promise<Blob>;
  reset?(): void;
}

interface ImageCropperProps {
  imageSrc: string;
  onCropComplete: (blob: Blob) => void;
  onRotate?: () => void;
}

export function ImageCropper({ imageSrc, onCropComplete, onRotate }: ImageCropperProps) {
  const [mode, setMode] = useState<CropMode>('none');
  const [brightness, setBrightness] = useState(1);
  const [contrast, setContrast] = useState(1);
  const [activeControl, setActiveControl] = useState<ActiveControl>(null);
  const [processing, setProcessing] = useState(false);
  const [ready, setReady] = useState(true);
  const [cropError, setCropError] = useState<string | null>(null);
  // Result of an applied crop shown in preview before final confirm
  const [croppedSrc, setCroppedSrc] = useState<string | null>(null);
  const [croppedBlob, setCroppedBlob] = useState<Blob | null>(null);
  const croppedSrcRef = useRef<string | null>(null);
  croppedSrcRef.current = croppedSrc;
  const modeRef = useRef<CropModeHandle>(null);

  useEffect(() => () => {
    if (croppedSrcRef.current) URL.revokeObjectURL(croppedSrcRef.current);
  }, []);

  // When source image changes (rotation) discard previous crop result
  useEffect(() => {
    if (croppedSrcRef.current) {
      URL.revokeObjectURL(croppedSrcRef.current);
      croppedSrcRef.current = null;
    }
    setCroppedSrc(null);
    setCroppedBlob(null);
  }, [imageSrc]);

  const isCropActive = mode === 'rect' || mode === 'perspective';
  const displaySrc = mode === 'none' ? (croppedSrc ?? imageSrc) : imageSrc;

  const handleModeChange = (m: 'rect' | 'perspective') => {
    setCropError(null);
    if (mode === m) {
      setMode('none');
      setReady(true);
    } else {
      setMode(m);
      setReady(false);
      setActiveControl(null);
    }
  };

  const toggleControl = (c: 'brightness' | 'contrast') => {
    setActiveControl(prev => prev === c ? null : c);
  };

  // ✓ button: save crop result and stay in editor to review
  const handleApplyCrop = async () => {
    if (!isCropActive || !ready || processing || !modeRef.current) return;
    setProcessing(true);
    setCropError(null);
    try {
      const blob = await modeRef.current.getCroppedBlob();
      if (croppedSrcRef.current) URL.revokeObjectURL(croppedSrcRef.current);
      setCroppedBlob(blob);
      setCroppedSrc(URL.createObjectURL(blob));
      setMode('none');
      setActiveControl(null);
      setReady(true);
    } catch {
      setCropError('Не удалось применить обрезку. Попробуйте ещё раз.');
    } finally {
      setProcessing(false);
    }
  };

  // Далее: bake B/C onto current result (cropped or original) and proceed to OCR
  const handleProceed = async () => {
    if (processing) return;
    setProcessing(true);
    setCropError(null);
    try {
      let blob: Blob = croppedBlob ?? await fetch(imageSrc).then(r => r.blob());
      if (brightness !== 1 || contrast !== 1) {
        blob = await applyBrightnessContrast(blob, brightness, contrast);
      }
      onCropComplete(blob);
    } catch {
      setCropError('Не удалось обработать изображение. Попробуйте ещё раз.');
    } finally {
      setProcessing(false);
    }
  };

  const filterStyle = (brightness !== 1 || contrast !== 1)
    ? `brightness(${brightness}) contrast(${contrast})`
    : undefined;

  const activeValue = activeControl === 'brightness' ? brightness : contrast;
  const setActiveValue = (v: number) => {
    if (activeControl === 'brightness') setBrightness(v);
    else if (activeControl === 'contrast') setContrast(v);
  };
  const resetActiveValue = () => {
    if (activeControl === 'brightness') setBrightness(1);
    else if (activeControl === 'contrast') setContrast(1);
  };

  return (
    <div className="rounded-xl overflow-hidden border border-gray-200 flex flex-col">
      {/* Image / crop area */}
      <div
        className="bg-gray-900 flex items-center justify-center touch-manipulation overflow-hidden"
        style={{ minHeight: 240, maxHeight: '55vh' }}
      >
        {mode === 'none' && (
          <div className="overflow-hidden p-2 flex items-center justify-center">
            <img
              src={displaySrc}
              alt="Превью"
              className="max-w-full max-h-[55vh] w-auto h-auto block"
              style={filterStyle ? { filter: filterStyle } : undefined}
            />
          </div>
        )}
        {mode === 'rect' && (
          <RectCropMode
            ref={modeRef}
            key={`rect-${imageSrc}`}
            imageSrc={imageSrc}
            onReady={setReady}
            filterStyle={filterStyle}
          />
        )}
        {mode === 'perspective' && (
          <PerspectiveCropMode
            ref={modeRef}
            key={`perspective-${imageSrc}`}
            imageSrc={imageSrc}
            onReady={setReady}
            filterStyle={filterStyle}
          />
        )}
      </div>


      {/* Collapsible slider panel */}
      {activeControl && (
        <div className="flex items-center gap-3 px-4 py-2.5 bg-gray-50 border-t border-gray-200">
          <span className="text-xs font-medium text-gray-500 w-24 shrink-0">
            {activeControl === 'brightness' ? 'Яркость' : 'Контраст'}{' '}
            <span className="tabular-nums">{Math.round(activeValue * 100)}%</span>
          </span>
          <input
            type="range" min="0.5" max="2" step="0.05"
            value={activeValue}
            onChange={(e) => setActiveValue(parseFloat(e.target.value))}
            className="flex-1 accent-rose-600"
          />
          <button
            type="button"
            onClick={resetActiveValue}
            disabled={activeValue === 1}
            title="Сбросить"
            aria-label="Сбросить"
            className="w-8 h-8 rounded-full flex items-center justify-center text-gray-400 hover:bg-gray-200 hover:text-gray-700 disabled:opacity-30 disabled:cursor-default transition-colors shrink-0"
          >
            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
              <path d="M6 18 18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
      )}

      {/* Error message */}
      {cropError && (
        <div className="px-4 py-2 bg-red-50 border-t border-red-200 text-xs text-red-700">
          {cropError}
        </div>
      )}

      {/* Bottom toolbar */}
      <div className="bg-white border-t border-gray-200">
        <div className="flex items-center px-3 py-2 gap-1">
          {/* Crop mode icons */}
          <ToolIconButton active={mode === 'rect'} onClick={() => handleModeChange('rect')} label="Прямоугольник">
            <RectCropIcon />
          </ToolIconButton>
          <ToolIconButton active={mode === 'perspective'} onClick={() => handleModeChange('perspective')} label="Перспектива">
            <PerspectiveIcon />
          </ToolIconButton>
          {onRotate && (
            <ToolIconButton active={false} onClick={onRotate} label="Повернуть">
              <RotateIcon />
            </ToolIconButton>
          )}

          <div className="flex-1" />

          {/* Brightness / contrast — disabled while crop mode is active */}
          <ToolIconButton
            active={activeControl === 'brightness'}
            dot={brightness !== 1}
            onClick={() => toggleControl('brightness')}
            label="Яркость"
            disabled={isCropActive}
          >
            <BrightnessIcon />
          </ToolIconButton>
          <ToolIconButton
            active={activeControl === 'contrast'}
            dot={contrast !== 1}
            onClick={() => toggleControl('contrast')}
            label="Контраст"
            disabled={isCropActive}
          >
            <ContrastIcon />
          </ToolIconButton>

          <div className="w-px h-5 bg-gray-200 mx-1.5" />

          {/* Cancel crop — only shown when a crop mode is active */}
          {isCropActive && (
            <button
              type="button"
              onClick={() => { setMode('none'); setReady(true); }}
              className="w-10 h-10 rounded-full flex items-center justify-center text-gray-500 hover:bg-gray-100 active:bg-gray-200 transition-colors"
              aria-label="Выйти из режима обрезки"
            >
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M6 18 18 6M6 6l12 12" />
              </svg>
            </button>
          )}

          {/* ✓ Save crop — only shown when a crop mode is active */}
          {isCropActive && (
            <button
              type="button"
              onClick={handleApplyCrop}
              disabled={!ready || processing}
              className="w-10 h-10 rounded-full flex items-center justify-center bg-gray-700 text-white hover:bg-gray-800 active:bg-gray-900 disabled:opacity-50 transition-colors"
              aria-label="Сохранить обрезку"
              title="Сохранить обрезку"
            >
              {processing ? <Spinner /> : (
                <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="m4.5 12.75 6 6 9-13.5" />
                </svg>
              )}
            </button>
          )}

          {/* Далее → inline on sm+ */}
          <button
            type="button"
            onClick={handleProceed}
            disabled={processing || isCropActive}
            className="hidden sm:inline-flex items-center justify-center px-4 py-2 rounded-full bg-rose-600 text-white text-sm font-medium hover:bg-rose-700 active:bg-rose-800 disabled:opacity-50 transition-colors whitespace-nowrap"
          >
            {processing && !isCropActive ? <Spinner /> : 'Далее →'}
          </button>
        </div>

        {/* Далее → full-width on mobile */}
        <div className="sm:hidden px-3 pb-3">
          <button
            type="button"
            onClick={handleProceed}
            disabled={processing || isCropActive}
            className="w-full py-2.5 rounded-full bg-rose-600 text-white text-sm font-medium hover:bg-rose-700 active:bg-rose-800 disabled:opacity-50 transition-colors"
          >
            {processing && !isCropActive ? <Spinner /> : 'Далее →'}
          </button>
        </div>
      </div>
    </div>
  );
}

// ─── Tool icon button ──────────────────────────────────────────────────────────

function ToolIconButton({
  active, dot = false, disabled = false, onClick, label, children,
}: {
  active: boolean;
  dot?: boolean;
  disabled?: boolean;
  onClick: () => void;
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className="relative">
      <button
        type="button"
        onClick={onClick}
        title={label}
        aria-label={label}
        disabled={disabled}
        className={`w-10 h-10 rounded-full flex items-center justify-center transition-all ${
          disabled
            ? 'text-gray-300 cursor-default'
            : active
              ? 'bg-rose-600 text-white shadow-sm'
              : 'text-gray-500 hover:bg-gray-100 active:bg-gray-200'
        }`}
      >
        {children}
      </button>
      {dot && !active && (
        <span className="absolute top-1.5 right-1.5 w-1.5 h-1.5 rounded-full bg-rose-500 pointer-events-none" />
      )}
    </div>
  );
}

// ─── Rectangular crop ──────────────────────────────────────────────────────────

const RectCropMode = forwardRef<CropModeHandle, {
  imageSrc: string;
  onReady: (v: boolean) => void;
  filterStyle?: string;
}>(function RectCropMode({ imageSrc, onReady, filterStyle }, ref) {
  const [crop, setCrop] = useState<Crop>();
  const [completedCrop, setCompletedCrop] = useState<PixelCrop>();
  const imgRef = useRef<HTMLImageElement>(null);

  useEffect(() => { onReady(!!completedCrop); }, [completedCrop, onReady]);

  useImperativeHandle(ref, () => ({
    async getCroppedBlob() {
      if (!completedCrop || !imgRef.current) throw new Error('Not ready');
      return getRectCroppedBlob(imgRef.current, completedCrop);
    },
  }), [completedCrop]);

  const onImageLoad = useCallback((e: React.SyntheticEvent<HTMLImageElement>) => {
    const { width, height } = e.currentTarget;
    setCrop({ unit: '%', x: 5, y: 5, width: 90, height: 90 });
    setCompletedCrop({
      unit: 'px',
      x: Math.round(width * 0.05),
      y: Math.round(height * 0.05),
      width: Math.round(width * 0.9),
      height: Math.round(height * 0.9),
    });
  }, []);

  return (
    <ReactCrop
      crop={crop}
      onChange={setCrop}
      onComplete={setCompletedCrop}
      style={{ maxWidth: 'calc(100% - 1rem)', maxHeight: 'calc(55vh - 1rem)', margin: '0.5rem' }}
    >
      <img
        ref={imgRef}
        src={imageSrc}
        alt="Обрезка"
        onLoad={onImageLoad}
        style={{
          display: 'block',
          maxWidth: '100%',
          maxHeight: 'calc(55vh - 1rem)',
          width: 'auto',
          height: 'auto',
          ...(filterStyle ? { filter: filterStyle } : {}),
        }}
      />
    </ReactCrop>
  );
});

// ─── Perspective crop (4 draggable corners) ────────────────────────────────────

const DEFAULT_CORNERS: Point[] = [[0.05, 0.05], [0.95, 0.05], [0.95, 0.95], [0.05, 0.95]];

const PerspectiveCropMode = forwardRef<CropModeHandle, {
  imageSrc: string;
  onReady: (v: boolean) => void;
  filterStyle?: string;
}>(function PerspectiveCropMode({ imageSrc, onReady, filterStyle }, ref) {
  const imgRef = useRef<HTMLImageElement>(null);
  const [imgLoaded, setImgLoaded] = useState(false);
  const [corners, setCorners] = useState<Point[]>(DEFAULT_CORNERS);
  const [activeCorner, setActiveCorner] = useState<number | null>(null);
  const cornersRef = useRef(corners);
  cornersRef.current = corners;

  useEffect(() => { onReady(imgLoaded); }, [imgLoaded, onReady]);

  useImperativeHandle(ref, () => ({
    async getCroppedBlob() {
      const img = imgRef.current;
      if (!img) throw new Error('Not ready');
      const nw = img.naturalWidth;
      const nh = img.naturalHeight;
      const naturalCorners = cornersRef.current.map(([x, y]) => [x * nw, y * nh] as Point);
      const canvas = warpPerspective(img, naturalCorners);
      return new Promise<Blob>((resolve, reject) => {
        canvas.toBlob(b => b ? resolve(b) : reject(new Error('toBlob failed')), 'image/jpeg', 0.92);
      });
    },
    reset() { setCorners(DEFAULT_CORNERS); },
  }), []);

  useEffect(() => {
    if (activeCorner === null) return;
    const onMove = (e: PointerEvent) => {
      const imgEl = imgRef.current;
      if (!imgEl) return;
      const r = imgEl.getBoundingClientRect();
      const x = Math.max(0, Math.min(1, (e.clientX - r.left) / r.width));
      const y = Math.max(0, Math.min(1, (e.clientY - r.top) / r.height));
      setCorners(prev => {
        const next = [...prev] as Point[];
        next[activeCorner] = [x, y];
        return next;
      });
    };
    const onUp = () => setActiveCorner(null);
    document.addEventListener('pointermove', onMove);
    document.addEventListener('pointerup', onUp);
    return () => {
      document.removeEventListener('pointermove', onMove);
      document.removeEventListener('pointerup', onUp);
    };
  }, [activeCorner]);

  const svgPoints = corners.map(([x, y]) => `${x},${y}`).join(' ');

  return (
    <div className="relative m-2" style={{ display: 'inline-block', maxWidth: '100%', maxHeight: 'calc(55vh - 16px)' }}>
      <img
        ref={imgRef}
        src={imageSrc}
        alt="Перспектива"
        onLoad={() => setImgLoaded(true)}
        draggable={false}
        className="block max-w-full select-none"
        style={{
          maxHeight: 'calc(55vh - 16px)',
          userSelect: 'none',
          ...(filterStyle ? { filter: filterStyle } : {}),
        }}
      />
      {imgLoaded && (
        <>
          <svg
            className="absolute inset-0 w-full h-full pointer-events-none"
            viewBox="0 0 1 1"
            preserveAspectRatio="none"
          >
            <polygon
              points={svgPoints}
              fill="rgba(244,63,94,0.12)"
              stroke="#f43f5e"
              strokeWidth="0.004"
              vectorEffect="non-scaling-stroke"
            />
          </svg>
          {corners.map(([x, y], i) => (
            <div
              key={i}
              onPointerDown={(e) => { e.preventDefault(); setActiveCorner(i); }}
              style={{
                position: 'absolute',
                left: `${x * 100}%`,
                top: `${y * 100}%`,
                transform: 'translate(-50%, -50%)',
                width: 22,
                height: 22,
                borderRadius: '50%',
                background: '#f43f5e',
                border: '2.5px solid white',
                boxShadow: '0 1px 6px rgba(0,0,0,0.5)',
                cursor: 'grab',
                touchAction: 'none',
                zIndex: 10,
              }}
            />
          ))}
        </>
      )}
    </div>
  );
});

// ─── Icon components ───────────────────────────────────────────────────────────

function Spinner() {
  return (
    <svg className="animate-spin w-4 h-4" fill="none" viewBox="0 0 24 24">
      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
    </svg>
  );
}

function RectCropIcon() {
  return (
    <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2} strokeLinecap="round">
      <path d="M4 9V6a2 2 0 0 1 2-2h3M15 4h3a2 2 0 0 1 2 2v3M20 15v3a2 2 0 0 1-2 2h-3M9 20H6a2 2 0 0 1-2-2v-3" />
    </svg>
  );
}

function PerspectiveIcon() {
  return (
    <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
      <path d="M4 20h16L17 4H7L4 20Z" />
    </svg>
  );
}


function RotateIcon() {
  return (
    <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
      <polyline points="23 4 23 10 17 10" />
      <path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10" />
    </svg>
  );
}

function BrightnessIcon() {
  return (
    <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.8} strokeLinecap="round">
      <circle cx="12" cy="12" r="4" />
      <path d="M12 2v2M12 20v2M4.22 4.22l1.42 1.42M18.36 18.36l1.42 1.42M2 12h2M20 12h2M4.22 19.78l1.42-1.42M18.36 5.64l1.42-1.42" />
    </svg>
  );
}

function ContrastIcon() {
  return (
    <svg className="w-4 h-4" viewBox="0 0 24 24" strokeLinecap="round">
      <circle cx="12" cy="12" r="9" stroke="currentColor" strokeWidth={1.8} fill="none" />
      <path d="M12 3a9 9 0 0 0 0 18V3Z" fill="currentColor" />
    </svg>
  );
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

async function applyBrightnessContrast(blob: Blob, brightness: number, contrast: number): Promise<Blob> {
  return new Promise((resolve, reject) => {
    const img = new Image();
    const url = URL.createObjectURL(blob);
    img.onload = () => {
      URL.revokeObjectURL(url);
      const canvas = document.createElement('canvas');
      canvas.width = img.naturalWidth;
      canvas.height = img.naturalHeight;
      const ctx = canvas.getContext('2d');
      if (!ctx) { reject(new Error('No canvas context')); return; }
      ctx.filter = `brightness(${brightness}) contrast(${contrast})`;
      ctx.drawImage(img, 0, 0);
      canvas.toBlob(
        (b) => (b ? resolve(b) : reject(new Error('toBlob failed'))),
        'image/jpeg',
        0.92,
      );
    };
    img.onerror = () => { URL.revokeObjectURL(url); reject(new Error('Failed to load image')); };
    img.src = url;
  });
}

async function getRectCroppedBlob(image: HTMLImageElement, crop: PixelCrop): Promise<Blob> {
  const sx = image.naturalWidth / image.width;
  const sy = image.naturalHeight / image.height;
  const canvas = document.createElement('canvas');
  canvas.width = Math.round(crop.width * sx);
  canvas.height = Math.round(crop.height * sy);
  const ctx = canvas.getContext('2d');
  if (!ctx) throw new Error('No context');
  ctx.drawImage(image, crop.x * sx, crop.y * sy, canvas.width, canvas.height, 0, 0, canvas.width, canvas.height);
  return new Promise((resolve, reject) => {
    canvas.toBlob((b) => (b ? resolve(b) : reject(new Error('toBlob failed'))), 'image/jpeg', 0.92);
  });
}

// ─── Perspective warp ─────────────────────────────────────────────────────────

function solve8x8(A: number[][], b: number[]): number[] {
  const n = 8;
  const M = A.map((row, i) => [...row, b[i]]);
  for (let col = 0; col < n; col++) {
    let maxRow = col;
    for (let row = col + 1; row < n; row++) {
      if (Math.abs(M[row][col]) > Math.abs(M[maxRow][col])) maxRow = row;
    }
    [M[col], M[maxRow]] = [M[maxRow], M[col]];
    for (let row = col + 1; row < n; row++) {
      const f = M[row][col] / M[col][col];
      for (let k = col; k <= n; k++) M[row][k] -= f * M[col][k];
    }
  }
  const x = new Array<number>(n).fill(0);
  for (let row = n - 1; row >= 0; row--) {
    x[row] = M[row][n];
    for (let k = row + 1; k < n; k++) x[row] -= M[row][k] * x[k];
    x[row] /= M[row][row];
  }
  return x;
}

function computeInvH(srcPts: Point[], dstPts: Point[]): number[] {
  const A: number[][] = [];
  const b: number[] = [];
  for (let i = 0; i < 4; i++) {
    const [dx, dy] = dstPts[i];
    const [sx, sy] = srcPts[i];
    A.push([dx, dy, 1, 0, 0, 0, -sx * dx, -sx * dy]); b.push(sx);
    A.push([0, 0, 0, dx, dy, 1, -sy * dx, -sy * dy]); b.push(sy);
  }
  const h = solve8x8(A, b);
  return [...h, 1];
}

function applyH(H: number[], x: number, y: number): Point {
  const w = H[6] * x + H[7] * y + H[8];
  return [(H[0] * x + H[1] * y + H[2]) / w, (H[3] * x + H[4] * y + H[5]) / w];
}

const MAX_WARP_PX = 2048;

function warpPerspective(img: HTMLImageElement, corners: Point[]): HTMLCanvasElement {
  const [tl, tr, br, bl] = corners;
  const rawW = Math.max(Math.hypot(tr[0] - tl[0], tr[1] - tl[1]), Math.hypot(br[0] - bl[0], br[1] - bl[1]));
  const rawH = Math.max(Math.hypot(bl[0] - tl[0], bl[1] - tl[1]), Math.hypot(br[0] - tr[0], br[1] - tr[1]));
  const scale = Math.min(1, MAX_WARP_PX / Math.max(rawW, rawH));
  const W = Math.round(rawW * scale);
  const H = Math.round(rawH * scale);

  const dstPts: Point[] = [[0, 0], [W, 0], [W, H], [0, H]];
  const scaledCorners: Point[] = corners.map(([x, y]) => [x * scale, y * scale]);
  const H_inv = computeInvH(scaledCorners, dstPts);

  const srcCanvas = document.createElement('canvas');
  srcCanvas.width = Math.round(img.naturalWidth * scale);
  srcCanvas.height = Math.round(img.naturalHeight * scale);
  const srcCtx = srcCanvas.getContext('2d');
  if (!srcCtx) throw new Error('No canvas context');
  srcCtx.drawImage(img, 0, 0, srcCanvas.width, srcCanvas.height);
  const srcData = srcCtx.getImageData(0, 0, srcCanvas.width, srcCanvas.height);

  const dstCanvas = document.createElement('canvas');
  dstCanvas.width = W;
  dstCanvas.height = H;
  const dstCtx = dstCanvas.getContext('2d');
  if (!dstCtx) throw new Error('No canvas context');
  const dstData = dstCtx.createImageData(W, H);

  const sw = srcCanvas.width;
  const sh = srcCanvas.height;

  for (let y = 0; y < H; y++) {
    for (let x = 0; x < W; x++) {
      const [sx, sy] = applyH(H_inv, x, y);
      const x0 = Math.floor(sx), y0 = Math.floor(sy);
      const x1 = x0 + 1, y1 = y0 + 1;
      if (x0 < 0 || y0 < 0 || x1 >= sw || y1 >= sh) continue;
      const fx = sx - x0, fy = sy - y0;
      const di = (y * W + x) * 4;
      for (let c = 0; c < 4; c++) {
        const v00 = srcData.data[(y0 * sw + x0) * 4 + c];
        const v10 = srcData.data[(y0 * sw + x1) * 4 + c];
        const v01 = srcData.data[(y1 * sw + x0) * 4 + c];
        const v11 = srcData.data[(y1 * sw + x1) * 4 + c];
        dstData.data[di + c] = Math.round(v00 * (1 - fx) * (1 - fy) + v10 * fx * (1 - fy) + v01 * (1 - fx) * fy + v11 * fx * fy);
      }
    }
  }

  dstCtx.putImageData(dstData, 0, 0);
  return dstCanvas;
}
