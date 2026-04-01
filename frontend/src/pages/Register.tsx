import { useState } from 'react';
import { useNavigate, Navigate, Link } from 'react-router-dom';
import { authAPI } from '@/services/api';
import { useAuthStore } from '@/store/auth';
import { ROUTES } from '@/config';
import { Layout } from '@/components/Layout';

export function Register() {
  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [agreed, setAgreed] = useState(false);
  const navigate = useNavigate();
  const { isAuthenticated } = useAuthStore();

  if (isAuthenticated) {
    return <Navigate to={ROUTES.DASHBOARD} replace />;
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);

    try {
      await authAPI.register({ username, email, password });
      navigate(ROUTES.LOGIN);
    } catch (err: any) {
      const serverMsg = err.response?.data?.error;
      if (err.response?.status === 409) {
        setError('Неверный email или пароль');
      } else if (err.response?.status === 429) {
        setError('Слишком много попыток. Попробуйте позже');
      } else if (!err.response) {
        setError('Не удалось связаться с сервером. Проверьте подключение к интернету');
      } else {
        setError(serverMsg || 'Ошибка регистрации');
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
              Регистрация в <Link to={ROUTES.HOME} style={{ fontFamily: "'Prosto One', cursive" }} className="text-rose-600 hover:text-rose-700 transition-colors">Умное сердце</Link>
            </h2>
            <p className="mt-2 text-center text-sm text-gray-600">
              Уже есть аккаунт?{' '}
              <Link to={ROUTES.LOGIN} className="font-medium text-rose-600 hover:text-rose-500">
                Войти
              </Link>
            </p>
          </div>
          <form className="mt-6 space-y-5" onSubmit={handleSubmit}>
            {error && (
              <div className="bg-red-50 border border-red-200 text-red-800 px-4 py-3 rounded-xl">
                {error}
              </div>
            )}
            <div className="space-y-4">
              <div>
                <label htmlFor="username" className="sr-only">
                  Имя пользователя
                </label>
                <input
                  id="username"
                  name="username"
                  type="text"
                  required
                  className="appearance-none relative block w-full px-4 py-3 border border-gray-300 placeholder-gray-400 text-gray-900 rounded-xl focus:outline-none focus:ring-2 focus:ring-rose-500 focus:border-rose-500 sm:text-sm"
                  placeholder="Имя пользователя"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                />
              </div>
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
                  minLength={10}
                  maxLength={72}
                  className="appearance-none relative block w-full px-4 py-3 border border-gray-300 placeholder-gray-400 text-gray-900 rounded-xl focus:outline-none focus:ring-2 focus:ring-rose-500 focus:border-rose-500 sm:text-sm"
                  placeholder="Пароль (от 10 до 72 символов)"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                />
              </div>
            </div>

            <label className="flex items-start gap-2.5 cursor-pointer">
              <input
                type="checkbox"
                checked={agreed}
                onChange={(e) => setAgreed(e.target.checked)}
                className="mt-0.5 h-4 w-4 rounded border-gray-300 text-rose-600 focus:ring-rose-500"
              />
              <span className="text-xs text-gray-500 leading-relaxed">
                Я принимаю{' '}
                <Link to={ROUTES.TERMS} className="text-rose-600 hover:text-rose-500 underline" target="_blank">
                  условия оферты
                </Link>{' '}
                и даю согласие на обработку персональных данных в соответствии с{' '}
                <Link to={ROUTES.PRIVACY} className="text-rose-600 hover:text-rose-500 underline" target="_blank">
                  политикой конфиденциальности
                </Link>
              </span>
            </label>

            <div>
              <button
                type="submit"
                disabled={loading || !agreed}
                className="group relative w-full flex justify-center py-3 px-4 border border-transparent text-sm font-medium rounded-xl text-white bg-rose-600 hover:bg-rose-700 active:scale-95 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-rose-500 disabled:opacity-50 transition-all duration-150"
              >
                {loading ? 'Регистрация...' : 'Зарегистрироваться'}
              </button>
            </div>
          </form>
        </div>
      </div>
    </Layout>
  );
}

