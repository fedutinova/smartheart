import axios, { AxiosError } from 'axios';
import type {
  LoginRequest,
  RegisterRequest,
  TokenPair,
  EKGAnalysisRequest,
  Job,
  Request,
} from '@/types';
import { API_BASE_URL, JWT_STORAGE_KEY, REFRESH_TOKEN_KEY } from '@/config';
import { useAuthStore } from '@/store/auth';

const api = axios.create({
  baseURL: API_BASE_URL,
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
});

let isRefreshing = false;
let failedQueue: Array<{
  resolve: (token: string) => void;
  reject: (error: unknown) => void;
}> = [];

const processQueue = (error: unknown, token: string | null = null) => {
  failedQueue.forEach((prom) => {
    if (token) {
      prom.resolve(token);
    } else {
      prom.reject(error);
    }
  });
  failedQueue = [];
};

// Request interceptor для добавления токена
api.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem(JWT_STORAGE_KEY);
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  },
  (error) => Promise.reject(error)
);

// Response interceptor с автоматическим обновлением токена
api.interceptors.response.use(
  (response) => response,
  async (error: AxiosError) => {
    const originalRequest = error.config;
    if (!originalRequest) return Promise.reject(error);

    // If 401 and not already a refresh request
    if (error.response?.status === 401 && !originalRequest.url?.includes('/v1/auth/refresh')) {
      const refreshToken = localStorage.getItem(REFRESH_TOKEN_KEY);

      if (!refreshToken) {
        useAuthStore.getState().logout();
        window.location.href = '/login';
        return Promise.reject(error);
      }

      if (isRefreshing) {
        // Queue this request until refresh completes
        return new Promise((resolve, reject) => {
          failedQueue.push({
            resolve: (token: string) => {
              originalRequest.headers.Authorization = `Bearer ${token}`;
              resolve(api(originalRequest));
            },
            reject,
          });
        });
      }

      isRefreshing = true;

      try {
        const response = await axios.post<TokenPair>(
          `${API_BASE_URL}/v1/auth/refresh`,
          { refresh_token: refreshToken }
        );
        const { access_token, refresh_token } = response.data;

        // Sync both localStorage and Zustand store
        useAuthStore.getState().login({ access_token, refresh_token });

        processQueue(null, access_token);

        originalRequest.headers.Authorization = `Bearer ${access_token}`;
        return api(originalRequest);
      } catch (refreshError) {
        processQueue(refreshError, null);
        useAuthStore.getState().logout();
        window.location.href = '/login';
        return Promise.reject(refreshError);
      } finally {
        isRefreshing = false;
      }
    }

    return Promise.reject(error);
  }
);

export const authAPI = {
  register: async (data: RegisterRequest) => {
    const response = await api.post('/v1/auth/register', data);
    return response.data;
  },

  login: async (data: LoginRequest) => {
    const response = await api.post<TokenPair>('/v1/auth/login', data);
    return response.data;
  },

  refresh: async (refreshToken: string) => {
    const response = await api.post<TokenPair>('/v1/auth/refresh', {
      refresh_token: refreshToken,
    });
    return response.data;
  },

  logout: async (refreshToken: string) => {
    const response = await api.post('/v1/auth/logout', {
      refresh_token: refreshToken,
    });
    return response.data;
  },
};

export const ekgAPI = {
  submitAnalysis: async (data: EKGAnalysisRequest) => {
    const response = await api.post<{ job_id: string; request_id: string; status: string; message: string }>(
      '/v1/ekg/analyze',
      data
    );
    return response.data;
  },
};

export const jobAPI = {
  getJob: async (id: string) => {
    const response = await api.get<Job>(`/v1/jobs/${id}`);
    return response.data;
  },
};

export const requestAPI = {
  getRequest: async (id: string) => {
    const response = await api.get<Request>(`/v1/requests/${id}`);
    return response.data;
  },

  getUserRequests: async () => {
    const response = await api.get<Request[]>('/v1/requests');
    return response.data;
  },
};

export default api;

