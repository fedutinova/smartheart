import { useEffect, useState, useCallback } from 'react';
import { adminAPI, type AdminFeedback, type Paginated } from '@/services/api';
import { Pagination } from '@/components/Pagination';

export function Feedback() {
  const [data, setData] = useState<Paginated<AdminFeedback> | null>(null);
  const [offset, setOffset] = useState(0);
  const [expanded, setExpanded] = useState<string | null>(null);

  const load = useCallback(() => {
    adminAPI.feedback(20, offset).then(setData);
  }, [offset]);

  useEffect(() => { load(); }, [load]);

  const fmt = (d: string) => new Date(d).toLocaleDateString('ru-RU', {
    day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit',
  });

  const truncate = (s: string, n: number) => s.length > n ? s.slice(0, n) + '...' : s;

  return (
    <div>
      <h1 className="text-xl font-bold text-gray-900 mb-6">Обратная связь для чат-бота</h1>

      {!data ? (
        <div className="text-gray-400">Загрузка...</div>
      ) : (
        <>
          <div className="space-y-3">
            {(data.data ?? []).map((f) => (
              <div
                key={f.id}
                className="bg-white shadow rounded-lg p-4 cursor-pointer hover:shadow-md transition-shadow"
                onClick={() => setExpanded(expanded === f.id ? null : f.id)}
              >
                <div className="flex items-center justify-between mb-2">
                  <div className="flex items-center gap-3">
                    <span className={`w-6 h-6 rounded-full flex items-center justify-center text-xs font-bold ${
                      f.rating === 1 ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'
                    }`}>
                      {f.rating === 1 ? '+' : '-'}
                    </span>
                    <span className="text-sm text-gray-500">{f.user_email}</span>
                  </div>
                  <span className="text-xs text-gray-400">{fmt(f.created_at)}</span>
                </div>

                <p className="text-sm font-medium text-gray-900">
                  {expanded === f.id ? f.question : truncate(f.question, 120)}
                </p>

                {expanded === f.id && (
                  <div className="mt-3 pt-3 border-t border-gray-100">
                    <p className="text-xs text-gray-500 mb-1">Ответ:</p>
                    <p className="text-sm text-gray-700 whitespace-pre-wrap">{f.answer}</p>
                  </div>
                )}
              </div>
            ))}
          </div>
          <Pagination total={data.total} limit={data.limit} offset={data.offset} onChange={setOffset} />
        </>
      )}
    </div>
  );
}
