import { render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { useAuthStore } from '@/store/auth';
import { ProtectedRoute } from './ProtectedRoute';

function renderProtectedRoute() {
  return render(
    <MemoryRouter initialEntries={['/dashboard']}>
      <Routes>
        <Route path="/login" element={<div>Страница входа</div>} />
        <Route
          path="/dashboard"
          element={
            <ProtectedRoute>
              <div>Личный кабинет</div>
            </ProtectedRoute>
          }
        />
      </Routes>
    </MemoryRouter>,
  );
}

describe('ProtectedRoute', () => {
  beforeEach(() => {
    useAuthStore.setState({
      accessToken: null,
      refreshToken: null,
      isAuthenticated: false,
    });
  });

  it('redirects unauthenticated users to login', async () => {
    renderProtectedRoute();

    expect(await screen.findByText('Страница входа')).toBeInTheDocument();
    expect(screen.queryByText('Личный кабинет')).not.toBeInTheDocument();
  });

  it('renders protected content for authenticated users', async () => {
    useAuthStore.setState({
      accessToken: 'access-token',
      refreshToken: 'refresh-token',
      isAuthenticated: true,
    });

    renderProtectedRoute();

    expect(await screen.findByText('Личный кабинет')).toBeInTheDocument();
    expect(screen.queryByText('Страница входа')).not.toBeInTheDocument();
  });
});
