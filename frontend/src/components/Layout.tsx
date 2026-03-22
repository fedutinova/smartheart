import { useState, useRef, useEffect } from 'react';
import { Link, useNavigate, useLocation } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { useAuthStore } from '@/store/auth';
import { ROUTES, REFRESH_TOKEN_KEY } from '@/config';
import { storage } from '@/utils/storage';
import { authAPI, profileAPI } from '@/services/api';

export function Layout({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, logout } = useAuthStore();
  const navigate = useNavigate();
  const location = useLocation();
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const [profileOpen, setProfileOpen] = useState(false);
  const profileRef = useRef<HTMLDivElement>(null);

  const { data: profile } = useQuery({
    queryKey: ['profile'],
    queryFn: () => profileAPI.getMe(),
    enabled: isAuthenticated,
    staleTime: 60_000,
  });

  // Close dropdown on outside click
  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (profileRef.current && !profileRef.current.contains(e.target as Node)) {
        setProfileOpen(false);
      }
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, []);

  // Close dropdown on route change
  useEffect(() => {
    setProfileOpen(false);
  }, [location.pathname]);

  const handleLogout = async () => {
    const refreshToken = storage.get(REFRESH_TOKEN_KEY);
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

  const navLinks = [
    { to: ROUTES.DASHBOARD, label: 'Главная' },
    { to: ROUTES.ANALYZE, label: 'Анализ' },
    { to: ROUTES.HISTORY, label: 'История' },
    { to: ROUTES.KNOWLEDGE_BASE, label: 'База знаний' },
    { to: ROUTES.CONTACTS, label: 'Контакты' },
  ];

  return (
    <div className="min-h-screen bg-gray-50 flex flex-col">
      <nav className="bg-white shadow-sm">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex justify-between h-16">
            <div className="flex">
              <div className="flex-shrink-0 flex items-center">
                <Link to={ROUTES.DASHBOARD} className="text-2xl text-rose-600 hover:text-rose-700 transition-colors" style={{ fontFamily: "'Prosto One', cursive" }}>
                  Умное сердце
                </Link>
              </div>
              <div className="hidden sm:ml-6 sm:flex sm:space-x-8">
                {navLinks.map((link) => (
                  <Link
                    key={link.to}
                    to={link.to}
                    className={`inline-flex items-center px-1 pt-1 text-sm font-medium ${
                      isActive(link.to)
                        ? 'text-rose-600 border-b-2 border-rose-600'
                        : 'text-gray-500 hover:text-gray-700'
                    }`}
                  >
                    {link.label}
                  </Link>
                ))}
              </div>
            </div>
            <div className="flex items-center">
              {/* Desktop profile dropdown */}
              <div className="hidden sm:block relative" ref={profileRef}>
                <button
                  onClick={() => setProfileOpen(!profileOpen)}
                  className={`flex items-center p-2 rounded-full transition-colors ${
                    profileOpen || isActive(ROUTES.ACCOUNT) ? 'text-rose-600' : 'text-gray-400 hover:text-gray-600'
                  }`}
                >
                  <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 6a3.75 3.75 0 1 1-7.5 0 3.75 3.75 0 0 1 7.5 0ZM4.501 20.118a7.5 7.5 0 0 1 14.998 0A17.933 17.933 0 0 1 12 21.75c-2.676 0-5.216-.584-7.499-1.632Z" />
                  </svg>
                </button>

                {profileOpen && (
                  <div className="absolute right-0 mt-1 w-56 bg-white rounded-xl shadow-lg border border-gray-100 py-2 z-50">
                    {profile && (
                      <div className="px-4 py-2 border-b border-gray-100">
                        <p className="text-sm font-medium text-gray-900 truncate">{profile.username}</p>
                        <p className="text-xs text-gray-400 truncate">{profile.email}</p>
                      </div>
                    )}
                    <Link
                      to={ROUTES.ACCOUNT}
                      className="block px-4 py-2 text-sm text-gray-700 hover:bg-gray-50 transition-colors"
                    >
                      Личный кабинет
                    </Link>
                    <button
                      onClick={handleLogout}
                      className="block w-full text-left px-4 py-2 text-sm text-gray-700 hover:bg-gray-50 transition-colors"
                    >
                      Выход
                    </button>
                  </div>
                )}
              </div>

              {/* Mobile hamburger button */}
              <button
                type="button"
                onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
                className="sm:hidden inline-flex items-center justify-center p-2 rounded-md text-gray-500 hover:text-gray-700 hover:bg-gray-100"
                aria-label="Открыть меню"
                aria-expanded={mobileMenuOpen}
              >
                {mobileMenuOpen ? (
                  <svg className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                  </svg>
                ) : (
                  <svg className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h16" />
                  </svg>
                )}
              </button>
            </div>
          </div>
        </div>

        {/* Mobile menu */}
        {mobileMenuOpen && (
          <div className="sm:hidden border-t border-gray-200">
            <div className="py-2 space-y-1">
              {navLinks.map((link) => (
                <Link
                  key={link.to}
                  to={link.to}
                  onClick={() => setMobileMenuOpen(false)}
                  className={`block px-4 py-2 text-base font-medium ${
                    isActive(link.to)
                      ? 'text-rose-600 bg-rose-50 border-l-4 border-rose-600'
                      : 'text-gray-500 hover:text-gray-700 hover:bg-gray-50'
                  }`}
                >
                  {link.label}
                </Link>
              ))}
              <Link
                to={ROUTES.ACCOUNT}
                onClick={() => setMobileMenuOpen(false)}
                className={`block px-4 py-2 text-base font-medium ${
                  isActive(ROUTES.ACCOUNT)
                    ? 'text-rose-600 bg-rose-50 border-l-4 border-rose-600'
                    : 'text-gray-500 hover:text-gray-700 hover:bg-gray-50'
                }`}
              >
                Личный кабинет
              </Link>
              <button
                onClick={() => {
                  setMobileMenuOpen(false);
                  handleLogout();
                }}
                className="block w-full text-left px-4 py-2 text-base font-medium text-gray-500 hover:text-gray-700 hover:bg-gray-50"
              >
                Выход
              </button>
            </div>
          </div>
        )}
      </nav>

      <main className="w-full max-w-7xl mx-auto py-6 px-4 sm:px-6 lg:px-8 flex-1">
        {children}
      </main>

      <footer className="border-t border-gray-100 py-6 mt-8">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 flex flex-col sm:flex-row items-center justify-between gap-3 text-xs text-gray-400">
          <span>Самозанятая Федутинова А. А., ИНН 575212369164, НПД 422-ФЗ</span>
          <div className="flex gap-4">
            <Link to={ROUTES.TERMS} className="hover:text-gray-600 transition-colors">Оферта</Link>
            <Link to={ROUTES.PRIVACY} className="hover:text-gray-600 transition-colors">Конфиденциальность</Link>
            <a href="mailto:support@smartheart.cloud" className="hover:text-gray-600 transition-colors">support@smartheart.cloud</a>
          </div>
        </div>
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 mt-3 text-center text-[11px] text-gray-300">
          Сервис не является медицинским изделием. Результаты носят информационный характер и не заменяют консультацию врача.
        </div>
      </footer>
    </div>
  );
}
