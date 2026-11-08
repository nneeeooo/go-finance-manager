import { createFileRoute } from "@tanstack/react-router";
import { useCallback, useEffect, useState, type ReactNode } from "react";
import { ArrowDownRight, ArrowUpRight, CalendarClock, Check, ChevronDown, Loader2, Plus, RotateCcw, Trash2 } from "lucide-react";

import { BankBadge } from "@/components/BankBadge";
import { LoginRequired, PageHeader } from "@/components/PageHeader";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { apiError, apiFetch } from "@/lib/api";
import { BANKS, CURRENCIES, bankName, formatMoney, parseMoney } from "@/lib/banks";
import { paidPayments, remainingPayments, takenOn, totalToRepay, type Loan } from "@/lib/loans";
import { nbuRate } from "@/lib/rates";
import { cn } from "@/lib/utils";
import { useAuthStore } from "@/stores/auth-store";

export const Route = createFileRoute("/loans")({
  component: Loans,
});

const date = (d: Date) => d.toLocaleDateString("uk-UA", { day: "numeric", month: "long", year: "numeric" });
const shortDate = (d: Date) => d.toLocaleDateString("uk-UA", { day: "2-digit", month: "2-digit", year: "numeric" });
const selectClass = "h-8 rounded-lg border border-input bg-transparent px-2.5 text-sm outline-none focus-visible:ring-3 focus-visible:ring-ring/50";
const GOOD = "text-emerald-600 dark:text-emerald-400";
const BAD = "text-red-600 dark:text-red-400";

function Loans() {
  const user = useAuthStore((state) => state.user);
  const initialized = useAuthStore((state) => state.initialized);
  const [loans, setLoans] = useState<Loan[] | null>(null);
  const [adding, setAdding] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    const response = await apiFetch("/loans");
    if (response.ok) setLoans(await response.json());
    else setError(await apiError(response));
  }, []);

  useEffect(() => {
    if (!initialized || !user) return;
    apiFetch("/loans")
      .then(async (r) => (r.ok ? setLoans(await r.json()) : setError(await apiError(r))))
      .catch(console.error);
  }, [initialized, user]);

  if (!initialized) return null;
  if (!user) return <LoginRequired />;

  return (
    // Children keep their height and the page scrolls; without shrink-0 the flex column squeezes the cards and clips their content
    <div className="flex min-h-0 flex-1 flex-col gap-5 overflow-y-auto pb-4 [&>*]:shrink-0">
      <PageHeader
        title="Loans"
        description="Banks give no API for loans: enter each one from its contract and mark payments as you make them."
        action={
          <Button onClick={() => setAdding((v) => !v)} variant={adding ? "outline" : "default"}>
            <Plus />
            {adding ? "Close" : "Add loan"}
          </Button>
        }
      />
      {error && <p className="text-sm text-destructive">{error}</p>}
      {adding && (
        <AddLoan
          onDone={() => {
            setAdding(false);
            load().catch(console.error);
          }}
        />
      )}
      {loans?.length === 0 && !adding && <p className="text-sm text-muted-foreground">No loans yet.</p>}
      {loans?.map((loan) => <LoanCard key={loan.id} loan={loan} onChanged={() => load().catch(console.error)} />)}
    </div>
  );
}

/** Percent paid as a ring */
function Ring({ percent }: { percent: number }) {
  const r = 42;
  const c = 2 * Math.PI * r;
  return (
    <div className="relative size-28 shrink-0">
      <svg viewBox="0 0 100 100" className="size-full -rotate-90">
        <circle cx="50" cy="50" r={r} fill="none" strokeWidth="9" className="stroke-muted" />
        <circle
          cx="50" cy="50" r={r} fill="none" strokeWidth="9" strokeLinecap="round"
          className="stroke-emerald-500 transition-[stroke-dashoffset] duration-700"
          strokeDasharray={c} strokeDashoffset={c * (1 - percent / 100)}
        />
      </svg>
      <div className="absolute inset-0 flex flex-col items-center justify-center">
        <span className="text-2xl font-semibold tabular-nums">{Math.round(percent)}%</span>
        <span className="text-[11px] text-muted-foreground">paid</span>
      </div>
    </div>
  );
}

function Stat({ label, value, hint, hintClass }: { label: string; value: ReactNode; hint?: ReactNode; hintClass?: string }) {
  return (
    <div className="flex min-w-0 flex-col gap-0.5 rounded-xl bg-muted/50 px-4 py-3">
      <span className="text-xs uppercase tracking-wide text-muted-foreground">{label}</span>
      <span className="truncate text-xl font-semibold tabular-nums">{value}</span>
      {hint && <span className={cn("text-xs text-muted-foreground", hintClass)}>{hint}</span>}
    </div>
  );
}

