import { useCallback, useEffect, useRef, useState } from 'react';
import { requestAPI } from '@/services/api';

function isSafeImageURL(url: string): boolean {
  if (url.startsWith('blob:')) return true;
  try {
    const parsed = new URL(url);
    return parsed.protocol === 'https:' || parsed.protocol === 'http:';
  } catch {
    return false;
  }
}

export function RequestImage({ requestId, fileId }: { requestId: string; fileId: string }) {
  const [src, setSrc] = useState<string | null>(null);
  const [showModal, setShowModal] = useState(false);
  const [blobFallbackTried, setBlobFallbackTried] = useState(false);
  const [loadFailed, setLoadFailed] = useState(false);
  const objectUrlRef = useRef<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setSrc(null);
    setBlobFallbackTried(false);
    setLoadFailed(false);
    if (objectUrlRef.current) {
      URL.revokeObjectURL(objectUrlRef.current);
      objectUrlRef.current = null;
    }

    const loadBlobFallback = async () => {
      try {
        const blobURL = await requestAPI.getFileURL(requestId, fileId);
        if (cancelled) {
          URL.revokeObjectURL(blobURL);
          return;
        }
        objectUrlRef.current = blobURL;
        setBlobFallbackTried(true);
        setLoadFailed(false);
        setSrc(blobURL);
      } catch {
        if (!cancelled) {
          setLoadFailed(true);
        }
      }
    };

    const load = async () => {
      try {
        const directURL = await requestAPI.getFileDirectURL(requestId, fileId);
        if (!cancelled && isSafeImageURL(directURL)) {
          setLoadFailed(false);
          setSrc(directURL);
        }
        return;
      } catch {
        // Fallback for storage backends that do not support direct URLs.
      }

      await loadBlobFallback();
    };

    void load();

    return () => {
      cancelled = true;
      if (objectUrlRef.current) {
        URL.revokeObjectURL(objectUrlRef.current);
        objectUrlRef.current = null;
      }
    };
  }, [requestId, fileId]);

  const handleImageError = useCallback(async () => {
    if (blobFallbackTried) return;

    try {
      const blobURL = await requestAPI.getFileURL(requestId, fileId);
      setBlobFallbackTried(true);
      if (objectUrlRef.current) {
        URL.revokeObjectURL(objectUrlRef.current);
      }
      objectUrlRef.current = blobURL;
      setLoadFailed(false);
      setSrc(blobURL);
    } catch {
      setBlobFallbackTried(true);
      setLoadFailed(true);
    }
  }, [blobFallbackTried, fileId, requestId]);

  if (!src && !loadFailed) return (
    <div className="bg-white shadow rounded-lg p-4 sm:p-6 mb-4 sm:mb-6">
      <div className="h-6 w-48 bg-gray-200 rounded mb-4 animate-pulse" />
      <div className="h-48 bg-gray-200 rounded animate-pulse" />
    </div>
  );

  if (loadFailed || !src) {
    return (
      <div className="bg-white shadow rounded-lg p-4 sm:p-6 mb-4 sm:mb-6">
        <h2 className="text-lg sm:text-xl font-bold text-gray-900 mb-3 sm:mb-4">Исходное изображение</h2>
        <div className="rounded-xl border border-amber-200 bg-amber-50 px-4 py-6 text-sm text-amber-800">
          Не удалось загрузить изображение ЭКГ.
        </div>
      </div>
    );
  }

  return (
    <>
      <div className="bg-white shadow rounded-lg p-4 sm:p-6 mb-4 sm:mb-6">
        <h2 className="text-lg sm:text-xl font-bold text-gray-900 mb-3 sm:mb-4">Исходное изображение</h2>
        <div className="flex justify-center">
          <img
            src={src}
            alt="Исходное ЭКГ изображение"
            decoding="async"
            className="max-w-full h-auto rounded-lg shadow-md cursor-pointer hover:opacity-90 transition-opacity"
            onError={() => { void handleImageError(); }}
            onLoad={() => setLoadFailed(false)}
            onClick={() => setShowModal(true)}
          />
        </div>
      </div>

      {showModal && (
        <ImageModal
          src={src}
          onClose={() => setShowModal(false)}
          onImageError={() => { void handleImageError(); }}
          onImageLoad={() => setLoadFailed(false)}
        />
      )}
    </>
  );
}

// ---------------------------------------------------------------------------
// Full-screen image viewer with pinch-to-zoom, pan, and double-tap support
// ---------------------------------------------------------------------------

interface ImageModalProps {
  src: string;
  onClose: () => void;
  onImageError: () => void;
  onImageLoad: () => void;
}

