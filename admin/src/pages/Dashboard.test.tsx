import { render, screen } from '@testing-library/react';
import { vi } from 'vitest';
import { Dashboard } from './Dashboard';

const { mockStats } = vi.hoisted(() => ({
  mockStats: vi.fn(),
}));

vi.mock('@/services/api', () => ({
  adminAPI: {
    stats: mockStats,
    users: vi.fn(),
    payments: vi.fn(),
    feedback: vi.fn(),
  },
  getToken: vi.fn(),
  clearTokens: vi.fn(),
  saveTokens: vi.fn(),
  authAPI: {
    login: vi.fn(),
    me: vi.fn(),
  },
}));

describe('Dashboard', () => {
  beforeEach(() => {
    mockStats.mockReset();
  });

  it('shows loading state before stats are loaded', () => {
    mockStats.mockReturnValue(new Promise(() => {}));

    render(<Dashboard />);

    expect(screen.getByText('Загрузка...')).toBeInTheDocument();
  });

  it('shows an error message when stats request fails', async () => {
    mockStats.mockRejectedValue(new Error('failed'));

    render(<Dashboard />);

    expect(await screen.findByText('Не удалось загрузить статистику')).toBeInTheDocument();
  });

  it('renders summary cards when stats are loaded', async () => {
    mockStats.mockResolvedValue({
      users_count: 12,
      requests_by_status: {
        completed: 7,
        failed: 1,
        pending: 2,
      },
      requests_daily: [
        { date: '2026-04-01', count: 3 },
        { date: '2026-04-02', count: 5 },
      ],
      payments_succeeded: 4,
      payments_total_rub: 1999,
      feedback_positive: 9,
      feedback_negative: 3,
      feedback_satisfaction_pct: 75,
    });

    render(<Dashboard />);

    expect(await screen.findByRole('heading', { name: 'Статистика' })).toBeInTheDocument();
    expect(screen.getByText('Пользователей')).toBeInTheDocument();
    expect(screen.getByText('10')).toBeInTheDocument();
    expect(screen.getByText('4')).toBeInTheDocument();
    expect(screen.getByText('7 завершено, 1 ошибок')).toBeInTheDocument();
    expect(screen.getByText('+9 / −3')).toBeInTheDocument();
    expect(screen.getAllByText('12')).toHaveLength(2);
    expect(screen.getByText('75%')).toBeInTheDocument();
  });
});
