import { useEffect, useMemo, useState } from "react";
import { BANK_FILTER_IDS, useDashboardStore } from "@/stores/dashboard-store";
import { useAuthStore } from "@/stores/auth-store";
import type { Transaction } from "../types";
import { BANK_ITEMS, calculateSummaryByCurrency, formatCurrency, getCurrencySymbol } from "@/lib/chart-utils";
import { apiFetch } from "@/lib/api";
import { owedOn, type Loan } from "@/lib/loans";

type FinanceSummaryProps = {
  /** Already limited to the chosen bank, account and period */
  transactions: Transaction[];
  period: string;
};

const hryvniaFirst = (a: number, b: number) => (a === 980 ? -1 : b === 980 ? 1 : a - b);

function FinanceSummary({ transactions, period }: FinanceSummaryProps) {
  const bank = useDashboardStore((state) => state.bank);
  const selectedAccountId = useDashboardStore((state) => state.selectedAccountId);
  const accounts = useDashboardStore((state) => state.accounts);

  const selectedAccount = useMemo(
    () => accounts.find((a) => a.id === selectedAccountId),
    [accounts, selectedAccountId],
  );

  // Balance: what the accounts hold now (their stored balance), never a sum of imported operations
  const balances = useMemo(() => {
    const shown = selectedAccount
      ? [selectedAccount]
      : bank === "all"
        ? accounts
        : accounts.filter((a) => a.bank === BANK_FILTER_IDS[bank]);
    const sums = new Map<number, number>();
    for (const a of shown) sums.set(a.currency, (sums.get(a.currency) ?? 0) + a.balance);
    return [...sums.entries()].sort(([a], [b]) => hryvniaFirst(a, b));
  }, [accounts, bank, selectedAccount]);

  // Loans: what is still to repay, per currency. Not tied to one account, so a single account shows none.
  const user = useAuthStore((state) => state.user);
  const [loans, setLoans] = useState<Loan[]>([]);
  useEffect(() => {
    if (!user) return;
    apiFetch("/loans")
      .then((r) => (r.ok ? r.json() : []))
      .then(setLoans)
      .catch(console.error);
  }, [user]);

  const afterLoans = useMemo(() => {
    if (selectedAccount) return [];
    const owed = new Map<number, number>();
    for (const loan of loans) {
      if (bank !== "all" && loan.bank !== BANK_FILTER_IDS[bank]) continue;
      owed.set(loan.currency, (owed.get(loan.currency) ?? 0) + owedOn(loan));
    }
    const held = new Map(balances);
    return [...owed.entries()]
      .sort(([a], [b]) => hryvniaFirst(a, b))
      .map(([currency, debt]) => ({ currency, debt, total: (held.get(currency) ?? 0) - debt }));
  }, [loans, bank, selectedAccount, balances]);

  // Income and expenses of the period; moves between own cards are not counted
  const summaries = useMemo(() => calculateSummaryByCurrency(transactions), [transactions]);

  return (
    <div className="grid grid-cols-2 divide-x border-b sm:grid-cols-4">
      {/* Balance */}
      <div className="p-4">
        <div className="text-sm text-muted-foreground">Balance now</div>
        <div className="mt-1 flex flex-col gap-1">
          {balances.length === 0 ? (
            <div className="text-xl font-bold text-muted-foreground">0.00 ₴</div>
          ) : (
            balances.map(([currency, minor]) => (
              <div
                key={currency}
                className={`text-xl font-bold ${minor < 0 ? "text-red-600 dark:text-red-400" : minor > 0 ? "text-emerald-600 dark:text-emerald-400" : "text-muted-foreground"}`}
              >
                {formatCurrency(minor / 100)} {getCurrencySymbol(currency)}
              </div>
            ))
          )}
        </div>
        {afterLoans.length > 0 && (
          <div className="mt-2 flex flex-col gap-0.5 border-t pt-2">
            <div className="text-xs text-muted-foreground">After repaying loans</div>
            {afterLoans.map((l) => (
              <div key={l.currency} className={`text-lg font-semibold ${l.total < 0 ? "text-red-600 dark:text-red-400" : ""}`}>
                {formatCurrency(l.total / 100)} {getCurrencySymbol(l.currency)}
                <span className="ml-1 text-xs font-normal text-muted-foreground">(loans {formatCurrency(l.debt / 100)})</span>
              </div>
            ))}
          </div>
        )}
        <div className="text-xs text-muted-foreground truncate max-w-44">
          {selectedAccount
            ? selectedAccount.name
            : bank === "all"
              ? "All accounts"
              : `${BANK_ITEMS.find((b) => b.value === bank)?.label} (All)`}
        </div>
      </div>

      {/* Transactions */}
      <div className="p-4">
        <div className="text-sm text-muted-foreground">Transactions</div>
        <div className="mt-1 text-xl font-bold">{transactions.length.toLocaleString()}</div>
        <div className="text-xs text-muted-foreground truncate max-w-44">{period}</div>
      </div>

      {/* Income */}
      <div className="p-4">
        <div className="text-sm text-muted-foreground">Income</div>
        <div className="mt-1 flex flex-col gap-1">
          {summaries.length === 0 ? (
            <div className="text-xl font-bold text-muted-foreground">+0.00 ₴</div>
          ) : (
            summaries.map((s) => (
              <div key={s.currency} className="text-xl font-bold text-emerald-600 dark:text-emerald-400">
                +{formatCurrency(s.income)} {s.symbol}
              </div>
            ))
          )}
        </div>
        <div className="text-xs text-muted-foreground">Received, own transfers excluded</div>
      </div>

      {/* Expenses */}
      <div className="p-4">
        <div className="text-sm text-muted-foreground">Expenses</div>
        <div className="mt-1 flex flex-col gap-1">
          {summaries.length === 0 ? (
            <div className="text-xl font-bold text-muted-foreground">-0.00 ₴</div>
          ) : (
            summaries.map((s) => (
              <div key={s.currency} className="text-xl font-bold text-red-600 dark:text-red-400">
                -{formatCurrency(s.expenses)} {s.symbol}
              </div>
            ))
          )}
        </div>
        <div className="text-xs text-muted-foreground">Spent, own transfers excluded</div>
      </div>
    </div>
  );
}

export default FinanceSummary;
