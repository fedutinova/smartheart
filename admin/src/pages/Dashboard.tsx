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

  return (
    <div>
      <h1 className="text-xl font-bold text-gray-900 mb-6">Статистика</h1>
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
          label="Feedback"
          value={stats.feedback_positive + stats.feedback_negative}
          sub={`+${stats.feedback_positive} / -${stats.feedback_negative}`}
        />
      </div>
    </div>
  );
}
