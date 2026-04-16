import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { AxiosError, AxiosHeaders } from 'axios';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { vi } from 'vitest';
import { useAuthStore } from '@/store/auth';
import { Register } from './Register';

const { mockRegister } = vi.hoisted(() => ({
  mockRegister: vi.fn(),
}));

vi.mock('@/services/api', () => ({
  authAPI: {
    register: mockRegister,
    logout: vi.fn(),
  },
  profileAPI: {
    getMe: vi.fn(),
  },
}));

function renderRegister() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });

  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={['/register']}>
        <Routes>
          <Route path="/register" element={<Register />} />
          <Route path="/login" element={<div>Страница входа</div>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('Register', () => {
  beforeEach(() => {
    mockRegister.mockReset();
    useAuthStore.setState({
      accessToken: null,
      isAuthenticated: false,
      isInitializing: false,
    });
  });

  it('shows a validation error when passwords do not match', async () => {
    const user = userEvent.setup();
    renderRegister();

    await user.type(screen.getByPlaceholderText('Имя пользователя'), 'anna');
    await user.type(screen.getByPlaceholderText('Email адрес'), 'anna@example.com');
    await user.type(screen.getByPlaceholderText('Пароль (от 10 до 72 символов)'), 'Password123!');
    await user.type(screen.getByPlaceholderText('Повторите пароль'), 'AnotherPass123!');
    await user.click(screen.getByRole('checkbox'));
    await user.click(screen.getByRole('button', { name: 'Зарегистрироваться' }));

    expect(screen.getByText('Пароли не совпадают')).toBeInTheDocument();
    expect(mockRegister).not.toHaveBeenCalled();
  });

  it('submits registration data and redirects to login on success', async () => {
    const user = userEvent.setup();
    mockRegister.mockResolvedValue(undefined);

    renderRegister();

    await user.type(screen.getByPlaceholderText('Имя пользователя'), 'anna');
    await user.type(screen.getByPlaceholderText('Email адрес'), 'anna@example.com');
    await user.type(screen.getByPlaceholderText('Пароль (от 10 до 72 символов)'), 'Password123!');
    await user.type(screen.getByPlaceholderText('Повторите пароль'), 'Password123!');
    await user.click(screen.getByRole('checkbox'));
    await user.click(screen.getByRole('button', { name: 'Зарегистрироваться' }));

    expect(mockRegister).toHaveBeenCalledWith({
      username: 'anna',
      email: 'anna@example.com',
      password: 'Password123!',
    });
    expect(await screen.findByText('Страница входа')).toBeInTheDocument();
  });

  it('shows duplicate user error on 409', async () => {
    const user = userEvent.setup();
    mockRegister.mockRejectedValue(
      new AxiosError('conflict', 'ERR', undefined, undefined, {
        status: 409,
        data: { error: 'already exists' },
        statusText: 'Conflict',
        headers: {},
        config: { headers: new AxiosHeaders() },
      }),
    );

    renderRegister();

    await user.type(screen.getByPlaceholderText('Имя пользователя'), 'anna');
    await user.type(screen.getByPlaceholderText('Email адрес'), 'anna@example.com');
    await user.type(screen.getByPlaceholderText('Пароль (от 10 до 72 символов)'), 'Password123!');
    await user.type(screen.getByPlaceholderText('Повторите пароль'), 'Password123!');
    await user.click(screen.getByRole('checkbox'));
    await user.click(screen.getByRole('button', { name: 'Зарегистрироваться' }));

    expect(await screen.findByText('Пользователь с таким email уже существует')).toBeInTheDocument();
  });

  it('shows rate limit error on 429', async () => {
    const user = userEvent.setup();
    mockRegister.mockRejectedValue(
      new AxiosError('rate limit', 'ERR', undefined, undefined, {
        status: 429,
        data: { error: 'too many requests' },
        statusText: 'Too Many Requests',
        headers: {},
        config: { headers: new AxiosHeaders() },
      }),
    );

    renderRegister();

    await user.type(screen.getByPlaceholderText('Имя пользователя'), 'anna');
    await user.type(screen.getByPlaceholderText('Email адрес'), 'anna@example.com');
    await user.type(screen.getByPlaceholderText('Пароль (от 10 до 72 символов)'), 'Password123!');
    await user.type(screen.getByPlaceholderText('Повторите пароль'), 'Password123!');
    await user.click(screen.getByRole('checkbox'));
    await user.click(screen.getByRole('button', { name: 'Зарегистрироваться' }));

    expect(await screen.findByText('Слишком много попыток. Попробуйте позже')).toBeInTheDocument();
  });

  it('shows network error when no response', async () => {
    const user = userEvent.setup();
    mockRegister.mockRejectedValue(new AxiosError('Network Error', 'ERR_NETWORK'));

    renderRegister();

    await user.type(screen.getByPlaceholderText('Имя пользователя'), 'anna');
    await user.type(screen.getByPlaceholderText('Email адрес'), 'anna@example.com');
    await user.type(screen.getByPlaceholderText('Пароль (от 10 до 72 символов)'), 'Password123!');
    await user.type(screen.getByPlaceholderText('Повторите пароль'), 'Password123!');
    await user.click(screen.getByRole('checkbox'));
    await user.click(screen.getByRole('button', { name: 'Зарегистрироваться' }));

    expect(await screen.findByText('Не удалось связаться с сервером. Проверьте подключение к интернету')).toBeInTheDocument();
  });
});
