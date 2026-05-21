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

        {/* Survey banner */}
        <div className="mb-4 sm:mb-6 rounded-xl bg-gradient-to-r from-indigo-600 to-purple-600 px-5 py-4 flex flex-col sm:flex-row sm:items-center gap-3">
          <div className="flex-1 min-w-0">
            <p className="text-sm font-semibold text-white">Помогите улучшить сервис</p>
            <p className="text-sm text-indigo-100 mt-0.5">Пройдите короткий опрос о вашем опыте — займёт 2–3 минуты</p>
          </div>
          <a
            href="https://docs.google.com/forms/d/e/1FAIpQLSerfoqa5aM-7A-Yei7gZxs8CxGgmBu_dlMx4XqolAijSTbFBg/viewform"
            target="_blank"
            rel="noopener noreferrer"
            className="shrink-0 inline-block px-4 py-2 text-sm font-medium text-indigo-700 bg-white hover:bg-indigo-50 rounded-lg transition-colors"
          >
            Пройти опрос →
          </a>
        </div>

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

        {/* Onboarding — shown only before the first analysis */}
        {!isLoading && recentRequests.length === 0 && (
          <div className="mb-6 sm:mb-8 rounded-xl border border-gray-200 bg-white p-6 sm:p-8">
            <h2 className="text-lg font-semibold text-gray-900 mb-1">Добро пожаловать!</h2>
            <p className="text-sm text-gray-500 mb-6">Вот как работает сервис — всего 3 шага</p>
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-5 mb-7">
              {[
                {
                  num: '1',
                  title: 'Загрузите ЭКГ',
                  desc: 'Сфотографируйте плёнку или загрузите скан. Сервис автоматически обезличит изображение.',
                },
                {
                  num: '2',
                  title: 'Получите результат',
                  desc: 'Измерения по 12 отведениям, индексы ГЛЖ и справочная интерпретация за несколько секунд.',
                },
                {
                  num: '3',
                  title: 'Задайте вопрос',
                  desc: 'Уточните детали у чат-бота прямо на странице результата — с учётом вашей ЭКГ.',
                },
              ].map((step) => (
                <div key={step.num} className="flex gap-3 items-start">
                  <div className="w-7 h-7 rounded-full bg-rose-100 text-rose-600 text-sm font-bold flex items-center justify-center shrink-0 mt-0.5">
                    {step.num}
                  </div>
                  <div>
                    <p className="text-sm font-semibold text-gray-900">{step.title}</p>
                    <p className="text-xs text-gray-500 mt-0.5 leading-relaxed">{step.desc}</p>
                  </div>
                </div>
              ))}
            </div>
            <Link
              to={ROUTES.ANALYZE}
              className="inline-block px-5 py-2.5 text-sm font-medium text-white bg-rose-600 hover:bg-rose-700 rounded-xl transition-colors"
            >
              Начать первый анализ →
            </Link>
          </div>
        )}

        <div className="bg-white shadow rounded-lg">
          <div className="px-4 sm:px-6 py-3 sm:py-4 border-b border-gray-200">
            <h2 className="text-base sm:text-lg font-medium text-gray-900">История</h2>
          </div>
          {isLoading ? (
            <DashboardHistorySkeleton />
          ) : recentRequests.length === 0 ? (
            <div className="px-4 sm:px-6 py-6 text-center text-sm text-gray-400">
              Анализы появятся здесь после первой отправки
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
