import { createFileRoute } from "@tanstack/react-router";
import { useEffect, useMemo } from "react";

import { BankBadge } from "@/components/BankBadge";
import { LoginRequired, PageHeader } from "@/components/PageHeader";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { bankName, creditLine, formatMoney } from "@/lib/banks";
import { GRACE_RULES, graceReport, payNext } from "@/lib/grace";
import { cn } from "@/lib/utils";
import { useAuthStore } from "@/stores/auth-store";
import { useDashboardStore } from "@/stores/dashboard-store";

export const Route = createFileRoute("/grace")({
  component: Grace,
});

const date = (d: Date) => d.toLocaleDateString("uk-UA", { day: "numeric", month: "long", year: "numeric" });
const monthName = (d: Date) => d.toLocaleDateString("uk-UA", { month: "long", year: "numeric" });

function Grace() {
  const user = useAuthStore((state) => state.user);
  const initialized = useAuthStore((state) => state.initialized);
  const accounts = useDashboardStore((state) => state.accounts);
  const transactions = useDashboardStore((state) => state.transactions);
  const fetchTransactions = useDashboardStore((state) => state.fetchTransactions);

  useEffect(() => {
    if (initialized && user) fetchTransactions().catch(console.error); // also loads the accounts
  }, [initialized, user, fetchTransactions]);

  const reports = useMemo(
    () => accounts.map((a) => graceReport(a, transactions)).filter((r) => r !== null),
    [accounts, transactions],
  );

  if (!initialized) return null;
  if (!user) return <LoginRequired />;

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto pb-4">
      <PageHeader
        title="Grace period"
        description="What you owe on credit cards and the date each part must be repaid by to stay interest-free. Cards need a credit limit set on the Accounts page."
      />

      {reports.length === 0 && (
        <p className="text-sm text-muted-foreground">No credit cards yet: set a credit limit on a Monobank or PrivatBank account.</p>
      )}

      {reports.map((report) => {
        const { account } = report;
        const next = payNext(report);
        const overdue = report.buckets.filter((b) => b.overdue).reduce((sum, b) => sum + b.owed, 0);
        return (
          <Card key={account.id}>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <BankBadge bank={account.bank} className="size-7" />
                {account.name}
              </CardTitle>
              <CardDescription>
                {bankName(account.bank)}, grace {GRACE_RULES[account.bank].text} · {creditLine(account.balance, account.credit_limit, account.currency)}
              </CardDescription>
            </CardHeader>
            <CardContent className="flex flex-col gap-4">
              {report.owed === 0 ? (
                <p className="text-sm text-emerald-600 dark:text-emerald-400">Nothing owed: the card holds your own money.</p>
              ) : (
                <>
                  <div className="flex flex-col gap-1">
                    <span className="text-sm text-muted-foreground">Owed now</span>
                    <span className="text-2xl font-semibold tabular-nums text-red-600 dark:text-red-400">{formatMoney(report.owed, account.currency)}</span>
                  </div>
                  {next ? (
                    <p className="text-sm">
                      To stay in the grace period, repay <b className="tabular-nums">{formatMoney(next.amount, account.currency)}</b> by <b>{date(next.by)}</b>
                      {overdue > 0 && " (includes the overdue part: repayments settle the oldest debt first)"}.
                    </p>
                  ) : (
                    <p className="text-sm text-destructive">Every part of the debt is past its grace date: interest is running.</p>
                  )}
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Spent in</TableHead>
                        <TableHead className="text-right">Still owed</TableHead>
                        <TableHead>Repay by</TableHead>
                        <TableHead className="text-right">Days left</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {report.buckets.map((b) => (
                        <TableRow key={b.month.getTime()}>
                          <TableCell className="capitalize">{monthName(b.month)}</TableCell>
                          <TableCell className="text-right tabular-nums">{formatMoney(b.owed, account.currency)}</TableCell>
                          <TableCell>{date(b.deadline)}</TableCell>
                          <TableCell className={cn("text-right tabular-nums", b.overdue ? "text-destructive" : b.daysLeft <= 7 && "text-amber-600 dark:text-amber-400")}>
                            {b.overdue ? "overdue" : b.daysLeft}
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                  {report.unexplained > 0 && (
                    <p className="text-xs text-muted-foreground">
                      {formatMoney(report.unexplained, account.currency)} of the debt is older than the saved history, so it has no date. Import older operations to date it.
                    </p>
                  )}
                </>
              )}
            </CardContent>
          </Card>
        );
      })}

      <p className="text-xs text-muted-foreground">
        Rules from the banks' public terms (Monobank: last day of the next month, up to 62 days; PrivatBank Універсальна: the 25th of the next month, up to 55 days;
        Власний Рахунок: "up to 62 days", read here as the last day of the next month). Cash withdrawals and transfers from credit money may have no grace
        (Власний Рахунок charges 4% + 20 грн for cash and 4% + 5 грн for transfers), but are counted here. Check the figure in the bank's app before the deadline.
      </p>
    </div>
  );
}
