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

// Recover from stale dynamic-import chunks after a redeploy.
//
// Vite hashes lazy-loaded chunk filenames (e.g. piiDetector-<hash>.js). When a
// new build is deployed, an open tab still holds the old index.html and tries
// to fetch a chunk that no longer exists on the server — the fetch fails with
// "Failed to fetch dynamically imported module" and React surfaces it as a
// user-facing error. Caddy already serves index.html with no-cache, but some
// browsers (Safari iOS, bfcache, aggressive proxies) ignore that and keep the
// old document. The cleanest fix is a one-shot reload that pulls the current
// index.html with current chunk names.
//
// `vite:preloadError` is fired by Vite's preload helper before the SyntaxError
// reaches React. We guard with sessionStorage so a *real* missing file doesn't
// reload-loop the page.
if (typeof window !== 'undefined') {
  window.addEventListener('vite:preloadError', (event) => {
    const RELOAD_KEY = 'vite_preload_reload_attempt';
    if (sessionStorage.getItem(RELOAD_KEY)) {
      // We already tried reloading once and the chunk is still missing — let
      // the error propagate so the user sees something rather than spinning
      // in a reload loop.
      return;
    }
    sessionStorage.setItem(RELOAD_KEY, '1');
    event.preventDefault();
    window.location.reload();
  });
  // Clear the guard once the page has loaded successfully, so the next
  // genuine stale-chunk event can trigger a fresh reload.
  window.addEventListener('load', () => {
    sessionStorage.removeItem('vite_preload_reload_attempt');
  });
}

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
