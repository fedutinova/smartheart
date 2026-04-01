import axios from 'axios';

const API_BASE = import.meta.env.VITE_API_URL || '/api';
const TOKEN_KEY = 'admin_token';
const REFRESH_KEY = 'admin_refresh';

const api = axios.create({ baseURL: API_BASE, timeout: 15_000 });

api.interceptors.request.use((config) => {
  const token = localStorage.getItem(TOKEN_KEY);
  if (token) config.headers.Authorization = `Bearer ${token}`;
  return config;
});

let isRefreshing = false;
let failedQueue: Array<{ resolve: (t: string) => void; reject: (e: unknown) => void }> = [];

const processQueue = (error: unknown, token: string | null = null) => {
  failedQueue.forEach((p) => (token ? p.resolve(token) : p.reject(error)));
  failedQueue = [];
};

api.interceptors.response.use(
  (res) => res,
  async (error) => {
    const original = error.config;
    if (error.response?.status !== 401 || original?._retry) {
      return Promise.reject(error);
    }

    if (original.url?.includes('/v1/auth/')) {
      return Promise.reject(error);
    }

    const refreshToken = localStorage.getItem(REFRESH_KEY);
    if (!refreshToken) {
      clearTokens();
      window.location.href = '/login';
      return Promise.reject(error);
    }

    if (isRefreshing) {
      return new Promise((resolve, reject) => {
        failedQueue.push({
          resolve: (token: string) => {
            original.headers.Authorization = `Bearer ${token}`;
            resolve(api(original));
          },
          reject,
        });
      });
    }

    isRefreshing = true;
    original._retry = true;

    try {
      const { data } = await axios.post<LoginResponse>(`${API_BASE}/v1/auth/refresh`, {
        refresh_token: refreshToken,
      });
      saveTokens(data.access_token, data.refresh_token);
      processQueue(null, data.access_token);
      original.headers.Authorization = `Bearer ${data.access_token}`;
      return api(original);
    } catch (refreshError) {
      processQueue(refreshError, null);
      clearTokens();
      window.location.href = '/login';
      return Promise.reject(refreshError);
    } finally {
      isRefreshing = false;
    }
  },
);

export function getToken() { return localStorage.getItem(TOKEN_KEY); }

export function saveTokens(access: string, refresh: string) {
  localStorage.setItem(TOKEN_KEY, access);
  localStorage.setItem(REFRESH_KEY, refresh);
}

export function clearTokens() {
  localStorage.removeItem(TOKEN_KEY);
  localStorage.removeItem(REFRESH_KEY);
}

export interface LoginResponse {
  access_token: string;
  refresh_token: string;
}

export interface UserProfile {
  id: string;
  username: string;
  email: string;
  roles: string[];
}

export interface DailyCount {
  date: string;
  count: number;
}

export interface AdminStats {
  users_count: number;
  requests_by_status: Record<string, number>;
  requests_daily: DailyCount[];
  payments_succeeded: number;
  payments_total_rub: number;
  feedback_positive: number;
  feedback_negative: number;
  feedback_satisfaction_pct: number;
}

export interface Paginated<T> {
  data: T[];
  total: number;
  limit: number;
  offset: number;
}

export interface AdminUser {
  id: string;
  username: string;
  email: string;
  roles: string[];
  paid_analyses_remaining: number;
  subscription_expires_at: string | null;
  requests_count: number;
  created_at: string;
}

export interface AdminPayment {
  id: string;
  user_id: string;
  user_email: string;
  status: string;
  amount_kopecks: number;
  payment_type: string;
  description: string;
  created_at: string;
  confirmed_at: string | null;
}

export interface AdminFeedback {
  id: string;
  user_id: string;
  user_email: string;
  question: string;
  answer: string;
  rating: number;
  created_at: string;
}

export const authAPI = {
  login: async (email: string, password: string) => {
    const { data } = await api.post<LoginResponse>('/v1/auth/login', { email, password });
    return data;
  },
  me: async () => {
    const { data } = await api.get<UserProfile>('/v1/me');
    return data;
  },
};

export const adminAPI = {
  stats: async () => {
    const { data } = await api.get<AdminStats>('/v1/admin/stats');
    return data;
  },
  users: async (limit = 20, offset = 0, search = '') => {
    const { data } = await api.get<Paginated<AdminUser>>('/v1/admin/users', {
      params: { limit, offset, search: search || undefined },
    });
    return data;
  },
  payments: async (limit = 20, offset = 0) => {
    const { data } = await api.get<Paginated<AdminPayment>>('/v1/admin/payments', {
      params: { limit, offset },
    });
    return data;
  },
  feedback: async (limit = 20, offset = 0) => {
    const { data } = await api.get<Paginated<AdminFeedback>>('/v1/admin/feedback', {
      params: { limit, offset },
    });
    return data;
  },
};
