export const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080';
export const API_TIMEOUT = 30_000;
export const API_TIMEOUT_UPLOAD = 60_000;
export const API_TIMEOUT_RAG = 120_000;

export const ROUTES = {
  HOME: '/',
  LOGIN: '/login',
  REGISTER: '/register',
  DASHBOARD: '/dashboard',
  ANALYZE: '/analyze',
  HISTORY: '/history',
  KNOWLEDGE_BASE: '/knowledge-base',
  CONTACTS: '/contacts',
  RESULTS: '/results/:id',
  ACCOUNT: '/account',
  FORGOT_PASSWORD: '/forgot-password',
  RESET_PASSWORD: '/reset-password',
  PRIVACY: '/privacy',
  TERMS: '/terms',
};

export const JWT_STORAGE_KEY = 'access_token';

/**
 * Key in sessionStorage for storing the auth error reason
 * so the Login page can display a meaningful message after redirect.
 */
export const AUTH_ERROR_KEY = 'auth_error';

