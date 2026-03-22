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

type Mode = 'file' | 'camera' | 'url';
type Step = 'select' | 'crop' | 'ready';

export function Analyze() {
  const [mode, setMode] = useState<Mode>('file');
  const [step, setStep] = useState<Step>('select');
  const [notes, setNotes, clearNotes] = useDraft('analyze_notes');
  const [error, setError] = useState('');
  const [showPayment, setShowPayment] = useState(false);
  const navigate = useNavigate();
  const { addJob } = usePendingJobs();
  const { quota, refetch: refetchQuota } = useQuota();
  const queryClient = useQueryClient();

  // File/camera mode state
  const [previewSrc, setPreviewSrc] = useState<string | null>(null);
  const [croppedBlob, setCroppedBlob] = useState<Blob | null>(null);
  const [croppedPreview, setCroppedPreview] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const cameraInputRef = useRef<HTMLInputElement>(null);

  // URL mode state — persisted as draft
  const [imageUrl, setImageUrl, clearImageUrl] = useDraft('analyze_url');

  const mutation = useMutation({
    mutationFn: () => {
      if ((mode === 'file' || mode === 'camera') && croppedBlob) {
        return ekgAPI.submitAnalysisFile(croppedBlob, notes || undefined);
      }
      return ekgAPI.submitAnalysis({ image_temp_url: imageUrl, notes: notes || undefined });
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

  const handleFileSelect = useCallback((file: File) => {
    if (!file.type.startsWith('image/') && file.type !== 'application/pdf') {
      setError('Поддерживаются только изображения и PDF');
      return;
    }
    if (file.size > 10 * 1024 * 1024) {
      setError('Файл слишком большой (макс. 10MB)');
      return;
    }
    setError('');
    const url = URL.createObjectURL(file);
    setPreviewSrc(url);
    setStep('crop');
  }, []);

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
    if (previewSrc) URL.revokeObjectURL(previewSrc);
    setPreviewSrc(null);
    setCroppedBlob(null);
    setStep('select');
  };

  const handleRecrop = () => {
    if (croppedPreview) URL.revokeObjectURL(croppedPreview);
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

            {/* Mode toggle */}
            <div className="flex flex-wrap gap-1.5 sm:gap-2 text-sm">
              <button
                type="button"
                onClick={() => switchMode('file')}
                className={`px-3 py-1.5 rounded-md transition-colors ${
                  mode === 'file'
                    ? 'bg-rose-100 text-rose-700 font-medium'
                    : 'text-gray-500 hover:text-gray-700'
                }`}
              >
                Загрузить файл
              </button>
              <button
                type="button"
                onClick={() => switchMode('camera')}
                className={`px-3 py-1.5 rounded-md transition-colors ${
                  mode === 'camera'
                    ? 'bg-rose-100 text-rose-700 font-medium'
                    : 'text-gray-500 hover:text-gray-700'
                }`}
              >
                Камера
              </button>
              <button
                type="button"
                onClick={() => switchMode('url')}
                className={`px-3 py-1.5 rounded-md transition-colors ${
                  mode === 'url'
                    ? 'bg-rose-100 text-rose-700 font-medium'
                    : 'text-gray-500 hover:text-gray-700'
                }`}
              >
                Указать URL
              </button>
            </div>

            {/* File upload mode */}
            {mode === 'file' && (
              <>
                {step === 'select' && (
                  <div
                    onDrop={handleDrop}
                    onDragOver={handleDragOver}
                    onClick={() => fileInputRef.current?.click()}
                    className="border-2 border-dashed border-gray-300 rounded-lg p-6 sm:p-8 text-center cursor-pointer hover:border-rose-400 hover:bg-rose-50 transition-colors active:bg-rose-100"
                  >
                    <input
                      ref={fileInputRef}
                      type="file"
                      accept="image/*,application/pdf"
                      onChange={handleInputChange}
                      className="hidden"
                    />
                    <div className="text-gray-500">
                      <svg
                        className="mx-auto h-10 w-10 sm:h-12 sm:w-12 text-gray-400 mb-3"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                      >
                        <path
                          strokeLinecap="round"
                          strokeLinejoin="round"
                          strokeWidth={1.5}
                          d="M12 16v-8m0 0l-3 3m3-3l3 3M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1"
                        />
                      </svg>
                      <p className="text-sm font-medium">
                        Перетащите изображение ЭКГ сюда
                      </p>
                      <p className="text-xs text-gray-400 mt-1">
                        или нажмите для выбора файла (JPEG, PNG, PDF, до 10MB)
                      </p>
                    </div>
                  </div>
                )}

                {step === 'crop' && previewSrc && (
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">
                      Обрежьте изображение
                    </label>
                    <ImageCropper
                      imageSrc={previewSrc}
                      onCropComplete={handleCropComplete}
                      onCancel={handleCropCancel}
                    />
                  </div>
                )}

                {step === 'ready' && croppedPreview && (
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">
                      Результат обрезки
                    </label>
                    <div className="border border-gray-300 rounded-lg p-3 sm:p-4 bg-gray-50">
                      <img
                        src={croppedPreview}
                        alt="Обрезанное изображение"
                        className="max-w-full h-auto rounded max-h-64 mx-auto"
                      />
                    </div>
                    <div className="mt-2 flex flex-wrap gap-x-1 gap-y-1">
                      <button
                        type="button"
                        onClick={handleRecrop}
                        className="text-sm text-rose-600 hover:text-rose-800 py-1"
                      >
                        Обрезать еще раз
                      </button>
                      <span className="mx-1.5 text-gray-300">|</span>
                      <button
                        type="button"
                        onClick={handleReset}
                        className="text-sm text-gray-500 hover:text-gray-700 py-1"
                      >
                        Выбрать другой файл
                      </button>
                    </div>
                  </div>
                )}
              </>
            )}

            {/* Camera mode */}
            {mode === 'camera' && (
              <>
                {step === 'select' && (
                  <div className="space-y-3">
                    <input
                      ref={cameraInputRef}
                      type="file"
                      accept="image/*"
                      capture="environment"
                      onChange={handleInputChange}
                      className="hidden"
                    />
                    <button
                      type="button"
                      onClick={() => cameraInputRef.current?.click()}
                      className="w-full border-2 border-dashed border-gray-300 rounded-lg p-6 sm:p-8 text-center cursor-pointer hover:border-rose-400 hover:bg-rose-50 transition-colors active:bg-rose-100"
                    >
                      <div className="text-gray-500">
                        <svg
                          className="mx-auto h-10 w-10 sm:h-12 sm:w-12 text-gray-400 mb-3"
                          fill="none"
                          viewBox="0 0 24 24"
                          stroke="currentColor"
                        >
                          <path
                            strokeLinecap="round"
                            strokeLinejoin="round"
                            strokeWidth={1.5}
                            d="M6.827 6.175A2.31 2.31 0 0 1 5.186 7.23c-.38.054-.757.112-1.134.175C2.999 7.58 2.25 8.507 2.25 9.574V18a2.25 2.25 0 0 0 2.25 2.25h15A2.25 2.25 0 0 0 21.75 18V9.574c0-1.067-.75-1.994-1.802-2.169a47.865 47.865 0 0 0-1.134-.175 2.31 2.31 0 0 1-1.64-1.055l-.822-1.316a2.192 2.192 0 0 0-1.736-1.039 48.774 48.774 0 0 0-5.232 0 2.192 2.192 0 0 0-1.736 1.039l-.821 1.316Z"
                          />
                          <path
                            strokeLinecap="round"
                            strokeLinejoin="round"
                            strokeWidth={1.5}
                            d="M16.5 12.75a4.5 4.5 0 1 1-9 0 4.5 4.5 0 0 1 9 0Z"
                          />
                        </svg>
                        <p className="text-sm font-medium">
                          Сфотографировать ЭКГ
                        </p>
                        <p className="text-xs text-gray-400 mt-1">
                          Откроется камера устройства
                        </p>
                      </div>
                    </button>
                    <div className="bg-blue-50 border border-blue-100 rounded-lg p-3">
                      <p className="text-xs text-blue-700">
                        Для лучшего результата: расположите ЭКГ на ровной поверхности, обеспечьте хорошее освещение, избегайте бликов и теней
                      </p>
                    </div>
                  </div>
                )}

                {step === 'crop' && previewSrc && (
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">
                      Обрежьте фотографию
                    </label>
                    <ImageCropper
                      imageSrc={previewSrc}
                      onCropComplete={handleCropComplete}
                      onCancel={handleCropCancel}
                    />
                  </div>
                )}

                {step === 'ready' && croppedPreview && (
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">
                      Результат обрезки
                    </label>
                    <div className="border border-gray-300 rounded-lg p-3 sm:p-4 bg-gray-50">
                      <img
                        src={croppedPreview}
                        alt="Обрезанное фото"
                        className="max-w-full h-auto rounded max-h-64 mx-auto"
                      />
                    </div>
                    <div className="mt-2 flex flex-wrap gap-x-1 gap-y-1">
                      <button
                        type="button"
                        onClick={handleRecrop}
                        className="text-sm text-rose-600 hover:text-rose-800 py-1"
                      >
                        Обрезать еще раз
                      </button>
                      <span className="mx-1.5 text-gray-300">|</span>
                      <button
                        type="button"
                        onClick={handleReset}
                        className="text-sm text-gray-500 hover:text-gray-700 py-1"
                      >
                        Сфотографировать еще раз
                      </button>
                    </div>
                  </div>
                )}
              </>
            )}

            {mode === 'url' && (
              <>
                <div>
                  <label htmlFor="imageUrl" className="block text-sm font-medium text-gray-700 mb-2">
                    URL изображения ЭКГ *
                  </label>
                  <input
                    id="imageUrl"
                    type="url"
                    required
                    className="block w-full rounded-md border-gray-300 shadow-sm focus:border-rose-500 focus:ring-rose-500 text-sm"
                    placeholder="https://example.com/ekg.jpg"
                    value={imageUrl}
                    onChange={(e) => setImageUrl(e.target.value)}
                  />
                  <p className="mt-1 text-xs sm:text-sm text-gray-500">
                    Поддерживаемые форматы: JPEG, PNG, GIF, WebP, BMP, TIFF, PDF
                  </p>
                </div>

                {imageUrl && (
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">
                      Предпросмотр
                    </label>
                    <div className="border border-gray-300 rounded-lg p-3 sm:p-4 bg-gray-50">
                      <img
                        src={imageUrl}
                        alt="Preview"
                        className="max-w-full h-auto rounded"
                        onError={(e) => {
                          e.currentTarget.src =
                            'data:image/svg+xml,%3Csvg xmlns="http://www.w3.org/2000/svg" width="400" height="200"%3E%3Ctext x="50%25" y="50%25" text-anchor="middle" dy=".3em"%3EНе удалось загрузить изображение%3C/text%3E%3C/svg%3E';
                        }}
                      />
                    </div>
                  </div>
                )}
              </>
            )}

            {/* Notes */}
            <div>
              <label htmlFor="notes" className="block text-sm font-medium text-gray-700 mb-2">
                Примечания (опционально)
              </label>
              <textarea
                id="notes"
                rows={3}
                maxLength={2000}
                className="block w-full rounded-md border-gray-300 shadow-sm focus:border-rose-500 focus:ring-rose-500 text-sm"
                placeholder="Дополнительная информация о пациенте или ЭКГ..."
                value={notes}
                onChange={(e) => setNotes(e.target.value)}
              />
              {notes.length > 1800 && (
                <p className="mt-1 text-sm text-yellow-600">{notes.length}/2000 символов</p>
              )}
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
