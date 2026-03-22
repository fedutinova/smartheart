import { useQuery } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { Layout } from '@/components/Layout';
import { profileAPI, authAPI } from '@/services/api';
import { useAuthStore } from '@/store/auth';
import { useQuota } from '@/hooks/useQuota';
import { PaymentModal } from '@/components/PaymentModal';
import { ROUTES, REFRESH_TOKEN_KEY } from '@/config';
import { storage } from '@/utils/storage';
import { useState } from 'react';

function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString('ru-RU', {
    day: 'numeric',
    month: 'long',
    year: 'numeric',
  });
}

function formatPrice(kopecks: number): string {
  const rub = Math.floor(kopecks / 100);
  const kop = kopecks % 100;
  return kop === 0 ? `${rub}` : `${rub}.${String(kop).padStart(2, '0')}`;
}

export function Account() {
  const { data: profile, isLoading: profileLoading } = useQuery({
    queryKey: ['profile'],
    queryFn: () => profileAPI.getMe(),
  });
  const { quota, isLoading: quotaLoading } = useQuota();
  const [showPayment, setShowPayment] = useState(false);
  const navigate = useNavigate();
  const { logout } = useAuthStore();

  const handleLogout = async () => {
    const refreshToken = storage.get(REFRESH_TOKEN_KEY);
    if (refreshToken) {
      try { await authAPI.logout(refreshToken); } catch { /* ignore */ }
    }
    logout();
    navigate(ROUTES.LOGIN);
  };

  const isLoading = profileLoading || quotaLoading;

  if (isLoading) {
    return (
      <Layout>
        <div className="text-center py-12 text-gray-400">Загрузка...</div>
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
                <p className="text-sm text-gray-900 mt-0.5">{formatDate(profile.created_at)}</p>
              </div>
            </div>
          </div>
        )}

        {/* Quota & Subscription */}
        {quota && (
          <div className="bg-white shadow rounded-xl p-6 mb-6">
            <h2 className="text-sm font-medium text-gray-400 mb-4">Анализы</h2>

            <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-6">
              <div className="bg-gray-50 rounded-lg p-4 text-center">
                <p className="text-3xl font-semibold text-gray-900">{quota.free_remaining}</p>
                <p className="text-xs text-gray-400 mt-1">бесплатных сегодня</p>
              </div>
              <div className="bg-gray-50 rounded-lg p-4 text-center">
                <p className="text-3xl font-semibold text-rose-600">{quota.paid_analyses_remaining}</p>
                <p className="text-xs text-gray-400 mt-1">оплаченных</p>
              </div>
              <div className="bg-gray-50 rounded-lg p-4 text-center">
                <p className="text-3xl font-semibold text-gray-900">{quota.used_today}</p>
                <p className="text-xs text-gray-400 mt-1">использовано сегодня</p>
              </div>
            </div>

            <div className="flex items-center justify-between pt-4 border-t border-gray-100">
              <p className="text-sm text-gray-500">
                Бесплатный лимит: {quota.daily_limit} анализа в день.
                Стоимость дополнительного: {formatPrice(quota.price_per_analysis_kopecks)} руб.
              </p>
              <button
                onClick={() => setShowPayment(true)}
                className="px-4 py-2 bg-rose-600 text-white text-sm rounded-lg hover:bg-rose-700 transition-colors"
              >
                Купить анализы
              </button>
            </div>
          </div>
        )}

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
