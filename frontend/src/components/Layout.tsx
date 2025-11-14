import { useEffect } from 'react';
import { Link, useNavigate, useLocation } from 'react-router-dom';
import { useAuthStore } from '@/store/auth';
import { ROUTES, REFRESH_TOKEN_KEY } from '@/config';
import { storage } from '@/utils/storage';
import { authAPI } from '@/services/api';

export function Layout({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, logout } = useAuthStore();
  const navigate = useNavigate();
  const location = useLocation();

  const handleLogout = async () => {
    const refreshToken = storage.get<string>(REFRESH_TOKEN_KEY);
    if (refreshToken) {
      try {
        await authAPI.logout(refreshToken);
      } catch (error) {
        console.error('Logout error:', error);
      }
    }
    logout();
    navigate(ROUTES.LOGIN);
  };

  const isActive = (path: string) => location.pathname === path;

  if (!isAuthenticated) {
    return <>{children}</>;
  }

  return (
    <div className="min-h-screen bg-gray-50">
      <nav className="bg-white border-b border-gray-200">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex justify-between h-16">
            <div className="flex">
              <div className="flex-shrink-0 flex items-center">
                <span className="text-2xl font-bold text-blue-600">❤️ Умное сердце</span>
              </div>
              <div className="hidden sm:ml-6 sm:flex sm:space-x-8">
                <Link
                  to={ROUTES.DASHBOARD}
                  className={`inline-flex items-center px-1 pt-1 text-sm font-medium ${
                    isActive(ROUTES.DASHBOARD)
                      ? 'text-blue-600 border-b-2 border-blue-600'
                      : 'text-gray-500 hover:text-gray-700'
                  }`}
                >
                  Главная
                </Link>
                <Link
                  to={ROUTES.ANALYZE}
                  className={`inline-flex items-center px-1 pt-1 text-sm font-medium ${
                    isActive(ROUTES.ANALYZE)
                      ? 'text-blue-600 border-b-2 border-blue-600'
                      : 'text-gray-500 hover:text-gray-700'
                  }`}
                >
                  Анализ
                </Link>
                <Link
                  to={ROUTES.HISTORY}
                  className={`inline-flex items-center px-1 pt-1 text-sm font-medium ${
                    isActive(ROUTES.HISTORY)
                      ? 'text-blue-600 border-b-2 border-blue-600'
                      : 'text-gray-500 hover:text-gray-700'
                  }`}
                >
                  История
                </Link>
                <Link
                  to={ROUTES.KNOWLEDGE_BASE}
                  className={`inline-flex items-center px-1 pt-1 text-sm font-medium ${
                    isActive(ROUTES.KNOWLEDGE_BASE)
                      ? 'text-blue-600 border-b-2 border-blue-600'
                      : 'text-gray-500 hover:text-gray-700'
                  }`}
                >
                  База знаний
                </Link>
                <Link
                  to={ROUTES.CONTACTS}
                  className={`inline-flex items-center px-1 pt-1 text-sm font-medium ${
                    isActive(ROUTES.CONTACTS)
                      ? 'text-blue-600 border-b-2 border-blue-600'
                      : 'text-gray-500 hover:text-gray-700'
                  }`}
                >
                  Контакты
                </Link>
              </div>
            </div>
            <div className="flex items-center">
              <button
                onClick={handleLogout}
                className="text-gray-500 hover:text-gray-700 px-3 py-2 text-sm font-medium"
              >
                Выход
              </button>
            </div>
          </div>
        </div>
      </nav>

      <main className="max-w-7xl mx-auto py-6 sm:px-6 lg:px-8">
        {children}
      </main>
    </div>
  );
}

