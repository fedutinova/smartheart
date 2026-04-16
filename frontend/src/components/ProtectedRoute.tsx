import { Navigate } from 'react-router-dom';
import { useAuthStore } from '@/store/auth';
import { ROUTES } from '@/config';

export function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isInitializing } = useAuthStore();

  if (isInitializing) {
    return (
      <div className="min-h-screen bg-gray-50 flex items-center justify-center">
        <div role="status" aria-label="Загрузка" className="h-8 w-8 border-4 border-rose-200 border-t-rose-600 rounded-full animate-spin" />
      </div>
    );
  }

  if (!isAuthenticated) {
    return <Navigate to={ROUTES.LOGIN} replace />;
  }

  return <>{children}</>;
}
