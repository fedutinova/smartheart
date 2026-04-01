import { useState } from 'react';
import { paymentAPI } from '@/services/api';
import { formatPrice } from '@/utils/format';
import { getApiError } from '@/utils/apiError';
import type { QuotaInfo } from '@/types';

interface PaymentModalProps {
  quota: QuotaInfo;
  onClose: () => void;
  onSuccess?: () => void;
}

export function PaymentModal({ quota, onClose }: PaymentModalProps) {
  const hasActiveSub = quota.subscription_expires_at && new Date(quota.subscription_expires_at) > new Date();
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const subscriptionPrice = quota.subscription_price_kopecks || 0;

  const handleSubscribe = async () => {
    setIsLoading(true);
    setError(null);
    try {
      const result = await paymentAPI.createSubscription();
      window.location.href = result.confirmation_url;
    } catch (err: unknown) {
      setError(getApiError(err).message || 'Ошибка при создании платежа');
      setIsLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" onClick={onClose}>
      <div
        className="bg-white rounded-xl shadow-2xl max-w-md w-full p-4 sm:p-6"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between mb-5">
          <h2 className="text-lg font-bold text-gray-900">Подписка</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600" aria-label="Закрыть">
            <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        {hasActiveSub ? (
          <div className="p-4 bg-green-50 border border-green-200 rounded-lg">
            <p className="text-sm font-medium text-green-800">Подписка активна</p>
            <p className="text-xs text-green-600 mt-1">
              до {new Date(quota.subscription_expires_at!).toLocaleDateString('ru-RU', { day: 'numeric', month: 'long', year: 'numeric' })}
            </p>
          </div>
        ) : (
          <>
            <div className="p-5 border-2 border-rose-200 bg-rose-50/50 rounded-xl mb-4">
              <div className="flex items-baseline justify-between mb-2">
                <span className="text-lg font-bold text-gray-900">30 дней</span>
                <span className="text-2xl font-bold text-rose-600">{formatPrice(subscriptionPrice)} &#8381;</span>
              </div>
              <ul className="space-y-1.5 text-sm text-gray-600">
                <li className="flex items-center gap-2">
                  <svg className="w-4 h-4 text-rose-500 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="m4.5 12.75 6 6 9-13.5" />
                  </svg>
                  Безлимитные анализы ЭКГ
                </li>
                <li className="flex items-center gap-2">
                  <svg className="w-4 h-4 text-rose-500 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="m4.5 12.75 6 6 9-13.5" />
                  </svg>
                  Без дневных ограничений
                </li>
                <li className="flex items-center gap-2">
                  <svg className="w-4 h-4 text-rose-500 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="m4.5 12.75 6 6 9-13.5" />
                  </svg>
                  Продлевается по необходимости
                </li>
              </ul>
            </div>
            <button
              onClick={handleSubscribe}
              disabled={isLoading}
              className="w-full py-3 bg-rose-600 text-white rounded-lg font-medium hover:bg-rose-700 disabled:opacity-50 transition-colors"
            >
              {isLoading ? 'Переход к оплате...' : `Оформить: ${formatPrice(subscriptionPrice)} \u20BD`}
            </button>
          </>
        )}

        {error && (
          <div className="mt-4 p-3 bg-red-50 border border-red-200 rounded-lg text-sm text-red-700">
            {error}
          </div>
        )}

        <p className="mt-4 text-xs text-gray-400 text-center">
          Оплата через ЮKassa. Подписка активируется сразу после оплаты.
        </p>
      </div>
    </div>
  );
}
