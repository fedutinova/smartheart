import { useState } from 'react';
import { useNavigate, Navigate, Link } from 'react-router-dom';
import { authAPI } from '@/services/api';
import { useAuthStore } from '@/store/auth';
import { ROUTES } from '@/config';
import { getApiError } from '@/utils/apiError';
import { Layout } from '@/components/Layout';

const passwordAsciiOnly = /^[\x21-\x7E]+$/;

export function Register() {
  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [confirmPassword, setConfirmPassword] = useState('');
  const [showConfirmPassword, setShowConfirmPassword] = useState(false);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [agreed, setAgreed] = useState(false);
  const navigate = useNavigate();
  const { isAuthenticated, isInitializing } = useAuthStore();

  if (isInitializing) {
    return null;
  }

  if (isAuthenticated) {
    return <Navigate to={ROUTES.DASHBOARD} replace />;
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');

    if (password !== confirmPassword) {
      setError('Пароли не совпадают');
      return;
    }

    if (!passwordAsciiOnly.test(password)) {
      setError('Пароль должен содержать только английские буквы, цифры и спецсимволы');
      return;
    }

    setLoading(true);

    try {
      await authAPI.register({ username, email, password });
      navigate(ROUTES.LOGIN);
    } catch (err: unknown) {
      const { status, message } = getApiError(err);
      if ((status === 400 || status === 409) && message.includes('already exists')) {
        setError('Пользователь с таким email или именем уже существует');
      } else if (status === 429) {
        setError('Слишком много попыток. Попробуйте позже');
      } else if (!status) {
        setError('Не удалось связаться с сервером. Проверьте подключение к интернету');
      } else {
        setError(message || 'Ошибка регистрации');
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
                  onInvalid={(e) => {
                    const input = e.target as HTMLInputElement;
                    input.setCustomValidity('');
                    if (input.validity.valueMissing) input.setCustomValidity('Введите имя пользователя');
                  }}
                  onInput={(e) => (e.target as HTMLInputElement).setCustomValidity('')}
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
                  onInvalid={(e) => {
                    const input = e.target as HTMLInputElement;
                    input.setCustomValidity('');
                    if (input.validity.valueMissing) input.setCustomValidity('Введите email адрес');
                    else if (input.validity.typeMismatch) input.setCustomValidity('Введите корректный email адрес');
                  }}
                  onInput={(e) => (e.target as HTMLInputElement).setCustomValidity('')}
                />
              </div>
              <div className="relative">
                <label htmlFor="password" className="sr-only">
                  Пароль
                </label>
                <input
                  id="password"
                  name="password"
                  type={showPassword ? 'text' : 'password'}
                  required
                  minLength={10}
                  maxLength={72}
                  pattern="[\x21-\x7E]+"
                  title="Используйте английские буквы, цифры и спецсимволы"
                  className="appearance-none relative block w-full px-4 py-3 pr-11 border border-gray-300 placeholder-gray-400 text-gray-900 rounded-xl focus:outline-none focus:ring-2 focus:ring-rose-500 focus:border-rose-500 sm:text-sm"
                  placeholder="Пароль (от 10 до 72 символов)"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  onInvalid={(e) => {
                    const input = e.target as HTMLInputElement;
                    input.setCustomValidity('');
                    if (input.validity.valueMissing) input.setCustomValidity('Введите пароль');
                    else if (input.validity.tooShort) input.setCustomValidity('Пароль должен быть не менее 10 символов');
                    else if (input.validity.patternMismatch) input.setCustomValidity('Используйте английские буквы, цифры и спецсимволы');
                  }}
                  onInput={(e) => (e.target as HTMLInputElement).setCustomValidity('')}
                />
                <button
                  type="button"
                  tabIndex={-1}
                  className="absolute inset-y-0 right-0 flex items-center pr-3 text-gray-400 hover:text-gray-600"
                  onClick={() => setShowPassword((v) => !v)}
                  aria-label={showPassword ? 'Скрыть пароль' : 'Показать пароль'}
                >
                  {showPassword ? (
                    <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>
                  ) : (
                    <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94"/><path d="M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19"/><line x1="1" y1="1" x2="23" y2="23"/></svg>
                  )}
                </button>
              </div>
              <div className="relative">
                <label htmlFor="confirm-password" className="sr-only">
                  Повторите пароль
                </label>
                <input
                  id="confirm-password"
                  name="confirm-password"
                  type={showConfirmPassword ? 'text' : 'password'}
                  required
                  minLength={10}
                  maxLength={72}
                  pattern="[\x21-\x7E]+"
                  title="Используйте английские буквы, цифры и спецсимволы"
                  className="appearance-none relative block w-full px-4 py-3 pr-11 border border-gray-300 placeholder-gray-400 text-gray-900 rounded-xl focus:outline-none focus:ring-2 focus:ring-rose-500 focus:border-rose-500 sm:text-sm"
                  placeholder="Повторите пароль"
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  onInvalid={(e) => {
                    const input = e.target as HTMLInputElement;
                    input.setCustomValidity('');
                    if (input.validity.valueMissing) input.setCustomValidity('Повторите пароль');
                    else if (input.validity.tooShort) input.setCustomValidity('Пароль должен быть не менее 10 символов');
                    else if (input.validity.patternMismatch) input.setCustomValidity('Используйте английские буквы, цифры и спецсимволы');
                  }}
                  onInput={(e) => (e.target as HTMLInputElement).setCustomValidity('')}
                />
                <button
                  type="button"
                  tabIndex={-1}
                  className="absolute inset-y-0 right-0 flex items-center pr-3 text-gray-400 hover:text-gray-600"
                  onClick={() => setShowConfirmPassword((v) => !v)}
                  aria-label={showConfirmPassword ? 'Скрыть пароль' : 'Показать пароль'}
                >
                  {showConfirmPassword ? (
                    <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>
                  ) : (
                    <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94"/><path d="M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19"/><line x1="1" y1="1" x2="23" y2="23"/></svg>
                  )}
                </button>
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
