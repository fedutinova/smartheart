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

export const useAuthStore = create<AuthState>((set) => ({
  accessToken: null,
  refreshToken: null,
  isAuthenticated: false,

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
    const accessToken = storage.get<string>(JWT_STORAGE_KEY);
    const refreshToken = storage.get<string>(REFRESH_TOKEN_KEY);
    if (accessToken && refreshToken) {
      set({
        accessToken,
        refreshToken,
        isAuthenticated: true,
      });
    }
  },
}));

