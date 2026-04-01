import { useEffect, useState, useCallback } from 'react';
import { adminAPI, type AdminUser, type Paginated } from '@/services/api';
import { Pagination } from '@/components/Pagination';

export function Users() {
  const [data, setData] = useState<Paginated<AdminUser> | null>(null);
  const [search, setSearch] = useState('');
  const [offset, setOffset] = useState(0);

  const load = useCallback(() => {
    adminAPI.users(20, offset, search).then(setData);
  }, [offset, search]);

  useEffect(() => { load(); }, [load]);

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    setOffset(0);
    load();
  };

  const fmt = (d: string) => new Date(d).toLocaleDateString('ru-RU');

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-xl font-bold text-gray-900">Пользователи</h1>
        <form onSubmit={handleSearch} className="flex gap-2">
          <input
            type="text"
            placeholder="Поиск по email/имени..."
            className="px-3 py-1.5 border border-gray-300 rounded-lg text-sm w-64 focus:ring-1 focus:ring-rose-500"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
          <button type="submit" className="px-3 py-1.5 bg-rose-600 text-white rounded-lg text-sm hover:bg-rose-700">
            Найти
          </button>
        </form>
      </div>

      {!data ? (
        <div className="text-gray-400">Загрузка...</div>
      ) : (
        <>
          <div className="bg-white shadow rounded-lg overflow-hidden">
            <table className="w-full text-sm">
              <thead className="bg-gray-50 border-b border-gray-200">
                <tr>
                  <th className="text-left px-4 py-3 font-medium text-gray-600">Пользователь</th>
                  <th className="text-left px-4 py-3 font-medium text-gray-600">Email</th>
                  <th className="text-center px-4 py-3 font-medium text-gray-600">Роль</th>
                  <th className="text-center px-4 py-3 font-medium text-gray-600">Анализов</th>
                  <th className="text-center px-4 py-3 font-medium text-gray-600">Подписка</th>
                  <th className="text-right px-4 py-3 font-medium text-gray-600">Регистрация</th>
                </tr>
              </thead>
              <tbody>
                {(data.data ?? []).map((u) => (
                  <tr key={u.id} className="border-b border-gray-100 hover:bg-gray-50">
                    <td className="px-4 py-3 font-medium text-gray-900">{u.username}</td>
                    <td className="px-4 py-3 text-gray-600">{u.email}</td>
                    <td className="px-4 py-3 text-center">
                      <span className={`px-2 py-0.5 rounded-full text-xs font-medium ${
                        u.role === 'admin' ? 'bg-purple-100 text-purple-700' : 'bg-gray-100 text-gray-600'
                      }`}>
                        {u.role}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-center text-gray-700">{u.requests_count}</td>
                    <td className="px-4 py-3 text-center">
                      {u.subscription_expires_at ? (
                        <span className="text-green-600 text-xs">до {fmt(u.subscription_expires_at)}</span>
                      ) : (
                        <span className="text-gray-400 text-xs">-</span>
                      )}
                    </td>
                    <td className="px-4 py-3 text-right text-gray-500">{fmt(u.created_at)}</td>
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
