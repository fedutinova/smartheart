import { useEffect, useState } from 'react';
import { adminAPI, type AdminStats } from '@/services/api';

function StatCard({ label, value, sub }: { label: string; value: string | number; sub?: string }) {
  return (
    <div className="bg-white rounded-lg shadow p-5">
      <p className="text-sm text-gray-500">{label}</p>
      <p className="text-2xl font-bold text-gray-900 mt-1">{value}</p>
      {sub && <p className="text-xs text-gray-400 mt-1">{sub}</p>}
    </div>
  );
}

function DailyChart({ data }: { data: { date: string; count: number }[] }) {
  if (!data || data.length === 0) {
    return <p className="text-sm text-gray-400">Нет данных за последние 30 дней</p>;
  }

  const max = Math.max(...data.map((d) => d.count), 1);

  return (
    <div className="bg-white rounded-lg shadow p-5">
      <h2 className="text-sm font-medium text-gray-500 mb-4">Анализов за последние 30 дней</h2>
      <div className="flex items-end gap-[2px] h-32">
        {data.map((d) => {
          const pct = (d.count / max) * 100;
          const fmtDate = new Date(d.date).toLocaleDateString('ru-RU', { day: 'numeric', month: 'short' });
          return (
            <div key={d.date} className="flex-1 group relative flex flex-col items-center justify-end h-full">
              <div
                className="w-full bg-rose-500 rounded-t transition-all group-hover:bg-rose-600"
                style={{ height: `${Math.max(pct, 2)}%` }}
              />
              <div className="absolute -top-8 bg-gray-800 text-white text-xs px-2 py-1 rounded opacity-0 group-hover:opacity-100 transition-opacity whitespace-nowrap pointer-events-none">
                {fmtDate}: {d.count}
              </div>
            </div>
          );
        })}
      </div>
      <div className="flex justify-between mt-2 text-[10px] text-gray-400">
        <span>{new Date(data[0].date).toLocaleDateString('ru-RU', { day: 'numeric', month: 'short' })}</span>
        <span>{new Date(data[data.length - 1].date).toLocaleDateString('ru-RU', { day: 'numeric', month: 'short' })}</span>
      </div>
    </div>
  );
}

function SatisfactionBar({ pct }: { pct: number }) {
  const color = pct >= 80 ? 'bg-green-500' : pct >= 50 ? 'bg-yellow-500' : 'bg-red-500';
  return (
    <div className="bg-white rounded-lg shadow p-5">
      <h2 className="text-sm font-medium text-gray-500 mb-2">Удовлетворённость ответами чата</h2>
      <div className="flex items-center gap-3">
        <div className="flex-1 bg-gray-100 rounded-full h-3 overflow-hidden">
          <div className={`h-full rounded-full ${color} transition-all`} style={{ width: `${pct}%` }} />
        </div>
        <span className="text-lg font-bold text-gray-900">{pct.toFixed(0)}%</span>
      </div>
    </div>
  );
}

export function Dashboard() {
  const [stats, setStats] = useState<AdminStats | null>(null);
  const [error, setError] = useState('');

  useEffect(() => {
    adminAPI.stats().then(setStats).catch(() => setError('Не удалось загрузить статистику'));
  }, []);

  if (error) return <div className="text-red-600">{error}</div>;
  if (!stats) return <div className="text-gray-400">Загрузка...</div>;

  const totalRequests = Object.values(stats.requests_by_status).reduce((a, b) => a + b, 0);
  const completed = stats.requests_by_status['completed'] ?? 0;
  const failed = stats.requests_by_status['failed'] ?? 0;
  const totalFeedback = stats.feedback_positive + stats.feedback_negative;

  return (
    <div className="space-y-6">
      <h1 className="text-xl font-bold text-gray-900">Статистика</h1>

      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard label="Пользователей" value={stats.users_count} />
        <StatCard
          label="Анализов"
          value={totalRequests}
          sub={`${completed} завершено, ${failed} ошибок`}
        />
        <StatCard
          label="Платежей"
          value={stats.payments_succeeded}
          sub={`${stats.payments_total_rub.toFixed(0)} руб.`}
        />
        <StatCard
          label="Обратная связь"
          value={totalFeedback}
          sub={`+${stats.feedback_positive} / −${stats.feedback_negative}`}
        />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <DailyChart data={stats.requests_daily ?? []} />
        {totalFeedback > 0 && <SatisfactionBar pct={stats.feedback_satisfaction_pct} />}
      </div>
    </div>
  );
}
