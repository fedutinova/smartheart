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

const passwordErrorMap: [RegExp, string][] = [
  [/password must be at least/i, 'Пароль должен быть не менее 10 символов'],
  [/password must not exceed/i, 'Пароль слишком длинный (максимум 72 символа)'],
  [/password must contain only/i, 'Пароль должен содержать только английские буквы, цифры и спецсимволы'],
];

export function translatePasswordError(message: string): string {
  return passwordErrorMap.find(([re]) => re.test(message))?.[1] ?? 'Некорректные данные';
}
