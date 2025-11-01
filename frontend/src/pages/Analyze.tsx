import { useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { ekgAPI } from '@/services/api';
import { ROUTES } from '@/config';
import { Layout } from '@/components/Layout';

export function Analyze() {
  const [imageUrl, setImageUrl] = useState('');
  const [notes, setNotes] = useState('');
  const [error, setError] = useState('');
  const navigate = useNavigate();

  const mutation = useMutation({
    mutationFn: (data: { image_temp_url: string; notes?: string }) =>
      ekgAPI.submitAnalysis(data),
    onSuccess: (response) => {
      navigate(`${ROUTES.HISTORY}?job_id=${response.job_id}`);
    },
    onError: (err: any) => {
      setError(err.response?.data?.error || 'Ошибка при отправке анализа');
    },
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!imageUrl.trim()) {
      setError('Введите URL изображения');
      return;
    }
    setError('');
    mutation.mutate({ image_temp_url: imageUrl, notes: notes || undefined });
  };

  return (
    <Layout>
      <div className="max-w-3xl mx-auto px-4 sm:px-0">
        <h1 className="text-3xl font-bold text-gray-900 mb-8">Анализ EKG</h1>

        <div className="bg-white shadow rounded-lg p-6">
          <form onSubmit={handleSubmit} className="space-y-6">
            {error && (
              <div className="bg-red-50 border border-red-200 text-red-800 px-4 py-3 rounded">
                {error}
              </div>
            )}

            <div>
              <label htmlFor="imageUrl" className="block text-sm font-medium text-gray-700 mb-2">
                URL изображения EKG *
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
                      e.currentTarget.src = 'data:image/svg+xml,%3Csvg xmlns="http://www.w3.org/2000/svg" width="400" height="200"%3E%3Ctext x="50%25" y="50%25" text-anchor="middle" dy=".3em"%3EНе удалось загрузить изображение%3C/text%3E%3C/svg%3E';
                    }}
                  />
                </div>
              </div>
            )}

            <div>
              <label htmlFor="notes" className="block text-sm font-medium text-gray-700 mb-2">
                Примечания (опционально)
              </label>
              <textarea
                id="notes"
                rows={4}
                className="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
                placeholder="Дополнительная информация о пациенте или EKG..."
                value={notes}
                onChange={(e) => setNotes(e.target.value)}
              />
            </div>

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
                disabled={mutation.isPending}
                className="px-6 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:opacity-50"
              >
                {mutation.isPending ? 'Отправка...' : 'Запустить анализ'}
              </button>
            </div>
          </form>
        </div>

        <div className="mt-8 bg-blue-50 border border-blue-200 rounded-lg p-6">
          <h3 className="text-lg font-medium text-blue-900 mb-2">ℹ️ Информация</h3>
          <ul className="space-y-2 text-sm text-blue-800">
            <li>• Максимальный размер файла: 10MB</li>
            <li>• Таймаут загрузки: 30 секунд</li>
            <li>• Обработка обычно занимает 2-5 секунд</li>
            <li>• Результаты будут доступны в истории</li>
          </ul>
        </div>
      </div>
    </Layout>
  );
}

