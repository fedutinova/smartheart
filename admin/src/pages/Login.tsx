import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { authAPI, saveTokens, clearTokens, type LoginResponse } from '@/services/api';

function parseRolesFromJWT(token: string): string[] {
  try {
    const payload = JSON.parse(atob(token.split('.')[1]));
    return payload.roles ?? [];
  } catch {
    return [];
  }
}

export function Login() {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const navigate = useNavigate();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      const tokens = await authAPI.login(email, password);
      saveTokens(tokens.access_token, tokens.refresh_token);
    } catch {
      setError('Неверный email или пароль');
      setLoading(false);
      return;
    }

    try {
      const profile = await authAPI.me();
      const roles = profile.roles ?? parseRolesFromJWT(tokens.access_token);
      if (!roles.includes('admin')) {
        clearTokens();
        setError('Нет прав администратора');
        return;
      }
      navigate('/');
    } catch (err) {
      console.error('Failed to fetch profile:', err);
      clearTokens();
      setError('Не удалось проверить права доступа');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-50">
      <div className="max-w-sm w-full bg-white shadow rounded-xl p-8">
        <h1 className="text-xl font-bold text-gray-900 mb-6 text-center">SmartHeart Admin</h1>
        <form onSubmit={handleSubmit} className="space-y-4">
          {error && (
            <div className="bg-red-50 border border-red-200 text-red-700 px-3 py-2 rounded text-sm">
              {error}
            </div>
          )}
          <input
            type="email"
            required
            placeholder="Email"
            className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-rose-500 focus:border-rose-500"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
          />
          <input
            type="password"
            required
            placeholder="Пароль"
            className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-rose-500 focus:border-rose-500"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
          <button
            type="submit"
            disabled={loading}
            className="w-full py-2 bg-rose-600 text-white rounded-lg font-medium hover:bg-rose-700 disabled:opacity-50 text-sm"
          >
            {loading ? 'Вход...' : 'Войти'}
          </button>
        </form>
      </div>
    </div>
  );
}
