import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { vi } from 'vitest';
import { Login } from './Login';

const { mockLogin, mockMe, mockSaveTokens, mockClearTokens } = vi.hoisted(() => ({
  mockLogin: vi.fn(),
  mockMe: vi.fn(),
  mockSaveTokens: vi.fn(),
  mockClearTokens: vi.fn(),
}));

vi.mock('@/services/api', () => ({
  getToken: vi.fn(),
  saveTokens: mockSaveTokens,
  clearTokens: mockClearTokens,
  authAPI: {
    login: mockLogin,
    me: mockMe,
  },
  adminAPI: {
    stats: vi.fn(),
    users: vi.fn(),
    payments: vi.fn(),
    feedback: vi.fn(),
  },
}));

function renderLogin() {
  return render(
    <MemoryRouter initialEntries={['/login']}>
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route path="/" element={<div>Админ-панель</div>} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('Admin Login', () => {
  beforeEach(() => {
    mockLogin.mockReset();
    mockMe.mockReset();
    mockSaveTokens.mockReset();
    mockClearTokens.mockReset();
  });

  it('rejects a user without admin role', async () => {
    const user = userEvent.setup();
    mockLogin.mockResolvedValue({
      access_token: 'access-token',
      refresh_token: 'refresh-token',
    });
    mockMe.mockResolvedValue({
      id: 'user-1',
      username: 'anna',
      email: 'anna@example.com',
      roles: ['user'],
    });

    renderLogin();

    await user.type(screen.getByPlaceholderText('Email'), 'anna@example.com');
    await user.type(screen.getByPlaceholderText('Пароль'), 'Password123!');
    await user.click(screen.getByRole('button', { name: 'Войти' }));

    expect(mockLogin).toHaveBeenCalledWith('anna@example.com', 'Password123!');
    expect(mockSaveTokens).toHaveBeenCalledWith('access-token', 'refresh-token');
    expect(mockClearTokens).toHaveBeenCalled();
    expect(await screen.findByText('Нет прав администратора')).toBeInTheDocument();
  });

  it('redirects an admin user to the dashboard', async () => {
    const user = userEvent.setup();
    mockLogin.mockResolvedValue({
      access_token: 'access-token',
      refresh_token: 'refresh-token',
    });
    mockMe.mockResolvedValue({
      id: 'user-1',
      username: 'anna',
      email: 'anna@example.com',
      roles: ['admin'],
    });

    renderLogin();

    await user.type(screen.getByPlaceholderText('Email'), 'anna@example.com');
    await user.type(screen.getByPlaceholderText('Пароль'), 'Password123!');
    await user.click(screen.getByRole('button', { name: 'Войти' }));

    expect(mockSaveTokens).toHaveBeenCalledWith('access-token', 'refresh-token');
    expect(mockClearTokens).not.toHaveBeenCalled();
    expect(await screen.findByText('Админ-панель')).toBeInTheDocument();
  });
});
