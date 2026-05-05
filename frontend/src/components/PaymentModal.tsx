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
  const [promoCode, setPromoCode] = useState('');
  const [promoDiscount, setPromoDiscount] = useState<number | null>(null);
  const [promoError, setPromoError] = useState<string | null>(null);

  const subscriptionPrice = quota.subscription_price_kopecks || 0;
  const finalPrice = promoDiscount !== null ? Math.max(0, subscriptionPrice - (subscriptionPrice * promoDiscount) / 100) : subscriptionPrice;

  const handleApplyPromoCode = async () => {
    if (!promoCode.trim()) {
      setPromoError('Введите промокод');
      return;
    }
    setPromoError(null);
    try {
      const response = await fetch('/api/v1/promo/validate', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ code: promoCode }),
      });
      const data = await response.json();
      if (data.is_valid) {
        setPromoDiscount(data.discount_percent);
      } else {
        setPromoError(data.reason || 'Неверный промокод');
      }
    } catch {
      setPromoError('Ошибка при проверке промокода');
    }
  };

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
                <div className="text-right">
                  {promoDiscount ? (
                    <>
                      <span className="text-sm text-gray-500 line-through">{formatPrice(subscriptionPrice)} &#8381;</span>
                      <span className="block text-2xl font-bold text-rose-600">{formatPrice(finalPrice)} &#8381;</span>
                      <span className="text-xs text-green-600">Скидка {promoDiscount}%</span>
                    </>
                  ) : (
                    <span className="text-2xl font-bold text-rose-600">{formatPrice(subscriptionPrice)} &#8381;</span>
                  )}
                </div>
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

            <div className="mb-4 p-4 border border-gray-200 rounded-lg">
              <label className="block text-sm font-medium text-gray-700 mb-2">Промокод (опционально)</label>
              <div className="flex gap-2">
                <input
                  type="text"
                  value={promoCode}
                  onChange={(e) => {
                    setPromoCode(e.target.value.toUpperCase());
                    setPromoError(null);
                  }}
                  placeholder="Введите код"
                  disabled={promoDiscount !== null}
                  className="flex-1 px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-rose-500 disabled:bg-gray-50"
                />
                <button
                  onClick={handleApplyPromoCode}
                  disabled={promoDiscount !== null || !promoCode.trim()}
                  className="px-3 py-2 bg-gray-200 text-gray-700 rounded-lg text-sm font-medium hover:bg-gray-300 disabled:opacity-50 transition-colors"
                >
                  Применить
                </button>
                {promoDiscount !== null && (
                  <button
                    onClick={() => {
                      setPromoCode('');
                      setPromoDiscount(null);
                      setPromoError(null);
                    }}
                    className="px-2 py-2 text-gray-600 hover:text-gray-800"
                  >
                    ✕
                  </button>
                )}
              </div>
              {promoError && <p className="text-xs text-red-600 mt-1">{promoError}</p>}
            </div>

            <button
              onClick={handleSubscribe}
              disabled={isLoading}
              className="w-full py-3 bg-rose-600 text-white rounded-lg font-medium hover:bg-rose-700 disabled:opacity-50 transition-colors"
            >
              {isLoading ? 'Переход к оплате...' : `Оформить: ${formatPrice(finalPrice)} ₽`}
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
