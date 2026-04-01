import { NavLink, Outlet, useNavigate } from 'react-router-dom';
import { clearTokens } from '@/services/api';

const NAV = [
  { to: '/', label: 'Статистика' },
  { to: '/users', label: 'Пользователи' },
  { to: '/payments', label: 'Платежи' },
  { to: '/feedback', label: 'Feedback' },
];

export function AdminLayout() {
  const navigate = useNavigate();

  const handleLogout = () => {
    clearTokens();
    navigate('/login');
  };

  return (
    <div className="min-h-screen bg-gray-50">
      <header className="bg-white border-b border-gray-200">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 flex items-center justify-between h-14">
          <div className="flex items-center gap-6">
            <span className="font-bold text-gray-900">SmartHeart Admin</span>
            <nav className="flex gap-1">
              {NAV.map((item) => (
                <NavLink
                  key={item.to}
                  to={item.to}
                  end={item.to === '/'}
                  className={({ isActive }) =>
                    `px-3 py-1.5 rounded-md text-sm transition-colors ${
                      isActive
                        ? 'bg-rose-50 text-rose-700 font-medium'
                        : 'text-gray-600 hover:text-gray-900 hover:bg-gray-100'
                    }`
                  }
                >
                  {item.label}
                </NavLink>
              ))}
            </nav>
          </div>
          <button
            onClick={handleLogout}
            className="text-sm text-gray-500 hover:text-gray-700"
          >
            Выйти
          </button>
        </div>
      </header>
      <main className="max-w-7xl mx-auto px-4 sm:px-6 py-6">
        <Outlet />
      </main>
    </div>
  );
}
