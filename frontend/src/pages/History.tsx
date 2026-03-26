import { Link } from 'react-router-dom';
import { formatDate, formatStatus, getStatusColor, formatECGParams } from '@/utils/format';
import { Layout } from '@/components/Layout';
import { useUserRequests } from '@/hooks/useUserRequests';
import { useSessionState } from '@/hooks/useSessionState';

const PAGE_SIZE = 20;

export function History() {
  const [page, setPage] = useSessionState('history_page', 0);
  const offset = page * PAGE_SIZE;
  const { requests: filteredRequests, isLoading, error, total } = useUserRequests(PAGE_SIZE, offset);

  const totalPages = Math.ceil(total / PAGE_SIZE);
  const hasNext = page < totalPages - 1;
  const hasPrev = page > 0;

  return (
    <Layout>
      <div>
        <h1 className="text-2xl font-semibold text-gray-900 mb-6">История анализов</h1>

        <div className="bg-white shadow rounded-lg">
          {isLoading ? (
            <div className="px-4 sm:px-6 py-8 text-center text-gray-500">Загрузка...</div>
          ) : error ? (
            <div className="px-4 sm:px-6 py-8 text-center text-red-500">
              Ошибка при загрузке данных
            </div>
          ) : !filteredRequests || filteredRequests.length === 0 ? (
            <div className="px-4 sm:px-6 py-8 text-center text-gray-500">
              <p>История пуста</p>
            </div>
          ) : (
            <>
              {/* Mobile: card list */}
              <div className="sm:hidden divide-y divide-gray-200">
                {filteredRequests.map((request) => (
                  <Link
                    key={request.id}
                    to={`/results/${request.id}`}
                    className="block px-4 py-3 hover:bg-gray-50 active:bg-gray-100"
                  >
                    <div className="flex items-center justify-between mb-1">
                      <span className="text-sm text-gray-700">{formatECGParams(request) || <span className="font-mono text-gray-500">{request.id.slice(0, 8)}...</span>}</span>
                      <span
                        className={`ml-2 flex-shrink-0 px-2 inline-flex text-xs leading-5 font-semibold rounded-full ${getStatusColor(request.status)}`}
                      >
                        {formatStatus(request.status)}
                      </span>
                    </div>
                    <div className="text-xs text-gray-400">
                      {formatDate(request.created_at)}
                    </div>
                  </Link>
                ))}
              </div>

              {/* Desktop: table */}
              <div className="hidden sm:block overflow-x-auto">
                <table className="min-w-full divide-y divide-gray-200">
                  <thead className="bg-gray-50">
                    <tr>
                      <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Параметры</th>
                      <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Статус</th>
                      <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Создано</th>
                      <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Обновлено</th>
                      <th className="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">Действия</th>
                    </tr>
                  </thead>
                  <tbody className="bg-white divide-y divide-gray-200">
                    {filteredRequests.map((request) => (
                      <tr key={request.id} className="hover:bg-gray-50">
                        <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-700">
                          {formatECGParams(request) || <span className="font-mono text-gray-400">{request.id.slice(0, 8)}...</span>}
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap">
                          <span
                            className={`px-2 inline-flex text-xs leading-5 font-semibold rounded-full ${getStatusColor(request.status)}`}
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
                          <Link to={`/results/${request.id}`} className="text-rose-600 hover:text-rose-900">
                            Просмотр
                          </Link>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </>
          )}

          {/* Pagination */}
          {total > PAGE_SIZE && (
            <div className="px-4 sm:px-6 py-3 border-t border-gray-200 flex items-center justify-between">
              <p className="text-xs sm:text-sm text-gray-500">
                {offset + 1}&ndash;{Math.min(offset + PAGE_SIZE, total)} из {total}
              </p>
              <div className="flex gap-2">
                <button
                  onClick={() => setPage((p: number) => p - 1)}
                  disabled={!hasPrev}
                  className="px-3 py-1.5 text-sm border border-gray-300 rounded-md hover:bg-gray-50 active:bg-gray-100 disabled:opacity-40 disabled:cursor-default"
                >
                  Назад
                </button>
                <button
                  onClick={() => setPage((p: number) => p + 1)}
                  disabled={!hasNext}
                  className="px-3 py-1.5 text-sm border border-gray-300 rounded-md hover:bg-gray-50 active:bg-gray-100 disabled:opacity-40 disabled:cursor-default"
                >
                  Вперёд
                </button>
              </div>
            </div>
          )}
        </div>
      </div>
    </Layout>
  );
}
