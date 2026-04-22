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

// Common error message constants
export const ERR_RATE_LIMIT = 'Слишком много попыток. Попробуйте позже';
export const ERR_NETWORK = 'Не удалось связаться с сервером. Проверьте подключение к интернету.';

/**
 * Translates backend validation error messages to user-friendly Russian text.
 * Patterns match error messages from back-api/handler/respond.go:formatValidationErrors
 * and back-api/service/password.go:validatePassword
 */
const validationErrorMap: [RegExp, string][] = [
  [/required/i, 'Заполните все обязательные поля'],
  [/username must not exceed/i, 'Имя пользователя слишком длинное'],
  [/invalid email/i, 'Некорректный email адрес'],
  [/password must be at least/i, 'Пароль должен быть не менее 10 символов'],
  [/password must not exceed/i, 'Пароль слишком длинный (максимум 72 символа)'],
  [/password must contain only/i, 'Пароль должен содержать только английские буквы, цифры и спецсимволы'],
  [/invalid request body/i, 'Некорректные данные'],
];

export function translateValidationError(message: string): string {
  return validationErrorMap.find(([re]) => re.test(message))?.[1] ?? 'Ошибка валидации';
}

/**
 * @deprecated Use translateValidationError instead
 */
export function translatePasswordError(message: string): string {
  return translateValidationError(message);
}
