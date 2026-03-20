import { lazy, Suspense } from 'react';
import { Routes, Route, Navigate } from 'react-router-dom';
import { ROUTES } from '@/config';
import { ProtectedRoute } from '@/components/ProtectedRoute';
import { ErrorBoundary } from '@/components/ErrorBoundary';
import { ToastContainer } from '@/components/Toast';
import { useToastNotifications } from '@/hooks/useToastNotifications';
import { Login } from '@/pages/Login';
import { Register } from '@/pages/Register';

const Dashboard = lazy(() => import('@/pages/Dashboard').then((m) => ({ default: m.Dashboard })));
const Analyze = lazy(() => import('@/pages/Analyze').then((m) => ({ default: m.Analyze })));
const History = lazy(() => import('@/pages/History').then((m) => ({ default: m.History })));
const KnowledgeBase = lazy(() => import('@/pages/KnowledgeBase').then((m) => ({ default: m.KnowledgeBase })));
const Contacts = lazy(() => import('@/pages/Contacts').then((m) => ({ default: m.Contacts })));
const Results = lazy(() => import('@/pages/Results').then((m) => ({ default: m.Results })));

function PageLoader() {
  return (
    <div className="min-h-screen bg-gray-50 flex items-center justify-center">
      <div className="text-gray-400 text-sm">Загрузка...</div>
    </div>
  );
}

function App() {
  // SSE + toasts live here — once for the entire app lifetime,
  // not inside Layout which remounts on every navigation.
  const { toasts, dismiss } = useToastNotifications();

  return (
    <ErrorBoundary>
      <ToastContainer toasts={toasts} onDismiss={dismiss} />
      <Suspense fallback={<PageLoader />}>
        <Routes>
          <Route path={ROUTES.LOGIN} element={<Login />} />
          <Route path={ROUTES.REGISTER} element={<Register />} />
          <Route path={ROUTES.HOME} element={<Navigate to={ROUTES.DASHBOARD} replace />} />

          <Route
            path={ROUTES.DASHBOARD}
            element={
              <ProtectedRoute>
                <Dashboard />
              </ProtectedRoute>
            }
          />
          <Route
            path={ROUTES.ANALYZE}
            element={
              <ProtectedRoute>
                <Analyze />
              </ProtectedRoute>
            }
          />
          <Route
            path={ROUTES.HISTORY}
            element={
              <ProtectedRoute>
                <History />
              </ProtectedRoute>
            }
          />
          <Route
            path={ROUTES.KNOWLEDGE_BASE}
            element={
              <ProtectedRoute>
                <KnowledgeBase />
              </ProtectedRoute>
            }
          />
          <Route
            path={ROUTES.CONTACTS}
            element={
              <ProtectedRoute>
                <Contacts />
              </ProtectedRoute>
            }
          />
          <Route
            path="/results/:id"
            element={
              <ProtectedRoute>
                <Results />
              </ProtectedRoute>
            }
          />

          <Route path="*" element={<Navigate to={ROUTES.DASHBOARD} replace />} />
        </Routes>
      </Suspense>
    </ErrorBoundary>
  );
}

export default App;
