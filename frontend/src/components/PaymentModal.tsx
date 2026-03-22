import { useState } from 'react';
import { paymentAPI } from '@/services/api';
import type { QuotaInfo } from '@/types';

interface PaymentModalProps {
  quota: QuotaInfo;
  onClose: () => void;
  onSuccess?: () => void;
}

const PACK_OPTIONS = [
  { count: 1, label: '1 anализ' },
  { count: 5, label: '5 анализов' },
  { count: 10, label: '10 анализов' },
];

function formatPrice(kopecks: number): string {
  const rub = Math.floor(kopecks / 100);
  const kop = kopecks % 100;
  return kop === 0 ? `${rub}` : `${rub}.${String(kop).padStart(2, '0')}`;
}

export function PaymentModal({ quota, onClose }: PaymentModalProps) {
  const [selected, setSelected] = useState(5);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const pricePerAnalysis = quota.price_per_analysis_kopecks;

  const handlePurchase = async () => {
    setIsLoading(true);
    setError(null);
    try {
      const result = await paymentAPI.createPayment(selected);
      // Redirect to YooKassa payment page
      window.location.href = result.confirmation_url;
    } catch (err: any) {
      setError(err.response?.data?.error || 'Ошибка при создании платежа');
      setIsLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" onClick={onClose}>
      <div
        className="bg-white rounded-xl shadow-2xl max-w-md w-full p-6"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-bold text-gray-900">Купить анализы</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600">
            <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        <div className="mb-4 p-3 bg-rose-50 rounded-lg">
          <div className="text-sm text-rose-800">
            <p>Бесплатный лимит: <strong>{quota.daily_limit}</strong> анализа в день</p>
            <p>Использовано сегодня: <strong>{quota.used_today}</strong></p>
            {quota.paid_analyses_remaining > 0 && (
              <p>Оплаченных осталось: <strong>{quota.paid_analyses_remaining}</strong></p>
            )}
          </div>
        </div>

        <div className="space-y-2 mb-4">
          {PACK_OPTIONS.map((opt) => (
            <button
              key={opt.count}
              onClick={() => setSelected(opt.count)}
              className={`w-full flex items-center justify-between px-4 py-3 rounded-lg border-2 transition-colors ${
                selected === opt.count
                  ? 'border-rose-500 bg-rose-50'
                  : 'border-gray-200 hover:border-gray-300'
              }`}
            >
              <span className="font-medium text-gray-900">{opt.label}</span>
              <span className="text-gray-600">{formatPrice(pricePerAnalysis * opt.count)} &#8381;</span>
            </button>
          ))}
        </div>

        {error && (
          <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded-lg text-sm text-red-700">
            {error}
          </div>
        )}

        <button
          onClick={handlePurchase}
          disabled={isLoading}
          className="w-full py-3 bg-rose-600 text-white rounded-lg font-medium hover:bg-rose-700 disabled:opacity-50 transition-colors"
        >
          {isLoading ? 'Переход к оплате...' : `Оплатить ${formatPrice(pricePerAnalysis * selected)} \u20BD`}
        </button>

        <p className="mt-3 text-xs text-gray-400 text-center">
          Оплата через ЮKassa. После оплаты анализы будут доступны сразу.
        </p>
      </div>
    </div>
  );
}
