import { AxiosError } from 'axios';

interface ApiError {
  status?: number;
  message: string;
}

export function getApiError(err: unknown): ApiError {
  if (err instanceof AxiosError) {
    return {
      status: err.response?.status,
      message: err.response?.data?.error ?? err.message,
    };
  }
  if (err instanceof Error) {
    return { message: err.message };
  }
  return { message: 'Неизвестная ошибка' };
}
