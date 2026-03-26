import { useState, useCallback, useRef } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { ekgAPI } from '@/services/api';
import { ROUTES } from '@/config';
import { Layout } from '@/components/Layout';
import { ImageCropper } from '@/components/ImageCropper';
import { PaymentModal } from '@/components/PaymentModal';
import { useDraft } from '@/hooks/useDraft';
import { usePendingJobs } from '@/hooks/usePendingJobs';
import { useQuota } from '@/hooks/useQuota';
import type { ECGCalibrationParams } from '@/types';

type Mode = 'file' | 'camera' | 'url';
type Step = 'select' | 'crop' | 'ready';

export function Analyze() {
  const [mode, setMode] = useState<Mode>('file');
  const [step, setStep] = useState<Step>('select');
  const [, , clearNotes] = useDraft('analyze_notes');
  const [error, setError] = useState('');
  const [showPayment, setShowPayment] = useState(false);
  const navigate = useNavigate();
  const { addJob } = usePendingJobs();
  const { quota, refetch: refetchQuota } = useQuota();
  const queryClient = useQueryClient();

  // Calibration params
  const [age, setAge] = useState<string>('');
  const [sex, setSex] = useState<string>('');
  const [paperSpeed, setPaperSpeed] = useState<number>(25);
  const [mmPerMvLimb, setMmPerMvLimb] = useState<number>(10);
  const [mmPerMvChest, setMmPerMvChest] = useState<number>(10);

  // File/camera mode state
  const [previewSrc, setPreviewSrc] = useState<string | null>(null);
  const [croppedBlob, setCroppedBlob] = useState<Blob | null>(null);
  const [croppedPreview, setCroppedPreview] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const cameraInputRef = useRef<HTMLInputElement>(null);

  // URL mode state — persisted as draft
  const [imageUrl, setImageUrl, clearImageUrl] = useDraft('analyze_url');

  const getCalibrationParams = (): ECGCalibrationParams => ({
    age: age ? parseInt(age, 10) : undefined,
    sex: sex || undefined,
    paper_speed_mms: paperSpeed,
    mm_per_mv_limb: mmPerMvLimb,
    mm_per_mv_chest: mmPerMvChest,
  });

  const mutation = useMutation({
    mutationFn: () => {
      const params = getCalibrationParams();
      if ((mode === 'file' || mode === 'camera') && croppedBlob) {
        return ekgAPI.submitAnalysisFile(croppedBlob, undefined, params);
      }
      return ekgAPI.submitAnalysis({
        image_temp_url: imageUrl,
        ...params,
      });
    },
    onSuccess: (response) => {
      clearNotes();
      clearImageUrl();
      addJob(response.request_id);
      queryClient.invalidateQueries({ queryKey: ['quota'] });
      navigate(`/results/${response.request_id}`);
    },
    onError: (err: any) => {
      if (err.response?.status === 402) {
        setShowPayment(true);
        return;
      }
      setError(err.response?.data?.error || 'Ошибка при отправке анализа');
    },
  });

  const compressImage = useCallback((file: File): Promise<string> => {
    return new Promise((resolve, reject) => {
      const img = new Image();
      img.onload = () => {
        const MAX_PIXELS = 4096;
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
      img.onerror = () => reject(new Error('Не удалось загрузить изображение'));
      img.src = URL.createObjectURL(file);
    });
  }, []);

  const setBlobFromSrc = useCallback(async (src: string) => {
    const res = await fetch(src);
    const blob = await res.blob();
    setCroppedBlob(blob);
    setCroppedPreview(src);
  }, []);

  const handleFileSelect = useCallback(async (file: File) => {
    if (!file.type.startsWith('image/') && file.type !== 'application/pdf') {
      setError('Поддерживаются только изображения и PDF');
      return;
    }
    if (file.size > 10 * 1024 * 1024) {
      if (mode === 'camera' && file.type.startsWith('image/')) {
        try {
          const compressedUrl = await compressImage(file);
          setError('');
          setPreviewSrc(compressedUrl);
          await setBlobFromSrc(compressedUrl);
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
  }, [mode, compressImage, setBlobFromSrc]);

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) handleFileSelect(file);
  };

  const handleDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault();
      const file = e.dataTransfer.files[0];
      if (file) handleFileSelect(file);
    },
    [handleFileSelect],
  );

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault();
  };

  const handleCropComplete = (blob: Blob) => {
    setCroppedBlob(blob);
    const url = URL.createObjectURL(blob);
    setCroppedPreview(url);
    setStep('ready');
  };

  const handleCropCancel = () => {
    // Return to ready with original (uncropped) image
    if (previewSrc) {
      setCroppedPreview(previewSrc);
      setBlobFromSrc(previewSrc);
    }
    setStep('ready');
  };

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

  const handleRecrop = () => {
    if (croppedPreview && croppedPreview !== previewSrc) URL.revokeObjectURL(croppedPreview);
    setCroppedBlob(null);
    setCroppedPreview(null);
    setStep('crop');
  };

  const handleReset = () => {
    if (previewSrc) URL.revokeObjectURL(previewSrc);
    if (croppedPreview) URL.revokeObjectURL(croppedPreview);
    setPreviewSrc(null);
    setCroppedBlob(null);
    setCroppedPreview(null);
    setStep('select');
    setError('');
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if ((mode === 'file' || mode === 'camera') && !croppedBlob) {
      setError(mode === 'camera' ? 'Сделайте фото и обрежьте изображение' : 'Выберите и обрежьте изображение');
      return;
    }
    if (mode === 'url' && !imageUrl.trim()) {
      setError('Введите URL изображения');
      return;
    }
    setError('');
    mutation.mutate();
  };

  const switchMode = (newMode: Mode) => {
    handleReset();
    clearImageUrl();
    setMode(newMode);
  };

  const canSubmit =
    (mode === 'file' || mode === 'camera')
      ? step === 'ready' && croppedBlob !== null
      : imageUrl.trim() !== '';

  return (
    <Layout>
      {showPayment && quota && (
        <PaymentModal
          quota={quota}
          onClose={() => setShowPayment(false)}
          onSuccess={() => { refetchQuota(); setShowPayment(false); }}
        />
      )}
      <div className="max-w-4xl mx-auto">
        <h1 className="text-2xl font-semibold text-gray-900 mb-6">Анализ ЭКГ</h1>

        {/* Quota */}
        {quota && (
          <div className="mb-4 flex items-center justify-between text-sm">
            <div className="flex items-center gap-3 text-gray-400">
              <span className="flex items-center gap-1.5">
                <span className={`inline-block w-1.5 h-1.5 rounded-full ${quota.needs_payment ? 'bg-amber-400' : 'bg-green-400'}`} />
                {quota.needs_payment
                  ? 'Лимит исчерпан'
                  : `${quota.free_remaining} из ${quota.daily_limit} бесплатных`}
              </span>
              {quota.paid_analyses_remaining > 0 && (
                <span className="text-rose-500">+{quota.paid_analyses_remaining} оплач.</span>
              )}
            </div>
            {quota.needs_payment && (
              <button
                type="button"
                onClick={() => setShowPayment(true)}
                className="text-xs text-rose-600 hover:text-rose-700 font-medium"
              >
                Купить
              </button>
            )}
          </div>
        )}

        <div className="bg-white shadow rounded-lg p-4 sm:p-6">
          <form onSubmit={handleSubmit} className="space-y-5 sm:space-y-6">
            {error && (
              <div className="bg-red-50 border border-red-200 text-red-800 px-4 py-3 rounded text-sm">
                {error}
              </div>
            )}

            {/* Image source */}
            {step === 'select' && (
              <div className="space-y-3">
                <input
                  ref={fileInputRef}
                  type="file"
                  accept="image/*,application/pdf"
                  onChange={handleInputChange}
                  className="hidden"
                />
                <input
                  ref={cameraInputRef}
                  type="file"
                  accept="image/*"
                  capture="environment"
                  onChange={handleInputChange}
                  className="hidden"
                />

                {/* Mobile: split card — Файл | Камера */}
                <div className="sm:hidden rounded-xl border border-gray-200 overflow-hidden">
                  <div className="grid grid-cols-2 divide-x divide-gray-200">
                    <button
                      type="button"
                      onClick={() => fileInputRef.current?.click()}
                      className="flex flex-col items-center gap-2 py-8 hover:bg-rose-50 active:bg-rose-100 transition-colors group"
                    >
                      <svg className="w-7 h-7 text-gray-400 group-hover:text-rose-500 transition-colors" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                        <path strokeLinecap="round" strokeLinejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 0 0-3.375-3.375h-1.5A1.125 1.125 0 0 1 13.5 7.125v-1.5a3.375 3.375 0 0 0-3.375-3.375H8.25m2.25 0H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 0 0-9-9Z" />
                      </svg>
                      <span className="text-xs text-gray-600 group-hover:text-rose-600 font-medium transition-colors">Файл</span>
                    </button>
                    <button
                      type="button"
                      onClick={() => cameraInputRef.current?.click()}
                      className="flex flex-col items-center gap-2 py-8 hover:bg-rose-50 active:bg-rose-100 transition-colors group"
                    >
                      <svg className="w-7 h-7 text-gray-400 group-hover:text-rose-500 transition-colors" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                        <path strokeLinecap="round" strokeLinejoin="round" d="M6.827 6.175A2.31 2.31 0 0 1 5.186 7.23c-.38.054-.757.112-1.134.175C2.999 7.58 2.25 8.507 2.25 9.574V18a2.25 2.25 0 0 0 2.25 2.25h15A2.25 2.25 0 0 0 21.75 18V9.574c0-1.067-.75-1.994-1.802-2.169a47.865 47.865 0 0 0-1.134-.175 2.31 2.31 0 0 1-1.64-1.055l-.822-1.316a2.192 2.192 0 0 0-1.736-1.039 48.774 48.774 0 0 0-5.232 0 2.192 2.192 0 0 0-1.736 1.039l-.821 1.316Z" />
                        <path strokeLinecap="round" strokeLinejoin="round" d="M16.5 12.75a4.5 4.5 0 1 1-9 0 4.5 4.5 0 0 1 9 0Z" />
                      </svg>
                      <span className="text-xs text-gray-600 group-hover:text-rose-600 font-medium transition-colors">Камера</span>
                    </button>
                  </div>
                </div>

                {/* Desktop: drop zone */}
                <div
                  onDrop={handleDrop}
                  onDragOver={handleDragOver}
                  onClick={() => fileInputRef.current?.click()}
                  className="hidden sm:flex flex-col items-center justify-center rounded-xl border-2 border-dashed border-gray-200 py-12 cursor-pointer hover:border-rose-300 hover:bg-rose-50/50 transition-all group"
                >
                  <svg className="w-10 h-10 text-gray-300 group-hover:text-rose-400 transition-colors mb-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="m2.25 15.75 5.159-5.159a2.25 2.25 0 0 1 3.182 0l5.159 5.159m-1.5-1.5 1.409-1.409a2.25 2.25 0 0 1 3.182 0l2.909 2.909M3.75 21h16.5A2.25 2.25 0 0 0 22.5 18.75V5.25A2.25 2.25 0 0 0 20.25 3H3.75A2.25 2.25 0 0 0 1.5 5.25v13.5A2.25 2.25 0 0 0 3.75 21Z" />
                  </svg>
                  <p className="text-sm text-gray-500">
                    Перетащите или <span className="text-rose-600 font-medium">выберите файл</span>
                  </p>
                  <p className="text-xs text-gray-400 mt-1">JPEG, PNG, PDF · до 10 МБ</p>
                </div>

                {/* URL input */}
                {mode !== 'url' ? (
                  <button
                    type="button"
                    onClick={() => switchMode('url')}
                    className="w-full flex items-center gap-2 rounded-xl border border-gray-200 px-4 py-3 text-sm text-gray-400 hover:text-gray-600 hover:border-gray-300 transition-colors"
                  >
                    <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M13.19 8.688a4.5 4.5 0 0 1 1.242 7.244l-4.5 4.5a4.5 4.5 0 0 1-6.364-6.364l1.757-1.757m13.35-.622 1.757-1.757a4.5 4.5 0 0 0-6.364-6.364l-4.5 4.5a4.5 4.5 0 0 0 1.242 7.244" />
                    </svg>
                    Вставить ссылку на изображение
                  </button>
                ) : (
                  <div className="rounded-xl border border-gray-200 overflow-hidden">
                    <div className="flex items-center gap-2 p-3">
                      <svg className="w-4 h-4 text-gray-400 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                        <path strokeLinecap="round" strokeLinejoin="round" d="M13.19 8.688a4.5 4.5 0 0 1 1.242 7.244l-4.5 4.5a4.5 4.5 0 0 1-6.364-6.364l1.757-1.757m13.35-.622 1.757-1.757a4.5 4.5 0 0 0-6.364-6.364l-4.5 4.5a4.5 4.5 0 0 0 1.242 7.244" />
                      </svg>
                      <input
                        id="imageUrl"
                        type="url"
                        autoFocus
                        className="flex-1 border-0 bg-transparent focus:ring-0 text-sm p-0 placeholder-gray-400"
                        placeholder="https://example.com/ekg.jpg"
                        value={imageUrl}
                        onChange={(e) => setImageUrl(e.target.value)}
                      />
                      <button
                        type="button"
                        onClick={() => switchMode('file')}
                        className="p-1 rounded text-gray-400 hover:text-gray-600 transition-colors"
                      >
                        <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                          <path strokeLinecap="round" strokeLinejoin="round" d="M6 18 18 6M6 6l12 12" />
                        </svg>
                      </button>
                    </div>
                    {imageUrl && (
                      <div className="border-t border-gray-200 bg-gray-50 p-2">
                        <img
                          src={imageUrl}
                          alt="Preview"
                          className="max-w-full h-auto block mx-auto rounded-lg"
                          onError={(e) => {
                            e.currentTarget.src =
                              'data:image/svg+xml,%3Csvg xmlns="http://www.w3.org/2000/svg" width="400" height="200"%3E%3Ctext x="50%25" y="50%25" text-anchor="middle" dy=".3em"%3EНе удалось загрузить изображение%3C/text%3E%3C/svg%3E';
                          }}
                        />
                      </div>
                    )}
                  </div>
                )}
              </div>
            )}

            {/* Crop step */}
            {step === 'crop' && previewSrc && (
              <ImageCropper
                imageSrc={previewSrc}
                onCropComplete={handleCropComplete}
                onCancel={handleCropCancel}
              />
            )}

            {/* Ready — preview with overlay actions */}
            {step === 'ready' && croppedPreview && (
              <div className="relative rounded-xl overflow-hidden border border-gray-200 bg-gray-50">
                <img
                  src={croppedPreview}
                  alt="ЭКГ"
                  className="max-w-full h-auto max-h-[50vh] sm:max-h-[500px] mx-auto block"
                />
                <div className="absolute top-2 right-2 flex gap-1.5">
                  <button
                    type="button"
                    onClick={rotateImage}
                    className="p-2 rounded-full bg-black/50 text-white hover:bg-black/70 backdrop-blur-sm transition-colors"
                    title="Повернуть"
                  >
                    <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 0h4.992m-4.993 0 3.181 3.183a8.25 8.25 0 0 0 13.803-3.7M4.031 9.865a8.25 8.25 0 0 1 13.803-3.7l3.181 3.182" />
                    </svg>
                  </button>
                  <button
                    type="button"
                    onClick={handleRecrop}
                    className="p-2 rounded-full bg-black/50 text-white hover:bg-black/70 backdrop-blur-sm transition-colors"
                    title="Обрезать"
                  >
                    <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M7.5 3.75H6A2.25 2.25 0 0 0 3.75 6v1.5M16.5 3.75H18A2.25 2.25 0 0 1 20.25 6v1.5m0 9V18A2.25 2.25 0 0 1 18 20.25h-1.5m-9 0H6A2.25 2.25 0 0 1 3.75 18v-1.5" />
                    </svg>
                  </button>
                  <button
                    type="button"
                    onClick={handleReset}
                    className="p-2 rounded-full bg-black/50 text-white hover:bg-black/70 backdrop-blur-sm transition-colors"
                    title="Заменить"
                  >
                    <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M6 18 18 6M6 6l12 12" />
                    </svg>
                  </button>
                </div>
              </div>
            )}

            {/* Calibration params */}
            <div className="rounded-xl bg-gray-50 p-4 space-y-4">
              <div className="flex items-center gap-4">
                <div className="flex-1 min-w-0">
                  <label htmlFor="age" className="block text-[11px] uppercase tracking-wide text-gray-600 font-medium mb-1.5">Возраст</label>
                  <input
                    id="age"
                    type="number"
                    min={1}
                    max={150}
                    placeholder="—"
                    className="w-full bg-white rounded-lg border-0 ring-1 ring-gray-200 focus:ring-2 focus:ring-rose-500 text-sm py-2 px-3"
                    value={age}
                    onChange={(e) => setAge(e.target.value)}
                  />
                </div>
                <div className="flex-1 min-w-0">
                  <label className="block text-[11px] uppercase tracking-wide text-gray-600 font-medium mb-1.5">Пол</label>
                  <div className="flex rounded-lg bg-white ring-1 ring-gray-200 p-0.5">
                    {[
                      { value: '', label: '—' },
                      { value: 'male', label: 'М' },
                      { value: 'female', label: 'Ж' },
                    ].map((opt) => (
                      <button
                        key={opt.value}
                        type="button"
                        onClick={() => setSex(opt.value)}
                        className={`flex-1 text-sm py-1.5 rounded-md transition-all ${
                          sex === opt.value
                            ? 'bg-rose-500 text-white shadow-sm font-medium'
                            : 'text-gray-800 hover:text-gray-900'
                        }`}
                      >
                        {opt.label}
                      </button>
                    ))}
                  </div>
                </div>
              </div>

              <div className="grid grid-cols-2 sm:grid-cols-3 gap-3">
                <div>
                  <label className="block text-[11px] uppercase tracking-wide text-gray-600 font-medium mb-1.5">Скорость мм/с</label>
                  <div className="flex rounded-lg bg-white ring-1 ring-gray-200 p-0.5">
                    {[25, 50].map((v) => (
                      <button
                        key={v}
                        type="button"
                        onClick={() => setPaperSpeed(v)}
                        className={`flex-1 text-sm py-1.5 rounded-md transition-all ${
                          paperSpeed === v
                            ? 'bg-rose-500 text-white shadow-sm font-medium'
                            : 'text-gray-800 hover:text-gray-900'
                        }`}
                      >
                        {v}
                      </button>
                    ))}
                  </div>
                </div>
                <div>
                  <label className="block text-[11px] uppercase tracking-wide text-gray-600 font-medium mb-1.5">мм/мВ конечн.</label>
                  <div className="flex rounded-lg bg-white ring-1 ring-gray-200 p-0.5">
                    {[5, 10, 20].map((v) => (
                      <button
                        key={v}
                        type="button"
                        onClick={() => setMmPerMvLimb(v)}
                        className={`flex-1 text-sm py-1.5 rounded-md transition-all ${
                          mmPerMvLimb === v
                            ? 'bg-rose-500 text-white shadow-sm font-medium'
                            : 'text-gray-800 hover:text-gray-900'
                        }`}
                      >
                        {v}
                      </button>
                    ))}
                  </div>
                </div>
                <div>
                  <label className="block text-[11px] uppercase tracking-wide text-gray-600 font-medium mb-1.5">мм/мВ грудные</label>
                  <div className="flex rounded-lg bg-white ring-1 ring-gray-200 p-0.5">
                    {[5, 10, 20].map((v) => (
                      <button
                        key={v}
                        type="button"
                        onClick={() => setMmPerMvChest(v)}
                        className={`flex-1 text-sm py-1.5 rounded-md transition-all ${
                          mmPerMvChest === v
                            ? 'bg-rose-500 text-white shadow-sm font-medium'
                            : 'text-gray-800 hover:text-gray-900'
                        }`}
                      >
                        {v}
                      </button>
                    ))}
                  </div>
                </div>
              </div>
            </div>

            {/* Actions */}
            <div className="flex items-center justify-between pt-2 sm:pt-4">
              <button
                type="button"
                onClick={() => navigate(ROUTES.DASHBOARD)}
                className="text-gray-600 hover:text-gray-800 text-sm sm:text-base"
              >
                Отмена
              </button>
              <button
                type="submit"
                disabled={mutation.isPending || !canSubmit}
                className="px-5 sm:px-6 py-2.5 bg-rose-600 text-white rounded-xl hover:bg-rose-700 focus:outline-none focus:ring-2 focus:ring-rose-500 focus:ring-offset-2 disabled:opacity-50 text-sm sm:text-base font-medium transition-colors"
              >
                {mutation.isPending ? 'Отправка...' : 'Запустить анализ'}
              </button>
            </div>
          </form>
        </div>

        <div className="mt-6 sm:mt-8 flex flex-wrap gap-x-6 gap-y-2 text-xs text-gray-400">
          <span>JPEG, PNG, PDF до 10 MB</span>
          {mode === 'camera' && <span>Держите телефон параллельно бумаге</span>}
        </div>
      </div>
    </Layout>
  );
}
