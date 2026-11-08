import mcc from "mcc";
import currencyCodes from "currency-codes";
import { BANK_FILTER_IDS, type BankFilter } from "@/stores/dashboard-store";
import type { ChartMode, Transaction } from "@/types";

export interface PeriodFilter {
  mode: ChartMode;
  year: number;
  month: number; // 0-11
}

export interface ActivityPoint {
  label: string;
  displayLabel: string;
  total: number;
  income: number;
  expense: number;
}

export interface CategoryPoint {
  id: string;
  name: string;
  amount: number;
  percentage: number;
  count: number;
  fill: string;
}

export const MONTH_ITEMS = Array.from({ length: 12 }, (_, month) => ({
  label: new Date(2000, month).toLocaleDateString("en-US", {
    month: "long",
  }),
  shortLabel: new Date(2000, month).toLocaleDateString("en-US", {
    month: "short",
  }),
  value: String(month),
}));

export const PERIOD_MODE_ITEMS: { label: string; value: ChartMode }[] = [
  { label: "All time", value: "all" },
  { label: "Quarters", value: "quarters" },
  { label: "Months", value: "months" },
  { label: "Days", value: "days" },
];

export const BANK_ITEMS: { label: string; value: BankFilter }[] = [
  { label: "All banks", value: "all" },
  { label: "Monobank", value: "monobank" },
  { label: "PrivatBank", value: "privatbank" },
  { label: "Власний Рахунок", value: "vstbank" },
];

export function formatCurrency(amount: number): string {
  return amount.toLocaleString("uk-UA", {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  });
}

export function filterTransactionsBySelection(
  transactions: Transaction[],
  bank: BankFilter,
  selectedAccountId: string = "all",
): Transaction[] {
  return transactions.filter((tx) => {
    if (selectedAccountId !== "all") {
      return tx.account_id === selectedAccountId;
    }
    if (bank !== "all") {
      return tx.bank_id === BANK_FILTER_IDS[bank];
    }
    return true;
  });
}

export function filterTransactionsByBank(
  transactions: Transaction[],
  bank: BankFilter,
): Transaction[] {
  return filterTransactionsBySelection(transactions, bank, "all");
}

export function getAvailableYears(transactions: Transaction[]): number[] {
  const years = Array.from(
    new Set(
      transactions.map((tx) => new Date(tx.occurred_at).getFullYear()),
    ),
  ).filter((y) => !Number.isNaN(y)).sort((a, b) => b - a);

  if (years.length === 0) {
    return [new Date().getFullYear()];
  }
  return years;
}

export function filterTransactionsByPeriod(
  transactions: Transaction[],
  filter: PeriodFilter,
): Transaction[] {
  const { mode, year, month } = filter;
  return transactions.filter((tx) => {
    const date = new Date(tx.occurred_at);
    if (Number.isNaN(date.getTime())) return false;

    switch (mode) {
      case "all":
        return true;
      case "quarters":
      case "months":
        return date.getFullYear() === year;
      case "days":
        return date.getFullYear() === year && date.getMonth() === month;
      default:
        return true;
    }
  });
}

export function calculateActivityChartData(
  transactions: Transaction[],
  filter: PeriodFilter,
): ActivityPoint[] {
  const { mode, year, month } = filter;
  const groups = new Map<
    string,
    { total: number; income: number; expense: number }
  >();

  // Pre-fill fixed buckets for cleaner visualizations
  if (mode === "quarters") {
    for (let q = 1; q <= 4; q += 1) {
      groups.set(`Q${q}`, { total: 0, income: 0, expense: 0 });
    }
  } else if (mode === "months") {
    for (let m = 0; m < 12; m += 1) {
      groups.set(String(m), { total: 0, income: 0, expense: 0 });
    }
  }

  for (const tx of transactions) {
    const date = new Date(tx.occurred_at);
    if (Number.isNaN(date.getTime())) continue;

    let key = "";
    switch (mode) {
      case "all":
        key = String(date.getFullYear());
        break;
      case "quarters":
        if (date.getFullYear() !== year) continue;
        key = `Q${Math.floor(date.getMonth() / 3) + 1}`;
        break;
      case "months":
        if (date.getFullYear() !== year) continue;
        key = String(date.getMonth());
        break;
      case "days":
        if (date.getFullYear() !== year || date.getMonth() !== month) continue;
        key = `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}-${String(date.getDate()).padStart(2, "0")}`;
        break;
    }

    const current = groups.get(key) ?? { total: 0, income: 0, expense: 0 };
    current.total += tx.amount;
    if (tx.amount > 0) {
      current.income += tx.amount;
    } else {
      current.expense += Math.abs(tx.amount);
    }
    groups.set(key, current);
  }

  return Array.from(groups.entries())
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([label, amounts]) => {
      let displayLabel = label;
      if (mode === "months") {
        const monthIdx = Number(label);
        displayLabel = MONTH_ITEMS[monthIdx]?.shortLabel ?? label;
      } else if (mode === "days") {
        displayLabel = String(new Date(label).getDate());
      }

      return {
        label,
        displayLabel,
        total: Math.round(amounts.total) / 100,
        income: Math.round(amounts.income) / 100,
        expense: Math.round(amounts.expense) / 100,
      };
    });
}

