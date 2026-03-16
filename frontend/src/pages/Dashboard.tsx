import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { requestAPI } from '@/services/api';
import { formatRelative, formatStatus, getStatusColor } from '@/utils/format';
import { ROUTES } from '@/config';
import { Layout } from '@/components/Layout';
import type { Request } from '@/types';

export function Dashboard() {
  const { data: requests, isLoading } = useQuery<Request[]>({
    queryKey: ['requests'],
    queryFn: () => requestAPI.getUserRequests(),
  });

  // Hide internal GPT sub-requests created by the EKG worker pipeline.
  // These contain the EKG prompt template in text_query and are not user-facing.
  const userRequests = requests?.filter(
    (request) => !request.text_query?.includes('Analyze this ECG/EKG image')
  ) || [];
  const recentRequests = userRequests.slice(0, 5);

  return (
    <Layout>
      <div className="px-4 sm:px-0">
        <h1 className="text-3xl font-bold text-gray-900 mb-8">Панель управления</h1>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-6 mb-8">
          <Link
            to={ROUTES.ANALYZE}
            className="block bg-gradient-to-r from-blue-500 to-blue-600 text-white rounded-lg shadow-lg p-6 hover:from-blue-600 hover:to-blue-700 transition"
          >
            <div className="flex items-center">
              <div className="flex-shrink-0">
                <span className="text-4xl">📊</span>
              </div>
              <div className="ml-4">
                <h2 className="text-2xl font-bold">Новый анализ ЭКГ</h2>
                <p className="text-blue-100 mt-1">Загрузите изображение для анализа</p>
              </div>
            </div>
          </Link>
          <Link
            to={ROUTES.KNOWLEDGE_BASE}
            className="block bg-gradient-to-r from-purple-500 to-purple-600 text-white rounded-lg shadow-lg p-6 hover:from-purple-600 hover:to-purple-700 transition"
          >
            <div className="flex items-center">
              <div className="flex-shrink-0">
                <span className="text-4xl">📚</span>
              </div>
              <div className="ml-4">
                <h2 className="text-2xl font-bold">База знаний</h2>
                <p className="text-purple-100 mt-1">Полезная информация о здоровье сердца</p>
              </div>
            </div>
          </Link>
        </div>

        <div className="bg-white shadow rounded-lg">
          <div className="px-6 py-4 border-b border-gray-200">
            <h2 className="text-lg font-medium text-gray-900">Последние анализы</h2>
          </div>
          <div className="overflow-x-auto">
            {isLoading ? (
              <div className="px-6 py-8 text-center text-gray-500">Загрузка...</div>
            ) : recentRequests.length === 0 ? (
              <div className="px-6 py-8 text-center text-gray-500">
                <p>У вас пока нет анализов</p>
                <Link
                  to={ROUTES.ANALYZE}
                  className="text-blue-600 hover:text-blue-500 mt-2 inline-block"
                >
                  Создать первый анализ →
                </Link>
              </div>
            ) : (
              <table className="min-w-full divide-y divide-gray-200">
                <thead className="bg-gray-50">
                  <tr>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                      ID
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                      Статус
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                      Создано
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                      Действия
                    </th>
                  </tr>
                </thead>
                <tbody className="bg-white divide-y divide-gray-200">
                  {recentRequests.map((request) => (
                    <tr key={request.id} className="hover:bg-gray-50">
                      <td className="px-6 py-4 whitespace-nowrap text-sm font-mono text-gray-500">
                        {request.id.slice(0, 8)}...
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap">
                        <span
                          className={`px-2 inline-flex text-xs leading-5 font-semibold rounded-full ${getStatusColor(
                            request.status
                          )}`}
                        >
                          {formatStatus(request.status)}
                        </span>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                        {formatRelative(request.created_at)}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm font-medium">
                        <Link
                          to={`/results/${request.id}`}
                          className="text-blue-600 hover:text-blue-900"
                        >
                          Просмотр
                        </Link>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
          {recentRequests.length > 0 && (
            <div className="px-6 py-4 border-t border-gray-200 text-center">
              <Link
                to={ROUTES.HISTORY}
                className="text-blue-600 hover:text-blue-500 font-medium"
              >
                Показать все →
              </Link>
            </div>
          )}
        </div>
      </div>
    </Layout>
  );
}

