import { useState } from 'react';
import { useNavigate, Navigate, Link } from 'react-router-dom';
import { useMutation } from '@tanstack/react-query';
import { authAPI } from '@/services/api';
import { useAuthStore } from '@/store/auth';
import { ROUTES } from '@/config';
import { getApiError, translateValidationError, ERR_RATE_LIMIT, ERR_NETWORK } from '@/utils/apiError';
import { PasswordInput } from '@/components/PasswordInput';
import { Layout } from '@/components/Layout';

const passwordAsciiOnly = /^[\x21-\x7E]+$/;

export function Register() {
  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [error, setError] = useState('');
  const [agreed, setAgreed] = useState(false);
  const navigate = useNavigate();
  const { isAuthenticated, isInitializing } = useAuthStore();

  const registerMutation = useMutation({
    mutationFn: () => authAPI.register({ username, email, password }),
    onSuccess: () => {
      navigate(ROUTES.LOGIN);
    },
    onError: (err: unknown) => {
      const { status, message } = getApiError(err);
      if (status === 409) {
        setError('Пользователь с таким email уже существует');
      } else if (status === 429) {
        setError(ERR_RATE_LIMIT);
      } else if (status === 400) {
        setError(translateValidationError(message));
      } else if (!status) {
        setError(ERR_NETWORK);
      } else {
        setError('Ошибка регистрации');
      }
    },
  });

  if (isInitializing) {
    return null;
  }

  if (isAuthenticated) {
    return <Navigate to={ROUTES.DASHBOARD} replace />;
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    registerMutation.reset();

    if (password !== confirmPassword) {
      setError('Пароли не совпадают');
      return;
    }

    if (!passwordAsciiOnly.test(password)) {
      setError('Пароль должен содержать только английские буквы, цифры и спецсимволы');
      return;
    }

    registerMutation.mutate();
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
                  autoComplete="email"
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
              <PasswordInput
                id="password"
                name="password"
                placeholder="Пароль (от 10 до 72 символов)"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                minLength={10}
                maxLength={72}
                pattern="[\x21-\x7E]+"
                title="Используйте английские буквы, цифры и спецсимволы"
                autoComplete="new-password"
              />
              <PasswordInput
                id="confirm-password"
                name="confirm-password"
                placeholder="Повторите пароль"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                minLength={10}
                maxLength={72}
                pattern="[\x21-\x7E]+"
                title="Используйте английские буквы, цифры и спецсимволы"
                onInvalid={(e) => {
                  const input = e.target as HTMLInputElement;
                  input.setCustomValidity('');
                  if (input.validity.valueMissing) input.setCustomValidity('Повторите пароль');
                  else if (input.validity.tooShort) input.setCustomValidity('Пароль должен быть не менее 10 символов');
                  else if (input.validity.patternMismatch) input.setCustomValidity('Используйте английские буквы, цифры и спецсимволы');
                }}
              />
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
                disabled={registerMutation.isPending || !agreed}
                className="group relative w-full flex justify-center py-3 px-4 border border-transparent text-sm font-medium rounded-xl text-white bg-rose-600 hover:bg-rose-700 active:scale-95 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-rose-500 disabled:opacity-50 transition-all duration-150"
              >
                {registerMutation.isPending ? 'Регистрация...' : 'Зарегистрироваться'}
              </button>
            </div>
          </form>
        </div>
      </div>
    </Layout>
  );
}
