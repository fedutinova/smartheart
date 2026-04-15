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
  const [scale, setScale] = useState(1);
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
            onClick={() => { setScale(1); setShowModal(true); }}
          />
        </div>
      </div>

      {showModal && (
        <div
          className="fixed inset-0 z-50 bg-black/75 backdrop-blur-sm animate-fade-in"
          onClick={() => setShowModal(false)}
        >
          {/* Toolbar */}
          <div className="absolute top-0 left-0 right-0 z-10 flex items-center justify-between px-4 py-3 bg-gradient-to-b from-black/60 to-transparent">
            <span className="text-white/70 text-sm font-medium">
              {Math.round(scale * 100)}%
            </span>
            <div className="flex items-center gap-1.5">
              <button
                onClick={(e) => { e.stopPropagation(); setScale((s) => Math.max(s - 0.25, 0.25)); }}
                className="w-9 h-9 rounded-lg bg-white/10 text-white/80 hover:bg-white/20 hover:text-white flex items-center justify-center transition-all backdrop-blur-md"
              >
                <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M5 12h14" />
                </svg>
              </button>
              <button
                onClick={(e) => { e.stopPropagation(); setScale(1); }}
                className="h-9 px-3 rounded-lg bg-white/10 text-white/80 hover:bg-white/20 hover:text-white text-xs font-medium transition-all backdrop-blur-md"
              >
                Сброс
              </button>
              <button
                onClick={(e) => { e.stopPropagation(); setScale((s) => Math.min(s + 0.25, 5)); }}
                className="w-9 h-9 rounded-lg bg-white/10 text-white/80 hover:bg-white/20 hover:text-white flex items-center justify-center transition-all backdrop-blur-md"
              >
                <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M12 5v14m-7-7h14" />
                </svg>
              </button>
              <div className="w-px h-5 bg-white/20 mx-1" />
              <button
                onClick={() => setShowModal(false)}
                className="w-9 h-9 rounded-lg bg-white/10 text-white/80 hover:bg-red-500/80 hover:text-white flex items-center justify-center transition-all backdrop-blur-md"
              >
                <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>
          </div>

          {/* Image */}
          <div
            className="h-full w-full overflow-auto flex items-center justify-center pt-14"
            onClick={(e) => e.stopPropagation()}
            onDoubleClick={(e) => { e.stopPropagation(); setScale((s) => s < 2 ? 2 : 1); }}
          >
            <img
              src={src}
              alt="ЭКГ"
              decoding="async"
              style={{ transform: `scale(${scale})`, transformOrigin: 'center center' }}
              className="transition-transform duration-200 ease-out select-none"
              onError={() => { void handleImageError(); }}
              onLoad={() => setLoadFailed(false)}
              draggable={false}
            />
          </div>
        </div>
      )}
    </>
  );
}
