import { getCurrencySymbol } from "@/lib/chart-utils";

// Bank ids as the API stores them (models.Bank)
export const BANKS = {
  1: { name: "Monobank", short: "mono", className: "bg-zinc-900 text-white ring-zinc-700 dark:bg-white dark:text-zinc-900" },
  2: { name: "PrivatBank", short: "P24", className: "bg-emerald-600 text-white ring-emerald-500" },
  3: { name: "Cash", short: "₴", className: "bg-amber-500 text-white ring-amber-400" },
  4: { name: "Other", short: "•", className: "bg-slate-500 text-white ring-slate-400" },
  5: { name: "Власний Рахунок", short: "ВР", className: "bg-violet-600 text-white ring-violet-500" },
} as const;

export type BankId = keyof typeof BANKS;

export const BANK_IDS = Object.keys(BANKS).map(Number) as BankId[];

export const bankName = (bank: number) => BANKS[bank as BankId]?.name ?? `Bank ${bank}`;

// Currencies offered when adding an account (ISO 4217 numeric)
export const CURRENCIES = [
  { value: 980, label: "UAH ₴" },
  { value: 840, label: "USD $" },
  { value: 978, label: "EUR €" },
  { value: 985, label: "PLN zł" },
];

const NBSP = " "; // keeps the amount and its currency sign on one line

/** Minor units (kopecks) to "1 234,56 ₴" */
export function formatMoney(minor: number, currency: number): string {
  const value = (minor / 100).toLocaleString("uk-UA", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
  return `${value}${NBSP}${getCurrencySymbol(currency)}`;
}

/** "1 234,56" or "-1234.5" to minor units; empty is 0; null when not a number */
export function parseMoney(text: string): number | null {
  const minor = Math.round(parseFloat((text.trim() || "0").replace(/\s/g, "").replace(",", ".")) * 100);
  return Number.isNaN(minor) ? null : minor;
}

/** "Limit 35 000,00 ₴ · available 3 000,00 ₴" for a credit card; empty for a debit one */
export function creditLine(balance: number, creditLimit: number | undefined, currency: number): string {
  if (!creditLimit) return "";
  return `Limit ${formatMoney(creditLimit, currency)} · available ${formatMoney(creditLimit + balance, currency)}`;
}
