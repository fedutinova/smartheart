import { create } from 'zustand';
import { JWT_STORAGE_KEY } from '@/config';
import { storage } from '@/utils/storage';

interface AuthState {
  accessToken: string | null;
  isAuthenticated: boolean;
  /** True while the initial silent-refresh is in progress (page reload). */
  isInitializing: boolean;
  setAccessToken: (token: string) => void;
  logout: () => void;
  setInitializing: (v: boolean) => void;
}

export const useAuthStore = create<AuthState>((set) => ({
  accessToken: null,
  isAuthenticated: false,
  isInitializing: true,

  setAccessToken: (token: string) => {
    storage.set(JWT_STORAGE_KEY, token);
    set({
      accessToken: token,
      isAuthenticated: true,
    });
  },

  logout: () => {
    storage.remove(JWT_STORAGE_KEY);
    set({
      accessToken: null,
      isAuthenticated: false,
      isInitializing: false,
    });
  },

  setInitializing: (v: boolean) => set({ isInitializing: v }),
}));
