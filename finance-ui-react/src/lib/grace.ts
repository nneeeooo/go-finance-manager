import type { Account, Transaction } from "@/types";

/**
 * Grace period ("пільговий період") of a credit card, from the banks' public terms:
 *  - Monobank: a debt made in a month is free until the last day of the NEXT month (up to 62 days)
 *  - PrivatBank (Універсальна): a debt made in a month is free until the 25th of the NEXT month (up to 55 days)
 *  - Власний Рахунок (BVR): bvr.ua/credit-limit says only "up to 62 days"; taken like Monobank, the last day of the next
 *    month (the longest two months make 62 days). Not stated by the bank: check it in the app.
 * Assumptions: every outflow counts as grace-eligible (cash and transfers may have no grace at the bank), and
 * repayments settle the oldest debt first, so today's debt is made of the newest spending.
 */
export const GRACE_RULES: Record<number, { text: string; deadline: (year: number, month: number) => Date }> = {
  1: { text: "until the last day of the next month", deadline: (y, m) => new Date(y, m + 2, 0, 23, 59, 59) },
  2: { text: "until the 25th of the next month", deadline: (y, m) => new Date(y, m + 1, 25, 23, 59, 59) },
  5: { text: "until the last day of the next month (assumed, the bank says up to 62 days)", deadline: (y, m) => new Date(y, m + 2, 0, 23, 59, 59) },
};

export type GraceBucket = {
  month: Date; // first day of the month the spending was made in
  owed: number; // minor units of today's debt that were spent in that month
  deadline: Date;
  overdue: boolean;
  daysLeft: number; // negative when overdue
};

export type GraceReport = {
  account: Account;
  owed: number; // minor units, 0 when the card holds own money
  buckets: GraceBucket[]; // oldest deadline first
  unexplained: number; // debt older than the saved history, cannot be dated
};

const DAY = 86400000;

/** Debt of one credit card split by the month it was spent in, each with the date it must be repaid by */
export function graceReport(account: Account, transactions: Transaction[], now = new Date()): GraceReport | null {
  const rule = GRACE_RULES[account.bank];
  if (!rule || !account.credit_limit) return null;

  const owed = Math.max(0, -account.balance);
  let remaining = owed;
  const byMonth = new Map<string, { month: Date; owed: number }>();

  const outflows = transactions
    .filter((t) => t.account_id === account.id && t.amount < 0)
    .sort((a, b) => Date.parse(b.occurred_at) - Date.parse(a.occurred_at)); // newest first
  for (const t of outflows) {
    if (remaining <= 0) break;
    const take = Math.min(remaining, -t.amount);
    const date = new Date(t.occurred_at);
    const key = `${date.getFullYear()}-${date.getMonth()}`;
    const entry = byMonth.get(key) ?? { month: new Date(date.getFullYear(), date.getMonth(), 1), owed: 0 };
    entry.owed += take;
    byMonth.set(key, entry);
    remaining -= take;
  }

  const buckets = [...byMonth.values()]
    .map(({ month, owed: amount }) => {
      const deadline = rule.deadline(month.getFullYear(), month.getMonth());
      return { month, owed: amount, deadline, overdue: deadline < now, daysLeft: Math.ceil((deadline.getTime() - now.getTime()) / DAY) };
    })
    .sort((a, b) => a.deadline.getTime() - b.deadline.getTime());

  return { account, owed, buckets, unexplained: remaining };
}

/** What to pay to keep the grace period for the first deadline still open: the overdue debt plus that month's */
export function payNext(report: GraceReport): { amount: number; by: Date } | null {
  const upcoming = report.buckets.find((b) => !b.overdue);
  if (!upcoming) return null;
  const amount = report.buckets.filter((b) => b.deadline <= upcoming.deadline).reduce((sum, b) => sum + b.owed, 0);
  return { amount, by: upcoming.deadline };
}
