import { useState, useEffect, useRef } from 'react';
import { useMutation } from '@tanstack/react-query';
import { authAPI } from '@/services/api';
import { getApiError, translatePasswordError } from '@/utils/apiError';
import { useLogout } from '@/hooks/useLogout';

interface ChangePasswordModalProps {
  onClose: () => void;
}

export function ChangePasswordModal({ onClose }: ChangePasswordModalProps) {
  const [oldPassword, setOldPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');
  const handleLogout = useLogout();
  const logoutTimerRef = useRef<ReturnType<typeof setTimeout>>();

  useEffect(() => {
    return () => clearTimeout(logoutTimerRef.current);
  }, []);

  const mutation = useMutation({
    mutationFn: () => authAPI.changePassword(oldPassword, newPassword),
    onSuccess: () => {
      setSuccess('Пароль изменён. Вы будете перенаправлены на страницу входа...');
      setError('');
      logoutTimerRef.current = setTimeout(() => handleLogout(), 2000);
    },
    onError: (err: unknown) => {
      const { status, message } = getApiError(err);
      if (status === 401) {
        setError('Неверный текущий пароль');
      } else if (status === 400) {
        setError(translatePasswordError(message));
      } else if (!status) {
        setError('Не удалось связаться с сервером');
      } else {
        setError(message || 'Ошибка смены пароля');
      }
      setSuccess('');
    },
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setSuccess('');
    mutation.reset();

    if (newPassword !== confirmPassword) {
      setError('Пароли не совпадают');
      return;
    }

    mutation.mutate();
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" onClick={onClose}>
      <div
        className="bg-white rounded-xl shadow-2xl max-w-md w-full p-4 sm:p-6 animate-scale-in"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between mb-5">
          <h2 className="text-lg font-bold text-gray-900">Смена пароля</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600" aria-label="Закрыть">
            <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        <form className="space-y-4" onSubmit={handleSubmit}>
          {error && (
            <div className="bg-red-50 border border-red-200 text-red-800 px-4 py-3 rounded-xl text-sm">
              {error}
            </div>
          )}
          {success && (
            <div className="bg-green-50 border border-green-200 text-green-800 px-4 py-3 rounded-xl text-sm">
              {success}
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
            disabled={mutation.isPending}
            className="w-full py-2.5 bg-rose-600 text-white text-sm font-medium rounded-lg hover:bg-rose-700 active:scale-95 transition-all duration-150 disabled:opacity-50"
          >
            {mutation.isPending ? 'Сохранение...' : 'Изменить пароль'}
          </button>
        </form>
      </div>
    </div>
  );
}
