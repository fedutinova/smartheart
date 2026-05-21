import { useState } from 'react';
import { Layout } from '@/components/Layout';
import { PaymentModal } from '@/components/PaymentModal';
import { useQuota } from '@/hooks/useQuota';
import { formatPrice } from '@/utils/format';

export function Pricing() {
  const { quota, isLoading } = useQuota();
  const [showPayment, setShowPayment] = useState(false);

  const hasActiveSub =
    !!quota?.subscription_expires_at &&
    new Date(quota.subscription_expires_at) > new Date();

  return (
    <Layout>
      {showPayment && quota && (
        <PaymentModal
          quota={quota}
          onClose={() => setShowPayment(false)}
          onSuccess={() => setShowPayment(false)}
        />
      )}

      <div className="max-w-2xl mx-auto">
        <h1 className="text-2xl font-semibold text-gray-900 mb-2">Тарифы</h1>
        <p className="text-sm text-gray-500 mb-8">
          Выберите подходящий вариант для работы с сервисом
        </p>

        <div className="grid gap-4 sm:grid-cols-2">
          {/* Free plan */}
          <div className="rounded-xl border border-gray-200 bg-white p-6">
            <div className="mb-4">
              <span className="text-base font-semibold text-gray-900">Бесплатно</span>
              <p className="mt-1 text-2xl font-bold text-gray-900">0 ₽</p>
            </div>
            <ul className="space-y-2 text-sm text-gray-600 mb-6">
              <li className="flex items-center gap-2">
                <CheckIcon />
                {quota
                  ? `${quota.free_limit} бесплатных анализа ЭКГ при регистрации`
                  : 'Ограниченное число анализов ЭКГ'}
              </li>
              <li className="flex items-center gap-2">
                <CheckIcon />
                Чат-бот по кардиологии (без контекста ЭКГ)
              </li>
              <li className="flex items-center gap-2">
                <CheckIcon />
                История запросов
              </li>
            </ul>
            {!hasActiveSub && quota && (
              <p className="text-xs text-gray-400">
                Осталось: {quota.free_remaining} из {quota.free_limit}
              </p>
            )}
            {hasActiveSub && (
              <p className="text-xs text-gray-400">Текущий тариф не активен</p>
            )}
          </div>

          {/* Subscription plan */}
          <div className="rounded-xl border-2 border-rose-200 bg-rose-50/40 p-6 relative">
            <div className="absolute -top-3 left-4">
              <span className="bg-rose-600 text-white text-xs font-medium px-2.5 py-0.5 rounded-full">
                Рекомендуем
              </span>
            </div>
            <div className="mb-4">
              <span className="text-base font-semibold text-gray-900">Подписка</span>
              <div className="mt-1">
                {isLoading ? (
                  <div className="h-8 w-24 bg-gray-200 rounded animate-pulse" />
                ) : quota ? (
                  <p className="text-2xl font-bold text-rose-600">
                    {formatPrice(quota.subscription_price_kopecks)} ₽
                    <span className="text-sm font-normal text-gray-500"> / 30 дней</span>
                  </p>
                ) : (
                  <p className="text-2xl font-bold text-rose-600">—</p>
                )}
              </div>
            </div>
            <ul className="space-y-2 text-sm text-gray-600 mb-6">
              <li className="flex items-center gap-2">
                <CheckIcon className="text-rose-500" />
                Безлимитные анализы ЭКГ
              </li>
              <li className="flex items-center gap-2">
                <CheckIcon className="text-rose-500" />
                Без дневных ограничений
              </li>
              <li className="flex items-center gap-2">
                <CheckIcon className="text-rose-500" />
                Чат-бот с контекстом результата ЭКГ
              </li>
              <li className="flex items-center gap-2">
                <CheckIcon className="text-rose-500" />
                Продлевается по необходимости
              </li>
            </ul>
            {hasActiveSub ? (
              <div className="rounded-lg bg-green-50 border border-green-200 px-3 py-2 text-sm text-green-800">
                Подписка активна до{' '}
                {new Date(quota!.subscription_expires_at!).toLocaleDateString('ru-RU', {
                  day: 'numeric',
                  month: 'long',
                  year: 'numeric',
                })}
              </div>
            ) : (
              <button
                onClick={() => setShowPayment(true)}
                disabled={!quota}
                className="w-full py-2.5 bg-rose-600 text-white text-sm font-medium rounded-xl hover:bg-rose-700 disabled:opacity-50 transition-colors"
              >
                Оформить подписку
              </button>
            )}
          </div>
        </div>

        <p className="mt-6 text-xs text-gray-400 text-center">
          Оплата через ЮKassa. Подписка активируется сразу после оплаты.
          Сервис не является медицинским изделием.
        </p>
      </div>
    </Layout>
  );
}

function CheckIcon({ className = 'text-gray-400' }: { className?: string }) {
  return (
    <svg
      className={`w-4 h-4 flex-shrink-0 ${className}`}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      strokeWidth={2}
    >
      <path strokeLinecap="round" strokeLinejoin="round" d="m4.5 12.75 6 6 9-13.5" />
    </svg>
  );
}
