import { lazy, Suspense, useEffect } from 'react';
import { Routes, Route, Navigate, useLocation } from 'react-router-dom';
import { ROUTES } from '@/config';
import { ProtectedRoute } from '@/components/ProtectedRoute';
import { ErrorBoundary } from '@/components/ErrorBoundary';
import { ToastContainer } from '@/components/Toast';
import { useToastNotifications } from '@/hooks/useToastNotifications';
import { Login } from '@/pages/Login';
import { Register } from '@/pages/Register';
import { Landing } from '@/pages/Landing';

/**
 * Retry a dynamic import up to `retries` times, then force-reload the page
 * so the browser fetches fresh chunk URLs after a deployment.
 */
function lazyRetry<T extends Record<string, unknown>>(
  factory: () => Promise<T>,
  retries = 2,
): Promise<T> {
  return factory()
    .then((module) => {
      sessionStorage.removeItem('chunk_reload');
      return module;
    })
    .catch((err: unknown) => {
      if (retries > 0) {
        return new Promise<T>((resolve) =>
          setTimeout(() => resolve(lazyRetry(factory, retries - 1)), 500),
        );
      }
      // All retries exhausted — reload to pick up new chunk URLs
      const alreadyReloaded = sessionStorage.getItem('chunk_reload');
      if (!alreadyReloaded) {
        sessionStorage.setItem('chunk_reload', '1');
        window.location.reload();
      }
      throw err;
    });
}

const Dashboard = lazy(() => lazyRetry(() => import('@/pages/Dashboard').then((m) => ({ default: m.Dashboard }))));
const Analyze = lazy(() => lazyRetry(() => import('@/pages/Analyze').then((m) => ({ default: m.Analyze }))));
const History = lazy(() => lazyRetry(() => import('@/pages/History').then((m) => ({ default: m.History }))));
const KnowledgeBase = lazy(() => lazyRetry(() => import('@/pages/KnowledgeBase').then((m) => ({ default: m.KnowledgeBase }))));
const Contacts = lazy(() => lazyRetry(() => import('@/pages/Contacts').then((m) => ({ default: m.Contacts }))));
const Results = lazy(() => lazyRetry(() => import('@/pages/Results').then((m) => ({ default: m.Results }))));
const Account = lazy(() => lazyRetry(() => import('@/pages/Account').then((m) => ({ default: m.Account }))));
const Privacy = lazy(() => lazyRetry(() => import('@/pages/Privacy').then((m) => ({ default: m.Privacy }))));
const Terms = lazy(() => lazyRetry(() => import('@/pages/Terms').then((m) => ({ default: m.Terms }))));

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
  const { pathname } = useLocation();

  useEffect(() => {
    sessionStorage.removeItem('chunk_reload');
  }, []);

  return (
    <ErrorBoundary resetKey={pathname}>
      <ToastContainer toasts={toasts} onDismiss={dismiss} />
      <Suspense fallback={<PageLoader />}>
        <Routes>
          <Route path={ROUTES.LOGIN} element={<Login />} />
          <Route path={ROUTES.REGISTER} element={<Register />} />
          <Route path={ROUTES.PRIVACY} element={<Privacy />} />
          <Route path={ROUTES.TERMS} element={<Terms />} />
          <Route path={ROUTES.CONTACTS} element={<Contacts />} />
          <Route path={ROUTES.HOME} element={<Landing />} />

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
            path={ROUTES.ACCOUNT}
            element={
              <ProtectedRoute>
                <Account />
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

          <Route path="*" element={<Navigate to={ROUTES.HOME} replace />} />
        </Routes>
      </Suspense>
    </ErrorBoundary>
  );
}

export default App;
