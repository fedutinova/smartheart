import { Link } from 'react-router-dom';
import { formatRelative, formatStatus, getStatusColor, formatECGParams } from '@/utils/format';
import { ROUTES } from '@/config';
import { Layout } from '@/components/Layout';
import { useUserRequests } from '@/hooks/useUserRequests';
import { usePendingJobs } from '@/hooks/usePendingJobs';
import { DashboardHistorySkeleton } from '@/components/Skeleton';

export function Dashboard() {
  const { requests, isLoading } = useUserRequests();
  const recentRequests = requests.slice(0, 5);
  const { jobs: pendingJobs } = usePendingJobs();

  return (
    <Layout>
      <div>
        {/* Pending jobs banner — resumable after refresh */}
        {pendingJobs.length > 0 && (
          <div className="mb-4 sm:mb-6 bg-amber-50 border border-amber-200 rounded-lg p-3 sm:p-4">
            <p className="text-sm font-medium text-amber-800 mb-2">
              {pendingJobs.length === 1 ? 'Есть незавершённый анализ' : `Есть незавершённые анализы (${pendingJobs.length})`}
            </p>
            <div className="flex flex-wrap gap-2">
              {pendingJobs.map((job) => (
                <Link
                  key={job.requestId}
                  to={`/results/${job.requestId}`}
                  className="inline-flex items-center px-3 py-1.5 text-sm bg-amber-100 text-amber-900 rounded-md hover:bg-amber-200 transition-colors"
                >
                  {job.requestId.slice(0, 8)}... · Посмотреть
                </Link>
              ))}
            </div>
          </div>
        )}

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 sm:gap-6 mb-6 sm:mb-8">
          <Link
            to={ROUTES.ANALYZE}
            className="block bg-gradient-to-r from-rose-400 to-rose-500 text-white rounded-xl shadow-md p-5 sm:p-6 hover:from-rose-500 hover:to-rose-600 active:from-rose-600 active:to-rose-700 transition"
          >
            <div>
              <h2 className="text-xl sm:text-2xl font-bold">Новый анализ ЭКГ</h2>
              <p className="text-rose-100 mt-0.5 sm:mt-1 text-sm">Загрузите изображение для анализа</p>
            </div>
          </Link>
          <Link
            to={ROUTES.KNOWLEDGE_BASE}
            className="block bg-gradient-to-r from-purple-400 to-purple-500 text-white rounded-xl shadow-md p-5 sm:p-6 hover:from-purple-500 hover:to-purple-600 active:from-purple-600 active:to-purple-700 transition"
          >
            <div>
              <h2 className="text-xl sm:text-2xl font-bold">Чат-бот</h2>
              <p className="text-purple-100 mt-0.5 sm:mt-1 text-sm">Задайте вопрос по ЭКГ и кардиологии</p>
            </div>
          </Link>
        </div>

        <div className="bg-white shadow rounded-lg">
          <div className="px-4 sm:px-6 py-3 sm:py-4 border-b border-gray-200">
            <h2 className="text-base sm:text-lg font-medium text-gray-900">История</h2>
          </div>
          {isLoading ? (
            <DashboardHistorySkeleton />
          ) : recentRequests.length === 0 ? (
            <div className="px-4 sm:px-6 py-8 text-center text-gray-500">
              <p>У вас пока нет анализов</p>
              <Link
                to={ROUTES.ANALYZE}
                className="text-rose-600 hover:text-rose-500 mt-2 inline-block"
              >
                Создать первый анализ →
              </Link>
            </div>
          ) : (
            <>
              {/* Mobile: card list */}
              <div className="sm:hidden divide-y divide-gray-200">
                {recentRequests.map((request) => (
                  <Link
                    key={request.id}
                    to={`/results/${request.id}`}
                    className="flex items-center justify-between px-4 py-3 hover:bg-gray-50 active:bg-gray-100"
                  >
                    <div className="min-w-0 flex-1">
                      <p className="text-sm text-gray-700">{formatECGParams(request) || request.id.slice(0, 8) + '...'}</p>
                      <p className="text-xs text-gray-400 mt-0.5">{formatRelative(request.created_at)}</p>
                    </div>
                    <span
                      className={`ml-3 flex-shrink-0 px-2 inline-flex text-xs leading-5 font-semibold rounded-full ${getStatusColor(request.status)}`}
                    >
                      {formatStatus(request.status)}
                    </span>
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
                      <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Действия</th>
                    </tr>
                  </thead>
                  <tbody className="bg-white divide-y divide-gray-200">
                    {recentRequests.map((request) => (
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
                          {formatRelative(request.created_at)}
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap text-sm font-medium">
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
          {recentRequests.length > 0 && (
            <div className="px-4 sm:px-6 py-3 sm:py-4 border-t border-gray-200 text-center">
              <Link to={ROUTES.HISTORY} className="text-rose-600 hover:text-rose-500 font-medium text-sm sm:text-base">
                Показать все →
              </Link>
            </div>
          )}
        </div>
      </div>
    </Layout>
  );
}
