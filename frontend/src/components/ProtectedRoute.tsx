import { Navigate } from 'react-router-dom';
import { useAuthStore } from '@/store/auth';
import { ROUTES } from '@/config';

export function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated } = useAuthStore();

  if (!isAuthenticated) {
    return <Navigate to={ROUTES.LOGIN} replace />;
  }

  return <>{children}</>;
}

