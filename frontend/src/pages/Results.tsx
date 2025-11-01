import { useParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { requestAPI } from '@/services/api';
import { formatDate, formatStatus, getStatusColor } from '@/utils/format';
import { Layout } from '@/components/Layout';
import type { EKGAnalysisResult } from '@/types';

export function Results() {
  const { id } = useParams<{ id: string }>();
  
  const { data: request, isLoading, error } = useQuery({
    queryKey: ['request', id],
    queryFn: () => requestAPI.getRequest(id!),
    enabled: !!id,
  });

  if (isLoading) {
    return (
      <Layout>
        <div className="px-4 sm:px-0">
          <div className="text-center py-8 text-gray-500">Загрузка...</div>
        </div>
      </Layout>
    );
  }

  if (error || !request) {
    return (
      <Layout>
        <div className="px-4 sm:px-0">
          <div className="text-center py-8 text-red-500">
            Ошибка при загрузке результата
          </div>
        </div>
      </Layout>
    );
  }

  let ekgResult: EKGAnalysisResult | null = null;
  if (request.response?.content) {
    try {
      ekgResult = JSON.parse(request.response.content);
    } catch (err) {
      console.error('Failed to parse EKG result:', err);
    }
  }

  return (
    <Layout>
      <div className="max-w-4xl mx-auto px-4 sm:px-0">
        <h1 className="text-3xl font-bold text-gray-900 mb-8">Результаты анализа</h1>

        {/* Request Info */}
        <div className="bg-white shadow rounded-lg p-6 mb-6">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <label className="text-sm font-medium text-gray-500">ID запроса</label>
              <p className="mt-1 text-sm font-mono text-gray-900">{request.id}</p>
            </div>
            <div>
              <label className="text-sm font-medium text-gray-500">Статус</label>
              <p className="mt-1">
                <span
                  className={`px-2 inline-flex text-xs leading-5 font-semibold rounded-full ${getStatusColor(
                    request.status
                  )}`}
                >
                  {formatStatus(request.status)}
                </span>
              </p>
            </div>
            <div>
              <label className="text-sm font-medium text-gray-500">Создано</label>
              <p className="mt-1 text-sm text-gray-900">{formatDate(request.created_at)}</p>
            </div>
            <div>
              <label className="text-sm font-medium text-gray-500">Обновлено</label>
              <p className="mt-1 text-sm text-gray-900">{formatDate(request.updated_at)}</p>
            </div>
          </div>
        </div>

        {/* EKG Result */}
        {ekgResult && (
          <div className="bg-white shadow rounded-lg p-6 mb-6">
            <h2 className="text-xl font-bold text-gray-900 mb-4">Характеристики сигнала</h2>
            
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6 mb-6">
              <div className="bg-blue-50 rounded-lg p-4">
                <h3 className="text-sm font-medium text-blue-900 mb-2">Основные параметры</h3>
                <dl className="space-y-2">
                  <div className="flex justify-between">
                    <dt className="text-sm text-blue-700">Длина сигнала:</dt>
                    <dd className="text-sm font-semibold text-blue-900">
                      {ekgResult.signal_length.toFixed(2)}
                    </dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="text-sm text-blue-700">Ширина сигнала:</dt>
                    <dd className="text-sm font-semibold text-blue-900">
                      {ekgResult.signal_features.signal_width}
                    </dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="text-sm text-blue-700">Точек контура:</dt>
                    <dd className="text-sm font-semibold text-blue-900">
                      {ekgResult.signal_features.points_count}
                    </dd>
                  </div>
                </dl>
              </div>

              <div className="bg-green-50 rounded-lg p-4">
                <h3 className="text-sm font-medium text-green-900 mb-2">Амплитуда</h3>
                <dl className="space-y-2">
                  <div className="flex justify-between">
                    <dt className="text-sm text-green-700">Диапазон:</dt>
                    <dd className="text-sm font-semibold text-green-900">
                      {ekgResult.signal_features.amplitude_range}
                    </dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="text-sm text-green-700">Базовая линия:</dt>
                    <dd className="text-sm font-semibold text-green-900">
                      {ekgResult.signal_features.baseline.toFixed(2)}
                    </dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="text-sm text-green-700">Станд. отклонение:</dt>
                    <dd className="text-sm font-semibold text-green-900">
                      {ekgResult.signal_features.standard_deviation.toFixed(2)}
                    </dd>
                  </div>
                </dl>
              </div>
            </div>

            <div className="bg-purple-50 rounded-lg p-4 mb-6">
              <h3 className="text-sm font-medium text-purple-900 mb-2">Bounding Box</h3>
              <dl className="grid grid-cols-4 gap-4">
                <div>
                  <dt className="text-xs text-purple-700">Min X</dt>
                  <dd className="text-sm font-semibold text-purple-900">
                    {ekgResult.signal_features.bounding_box.min_x}
                  </dd>
                </div>
                <div>
                  <dt className="text-xs text-purple-700">Max X</dt>
                  <dd className="text-sm font-semibold text-purple-900">
                    {ekgResult.signal_features.bounding_box.max_x}
                  </dd>
                </div>
                <div>
                  <dt className="text-xs text-purple-700">Min Y</dt>
                  <dd className="text-sm font-semibold text-purple-900">
                    {ekgResult.signal_features.bounding_box.min_y}
                  </dd>
                </div>
                <div>
                  <dt className="text-xs text-purple-700">Max Y</dt>
                  <dd className="text-sm font-semibold text-purple-900">
                    {ekgResult.signal_features.bounding_box.max_y}
                  </dd>
                </div>
              </dl>
            </div>

            <div>
              <h3 className="text-sm font-medium text-gray-700 mb-2">Шаги обработки</h3>
              <div className="flex flex-wrap gap-2">
                {ekgResult.processing_steps.map((step, index) => (
                  <span
                    key={index}
                    className="px-3 py-1 bg-gray-100 text-gray-700 rounded-full text-xs font-medium"
                  >
                    {step}
                  </span>
                ))}
              </div>
            </div>

            {ekgResult.notes && (
              <div className="mt-6 pt-6 border-t border-gray-200">
                <h3 className="text-sm font-medium text-gray-700 mb-2">Примечания</h3>
                <p className="text-sm text-gray-600">{ekgResult.notes}</p>
              </div>
            )}
          </div>
        )}

        {/* Processing Info */}
        {request.response && (
          <div className="bg-white shadow rounded-lg p-6">
            <h2 className="text-xl font-bold text-gray-900 mb-4">Информация об обработке</h2>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              {request.response.model && (
                <div>
                  <label className="text-sm font-medium text-gray-500">Модель</label>
                  <p className="mt-1 text-sm text-gray-900">{request.response.model}</p>
                </div>
              )}
              {request.response.tokens_used && (
                <div>
                  <label className="text-sm font-medium text-gray-500">Токены</label>
                  <p className="mt-1 text-sm text-gray-900">{request.response.tokens_used}</p>
                </div>
              )}
              {request.response.processing_time_ms && (
                <div>
                  <label className="text-sm font-medium text-gray-500">Время обработки</label>
                  <p className="mt-1 text-sm text-gray-900">
                    {request.response.processing_time_ms}ms
                  </p>
                </div>
              )}
            </div>
          </div>
        )}
      </div>
    </Layout>
  );
}

