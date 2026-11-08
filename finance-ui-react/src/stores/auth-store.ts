import { create } from "zustand";

export type AuthUser = { id: string; email: string; role: "regular" | "admin" };

type AuthState = {
  user: AuthUser | null;
  /** true once /auth/me has answered, so views do not flash the signed-out state */
  initialized: boolean;

  setUser: (user: AuthUser | null) => void;
  loadUser: () => Promise<void>;
  /** Ends the session on the API, then leaves for the auth service's logout page */
  logout: () => Promise<void>;
};

// No token here or anywhere in JS: the session is an HttpOnly cookie, /auth/me says who is signed in.
export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  initialized: false,

  setUser: (user) => set({ user }),

  loadUser: async () => {
    try {
      const response = await fetch("/auth/me", { credentials: "include" });
      set({ user: response.ok ? ((await response.json()) as AuthUser) : null });
    } catch {
      set({ user: null });
    } finally {
      set({ initialized: true });
    }
  },

  logout: async () => {
    let target = "/";
    try {
      const response = await fetch("/auth/logout", {
        method: "POST",
        credentials: "include",
        headers: { "X-Requested-With": "fetch", Accept: "application/json" },
      });
      if (response.ok) target = (await response.json()).redirect_url ?? "/";
    } finally {
      set({ user: null });
    }
    window.location.assign(target);
  },
}));