export function calculateCategoryBreakdown(
  transactions: Transaction[],
): { items: CategoryPoint[]; totalExpense: number } {
  const expenseMap = new Map<string, { amount: number; count: number }>();
  let totalExpenseCents = 0;

  for (const tx of transactions) {
    if (tx.amount >= 0) continue; // Only count expenses
    const absAmount = Math.abs(tx.amount);
    totalExpenseCents += absAmount;

    let category = "Other";
    if (tx.mcc) {
      const mccInfo = mcc.get(tx.mcc);
      category = mccInfo?.irs_description || mccInfo?.edited_description || "Other";
    } else if (tx.description) {
      category = tx.description;
    }

    const entry = expenseMap.get(category) ?? { amount: 0, count: 0 };
    entry.amount += absAmount;
    entry.count += 1;
    expenseMap.set(category, entry);
  }

  const totalExpense = totalExpenseCents / 100;
  const sorted = Array.from(expenseMap.entries())
    .map(([name, data], index) => {
      const amount = data.amount / 100;
      const percentage = totalExpense > 0 ? (amount / totalExpense) * 100 : 0;
      return {
        id: `cat-${index}`,
        name,
        amount,
        percentage,
        count: data.count,
        fill: `var(--chart-${(index % 5) + 1})`,
      };
    })
    .sort((a, b) => b.amount - a.amount);

  return { items: sorted, totalExpense };
}

export function getCurrencySymbol(currencyNumber: number): string {
  switch (currencyNumber) {
    case 980:
      return "₴";
    case 840:
      return "$";
    case 978:
      return "€";
    case 985:
      return "zł";
    default:
      return currencyCodes.number(String(currencyNumber))?.code ?? String(currencyNumber);
  }
}

export interface CurrencySummary {
  currency: number;
  symbol: string;
  income: number;
  expenses: number;
}

/** PrivatBank's category for a move between the user's own cards: money did not come in or go out */
const OWN_TRANSFER_CATEGORY = "Переказ на свою картку";
export const isOwnTransfer = (tx: Transaction) => tx.category === OWN_TRANSFER_CATEGORY;

/** Income and expenses per currency; moves between own cards are left out */
export function calculateSummaryByCurrency(transactions: Transaction[]): CurrencySummary[] {
  const groups = new Map<number, { income: number; expenses: number }>();

  for (const tx of transactions) {
    if (isOwnTransfer(tx)) continue;
    const curr = tx.currency || 980;
    const entry = groups.get(curr) ?? { income: 0, expenses: 0 };
    if (tx.amount > 0) {
      entry.income += tx.amount;
    } else {
      entry.expenses += Math.abs(tx.amount);
    }
    groups.set(curr, entry);
  }

  return Array.from(groups.entries())
    .map(([curr, data]) => ({
      currency: curr,
      symbol: getCurrencySymbol(curr),
      income: data.income / 100,
      expenses: data.expenses / 100,
    }))
    .sort((a, b) => (a.currency === 980 ? -1 : b.currency === 980 ? 1 : a.currency - b.currency));
}

export function getPeriodDescription(filter: PeriodFilter): string {
  switch (filter.mode) {
    case "all":
      return "Net transaction total by year";
    case "quarters":
      return `Net transaction total · Quarters of ${filter.year}`;
    case "months":
      return `Net transaction total · Months of ${filter.year}`;
    case "days": {
      const monthName = MONTH_ITEMS[filter.month]?.label ?? "";
      return `Net transaction total · ${monthName} ${filter.year}`;
    }
    default:
      return "Financial overview";
  }
}
