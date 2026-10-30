import { useMemo } from "react";
import { useDashboardStore } from "@/stores/dashboard-store";
import type { Transaction } from "../types";
import {
  calculateSummaryByCurrency,
  filterTransactionsBySelection,
  formatCurrency,
} from "@/lib/chart-utils";

type FinanceSummaryProps = {
  transactions: Transaction[];
};

function FinanceSummary({ transactions }: FinanceSummaryProps) {
  const bank = useDashboardStore((state) => state.bank);
  const selectedAccountId = useDashboardStore((state) => state.selectedAccountId);
  const accounts = useDashboardStore((state) => state.accounts);

  const selectedAccount = useMemo(
    () => accounts.find((a) => a.id === selectedAccountId),
    [accounts, selectedAccountId],
  );

  const filteredTransactions = useMemo(
    () => filterTransactionsBySelection(transactions, bank, selectedAccountId),
    [transactions, bank, selectedAccountId],
  );

  const summaries = useMemo(
    () => calculateSummaryByCurrency(filteredTransactions),
    [filteredTransactions],
  );

  return (
    <div className="grid grid-cols-2 divide-x border-b sm:grid-cols-4">
      {/* Balance */}
      <div className="p-4">
        <div className="text-sm text-muted-foreground">Balance</div>
        <div className="mt-1 flex flex-col gap-1">
          {summaries.length === 0 ? (
            <div className="text-xl font-bold text-muted-foreground">0.00 ₴</div>
          ) : (
            summaries.map((s) => {
              const isPositive = s.balance > 0;
              const isNegative = s.balance < 0;
              return (
                <div
                  key={s.currency}
                  className={`text-xl font-bold ${
                    isPositive
                      ? "text-emerald-600 dark:text-emerald-400"
                      : isNegative
                        ? "text-red-600 dark:text-red-400"
                        : "text-muted-foreground"
                  }`}
                >
                  {formatCurrency(s.balance)} {s.symbol}
                </div>
              );
            })
          )}
        </div>
      </div>

      {/* Transactions */}
      <div className="p-4">
        <div className="text-sm text-muted-foreground">Transactions</div>
        <div className="mt-1 text-xl font-bold">
          {filteredTransactions.length.toLocaleString()}
        </div>
        <div className="text-xs text-muted-foreground truncate max-w-44">
          {selectedAccount
            ? selectedAccount.name
            : bank === "all"
              ? "All accounts"
              : bank === "monobank"
                ? "Monobank (All)"
                : "PrivatBank (All)"}
        </div>
      </div>

      {/* Income */}
      <div className="p-4">
        <div className="text-sm text-muted-foreground">Income</div>
        <div className="mt-1 flex flex-col gap-1">
          {summaries.length === 0 ? (
            <div className="text-xl font-bold text-muted-foreground">+0.00 ₴</div>
          ) : (
            summaries.map((s) => (
              <div
                key={s.currency}
                className="text-xl font-bold text-emerald-600 dark:text-emerald-400"
              >
                +{formatCurrency(s.income)} {s.symbol}
              </div>
            ))
          )}
        </div>
        <div className="text-xs text-muted-foreground">Total received</div>
      </div>

      {/* Expenses */}
      <div className="p-4">
        <div className="text-sm text-muted-foreground">Expenses</div>
        <div className="mt-1 flex flex-col gap-1">
          {summaries.length === 0 ? (
            <div className="text-xl font-bold text-muted-foreground">-0.00 ₴</div>
          ) : (
            summaries.map((s) => (
              <div
                key={s.currency}
                className="text-xl font-bold text-red-600 dark:text-red-400"
              >
                -{formatCurrency(s.expenses)} {s.symbol}
              </div>
            ))
          )}
        </div>
        <div className="text-xs text-muted-foreground">Total spent</div>
      </div>
    </div>
  );
}

export default FinanceSummary;
