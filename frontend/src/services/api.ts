import axios, { AxiosError } from 'axios';
import axiosRetry from 'axios-retry';
import type {
  LoginRequest,
  RegisterRequest,
  ECGAnalysisRequest,
  ECGCalibrationParams,
  Job,
  Request,
  PaginatedResponse,
  QuotaInfo,
  PaymentResult,
} from '@/types';
import { API_BASE_URL, API_TIMEOUT, API_TIMEOUT_UPLOAD, API_TIMEOUT_RAG, AUTH_ERROR_KEY } from '@/config';
import { useAuthStore } from '@/store/auth';
import { queryClient } from '@/services/queryClient';

const api = axios.create({
  baseURL: API_BASE_URL,
  timeout: API_TIMEOUT,
  withCredentials: true,
  headers: {
    'Content-Type': 'application/json',
  },
});

// Retry network errors and 5xx with exponential backoff (max 3 attempts).
// Only retry safe idempotent methods (GET, HEAD, OPTIONS) — never retry POST/PUT/DELETE.
axiosRetry(api, {
  retries: 3,
  retryDelay: axiosRetry.exponentialDelay,
  retryCondition: (error) => {
    const method = error.config?.method?.toUpperCase();
    if (method && !['GET', 'HEAD', 'OPTIONS'].includes(method)) {
      return false;
    }
    return axiosRetry.isNetworkOrIdempotentRequestError(error);
  },
});

let refreshPromise: Promise<string> | null = null;

/**
 * Refresh the access token via the httpOnly refresh-token cookie.
 * Deduplicates concurrent calls — if a refresh is already in-flight,
 * returns the same promise.
 * Used by both the axios interceptor and useEventSource.
 *
 * @param silent  When true the failed refresh does NOT set an auth-error
 *                banner or force logout. Used for the initial page-load
 *                attempt where a missing session is expected (first visit).
 */
export function ensureFreshToken(silent = false): Promise<string> {
  if (!refreshPromise) {
    refreshPromise = axios
      .post<{ access_token: string }>(`${API_BASE_URL}/v1/auth/refresh`, null, {
        withCredentials: true,
        timeout: API_TIMEOUT,
      })
      .then(({ data }) => {
        useAuthStore.getState().setAccessToken(data.access_token);
        return data.access_token;
      })
      .finally(() => {
        refreshPromise = null;
      });
  }

  // Each caller handles failure independently based on its own `silent` flag,
  // so a silent initial-load call doesn't swallow errors for a later
  // non-silent interceptor call sharing the same deduplicated promise.
  return refreshPromise.catch((err) => {
    if (!silent) {
      const isNetwork = err instanceof AxiosError && !err.response;
      const reason = isNetwork
        ? 'Не удалось связаться с сервером. Проверьте подключение к интернету.'
        : 'Время сессии истекло, войдите снова';
      sessionStorage.setItem(AUTH_ERROR_KEY, reason);
      useAuthStore.getState().logout();
      queryClient.clear();
    }
    throw err;
  });
}

// Request interceptor для добавления токена
api.interceptors.request.use(
  (config) => {
    const token = useAuthStore.getState().accessToken;
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

    // Skip token refresh for auth endpoints (401 = bad credentials, not expired token)
    // and for requests already retried once (prevents infinite loops).
    const isAuthEndpoint = originalRequest.url?.startsWith('/v1/auth/');
    const alreadyRetried = (originalRequest as unknown as Record<string, unknown>)._retried;
    if (error.response?.status === 401 && !isAuthEndpoint && !alreadyRetried) {
      try {
        const newToken = await ensureFreshToken();
        originalRequest.headers.Authorization = `Bearer ${newToken}`;
        (originalRequest as unknown as Record<string, unknown>)._retried = true;
        return api(originalRequest);
      } catch (refreshError) {
        return Promise.reject(refreshError);
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
    const response = await api.post<{ access_token: string }>('/v1/auth/login', data);
    return response.data;
  },

  logout: async () => {
    const response = await api.post('/v1/auth/logout');
    return response.data;
  },

  requestPasswordReset: async (email: string) => {
    const response = await api.post('/v1/auth/password-reset', { email });
    return response.data;
  },

  confirmPasswordReset: async (token: string, new_password: string) => {
    const response = await api.post('/v1/auth/password-reset/confirm', { token, new_password });
    return response.data;
  },

  changePassword: async (old_password: string, new_password: string) => {
    const response = await api.post('/v1/auth/password-change', { old_password, new_password });
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

type ECGSubmitResponse = { job_id: string; request_id: string; status: string; message: string };

export const ecgAPI = {
  submitAnalysis: async (data: ECGAnalysisRequest & Partial<ECGCalibrationParams>) => {
    const response = await api.post<ECGSubmitResponse>('/v1/ecg/analyze', data);
    return response.data;
  },

  submitAnalysisFile: async (imageBlob: Blob, notes?: string, params?: ECGCalibrationParams) => {
    const formData = new FormData();
    formData.append('image', imageBlob, 'ecg-image.jpg');
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
    const response = await api.post<ECGSubmitResponse>('/v1/ecg/analyze', formData, {
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

  getFileDirectURL: async (requestId: string, fileId: string): Promise<string> => {
    const response = await api.get<{ url: string }>(`/v1/requests/${requestId}/files/${fileId}/url`);
    return response.data.url;
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
  createSubscription: async () => {
    const response = await api.post<PaymentResult>('/v1/subscriptions');
    return response.data;
  },
};

export default api;
