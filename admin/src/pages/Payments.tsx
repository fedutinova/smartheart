import { useEffect, useState, useCallback } from 'react';
import { adminAPI, type AdminPayment, type Paginated } from '@/services/api';
import { Pagination } from '@/components/Pagination';

const STATUS_COLORS: Record<string, string> = {
  succeeded: 'bg-green-100 text-green-700',
  pending: 'bg-yellow-100 text-yellow-700',
  canceled: 'bg-gray-100 text-gray-500',
};

export function Payments() {
  const [data, setData] = useState<Paginated<AdminPayment> | null>(null);
  const [offset, setOffset] = useState(0);

  const load = useCallback(() => {
    adminAPI.payments(20, offset).then(setData);
  }, [offset]);

  useEffect(() => { load(); }, [load]);

  const fmt = (d: string) => new Date(d).toLocaleDateString('ru-RU');
  const rub = (kopecks: number) => `${(kopecks / 100).toFixed(0)} \u20BD`;

  return (
    <div>
      <h1 className="text-xl font-bold text-gray-900 mb-6">Платежи</h1>

      {!data ? (
        <div className="text-gray-400">Загрузка...</div>
      ) : (
        <>
          <div className="bg-white shadow rounded-lg overflow-hidden">
            <table className="w-full text-sm">
              <thead className="bg-gray-50 border-b border-gray-200">
                <tr>
                  <th className="text-left px-4 py-3 font-medium text-gray-600">Email</th>
                  <th className="text-center px-4 py-3 font-medium text-gray-600">Тип</th>
                  <th className="text-right px-4 py-3 font-medium text-gray-600">Сумма</th>
                  <th className="text-center px-4 py-3 font-medium text-gray-600">Статус</th>
                  <th className="text-right px-4 py-3 font-medium text-gray-600">Дата</th>
                </tr>
              </thead>
              <tbody>
                {(data.data ?? []).map((p) => (
                  <tr key={p.id} className="border-b border-gray-100 hover:bg-gray-50">
                    <td className="px-4 py-3 text-gray-700">{p.user_email}</td>
                    <td className="px-4 py-3 text-center text-gray-600">
                      {p.payment_type === 'subscription' ? 'Подписка' : 'Анализы'}
                    </td>
                    <td className="px-4 py-3 text-right font-medium text-gray-900">{rub(p.amount_kopecks)}</td>
                    <td className="px-4 py-3 text-center">
                      <span className={`px-2 py-0.5 rounded-full text-xs font-medium ${STATUS_COLORS[p.status] ?? 'bg-gray-100 text-gray-500'}`}>
                        {p.status}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-right text-gray-500">{fmt(p.created_at)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <Pagination total={data.total} limit={data.limit} offset={data.offset} onChange={setOffset} />
        </>
      )}
    </div>
  );
}
