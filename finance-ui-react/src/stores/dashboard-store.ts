import type { Transaction, Account } from "@/types";
import { create } from "zustand";
import { useAuthStore } from "@/stores/auth-store";
import { API_BASE_URL } from "@/lib/api";

export type BankFilter = "all" | "monobank" | "privatbank";

type DashboardState = {
  // filters
  bank: BankFilter;
  setBank: (bank: BankFilter) => void;
  selectedAccountId: string;
  setSelectedAccountId: (accountId: string) => void;

  // accounts
  accounts: Account[];
  fetchAccounts: () => Promise<void>;

  // transactions
  transactions: Transaction[];
  loading: boolean;
  fetched: boolean;
  fetchTransactions: () => Promise<void>;
  resetTransactions: () => void;
};

export const useDashboardStore = create<DashboardState>((set, get) => ({
  // filters
  bank: "all",
  selectedAccountId: "all",

  setBank: (bank) => {
    const { selectedAccountId, accounts } = get();
    let newAccountId = selectedAccountId;
    if (selectedAccountId !== "all" && bank !== "all") {
      const acc = accounts.find((a) => a.id === selectedAccountId);
      const targetBankId = bank === "monobank" ? 1 : 2;
      if (acc && acc.bank !== targetBankId) {
        newAccountId = "all";
      }
    }
    set({ bank, selectedAccountId: newAccountId });
  },

  setSelectedAccountId: (selectedAccountId) => {
    const { accounts } = get();
    if (selectedAccountId !== "all") {
      const acc = accounts.find((a) => a.id === selectedAccountId);
      if (acc) {
        const bankName: BankFilter =
          acc.bank === 1 ? "monobank" : acc.bank === 2 ? "privatbank" : "all";
        set({ selectedAccountId, bank: bankName });
        return;
      }
    }
    set({ selectedAccountId });
  },

  // accounts
  accounts: [],

  fetchAccounts: async () => {
    let accessToken = useAuthStore.getState().accessToken;
    if (!accessToken) return;

    try {
      let response = await fetch(`${API_BASE_URL}/accounts`, {
        headers: {
          Authorization: `Bearer ${accessToken}`,
        },
      });

      if (response.status === 401) {
        await useAuthStore.getState().refreshAccessToken();
        accessToken = useAuthStore.getState().accessToken;
        if (!accessToken) return;

        response = await fetch(`${API_BASE_URL}/accounts`, {
          headers: {
            Authorization: `Bearer ${accessToken}`,
          },
        });
      }

      if (response.ok) {
        const accounts: Account[] = await response.json();
        set({ accounts });
      }
    } catch (e) {
      console.error("Failed to fetch accounts", e);
    }
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
      // Also fetch accounts if not loaded
      if (get().accounts.length === 0) {
        get().fetchAccounts().catch(console.error);
      }

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
      selectedAccountId: "all",
      accounts: [],
      transactions: [],
      loading: false,
      fetched: false,
    });
  },
}));
