import { useState, useCallback, useRef } from 'react';
import { useMutation } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { ekgAPI } from '@/services/api';
import { ROUTES } from '@/config';
import { Layout } from '@/components/Layout';
import { ImageCropper } from '@/components/ImageCropper';
import { useDraft } from '@/hooks/useDraft';
import { usePendingJobs } from '@/hooks/usePendingJobs';

type Mode = 'file' | 'url';
type Step = 'select' | 'crop' | 'ready';

export function Analyze() {
  const [mode, setMode] = useState<Mode>('file');
  const [step, setStep] = useState<Step>('select');
  const [notes, setNotes, clearNotes] = useDraft('analyze_notes');
  const [error, setError] = useState('');
  const navigate = useNavigate();
  const { addJob } = usePendingJobs();

  // File mode state
  const [previewSrc, setPreviewSrc] = useState<string | null>(null);
  const [croppedBlob, setCroppedBlob] = useState<Blob | null>(null);
  const [croppedPreview, setCroppedPreview] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  // URL mode state — persisted as draft
  const [imageUrl, setImageUrl, clearImageUrl] = useDraft('analyze_url');

  const mutation = useMutation({
    mutationFn: () => {
      if (mode === 'file' && croppedBlob) {
        return ekgAPI.submitAnalysisFile(croppedBlob, notes || undefined);
      }
      return ekgAPI.submitAnalysis({ image_temp_url: imageUrl, notes: notes || undefined });
    },
    onSuccess: (response) => {
      clearNotes();
      clearImageUrl();
      addJob(response.request_id);
      navigate(`/results/${response.request_id}`);
    },
    onError: (err: any) => {
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
    if (mode === 'file' && !croppedBlob) {
      setError('Выберите и обрежьте изображение');
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
    mode === 'file' ? step === 'ready' && croppedBlob !== null : imageUrl.trim() !== '';

  return (
    <Layout>
      <div className="max-w-3xl mx-auto px-4 sm:px-0">
        <h1 className="text-3xl font-bold text-gray-900 mb-8">Анализ ЭКГ</h1>

        <div className="bg-white shadow rounded-lg p-6">
          <form onSubmit={handleSubmit} className="space-y-6">
            {error && (
              <div className="bg-red-50 border border-red-200 text-red-800 px-4 py-3 rounded">
                {error}
              </div>
            )}

            {/* Mode toggle */}
            <div className="flex gap-2 text-sm">
              <button
                type="button"
                onClick={() => switchMode('file')}
                className={`px-3 py-1.5 rounded-md ${
                  mode === 'file'
                    ? 'bg-blue-100 text-blue-700 font-medium'
                    : 'text-gray-500 hover:text-gray-700'
                }`}
              >
                Загрузить файл
              </button>
              <button
                type="button"
                onClick={() => switchMode('url')}
                className={`px-3 py-1.5 rounded-md ${
                  mode === 'url'
                    ? 'bg-blue-100 text-blue-700 font-medium'
                    : 'text-gray-500 hover:text-gray-700'
                }`}
              >
                Указать URL
              </button>
            </div>

            {mode === 'file' && (
              <>
                {/* File selection */}
                {step === 'select' && (
                  <div
                    onDrop={handleDrop}
                    onDragOver={handleDragOver}
                    onClick={() => fileInputRef.current?.click()}
                    className="border-2 border-dashed border-gray-300 rounded-lg p-8 text-center cursor-pointer hover:border-blue-400 hover:bg-blue-50 transition-colors"
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
                        className="mx-auto h-12 w-12 text-gray-400 mb-3"
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

                {/* Cropper */}
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

                {/* Cropped preview */}
                {step === 'ready' && croppedPreview && (
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">
                      Результат обрезки
                    </label>
                    <div className="border border-gray-300 rounded-lg p-4 bg-gray-50">
                      <img
                        src={croppedPreview}
                        alt="Обрезанное изображение"
                        className="max-w-full h-auto rounded max-h-64 mx-auto"
                      />
                    </div>
                    <div className="mt-2">
                      <button
                        type="button"
                        onClick={handleRecrop}
                        className="text-sm text-blue-600 hover:text-blue-800"
                      >
                        Обрезать заново
                      </button>
                      <span className="mx-2 text-gray-300">|</span>
                      <button
                        type="button"
                        onClick={handleReset}
                        className="text-sm text-gray-500 hover:text-gray-700"
                      >
                        Выбрать другой файл
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
                    className="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
                    placeholder="https://example.com/ekg.jpg"
                    value={imageUrl}
                    onChange={(e) => setImageUrl(e.target.value)}
                  />
                  <p className="mt-1 text-sm text-gray-500">
                    Поддерживаемые форматы: JPEG, PNG, GIF, WebP, BMP, TIFF, PDF
                  </p>
                </div>

                {imageUrl && (
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">
                      Предпросмотр
                    </label>
                    <div className="border border-gray-300 rounded-lg p-4 bg-gray-50">
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
                rows={4}
                maxLength={2000}
                className="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
                placeholder="Дополнительная информация о пациенте или ЭКГ..."
                value={notes}
                onChange={(e) => setNotes(e.target.value)}
              />
              {notes.length > 1800 && (
                <p className="mt-1 text-sm text-yellow-600">{notes.length}/2000 символов</p>
              )}
            </div>

            {/* Actions */}
            <div className="flex items-center justify-between pt-4">
              <button
                type="button"
                onClick={() => navigate(ROUTES.DASHBOARD)}
                className="text-gray-600 hover:text-gray-800"
              >
                Отмена
              </button>
              <button
                type="submit"
                disabled={mutation.isPending || !canSubmit}
                className="px-6 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:opacity-50"
              >
                {mutation.isPending ? 'Отправка...' : 'Запустить анализ'}
              </button>
            </div>
          </form>
        </div>

        <div className="mt-8 bg-blue-50 border border-blue-200 rounded-lg p-6">
          <h3 className="text-lg font-medium text-blue-900 mb-2">Информация</h3>
          <ul className="space-y-2 text-sm text-blue-800">
            <li>Максимальный размер файла: 10MB</li>
            <li>Обработка обычно занимает 2-5 секунд</li>
            <li>Результаты будут доступны в истории</li>
            {mode === 'file' && <li>Вы можете обрезать изображение перед отправкой</li>}
          </ul>
        </div>
      </div>
    </Layout>
  );
}
