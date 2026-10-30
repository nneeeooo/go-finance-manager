import { create } from "zustand";
import { API_BASE_URL } from "@/lib/api";

type AuthState = {
  accessToken: string | null;
  loading: boolean;
  initialized: boolean;

  setAccessToken: (accessToken: string) => void;
  clearAccessToken: () => void;
  refreshAccessToken: () => Promise<void>;
};

// Shared between all calls to refreshAccessToken() in this browser tab.
let refreshPromise: Promise<void> | null = null;

export const useAuthStore = create<AuthState>((set) => ({
  accessToken: null,
  loading: false,
  initialized: false,

  setAccessToken: (accessToken) => {
    set({ accessToken });
  },

  clearAccessToken: () => {
    set({ accessToken: null });
  },

  refreshAccessToken: async () => {
    // If a refresh is already happening, wait for it.
    if (refreshPromise) {
      return refreshPromise;
    }

    refreshPromise = (async () => {
      set({ loading: true });

      try {
        const response = await fetch(`${API_BASE_URL}/auth/refresh`, {
            method: "POST",
            credentials: "include",
          },
        );

        if (!response.ok) {
          set({ accessToken: null });
          return;
        }

        const data: {
          access_token: string;
          token_type: string;
        } = await response.json();

        set({
          accessToken: data.access_token,
        });
      } catch {
        set({
          accessToken: null,
        });
      } finally {
        set({ loading: false, initialized: true });

        // Allow another refresh after this one finishes.
        refreshPromise = null;
      }
    })();

    return refreshPromise;
  },
}));
