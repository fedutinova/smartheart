import { useState } from 'react';
import { Link, Navigate, useSearchParams } from 'react-router-dom';
import { useMutation } from '@tanstack/react-query';
import { authAPI } from '@/services/api';
import { useAuthStore } from '@/store/auth';
import { ROUTES } from '@/config';
import { getApiError, translatePasswordError } from '@/utils/apiError';
import { Layout } from '@/components/Layout';

const passwordAsciiOnly = /^[\x21-\x7E]+$/;

const EyeOpenIcon = (
  <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>
);

const EyeClosedIcon = (
  <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94"/><path d="M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19"/><line x1="1" y1="1" x2="23" y2="23"/></svg>
);

export function ResetPassword() {
  const [searchParams] = useSearchParams();
  const token = searchParams.get('token') || '';

  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [showConfirmPassword, setShowConfirmPassword] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState(false);
  const { isAuthenticated, isInitializing } = useAuthStore();

  const mutation = useMutation({
    mutationFn: () => authAPI.confirmPasswordReset(token, password),
    onSuccess: () => setSuccess(true),
    onError: (err: unknown) => {
      const { status, message } = getApiError(err);
      if (status === 401) {
        setError('Ссылка для сброса пароля недействительна или истекла. Запросите новую');
      } else if (status === 400) {
        setError(translatePasswordError(message));
      } else if (!status) {
        setError('Не удалось связаться с сервером. Проверьте подключение к интернету');
      } else {
        setError(message || 'Ошибка сброса пароля');
      }
    },
  });

  if (isInitializing) return null;
  if (isAuthenticated) return <Navigate to={ROUTES.DASHBOARD} replace />;

  if (!token) {
    return (
      <Layout>
        <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-rose-50 to-blue-50 py-12 px-4">
          <div className="max-w-md w-full bg-white shadow-xl rounded-2xl p-8 space-y-6 text-center">
            <h2 className="text-2xl font-extrabold text-gray-900">Некорректная ссылка</h2>
            <p className="text-sm text-gray-600">Ссылка для сброса пароля повреждена или отсутствует.</p>
            <Link
              to={ROUTES.FORGOT_PASSWORD}
              className="inline-block text-sm font-medium text-rose-600 hover:text-rose-500"
            >
              Запросить новую ссылку
            </Link>
          </div>
        </div>
      </Layout>
    );
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    mutation.reset();

    if (password !== confirmPassword) {
      setError('Пароли не совпадают');
      return;
    }

    if (!passwordAsciiOnly.test(password)) {
      setError('Пароль должен содержать только английские буквы, цифры и спецсимволы');
      return;
    }

    mutation.mutate();
  };

  return (
    <Layout>
      <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-rose-50 to-blue-50 py-12 px-4 sm:px-6 lg:px-8">
        <div className="max-w-md w-full bg-white shadow-xl rounded-2xl p-8 space-y-8 animate-scale-in">
          <div>
            <h2 className="text-center text-3xl font-extrabold text-gray-900">Новый пароль</h2>
            <p className="mt-2 text-center text-sm text-gray-600">Введите новый пароль для вашей учётной записи</p>
          </div>

          {success ? (
            <div className="space-y-4">
              <div className="bg-green-50 border border-green-200 text-green-800 px-4 py-3 rounded-xl text-sm">
                Пароль успешно изменён. Теперь вы можете войти с новым паролем.
              </div>
              <Link
                to={ROUTES.LOGIN}
                className="block w-full text-center py-3 px-4 border border-transparent text-sm font-medium rounded-xl text-white bg-rose-600 hover:bg-rose-700 transition-colors"
              >
                Войти
              </Link>
            </div>
          ) : (
            <form className="mt-6 space-y-5" onSubmit={handleSubmit}>
              {error && (
                <div className="bg-red-50 border border-red-200 text-red-800 px-4 py-3 rounded-xl">{error}</div>
              )}
              <div className="space-y-4">
                <div className="relative">
                  <label htmlFor="password" className="sr-only">Новый пароль</label>
                  <input
                    id="password"
                    name="password"
                    type={showPassword ? 'text' : 'password'}
                    required
                    autoComplete="new-password"
                    minLength={10}
                    maxLength={72}
                    pattern="[\x21-\x7E]+"
                    title="Используйте английские буквы, цифры и спецсимволы"
                    className="appearance-none relative block w-full px-4 py-3 pr-11 border border-gray-300 placeholder-gray-400 text-gray-900 rounded-xl focus:outline-none focus:ring-2 focus:ring-rose-500 focus:border-rose-500 sm:text-sm"
                    placeholder="Новый пароль (от 10 до 72 символов)"
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
                    {showPassword ? EyeOpenIcon : EyeClosedIcon}
                  </button>
                </div>
                <div className="relative">
                  <label htmlFor="confirm-password" className="sr-only">Повторите пароль</label>
                  <input
                    id="confirm-password"
                    name="confirm-password"
                    type={showConfirmPassword ? 'text' : 'password'}
                    required
                    autoComplete="new-password"
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
                    {showConfirmPassword ? EyeOpenIcon : EyeClosedIcon}
                  </button>
                </div>
              </div>

              <button
                type="submit"
                disabled={mutation.isPending}
                className="group relative w-full flex justify-center py-3 px-4 border border-transparent text-sm font-medium rounded-xl text-white bg-rose-600 hover:bg-rose-700 active:scale-95 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-rose-500 disabled:opacity-50 transition-all duration-150"
              >
                {mutation.isPending ? 'Сохранение...' : 'Сохранить новый пароль'}
              </button>
            </form>
          )}
        </div>
      </div>
    </Layout>
  );
}

