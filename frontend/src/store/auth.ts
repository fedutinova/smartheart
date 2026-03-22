import { create } from 'zustand';
import type { TokenPair } from '@/types';
import { JWT_STORAGE_KEY, REFRESH_TOKEN_KEY } from '@/config';
import { storage } from '@/utils/storage';

interface AuthState {
  accessToken: string | null;
  refreshToken: string | null;
  isAuthenticated: boolean;
  login: (tokens: TokenPair) => void;
  logout: () => void;
  initialize: () => void;
}

const initialAccessToken = storage.get(JWT_STORAGE_KEY);
const initialRefreshToken = storage.get(REFRESH_TOKEN_KEY);

export const useAuthStore = create<AuthState>((set) => ({
  accessToken: initialAccessToken,
  refreshToken: initialRefreshToken,
  isAuthenticated: !!(initialAccessToken && initialRefreshToken),

  login: (tokens: TokenPair) => {
    storage.set(JWT_STORAGE_KEY, tokens.access_token);
    storage.set(REFRESH_TOKEN_KEY, tokens.refresh_token);
    set({
      accessToken: tokens.access_token,
      refreshToken: tokens.refresh_token,
      isAuthenticated: true,
    });
  },

  logout: () => {
    storage.remove(JWT_STORAGE_KEY);
    storage.remove(REFRESH_TOKEN_KEY);
    set({
      accessToken: null,
      refreshToken: null,
      isAuthenticated: false,
    });
  },

  initialize: () => {
    const accessToken = storage.get(JWT_STORAGE_KEY);
    const refreshToken = storage.get(REFRESH_TOKEN_KEY);
    if (accessToken && refreshToken) {
      set({
        accessToken,
        refreshToken,
        isAuthenticated: true,
      });
    }
  },
}));

