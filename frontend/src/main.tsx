import { StrictMode, useEffect } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import { QueryClientProvider } from '@tanstack/react-query';
import { ReactQueryDevtools } from '@tanstack/react-query-devtools';
import App from './App';
import { useAuthStore } from './store/auth';
import { queryClient } from './services/queryClient';
import { ensureFreshToken } from './services/api';
import './index.css';

/** Max time we wait for the initial silent-refresh before giving up.
 *  Prevents a blank screen on devices where the network request hangs. */
const INIT_TIMEOUT_MS = 10_000;

// eslint-disable-next-line react-refresh/only-export-components
function Root() {
  useEffect(() => {
    const { setInitializing } = useAuthStore.getState();

    // Safety net: if the refresh request hangs (e.g. flaky mobile network
    // without a clean timeout from the OS), make sure we always unblock
    // the UI so the user at least sees the login page.
    const safetyTimer = setTimeout(() => {
      if (useAuthStore.getState().isInitializing) {
        setInitializing(false);
      }
    }, INIT_TIMEOUT_MS);

    // Try silent refresh via httpOnly cookie on every page load.
    // silent=true: if the cookie is missing (first visit) we don't want to
    // show "session expired" on the login page.
    // The token is kept in memory only (never persisted to localStorage
    // to avoid XSS exposure).
    ensureFreshToken(true)
      .catch(() => {
        // No valid session — user will see the login screen.
      })
      .finally(() => {
        setInitializing(false);
        clearTimeout(safetyTimer);
      });

    return () => { clearTimeout(safetyTimer); };
  }, []);

  return null;
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <Root />
      <BrowserRouter>
        <App />
      </BrowserRouter>
      <ReactQueryDevtools initialIsOpen={false} />
    </QueryClientProvider>
  </StrictMode>
);
