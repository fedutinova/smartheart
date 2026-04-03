import { useEffect, useState } from 'react';
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom';
import { getToken } from '@/services/api';
import { AdminLayout } from '@/components/AdminLayout';
import { Login } from '@/pages/Login';
import { Dashboard } from '@/pages/Dashboard';
import { Users } from '@/pages/Users';
import { Payments } from '@/pages/Payments';
import { Feedback } from '@/pages/Feedback';

function RequireAuth({ children }: { children: React.ReactNode }) {
  const [authenticated, setAuthenticated] = useState(() => !!getToken());

  useEffect(() => {
    const onStorage = () => setAuthenticated(!!getToken());
    window.addEventListener('storage', onStorage);
    return () => window.removeEventListener('storage', onStorage);
  }, []);

  if (!authenticated) return <Navigate to="/login" replace />;
  return <>{children}</>;
}

export function AdminRoutes() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route
        element={
          <RequireAuth>
            <AdminLayout />
          </RequireAuth>
        }
      >
        <Route index element={<Dashboard />} />
        <Route path="users" element={<Users />} />
        <Route path="payments" element={<Payments />} />
        <Route path="feedback" element={<Feedback />} />
      </Route>
    </Routes>
  );
}

export default function App() {
  return (
    <BrowserRouter>
      <AdminRoutes />
    </BrowserRouter>
  );
}
