import { useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuthStore } from '@/store/auth';
import { authAPI } from '@/services/api';
import { ROUTES } from '@/config';

export function useLogout() {
  const navigate = useNavigate();
  const { logout } = useAuthStore();

  return useCallback(async () => {
    try {
      await authAPI.logout();
    } catch { /* server may be unreachable — log out locally anyway */ }
    logout();
    navigate(ROUTES.LOGIN);
  }, [logout, navigate]);
}
