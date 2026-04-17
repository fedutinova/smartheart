import { useState } from 'react';
import { Link, Navigate } from 'react-router-dom';
import { useMutation } from '@tanstack/react-query';
import { authAPI } from '@/services/api';
import { useAuthStore } from '@/store/auth';
import { ROUTES } from '@/config';
import { getApiError } from '@/utils/apiError';
import { Layout } from '@/components/Layout';

export function ForgotPassword() {
  const [email, setEmail] = useState('');
  const [error, setError] = useState('');
  const [sent, setSent] = useState(false);
  const { isAuthenticated, isInitializing } = useAuthStore();

  const mutation = useMutation({
    mutationFn: () => authAPI.requestPasswordReset(email),
    onSuccess: () => setSent(true),
    onError: (err: unknown) => {
      const { status, message } = getApiError(err);
      if (status === 429) {
        setError('Слишком много попыток. Попробуйте позже');
      } else if (!status) {
        setError('Не удалось связаться с сервером. Проверьте подключение к интернету');
      } else {
        setError(message || 'Ошибка отправки');
      }
    },
  });

  if (isInitializing) return null;
  if (isAuthenticated) return <Navigate to={ROUTES.DASHBOARD} replace />;

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    mutation.reset();
    mutation.mutate();
  };

  return (
    <Layout>
      <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-rose-50 to-blue-50 py-12 px-4 sm:px-6 lg:px-8">
        <div className="max-w-md w-full bg-white shadow-xl rounded-2xl p-8 space-y-8 animate-scale-in">
          <div>
            <h2 className="text-center text-3xl font-extrabold text-gray-900">Сброс пароля</h2>
          </div>

          {sent ? (
            <div className="space-y-4">
              <div className="bg-green-50 border border-green-200 text-green-800 px-4 py-3 rounded-xl text-sm">
                Проверьте указанную почту.
              </div>
              <p className="text-center text-sm text-gray-500">
                Не получили письмо? Проверьте папку «Спам» или{' '}
                <button
                  type="button"
                  className="text-rose-600 hover:text-rose-500 font-medium"
                  onClick={() => {
                    setSent(false);
                    mutation.reset();
                  }}
                >
                  попробуйте ещё раз
                </button>
              </p>
              <Link
                to={ROUTES.LOGIN}
                className="block w-full text-center py-3 px-4 border border-rose-300 text-sm font-medium rounded-xl text-rose-600 bg-white hover:bg-rose-50 active:scale-95 transition-all duration-150"
              >
                Вернуться ко входу
              </Link>
            </div>
          ) : (
            <form className="mt-6 space-y-5" onSubmit={handleSubmit}>
              {error && (
                <div className="bg-red-50 border border-red-200 text-red-800 px-4 py-3 rounded-xl">
                  {error}
                </div>
              )}
              <div>
                <label htmlFor="email" className="sr-only">Email</label>
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

              <button
                type="submit"
                disabled={mutation.isPending}
                className="group relative w-full flex justify-center py-3 px-4 border border-transparent text-sm font-medium rounded-xl text-white bg-rose-600 hover:bg-rose-700 active:scale-95 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-rose-500 disabled:opacity-50 transition-all duration-150"
              >
                {mutation.isPending ? 'Отправка...' : 'Отправить ссылку'}
              </button>

              <Link
                to={ROUTES.LOGIN}
                className="block w-full text-center py-3 px-4 border border-rose-300 text-sm font-medium rounded-xl text-rose-600 bg-white hover:bg-rose-50 active:scale-95 transition-all duration-150"
              >
                Вернуться ко входу
              </Link>
            </form>
          )}
        </div>
      </div>
    </Layout>
  );
}
