import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { vi } from 'vitest';
import { AdminRoutes } from './App';

const { mockGetToken, mockStats, mockUsers, mockPayments, mockFeedback } = vi.hoisted(() => ({
  mockGetToken: vi.fn(),
  mockStats: vi.fn(),
  mockUsers: vi.fn(),
  mockPayments: vi.fn(),
  mockFeedback: vi.fn(),
}));

vi.mock('@/services/api', () => ({
  getToken: mockGetToken,
  clearTokens: vi.fn(),
  saveTokens: vi.fn(),
  authAPI: {
    login: vi.fn(),
    me: vi.fn(),
  },
  adminAPI: {
    stats: mockStats,
    users: mockUsers,
    payments: mockPayments,
    feedback: mockFeedback,
  },
}));

describe('AdminRoutes', () => {
  beforeEach(() => {
    mockGetToken.mockReset();
    mockStats.mockReset();
    mockUsers.mockReset();
    mockPayments.mockReset();
    mockFeedback.mockReset();

    mockStats.mockResolvedValue({
      users_count: 0,
      requests_by_status: {},
      requests_daily: [],
      payments_succeeded: 0,
      payments_total_rub: 0,
      feedback_positive: 0,
      feedback_negative: 0,
      feedback_satisfaction_pct: 0,
    });
    mockUsers.mockResolvedValue({ data: [], total: 0, limit: 20, offset: 0 });
    mockPayments.mockResolvedValue({ data: [], total: 0, limit: 20, offset: 0 });
    mockFeedback.mockResolvedValue({ data: [], total: 0, limit: 20, offset: 0 });
  });

  function renderRoutes(entries: string[]) {
    return render(
      <MemoryRouter initialEntries={entries}>
        <AdminRoutes />
      </MemoryRouter>,
    );
  }

  it('redirects unauthenticated users to login', async () => {
    mockGetToken.mockReturnValue(null);

    renderRoutes(['/payments']);

    expect(await screen.findByRole('button', { name: 'Войти' })).toBeInTheDocument();
    expect(screen.getByText('Админ')).toBeInTheDocument();
  });

  it('renders protected admin routes for authenticated users', async () => {
    mockGetToken.mockReturnValue('token');

    renderRoutes(['/payments']);

    expect(await screen.findByRole('heading', { name: 'Платежи' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Выйти' })).toBeInTheDocument();
    expect(mockPayments).toHaveBeenCalledWith(20, 0);
  });
});