type Rates = { then: number | null; now: number | null; paidUsd: number | null };

/**
 * The same loan in dollars: what it was worth when taken and what it is worth now, with what you have
 * really paid counted at the rate of each payment day. UAH loans only; rates are the National Bank's.
 */
function InDollars({ loan }: { loan: Loan }) {
  const [rates, setRates] = useState<Rates | null>(null);
  const taken = takenOn(loan);

  useEffect(() => {
    if (loan.currency !== 980) return;
    let live = true;
    const paid = paidPayments(loan);
    Promise.all([nbuRate("USD", taken), nbuRate("USD"), ...paid.map((p) => nbuRate("USD", p.due))]).then(([then, now, ...perPayment]) => {
      // A payment made before its date has no rate yet: it counts at today's
      const paidUsd = now ? paid.reduce((sum, p, i) => sum + p.amount / 100 / (perPayment[i] ?? now), 0) : null;
      if (live) setRates({ then, now, paidUsd });
    });
    return () => {
      live = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [loan]);

  if (loan.currency !== 980) return null;
  const left = remainingPayments(loan).reduce((sum, p) => sum + p.amount, 0);
  const paid = paidPayments(loan).reduce((sum, p) => sum + p.amount, 0);
  const total = totalToRepay(loan);

  if (rates === null) return <p className="text-sm text-muted-foreground">Loading National Bank rates…</p>;
  if (!rates.then || !rates.now || rates.paidUsd === null) return <p className="text-sm text-muted-foreground">The National Bank has no rate for one of the dates.</p>;
  const { then, now, paidUsd } = rates;

  const usd = (uah: number, rate: number) => uah / 100 / rate;
  const money = (dollars: number) => formatMoney(Math.round(dollars * 100), 840);
  // Everything the loan costs in dollars: what is paid (real rates) plus what is left (today's rate)
  const costThen = usd(total, then);
  const costNow = paidUsd + usd(left, now);
  const saved = costThen - costNow;
  const rows = [
    { label: "Borrowed", uah: loan.principal, then: usd(loan.principal, then), now: usd(loan.principal, now), note: "the same hryvnias, valued today" },
    { label: "Whole loan to repay", uah: total, then: usd(total, then), now: usd(total, now), note: "valued today" },
    { label: "Paid so far", uah: paid, then: usd(paid, then), now: paidUsd, note: "dollars you really spent: each payment at its day's rate" },
    { label: "Left to pay", uah: left, then: usd(left, then), now: usd(left, now), note: "valued today" },
  ];

  return (
    <div className="flex flex-col gap-3">
      <div className={cn("flex flex-wrap items-center gap-3 rounded-xl border px-4 py-3", saved >= 0 ? "border-emerald-500/30 bg-emerald-500/5" : "border-red-500/30 bg-red-500/5")}>
        {saved >= 0 ? <ArrowDownRight className={cn("size-6", GOOD)} /> : <ArrowUpRight className={cn("size-6", BAD)} />}
        <div className="flex flex-col">
          <span className="text-sm">
            The whole loan costs <b>{money(costNow)}</b> now against <b>{money(costThen)}</b> on the day it was taken
          </span>
          <span className={cn("text-sm font-semibold", saved >= 0 ? GOOD : BAD)}>
            {saved >= 0 ? "You gain" : "You lose"} {money(Math.abs(saved))} ({((saved / costThen) * 100).toFixed(1)}%) from the hryvnia rate
          </span>
        </div>
        <span className="ml-auto text-xs text-muted-foreground">
          USD/UAH {then.toFixed(2)} → {now.toFixed(2)}
        </span>
      </div>
      <div className="overflow-hidden rounded-xl border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead />
              <TableHead className="text-right">UAH</TableHead>
              <TableHead className="text-right">USD on {shortDate(taken)}</TableHead>
              <TableHead className="text-right">USD now</TableHead>
              <TableHead className="text-right">Change</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((r) => {
              const change = r.now - r.then;
              return (
                <TableRow key={r.label} title={r.note}>
                  <TableCell className="font-medium">{r.label}</TableCell>
                  <TableCell className="text-right tabular-nums">{formatMoney(r.uah, 980)}</TableCell>
                  <TableCell className="text-right tabular-nums">{money(r.then)}</TableCell>
                  <TableCell className="text-right tabular-nums">{money(r.now)}</TableCell>
                  <TableCell className={cn("text-right tabular-nums font-medium", change <= 0 ? GOOD : BAD)}>
                    {change > 0 ? "+" : "−"}
                    {money(Math.abs(change))}
                  </TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      </div>
      <p className="text-xs text-muted-foreground">
        National Bank rates. The loan is assumed taken on {date(taken)}, the month before the first payment. A negative change is good: the same hryvnias are fewer dollars.
      </p>
    </div>
  );
}

function LoanCard({ loan, onChanged }: { loan: Loan; onChanged: () => void }) {
  const [busy, setBusy] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const [showAll, setShowAll] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const payments = remainingPayments(loan);
  const owed = payments.reduce((sum, p) => sum + p.amount, 0);
  const next = payments[0];
  const overdue = payments.filter((p) => p.overdue);
  const percent = (loan.payments_paid / loan.payments_total) * 100;
  const shown = showAll ? payments : payments.slice(0, 6);

  const setPaid = async (paid: number) => {
    setBusy(true);
    setError(null);
    const response = await apiFetch(`/loans/${loan.id}`, { method: "PATCH", body: JSON.stringify({ payments_paid: paid }), headers: { "Content-Type": "application/json" } });
    setBusy(false);
    if (!response.ok) setError(await apiError(response));
    else onChanged();
  };

  const remove = async () => {
    setBusy(true);
    const response = await apiFetch(`/loans/${loan.id}`, { method: "DELETE" });
    setBusy(false);
    if (!response.ok) setError(await apiError(response));
    else onChanged();
  };

  return (
    // No card padding: the header band must reach the card's edges
    <Card className="gap-0 py-0">
      <CardHeader className="border-b bg-gradient-to-r from-muted/60 to-transparent py-4">
        <CardTitle className="flex items-center gap-3 text-lg">
          <BankBadge bank={loan.bank} className="size-9" />
          <span className="flex flex-col">
            {loan.name}
            <span className="text-xs font-normal text-muted-foreground">
              {bankName(loan.bank)} · borrowed {formatMoney(loan.principal, loan.currency)} · {loan.payments_total} payments
            </span>
          </span>
        </CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-6 py-5">
        <div className="flex flex-wrap items-center gap-6">
          <Ring percent={percent} />
          {next ? (
            <div className="grid min-w-0 flex-1 gap-3 sm:grid-cols-3">
              <Stat label="Left to pay" value={formatMoney(owed, loan.currency)} hint={`${payments.length} payments`} />
              <Stat
                label="Next payment"
                value={formatMoney(next.amount, loan.currency)}
                hint={
                  <span className="inline-flex items-center gap-1">
                    <CalendarClock className="size-3" />
                    by {date(next.due)}
                    {overdue.length > 0 && ` · ${overdue.length} overdue`}
                  </span>
                }
                hintClass={next.overdue ? BAD : undefined}
              />
              <Stat label="Paid" value={`${loan.payments_paid} of ${loan.payments_total}`} hint={formatMoney(loan.payments_paid * loan.payment, loan.currency)} />
            </div>
          ) : (
            <p className={cn("text-lg font-medium", GOOD)}>Paid off.</p>
          )}
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <Button size="sm" onClick={() => setPaid(loan.payments_paid + 1)} disabled={busy || !next}>
            {busy ? <Loader2 className="animate-spin" /> : <Check />}
            Mark next payment paid
          </Button>
          <Button size="sm" variant="outline" onClick={() => setPaid(loan.payments_paid - 1)} disabled={busy || loan.payments_paid === 0}>
            <RotateCcw />
            Undo
          </Button>
          {confirming ? (
            <Button size="sm" variant="destructive" onClick={remove} disabled={busy}>
              Delete for good?
            </Button>
          ) : (
            <Button size="sm" variant="ghost" className="ml-auto" onClick={() => setConfirming(true)} disabled={busy}>
              <Trash2 />
              Delete
            </Button>
          )}
          {error && <span className="text-sm text-destructive">{error}</span>}
        </div>

        <section className="flex flex-col gap-2">
          <h3 className="text-sm font-semibold">In dollars</h3>
          <InDollars loan={loan} />
        </section>

        {payments.length > 0 && (
          <section className="flex flex-col gap-2">
            <h3 className="text-sm font-semibold">Payment schedule</h3>
            <div className="overflow-hidden rounded-xl border">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>#</TableHead>
                    <TableHead>Due</TableHead>
                    <TableHead className="text-right">Payment</TableHead>
                    <TableHead className="text-right">Left after</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {shown.map((p, i) => (
                    <TableRow key={p.number} className={cn(i === 0 && "bg-emerald-500/5")}>
                      <TableCell className="text-muted-foreground">{p.number}</TableCell>
                      <TableCell className={cn(p.overdue && BAD)}>
                        {date(p.due)}
                        {i === 0 && <span className="ml-2 rounded-full bg-emerald-500/15 px-2 py-0.5 text-[11px] font-medium text-emerald-700 dark:text-emerald-300">next</span>}
                      </TableCell>
                      <TableCell className="text-right tabular-nums">{formatMoney(p.amount, loan.currency)}</TableCell>
                      <TableCell className="text-right tabular-nums text-muted-foreground">{formatMoney(p.left, loan.currency)}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
            {payments.length > 6 && (
              <Button variant="ghost" size="sm" className="self-start" onClick={() => setShowAll((v) => !v)}>
                <ChevronDown className={cn("transition-transform", showAll && "rotate-180")} />
                {showAll ? "Show fewer" : `Show all ${payments.length}`}
              </Button>
            )}
          </section>
        )}
      </CardContent>
    </Card>
  );
}

function AddLoan({ onDone }: { onDone: () => void }) {
  const [name, setName] = useState("");
  const [bank, setBank] = useState(1);
  const [currency, setCurrency] = useState(980);
  const [principal, setPrincipal] = useState("");
  const [payment, setPayment] = useState("");
  const [lastPayment, setLastPayment] = useState("");
  const [total, setTotal] = useState("");
  const [paid, setPaid] = useState("0");
  const [firstDue, setFirstDue] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const submit = async () => {
    const pay = parseMoney(payment);
    const last = parseMoney(lastPayment);
    const borrowed = parseMoney(principal);
    if (!name.trim() || !pay || last === null || borrowed === null || !Number(total) || !firstDue) {
      setError("Fill in the name, payment, number of payments and first due date");
      return;
    }
    setBusy(true);
    setError(null);
    const response = await apiFetch("/loans", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        name: name.trim(), bank, currency, principal: borrowed, payment: pay, last_payment: last,
        payments_total: Number(total), payments_paid: Number(paid) || 0, first_due: firstDue,
      }),
    });
    setBusy(false);
    if (!response.ok) setError(await apiError(response));
    else onDone();
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>New loan</CardTitle>
        <CardDescription>From the contract's payment schedule. Payments fall on the last day of each month, starting with the first due month.</CardDescription>
      </CardHeader>
      <CardContent className="grid gap-4 sm:grid-cols-3">
        <div className="flex flex-col gap-1.5 sm:col-span-3">
          <Label htmlFor="loan-name">Name</Label>
          <Input id="loan-name" value={name} onChange={(e) => setName(e.target.value)} placeholder="Monobank installment" />
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="loan-bank">Bank</Label>
          <select id="loan-bank" className={selectClass} value={bank} onChange={(e) => setBank(Number(e.target.value))}>
            {Object.entries(BANKS).map(([id, b]) => (
              <option key={id} value={id}>{b.name}</option>
            ))}
          </select>
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="loan-currency">Currency</Label>
          <select id="loan-currency" className={selectClass} value={currency} onChange={(e) => setCurrency(Number(e.target.value))}>
            {CURRENCIES.map((c) => (
              <option key={c.value} value={c.value}>{c.label}</option>
            ))}
          </select>
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="loan-principal">Amount borrowed</Label>
          <Input id="loan-principal" inputMode="decimal" value={principal} onChange={(e) => setPrincipal(e.target.value)} placeholder="110000.00" />
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="loan-payment">Monthly payment</Label>
          <Input id="loan-payment" inputMode="decimal" value={payment} onChange={(e) => setPayment(e.target.value)} placeholder="6673.33" />
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="loan-last">Last payment (empty: the same)</Label>
          <Input id="loan-last" inputMode="decimal" value={lastPayment} onChange={(e) => setLastPayment(e.target.value)} placeholder="6673.41" />
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="loan-total">Number of payments</Label>
          <Input id="loan-total" inputMode="numeric" value={total} onChange={(e) => setTotal(e.target.value)} placeholder="24" />
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="loan-paid">Paid so far</Label>
          <Input id="loan-paid" inputMode="numeric" value={paid} onChange={(e) => setPaid(e.target.value)} />
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="loan-first">First payment date</Label>
          <Input id="loan-first" type="date" value={firstDue} onChange={(e) => setFirstDue(e.target.value)} />
        </div>
        <div className="flex flex-wrap items-center gap-2 sm:col-span-3">
          <Button onClick={submit} disabled={busy}>
            {busy ? <Loader2 className="animate-spin" /> : <Plus />}
            Add loan
          </Button>
          {error && <span className="text-sm text-destructive">{error}</span>}
        </div>
      </CardContent>
    </Card>
  );
}
