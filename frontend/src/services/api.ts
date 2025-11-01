import axios, { AxiosError } from 'axios';
import type { 
  LoginRequest, 
  RegisterRequest, 
  TokenPair,
  EKGAnalysisRequest,
  Job,
  Request,
} from '@/types';
import { API_BASE_URL, JWT_STORAGE_KEY } from '@/config';

const api = axios.create({
  baseURL: API_BASE_URL,
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
});

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

// Response interceptor для обработки ошибок
api.interceptors.response.use(
  (response) => response,
  (error: AxiosError) => {
    if (error.response?.status === 401) {
      // Токен истек или невалидный
      localStorage.removeItem(JWT_STORAGE_KEY);
      localStorage.removeItem('refresh_token');
      window.location.href = '/login';
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