function ImageModal({ src, onClose, onImageError, onImageLoad }: ImageModalProps) {
  const [scale, setScale] = useState(1);
  const [translate, setTranslate] = useState({ x: 0, y: 0 });
  const [animating, setAnimating] = useState(false);

  const containerRef = useRef<HTMLDivElement>(null);

  // Touch state refs (not in state to avoid re-renders during gestures)
  const pinchStartDist = useRef<number | null>(null);
  const pinchBaseScale = useRef(1);
  const panStart = useRef<{ x: number; y: number } | null>(null);
  const panBaseTranslate = useRef({ x: 0, y: 0 });
  const lastTapRef = useRef(0);

  // Prevent page scroll while modal is open
  useEffect(() => {
    const prev = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    return () => { document.body.style.overflow = prev; };
  }, []);

  const resetView = useCallback(() => {
    setAnimating(true);
    setScale(1);
    setTranslate({ x: 0, y: 0 });
    setTimeout(() => setAnimating(false), 200);
  }, []);

  const zoomTo = useCallback((newScale: number) => {
    setAnimating(true);
    const clamped = Math.min(5, Math.max(0.25, newScale));
    if (clamped <= 1) {
      setScale(clamped);
      setTranslate({ x: 0, y: 0 });
    } else {
      setScale(clamped);
    }
    setTimeout(() => setAnimating(false), 200);
  }, []);

  const handleTouchStart = useCallback((e: React.TouchEvent) => {
    if (e.touches.length === 2) {
      // Pinch start
      pinchStartDist.current = Math.hypot(
        e.touches[0].clientX - e.touches[1].clientX,
        e.touches[0].clientY - e.touches[1].clientY,
      );
      pinchBaseScale.current = scale;
      panStart.current = null; // cancel any pan
    } else if (e.touches.length === 1) {
      // Double-tap detection
      const now = Date.now();
      if (now - lastTapRef.current < 300) {
        e.preventDefault();
        if (scale > 1.1) {
          resetView();
        } else {
          zoomTo(2.5);
        }
        lastTapRef.current = 0;
        return;
      }
      lastTapRef.current = now;

      // Pan start (only when zoomed in)
      if (scale > 1) {
        panStart.current = { x: e.touches[0].clientX, y: e.touches[0].clientY };
        panBaseTranslate.current = { ...translate };
      }
    }
  }, [scale, translate, resetView, zoomTo]);

  const handleTouchMove = useCallback((e: React.TouchEvent) => {
    if (e.touches.length === 2 && pinchStartDist.current !== null) {
      // Pinch zoom
      e.preventDefault();
      const dist = Math.hypot(
        e.touches[0].clientX - e.touches[1].clientX,
        e.touches[0].clientY - e.touches[1].clientY,
      );
      const newScale = Math.min(5, Math.max(0.25, pinchBaseScale.current * (dist / pinchStartDist.current)));
      setAnimating(false);
      setScale(newScale);
      if (newScale <= 1) {
        setTranslate({ x: 0, y: 0 });
      }
    } else if (e.touches.length === 1 && panStart.current && scale > 1) {
      // Pan
      e.preventDefault();
      const dx = e.touches[0].clientX - panStart.current.x;
      const dy = e.touches[0].clientY - panStart.current.y;
      setAnimating(false);
      setTranslate({
        x: panBaseTranslate.current.x + dx,
        y: panBaseTranslate.current.y + dy,
      });
    }
  }, [scale]);

  const handleTouchEnd = useCallback(() => {
    pinchStartDist.current = null;
    panStart.current = null;

    // Snap back to 1x if close enough
    if (scale < 1) {
      resetView();
    }
  }, [scale, resetView]);

  return (
    <div
      className="fixed inset-0 z-50 bg-black/75 backdrop-blur-sm animate-fade-in"
      onClick={onClose}
    >
      {/* Toolbar */}
      <div className="absolute top-0 left-0 right-0 z-10 flex items-center justify-between px-4 py-3 bg-gradient-to-b from-black/60 to-transparent">
        <span className="text-white/70 text-sm font-medium">
          {Math.round(scale * 100)}%
        </span>
        <div className="flex items-center gap-1.5">
          <button
            onClick={(e) => { e.stopPropagation(); zoomTo(scale - 0.25); }}
            className="w-9 h-9 rounded-lg bg-white/10 text-white/80 hover:bg-white/20 hover:text-white flex items-center justify-center transition-all backdrop-blur-md"
          >
            <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M5 12h14" />
            </svg>
          </button>
          <button
            onClick={(e) => { e.stopPropagation(); resetView(); }}
            className="h-9 px-3 rounded-lg bg-white/10 text-white/80 hover:bg-white/20 hover:text-white text-xs font-medium transition-all backdrop-blur-md"
          >
            Сброс
          </button>
          <button
            onClick={(e) => { e.stopPropagation(); zoomTo(scale + 0.25); }}
            className="w-9 h-9 rounded-lg bg-white/10 text-white/80 hover:bg-white/20 hover:text-white flex items-center justify-center transition-all backdrop-blur-md"
          >
            <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M12 5v14m-7-7h14" />
            </svg>
          </button>
          <div className="w-px h-5 bg-white/20 mx-1" />
          <button
            onClick={onClose}
            className="w-9 h-9 rounded-lg bg-white/10 text-white/80 hover:bg-red-500/80 hover:text-white flex items-center justify-center transition-all backdrop-blur-md"
          >
            <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
      </div>

      {/* Image area */}
      <div
        ref={containerRef}
        className="h-full w-full flex items-center justify-center pt-14"
        style={{ touchAction: 'none' }}
        onClick={(e) => e.stopPropagation()}
        onDoubleClick={(e) => {
          e.stopPropagation();
          if (scale > 1.1) resetView();
          else zoomTo(2.5);
        }}
        onTouchStart={handleTouchStart}
        onTouchMove={handleTouchMove}
        onTouchEnd={handleTouchEnd}
      >
        <img
          src={src}
          alt="ЭКГ"
          decoding="async"
          style={{
            transform: `translate(${translate.x}px, ${translate.y}px) scale(${scale})`,
            transformOrigin: 'center center',
            transition: animating ? 'transform 200ms ease-out' : 'none',
          }}
          className="max-w-full max-h-full select-none"
          onError={onImageError}
          onLoad={onImageLoad}
          draggable={false}
        />
      </div>
    </div>
  );
}
