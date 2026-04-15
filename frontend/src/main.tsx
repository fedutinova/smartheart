import { StrictMode, useEffect } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import { QueryClientProvider } from '@tanstack/react-query';
import { ReactQueryDevtools } from '@tanstack/react-query-devtools';
import App from './App';
import { useAuthStore } from './store/auth';
import { queryClient } from './services/queryClient';
import { JWT_STORAGE_KEY } from './config';
import { storage } from './utils/storage';
import { ensureFreshToken } from './services/api';
import './index.css';

/** Max time we wait for the initial silent-refresh before giving up.
 *  Prevents a blank screen on devices where the network request hangs. */
const INIT_TIMEOUT_MS = 10_000;

// eslint-disable-next-line react-refresh/only-export-components
function Root() {
  useEffect(() => {
    const { setAccessToken, setInitializing } = useAuthStore.getState();

    // Safety net: if the refresh request hangs (e.g. flaky mobile network
    // without a clean timeout from the OS), make sure we always unblock
    // the UI so the user at least sees the login page.
    const safetyTimer = setTimeout(() => {
      if (useAuthStore.getState().isInitializing) {
        setInitializing(false);
      }
    }, INIT_TIMEOUT_MS);

    // If we have an access token cached in localStorage (from the current
    // session, before a page reload), restore it into memory immediately
    // so protected routes render without a flash.  Then do a background
    // silent refresh to keep it fresh.
    const cached = storage.get(JWT_STORAGE_KEY);
    if (cached) {
      setAccessToken(cached);
      setInitializing(false);
      clearTimeout(safetyTimer);
      // Background refresh — don't block rendering.
      // If the cookie is expired the token will be stale, but the 401
      // interceptor will trigger a non-silent refresh → logout.
      ensureFreshToken(true).catch(() => {
        // Cookie gone — clear the stale cached token so the next 401
        // doesn't loop. The interceptor will handle the actual logout.
        storage.remove(JWT_STORAGE_KEY);
      });
      return () => { clearTimeout(safetyTimer); };
    }

    // No cached token — try silent refresh via httpOnly cookie.
    // silent=true: if the cookie is missing (first visit) we don't want to
    // show "session expired" on the login page.
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
