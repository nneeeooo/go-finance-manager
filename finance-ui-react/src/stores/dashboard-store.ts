import type { Transaction } from "@/types";
import { create } from "zustand";
import { useAuthStore } from "@/stores/auth-store";
import { API_BASE_URL } from "@/lib/api";

export type BankFilter = "all" | "monobank" | "privatbank";

type DashboardState = {
  // bank filter
  bank: BankFilter;
  setBank: (bank: BankFilter) => void;

  // transactions
  transactions: Transaction[];
  loading: boolean;
  fetched: boolean;
  fetchTransactions: () => Promise<void>;
  resetTransactions: () => void;
};

export const useDashboardStore = create<DashboardState>((set, get) => ({
  // bank filter
  bank: "all",

  setBank: (bank) => {
    set({ bank });
  },

  // transactions
  transactions: [],
  loading: false,
  fetched: false,

  fetchTransactions: async () => {
    if (get().fetched || get().loading) {
      return;
    }

    let accessToken = useAuthStore.getState().accessToken;

    if (!accessToken) {
      return;
    }

    set({ loading: true });

    try {
      let response = await fetch(`${API_BASE_URL}/transactions`, {
        headers: {
          Authorization: `Bearer ${accessToken}`,
        },
      });

      // AT expired → refresh and retry once.
      if (response.status === 401) {
        await useAuthStore.getState().refreshAccessToken();

        accessToken = useAuthStore.getState().accessToken;

        if (!accessToken) {
          throw new Error("Session expired");
        }

        response = await fetch(`${API_BASE_URL}/transactions`, {
          headers: {
            Authorization: `Bearer ${accessToken}`,
          },
        });
      }

      if (!response.ok) {
        throw new Error("Failed to fetch transactions");
      }

      const transactions: Transaction[] = await response.json();

      set({
        transactions,
        fetched: true,
      });
    } finally {
      set({ loading: false });
    }
  },
  resetTransactions: () => {
    set({
      bank: "all",
      transactions: [],
      loading: false,
      fetched: false,
    });
  },
}));
