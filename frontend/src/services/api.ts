import axios, { AxiosError } from 'axios';
import axiosRetry from 'axios-retry';
import type {
  LoginRequest,
  RegisterRequest,
  TokenPair,
  EKGAnalysisRequest,
  ECGCalibrationParams,
  Job,
  Request,
  PaginatedResponse,
  QuotaInfo,
  PaymentResult,
} from '@/types';
import { API_BASE_URL, API_TIMEOUT, API_TIMEOUT_UPLOAD, API_TIMEOUT_RAG, JWT_STORAGE_KEY, REFRESH_TOKEN_KEY, AUTH_ERROR_KEY } from '@/config';
import { useAuthStore } from '@/store/auth';

const api = axios.create({
  baseURL: API_BASE_URL,
  timeout: API_TIMEOUT,
  headers: {
    'Content-Type': 'application/json',
  },
});

// Retry network errors and 5xx with exponential backoff (max 3 attempts).
// Only retry idempotent methods (GET, HEAD, OPTIONS) — never retry POST/PUT/DELETE.
axiosRetry(api, {
  retries: 3,
  retryDelay: axiosRetry.exponentialDelay,
  retryCondition: (error) => axiosRetry.isNetworkOrIdempotentRequestError(error),
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
        sessionStorage.setItem(AUTH_ERROR_KEY, 'Сессия не найдена. Пожалуйста, войдите снова.');
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

        // Determine user-friendly error message
        const isNetwork = refreshError instanceof AxiosError && !refreshError.response;
        const reason = isNetwork
          ? 'Не удалось связаться с сервером. Проверьте подключение к интернету.'
          : 'Сессия истекла. Пожалуйста, войдите снова.';
        sessionStorage.setItem(AUTH_ERROR_KEY, reason);

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

export interface UserProfile {
  id: string;
  username: string;
  email: string;
  created_at: string;
}

export const profileAPI = {
  getMe: async () => {
    const response = await api.get<UserProfile>('/v1/me');
    return response.data;
  },
};

type EKGSubmitResponse = { job_id: string; request_id: string; status: string; message: string };

export const ekgAPI = {
  submitAnalysis: async (data: EKGAnalysisRequest & Partial<ECGCalibrationParams>) => {
    const response = await api.post<EKGSubmitResponse>('/v1/ekg/analyze', data);
    return response.data;
  },

  submitAnalysisFile: async (imageBlob: Blob, notes?: string, params?: ECGCalibrationParams) => {
    const formData = new FormData();
    formData.append('image', imageBlob, 'ekg-image.jpg');
    if (notes) {
      formData.append('notes', notes);
    }
    if (params) {
      if (params.age != null) formData.append('age', String(params.age));
      if (params.sex) formData.append('sex', params.sex);
      formData.append('paper_speed_mms', String(params.paper_speed_mms));
      formData.append('mm_per_mv_limb', String(params.mm_per_mv_limb));
      formData.append('mm_per_mv_chest', String(params.mm_per_mv_chest));
    }
    const response = await api.post<EKGSubmitResponse>('/v1/ekg/analyze', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: API_TIMEOUT_UPLOAD,
    });
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

  getUserRequests: async (limit = 50, offset = 0) => {
    const response = await api.get<PaginatedResponse<Request>>('/v1/requests', {
      params: { limit, offset },
    });
    return response.data;
  },

  getFileURL: async (requestId: string, fileId: string): Promise<string> => {
    const response = await api.get(`/v1/requests/${requestId}/files/${fileId}`, {
      responseType: 'blob',
    });
    return URL.createObjectURL(response.data);
  },
};

export interface RAGSource {
  doc_name: string;
  chunk_index: number;
  score: number;
  preview: string;
}

export interface RAGQueryMeta {
  model: string;
  temperature: number;
  n_results: number;
}

export interface RAGQueryResponse {
  answer: string;
  sources: RAGSource[];
  elapsed_ms: number;
  meta?: RAGQueryMeta;
}

export const ragAPI = {
  query: async (question: string, nResults = 5) => {
    const response = await api.post<RAGQueryResponse>('/v1/rag/query', {
      question,
      n_results: nResults,
    }, { timeout: API_TIMEOUT_RAG });
    return response.data;
  },
  submitFeedback: async (question: string, answer: string, rating: -1 | 1) => {
    await api.post('/v1/rag/feedback', { question, answer, rating });
  },
};

export const paymentAPI = {
  getQuota: async () => {
    const response = await api.get<QuotaInfo>('/v1/quota');
    return response.data;
  },
  createPayment: async (analysesCount: number) => {
    const response = await api.post<PaymentResult>('/v1/payments', { analyses_count: analysesCount });
    return response.data;
  },
};

export default api;

