import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { vi } from 'vitest';
import { AUTH_ERROR_KEY } from '@/config';
import { useAuthStore } from '@/store/auth';
import { Login } from './Login';

const { mockLogin, mockLogout, mockGetMe } = vi.hoisted(() => ({
  mockLogin: vi.fn(),
  mockLogout: vi.fn(),
  mockGetMe: vi.fn(),
}));

vi.mock('@/services/api', () => ({
  authAPI: {
    login: mockLogin,
    logout: mockLogout,
  },
  profileAPI: {
    getMe: mockGetMe,
  },
}));

function renderLogin() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });

  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={['/login']}>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route path="/dashboard" element={<div>Личный кабинет</div>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('Login', () => {
  beforeEach(() => {
    mockLogin.mockReset();
    mockLogout.mockReset();
    mockGetMe.mockReset();
    useAuthStore.setState({
      accessToken: null,
      isAuthenticated: false,
      isInitializing: false,
    });
    localStorage.clear();
    sessionStorage.clear();
  });

  it('shows auth notice from redirected session error', async () => {
    sessionStorage.setItem(AUTH_ERROR_KEY, 'Время сессии истекло, войдите снова');

    renderLogin();

    expect(await screen.findByText('Время сессии истекло, войдите снова')).toBeInTheDocument();
    expect(sessionStorage.getItem(AUTH_ERROR_KEY)).toBeNull();
  });

  it('stores access token and redirects to dashboard on successful login', async () => {
    const user = userEvent.setup();
    mockLogin.mockResolvedValue({
      access_token: 'access-token',
    });

    renderLogin();

    await user.type(screen.getByPlaceholderText('Email адрес'), 'anna@example.com');
    await user.type(screen.getByPlaceholderText('Пароль'), 'Password123!');
    await user.click(screen.getByRole('button', { name: 'Войти' }));

    expect(mockLogin).toHaveBeenCalledWith({
      email: 'anna@example.com',
      password: 'Password123!',
    });
    expect(await screen.findByText('Личный кабинет')).toBeInTheDocument();

    await waitFor(() => {
      expect(useAuthStore.getState().isAuthenticated).toBe(true);
      expect(useAuthStore.getState().accessToken).toBe('access-token');
    });
  });
});
