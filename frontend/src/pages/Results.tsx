import { useParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import ReactMarkdown from 'react-markdown';
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
    refetchInterval: (query) => {
      // Poll if request is still pending or processing
      const data = query.state.data;
      return data?.status === 'pending' || data?.status === 'processing' ? 2000 : false;
    },
  });

  let ekgResult: EKGAnalysisResult | null = null;
  let gptRequestId: string | null = null;
  if (request?.response?.content) {
    try {
      const parsed = JSON.parse(request.response.content);
      // Check if this is an EKG result
      if (parsed?.analysis_type === 'ekg_direct_v2') {
        ekgResult = parsed as EKGAnalysisResult;
        gptRequestId = ekgResult?.gpt_request_id || null;
      }
    } catch (err) {
      // If parsing fails or it's not an EKG result, it's likely a GPT response
      console.error('Failed to parse EKG result:', err);
    }
  }

  // Fetch GPT analysis result if available
  const { data: gptRequest } = useQuery({
    queryKey: ['request', gptRequestId],
    queryFn: () => requestAPI.getRequest(gptRequestId!),
    enabled: !!gptRequestId,
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

        {/* Original Image */}
        {request.files && request.files.length > 0 && request.files[0].s3_url && (
          <div className="bg-white shadow rounded-lg p-6 mb-6">
            <h2 className="text-xl font-bold text-gray-900 mb-4">Исходное изображение</h2>
            <div className="flex justify-center">
              <img
                src={request.files[0].s3_url}
                alt="Исходное ЭКГ изображение"
                className="max-w-full h-auto rounded-lg shadow-md"
                onError={(e) => {
                  // Fallback if image fails to load
                  const target = e.target as HTMLImageElement;
                  target.style.display = 'none';
                }}
              />
            </div>
          </div>
        )}

        {/* Analysis Result - GPT from linked request or direct GPT request */}
        {(gptRequest?.response || (!ekgResult && request.response && request.response.model !== 'ekg_direct_v2')) && (
          <div className="bg-gradient-to-br from-purple-50 to-blue-50 border border-purple-200 shadow rounded-lg p-6 mb-6">
            <div className="flex items-center mb-4">
              <span className="text-2xl mr-2">📒</span>
              <h2 className="text-xl font-bold text-gray-900">Заключение</h2>
            </div>
            <div className="bg-white rounded-lg p-4 border border-purple-100 mb-4">
              <ReactMarkdown className="prose prose-sm max-w-none prose-gray">
                {(gptRequest?.response?.content || request.response?.content) || ''}
              </ReactMarkdown>
            </div>
            {/* GPT Processing Info */}
            {(gptRequest?.response || request.response) && (
              <div className="bg-white rounded-lg p-4 border border-purple-100">
                <h3 className="text-sm font-medium text-gray-700 mb-4">Информация об обработке</h3>
                <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                  {(gptRequest?.response?.model || request.response?.model) && (
                    <div>
                      <label className="text-sm font-medium text-gray-500">Модель</label>
                      <p className="mt-1 text-sm text-gray-900">
                        {gptRequest?.response?.model || request.response?.model}
                      </p>
                    </div>
                  )}
                  {(gptRequest?.response?.tokens_used !== undefined || request.response?.tokens_used !== undefined) && (
                    <div>
                      <label className="text-sm font-medium text-gray-500">Токены</label>
                      <p className="mt-1 text-sm text-gray-900">
                        {gptRequest?.response?.tokens_used ?? request.response?.tokens_used}
                      </p>
                    </div>
                  )}
                  {(gptRequest?.response?.processing_time_ms !== undefined || request.response?.processing_time_ms !== undefined) && (
                    <div>
                      <label className="text-sm font-medium text-gray-500">Время обработки</label>
                      <p className="mt-1 text-sm text-gray-900">
                        {(gptRequest?.response?.processing_time_ms ?? request.response?.processing_time_ms)}ms
                      </p>
                    </div>
                  )}
                </div>
              </div>
            )}
          </div>
        )}

        {/* EKG Result Info */}
        {ekgResult && (
          <div className="bg-white shadow rounded-lg p-6 mb-6">
            <h2 className="text-xl font-bold text-gray-900 mb-4">Информация об анализе ЭКГ</h2>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div>
                <label className="text-sm font-medium text-gray-500">Время анализа</label>
                <p className="mt-1 text-sm text-gray-900">{formatDate(ekgResult.timestamp)}</p>
              </div>
              <div>
                <label className="text-sm font-medium text-gray-500">Статус GPT-интерпретации</label>
                <p className="mt-1 text-sm text-gray-900">
                  {ekgResult.gpt_interpretation_status || 'N/A'}
                </p>
              </div>
            </div>
            {ekgResult.notes && (
              <div className="mt-4 pt-4 border-t border-gray-200">
                <label className="text-sm font-medium text-gray-500">Примечания</label>
                <p className="mt-1 text-sm text-gray-600">{ekgResult.notes}</p>
              </div>
            )}
          </div>
        )}

      </div>
    </Layout>
  );
}

