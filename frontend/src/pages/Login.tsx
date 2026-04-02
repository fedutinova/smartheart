import { useState, useEffect } from 'react';
import { useNavigate, Navigate, Link } from 'react-router-dom';
import { authAPI } from '@/services/api';
import { useAuthStore } from '@/store/auth';
import { ROUTES, AUTH_ERROR_KEY } from '@/config';
import { getApiError } from '@/utils/apiError';
import { Layout } from '@/components/Layout';

export function Login() {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [authNotice, setAuthNotice] = useState('');
  const [loading, setLoading] = useState(false);
  const navigate = useNavigate();
  const { login, isAuthenticated } = useAuthStore();

  // Show auth error from redirect (expired session, network error, etc.)
  useEffect(() => {
    const reason = sessionStorage.getItem(AUTH_ERROR_KEY);
    if (reason) {
      setAuthNotice(reason);
      sessionStorage.removeItem(AUTH_ERROR_KEY);
    }
  }, []);

  if (isAuthenticated) {
    return <Navigate to={ROUTES.DASHBOARD} replace />;
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setAuthNotice('');
    setLoading(true);

    try {
      const tokens = await authAPI.login({ email, password });
      login(tokens);
      navigate(ROUTES.DASHBOARD);
    } catch (err: unknown) {
      const { status, message } = getApiError(err);
      if (status === 401) {
        setError('Неверный email или пароль');
      } else if (status === 429) {
        setError('Слишком много попыток. Попробуйте позже');
      } else if (!status) {
        setError('Не удалось связаться с сервером. Проверьте подключение к интернету');
      } else {
        setError(message || 'Ошибка входа');
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <Layout>
      <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-rose-50 to-blue-50 py-12 px-4 sm:px-6 lg:px-8">
        <div className="max-w-md w-full bg-white shadow-xl rounded-2xl p-8 space-y-8 animate-scale-in">
          <div>
            <h2 className="text-center text-3xl font-extrabold text-gray-900">
              Вход в{' '}
              <Link to={ROUTES.HOME} className="hover:text-rose-600 transition-colors" style={{ fontFamily: "'Prosto One', cursive" }}>
                Умное сердце
              </Link>
            </h2>
            <p className="mt-2 text-center text-sm text-gray-600">
              Или{' '}
              <Link to={ROUTES.REGISTER} className="font-medium text-rose-600 hover:text-rose-500">
                зарегистрироваться
              </Link>
            </p>
          </div>
          <form className="mt-6 space-y-5" onSubmit={handleSubmit}>
            {authNotice && (
              <div className="bg-amber-50 border border-amber-200 text-amber-800 px-4 py-3 rounded-xl text-sm">
                {authNotice}
              </div>
            )}
            {error && (
              <div className="bg-red-50 border border-red-200 text-red-800 px-4 py-3 rounded-xl">
                {error}
              </div>
            )}
            <div className="space-y-4">
              <div>
                <label htmlFor="email" className="sr-only">
                  Email
                </label>
                <input
                  id="email"
                  name="email"
                  type="email"
                  required
                  className="appearance-none relative block w-full px-4 py-3 border border-gray-300 placeholder-gray-400 text-gray-900 rounded-xl focus:outline-none focus:ring-2 focus:ring-rose-500 focus:border-rose-500 sm:text-sm"
                  placeholder="Email адрес"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                />
              </div>
              <div>
                <label htmlFor="password" className="sr-only">
                  Пароль
                </label>
                <input
                  id="password"
                  name="password"
                  type="password"
                  required
                  className="appearance-none relative block w-full px-4 py-3 border border-gray-300 placeholder-gray-400 text-gray-900 rounded-xl focus:outline-none focus:ring-2 focus:ring-rose-500 focus:border-rose-500 sm:text-sm"
                  placeholder="Пароль"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                />
              </div>
            </div>

            <div>
              <button
                type="submit"
                disabled={loading}
                className="group relative w-full flex justify-center py-3 px-4 border border-transparent text-sm font-medium rounded-xl text-white bg-rose-600 hover:bg-rose-700 active:scale-95 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-rose-500 disabled:opacity-50 transition-all duration-150"
              >
                {loading ? 'Вход...' : 'Войти'}
              </button>
            </div>
          </form>
          <p className="text-center text-[11px] text-gray-400 mt-4">
            <Link to={ROUTES.TERMS} className="hover:text-gray-500">Оферта</Link>
            {' · '}
            <Link to={ROUTES.PRIVACY} className="hover:text-gray-500">Конфиденциальность</Link>
          </p>
        </div>
      </div>
    </Layout>
  );
}

