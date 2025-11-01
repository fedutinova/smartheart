export const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080';
export const API_TIMEOUT = 30000;

export const ROUTES = {
  HOME: '/',
  LOGIN: '/login',
  REGISTER: '/register',
  DASHBOARD: '/dashboard',
  ANALYZE: '/analyze',
  HISTORY: '/history',
  KNOWLEDGE_BASE: '/knowledge-base',
  RESULTS: '/results/:id',
};

export const JWT_STORAGE_KEY = 'access_token';
export const REFRESH_TOKEN_KEY = 'refresh_token';

