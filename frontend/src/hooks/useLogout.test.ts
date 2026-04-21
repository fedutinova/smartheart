import { renderHook } from '@testing-library/react';
import { useNavigate, type NavigateFunction } from 'react-router-dom';
import { vi } from 'vitest';
import { useAuthStore } from '@/store/auth';
import { useLogout } from './useLogout';

const { mockLogout } = vi.hoisted(() => ({
  mockLogout: vi.fn(),
}));

vi.mock('@/services/api', () => ({
  authAPI: {
    logout: mockLogout,
  },
}));

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom');
  return {
    ...actual,
    useNavigate: vi.fn(),
  };
});

describe('useLogout', () => {
  let mockNavigate: NavigateFunction;

  beforeEach(() => {
    mockLogout.mockReset();
    mockNavigate = vi.fn() as NavigateFunction;
    vi.mocked(useNavigate).mockReturnValue(mockNavigate);
    useAuthStore.setState({
      accessToken: 'test-token',
      isAuthenticated: true,
      isInitializing: false,
    });
  });

  it('calls logout API, clears auth store, and navigates to login on success', async () => {
    mockLogout.mockResolvedValue({});

    const { result } = renderHook(() => useLogout());
    await result.current();

    expect(mockLogout).toHaveBeenCalled();
    expect(useAuthStore.getState().isAuthenticated).toBe(false);
    expect(useAuthStore.getState().accessToken).toBe(null);
    expect(mockNavigate).toHaveBeenCalledWith('/login');
  });

  it('still logs out locally when API fails', async () => {
    mockLogout.mockRejectedValue(new Error('Network error'));

    const { result } = renderHook(() => useLogout());
    await result.current();

    // API call attempted
    expect(mockLogout).toHaveBeenCalled();
    // But local state still cleared
    expect(useAuthStore.getState().isAuthenticated).toBe(false);
    expect(useAuthStore.getState().accessToken).toBe(null);
    // And navigation still happens
    expect(mockNavigate).toHaveBeenCalledWith('/login');
  });
});
