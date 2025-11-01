import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { requestAPI } from '@/services/api';
import { formatDate, formatStatus, getStatusColor } from '@/utils/format';
import { Layout } from '@/components/Layout';

export function History() {
  const { data: requests, isLoading, error } = useQuery({
    queryKey: ['requests'],
    queryFn: () => requestAPI.getUserRequests(),
  });

  // Filter out EKG Analysis requests (requests with text_query)
  const filteredRequests = requests?.filter((request) => !request.text_query) || [];

  return (
    <Layout>
      <div className="px-4 sm:px-0">
        <h1 className="text-3xl font-bold text-gray-900 mb-8">История анализов</h1>

        <div className="bg-white shadow rounded-lg">
          <div className="overflow-x-auto">
            {isLoading ? (
              <div className="px-6 py-8 text-center text-gray-500">Загрузка...</div>
            ) : error ? (
              <div className="px-6 py-8 text-center text-red-500">
                Ошибка при загрузке данных
              </div>
            ) : !filteredRequests || filteredRequests.length === 0 ? (
              <div className="px-6 py-8 text-center text-gray-500">
                <p>История пуста</p>
              </div>
            ) : (
              <table className="min-w-full divide-y divide-gray-200">
                <thead className="bg-gray-50">
                  <tr>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                      ID
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                      Тип
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                      Статус
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                      Создано
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                      Обновлено
                    </th>
                    <th className="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">
                      Действия
                    </th>
                  </tr>
                </thead>
                <tbody className="bg-white divide-y divide-gray-200">
                  {filteredRequests.map((request) => (
                    <tr key={request.id} className="hover:bg-gray-50">
                      <td className="px-6 py-4 whitespace-nowrap text-sm font-mono text-gray-500">
                        {request.id.slice(0, 8)}...
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
                        {request.text_query ? 'ЭКГ Анализ' : 'GPT Запрос'}
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
                        {formatDate(request.created_at)}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                        {formatDate(request.updated_at)}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
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
        </div>
      </div>
    </Layout>
  );
}

