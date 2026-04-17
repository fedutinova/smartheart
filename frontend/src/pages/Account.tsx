import { useQuery, useMutation } from '@tanstack/react-query';
import { Layout } from '@/components/Layout';
import { profileAPI, authAPI } from '@/services/api';
import { useQuota } from '@/hooks/useQuota';
import { useLogout } from '@/hooks/useLogout';
import { PaymentModal } from '@/components/PaymentModal';
import { useState, useEffect, useRef } from 'react';
import { formatPrice, formatDateLong } from '@/utils/format';
import { AccountSkeleton } from '@/components/Skeleton';
import { getApiError, translatePasswordError } from '@/utils/apiError';

export function Account() {
  const { data: profile, isLoading: profileLoading } = useQuery({
    queryKey: ['profile'],
    queryFn: () => profileAPI.getMe(),
  });
  const { quota, isLoading: quotaLoading } = useQuota();
  const [showPayment, setShowPayment] = useState(false);
  const handleLogout = useLogout();

  const [oldPassword, setOldPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [passwordError, setPasswordError] = useState('');
  const [passwordSuccess, setPasswordSuccess] = useState('');
  const logoutTimerRef = useRef<ReturnType<typeof setTimeout>>();

  useEffect(() => {
    return () => clearTimeout(logoutTimerRef.current);
  }, []);

  const changePasswordMutation = useMutation({
    mutationFn: () => authAPI.changePassword(oldPassword, newPassword),
    onSuccess: () => {
      setPasswordSuccess('Пароль изменён. Вы будете перенаправлены на страницу входа...');
      setOldPassword('');
      setNewPassword('');
      setConfirmPassword('');
      setPasswordError('');
      logoutTimerRef.current = setTimeout(() => handleLogout(), 2000);
    },
    onError: (err: unknown) => {
      const { status, message } = getApiError(err);
      if (status === 401) {
        setPasswordError('Неверный текущий пароль');
      } else if (status === 400) {
        setPasswordError(translatePasswordError(message));
      } else if (!status) {
        setPasswordError('Не удалось связаться с сервером');
      } else {
        setPasswordError(message || 'Ошибка смены пароля');
      }
      setPasswordSuccess('');
    },
  });

  const handleChangePassword = (e: React.FormEvent) => {
    e.preventDefault();
    setPasswordError('');
    setPasswordSuccess('');
    changePasswordMutation.reset();

    if (newPassword !== confirmPassword) {
      setPasswordError('Пароли не совпадают');
      return;
    }

    changePasswordMutation.mutate();
  };

  const isLoading = profileLoading || quotaLoading;
  const hasActiveSub = quota?.subscription_expires_at && new Date(quota.subscription_expires_at) > new Date();

  if (isLoading) {
    return (
      <Layout>
        <AccountSkeleton />
      </Layout>
    );
  }

  return (
    <Layout>
      {showPayment && quota && (
        <PaymentModal
          quota={quota}
          onClose={() => setShowPayment(false)}
          onSuccess={() => setShowPayment(false)}
        />
      )}
      <div className="max-w-4xl mx-auto">
        <h1 className="text-2xl font-semibold text-gray-900 mb-6">Личный кабинет</h1>

        {/* Profile */}
        {profile && (
          <div className="bg-white shadow rounded-xl p-6 mb-6">
            <h2 className="text-sm font-medium text-gray-400 mb-4">Профиль</h2>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <p className="text-xs text-gray-400">Имя</p>
                <p className="text-sm text-gray-900 mt-0.5">{profile.username}</p>
              </div>
              <div>
                <p className="text-xs text-gray-400">Email</p>
                <p className="text-sm text-gray-900 mt-0.5">{profile.email}</p>
              </div>
              <div>
                <p className="text-xs text-gray-400">Дата регистрации</p>
                <p className="text-sm text-gray-900 mt-0.5">{formatDateLong(profile.created_at)}</p>
              </div>
            </div>
          </div>
        )}

        {/* Subscription */}
        {quota && (
          <div className="bg-white shadow rounded-xl p-6 mb-6">
            <h2 className="text-sm font-medium text-gray-400 mb-4">Подписка</h2>
            {hasActiveSub ? (
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-sm font-medium text-green-700">Активна</p>
                  <p className="text-xs text-gray-400 mt-0.5">
                    до {formatDateLong(quota.subscription_expires_at!)}, безлимитные анализы
                  </p>
                </div>
                <span className="px-3 py-1 text-xs font-medium bg-green-100 text-green-700 rounded-full">
                  Активна
                </span>
              </div>
            ) : (
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-sm text-gray-600">Нет активной подписки</p>
                  <p className="text-xs text-gray-400 mt-0.5">
                    Безлимитные анализы на 30 дней, {formatPrice(quota.subscription_price_kopecks || 0)} руб/мес
                  </p>
                </div>
                <button
                  onClick={() => setShowPayment(true)}
                  className="px-4 py-2 bg-rose-600 text-white text-sm rounded-lg hover:bg-rose-700 transition-colors"
                >
                  Оформить
                </button>
              </div>
            )}
          </div>
        )}

        {/* Quota */}
        {quota && (
          <div className="bg-white shadow rounded-xl p-6 mb-6">
            <h2 className="text-sm font-medium text-gray-400 mb-4">Анализы</h2>

            {hasActiveSub ? (
              <div className="bg-gray-50 rounded-lg p-4 text-center">
                <p className="text-3xl font-semibold text-gray-900">{quota.used_today}</p>
                <p className="text-xs text-gray-400 mt-1">выполнено сегодня</p>
              </div>
            ) : (
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div className="bg-gray-50 rounded-lg p-4 text-center">
                  <p className="text-3xl font-semibold text-gray-900">{quota.free_remaining}</p>
                  <p className="text-xs text-gray-400 mt-1">бесплатных сегодня</p>
                </div>
                <div className="bg-gray-50 rounded-lg p-4 text-center">
                  <p className="text-3xl font-semibold text-gray-900">{quota.used_today}</p>
                  <p className="text-xs text-gray-400 mt-1">использовано сегодня</p>
                </div>
              </div>
            )}

            {!hasActiveSub && (
              <p className="text-xs text-gray-400 mt-4">
                Бесплатный лимит: {quota.daily_limit} анализа в день. Для безлимита оформите подписку.
              </p>
            )}
          </div>
        )}

        {/* Change Password */}
        <div className="bg-white shadow rounded-xl p-6 mb-6">
          <h2 className="text-sm font-medium text-gray-400 mb-4">Смена пароля</h2>
          <form className="space-y-4" onSubmit={handleChangePassword}>
            {passwordError && (
              <div className="bg-red-50 border border-red-200 text-red-800 px-4 py-3 rounded-xl text-sm">
                {passwordError}
              </div>
            )}
            {passwordSuccess && (
              <div className="bg-green-50 border border-green-200 text-green-800 px-4 py-3 rounded-xl text-sm">
                {passwordSuccess}
              </div>
            )}
            <div>
              <label htmlFor="old-password" className="block text-xs text-gray-400 mb-1">Текущий пароль</label>
              <input
                id="old-password"
                type="password"
                required
                autoComplete="current-password"
                className="appearance-none block w-full px-4 py-2.5 border border-gray-300 placeholder-gray-400 text-gray-900 rounded-lg focus:outline-none focus:ring-2 focus:ring-rose-500 focus:border-rose-500 sm:text-sm"
                value={oldPassword}
                onChange={(e) => setOldPassword(e.target.value)}
              />
            </div>
            <div>
              <label htmlFor="new-password" className="block text-xs text-gray-400 mb-1">Новый пароль</label>
              <input
                id="new-password"
                type="password"
                required
                autoComplete="new-password"
                minLength={10}
                maxLength={72}
                pattern="[\x21-\x7E]+"
                title="Используйте английские буквы, цифры и спецсимволы"
                className="appearance-none block w-full px-4 py-2.5 border border-gray-300 placeholder-gray-400 text-gray-900 rounded-lg focus:outline-none focus:ring-2 focus:ring-rose-500 focus:border-rose-500 sm:text-sm"
                placeholder="От 10 до 72 символов"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
              />
            </div>
            <div>
              <label htmlFor="confirm-new-password" className="block text-xs text-gray-400 mb-1">Повторите новый пароль</label>
              <input
                id="confirm-new-password"
                type="password"
                required
                autoComplete="new-password"
                minLength={10}
                maxLength={72}
                className="appearance-none block w-full px-4 py-2.5 border border-gray-300 placeholder-gray-400 text-gray-900 rounded-lg focus:outline-none focus:ring-2 focus:ring-rose-500 focus:border-rose-500 sm:text-sm"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
              />
            </div>
            <button
              type="submit"
              disabled={changePasswordMutation.isPending}
              className="px-4 py-2 bg-rose-600 text-white text-sm rounded-lg hover:bg-rose-700 transition-colors disabled:opacity-50"
            >
              {changePasswordMutation.isPending ? 'Сохранение...' : 'Изменить пароль'}
            </button>
          </form>
        </div>

        <button
          onClick={handleLogout}
          className="w-full sm:w-auto px-4 py-2 text-sm text-gray-400 hover:text-gray-600 transition-colors"
        >
          Выйти из аккаунта
        </button>
      </div>
    </Layout>
  );
}
