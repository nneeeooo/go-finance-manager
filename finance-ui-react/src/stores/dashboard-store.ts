import type { Transaction, Account } from "@/types";
import { create } from "zustand";
import { useAuthStore } from "@/stores/auth-store";
import { apiFetch } from "@/lib/api";

export type BankFilter = "all" | "monobank" | "privatbank" | "vstbank";

/** The bank id (as the API stores it) behind each dashboard bank filter */
export const BANK_FILTER_IDS = { monobank: 1, privatbank: 2, vstbank: 5 } as const;

/** The dashboard filter of a bank id; "all" for banks with no filter (cash, other) */
export const bankFilterOf = (bankId: number): BankFilter =>
  (Object.entries(BANK_FILTER_IDS).find(([, id]) => id === bankId)?.[0] as BankFilter | undefined) ?? "all";

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
      if (acc && acc.bank !== BANK_FILTER_IDS[bank]) {
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
        set({ selectedAccountId, bank: bankFilterOf(acc.bank) });
        return;
      }
    }
    set({ selectedAccountId });
  },

  // accounts
  accounts: [],

  fetchAccounts: async () => {
    if (!useAuthStore.getState().user) return;

    try {
      const response = await apiFetch("/accounts");
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

    if (!useAuthStore.getState().user) {
      return;
    }

    set({ loading: true });

    try {
      // Also fetch accounts if not loaded
      if (get().accounts.length === 0) {
        get().fetchAccounts().catch(console.error);
      }

      const response = await apiFetch("/transactions");

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
