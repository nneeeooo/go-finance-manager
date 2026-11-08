import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { useEffect, useMemo, useState } from "react";
import { ArrowRight, Check, FileSpreadsheet, Loader2, Pencil, Plus, RefreshCw, Trash2, X } from "lucide-react";

import { BankBadge } from "@/components/BankBadge";
import { LoginRequired, PageHeader } from "@/components/PageHeader";
import { Badge } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import { Card, CardAction, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { apiError, apiFetch } from "@/lib/api";
import { BANK_IDS, BANKS, CURRENCIES, bankName, creditLine, formatMoney, parseMoney, type BankId } from "@/lib/banks";
import { getCurrencySymbol } from "@/lib/chart-utils";
import { cn } from "@/lib/utils";
import { useAuthStore } from "@/stores/auth-store";
import { bankFilterOf, useDashboardStore } from "@/stores/dashboard-store";
import type { Account } from "@/types";

export const Route = createFileRoute("/accounts")({
  component: AccountsPage,
});

function AccountsPage() {
  const user = useAuthStore((state) => state.user);
  const initialized = useAuthStore((state) => state.initialized);
  const accounts = useDashboardStore((state) => state.accounts);
  const fetchAccounts = useDashboardStore((state) => state.fetchAccounts);
  const [loading, setLoading] = useState(false);
  const [adding, setAdding] = useState(false);

  const reload = async () => {
    setLoading(true);
    await fetchAccounts().catch(console.error);
    setLoading(false);
  };

  useEffect(() => {
    if (initialized && user) fetchAccounts().catch(console.error);
  }, [initialized, user, fetchAccounts]);

  // Total balance per currency, hryvnia first
  const totals = useMemo(() => {
    const sums = new Map<number, number>();
    for (const a of accounts) sums.set(a.currency, (sums.get(a.currency) ?? 0) + a.balance);
    return [...sums].sort(([a], [b]) => (a === 980 ? -1 : b === 980 ? 1 : a - b));
  }, [accounts]);

  // Grouped by bank, in the bank order
  const groups = useMemo(
    () =>
      BANK_IDS.map((bank) => ({ bank, accounts: accounts.filter((a) => a.bank === bank) })).filter((g) => g.accounts.length > 0),
    [accounts],
  );

  if (!initialized) return null;
  if (!user) return <LoginRequired />;

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto pb-4">
      <PageHeader
        title="Accounts"
        description={`${accounts.length} ${accounts.length === 1 ? "account" : "accounts"} in ${groups.length} ${groups.length === 1 ? "bank" : "banks"}`}
        action={
          <div className="flex gap-2">
            <Button variant="outline" onClick={reload} disabled={loading} aria-label="Refresh">
              <RefreshCw className={cn(loading && "animate-spin")} />
            </Button>
            <Button onClick={() => setAdding((v) => !v)}>
              {adding ? <X /> : <Plus />}
              {adding ? "Cancel" : "Add account"}
            </Button>
          </div>
        }
      />

      {totals.length > 0 && (
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          {totals.map(([currency, sum]) => (
            <Card key={currency} size="sm">
              <CardHeader>
                <CardDescription>Total, {getCurrencySymbol(currency)}</CardDescription>
                <CardTitle className={cn("text-xl! font-semibold tabular-nums", sum < 0 && "text-red-600 dark:text-red-400")}>
                  {formatMoney(sum, currency)}
                </CardTitle>
              </CardHeader>
            </Card>
          ))}
        </div>
      )}

      {adding && (
        <AddAccountCard
          onCreated={() => {
            setAdding(false);
            reload();
          }}
        />
      )}

      {accounts.length === 0 && !loading && !adding ? (
        <Card>
          <CardContent className="flex flex-col items-center gap-3 py-10 text-center text-sm text-muted-foreground">
            <p>No accounts yet. Add one, then import its statements.</p>
            <Button onClick={() => setAdding(true)}>
              <Plus />
              Add account
            </Button>
          </CardContent>
        </Card>
      ) : (
        groups.map((g) => (
          <section key={g.bank} className="flex flex-col gap-2">
            <h2 className="text-sm font-medium text-muted-foreground">{bankName(g.bank)}</h2>
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
              {g.accounts.map((a) => (
                <AccountCard key={a.id} account={a} onChanged={reload} />
              ))}
            </div>
          </section>
        ))
      )}
    </div>
  );
}

function AccountCard({ account, onChanged }: { account: Account; onChanged: () => void }) {
  const [editing, setEditing] = useState(false);
  if (editing) return <EditAccountCard account={account} onDone={() => { setEditing(false); onChanged(); }} onCancel={() => setEditing(false)} />;
  return <AccountView account={account} onChanged={onChanged} onEdit={() => setEditing(true)} />;
}

function AccountView({ account, onChanged, onEdit }: { account: Account; onChanged: () => void; onEdit: () => void }) {
  const navigate = useNavigate();
  const setSelectedAccountId = useDashboardStore((state) => state.setSelectedAccountId);
  const resetTransactions = useDashboardStore((state) => state.resetTransactions);
  const [confirming, setConfirming] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const filter = bankFilterOf(account.bank);
  const importBank = filter === "all" ? null : filter;
  const externalId = account.external_id || account.monobank_account_id;

  const remove = async () => {
    setBusy(true);
    setError(null);
    const response = await apiFetch(`/accounts/${account.id}`, { method: "DELETE" });
    setBusy(false);
    if (!response.ok) {
      setError(await apiError(response));
      return;
    }
    resetTransactions(); // the dashboard reloads without this account's operations
    onChanged();
  };

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center gap-3">
          <BankBadge bank={account.bank} />
          <div className="flex min-w-0 flex-col">
            <CardTitle className="truncate">{account.name}</CardTitle>
            <CardDescription>
              {bankName(account.bank)} · {CURRENCIES.find((c) => c.value === account.currency)?.label ?? getCurrencySymbol(account.currency)}
            </CardDescription>
          </div>
        </div>
        <CardAction>
          {confirming ? (
            <div className="flex gap-1">
              <Button variant="destructive" size="xs" onClick={remove} disabled={busy}>
                {busy ? <Loader2 className="animate-spin" /> : <Trash2 />}
                Delete
              </Button>
              <Button variant="ghost" size="icon-xs" onClick={() => setConfirming(false)} aria-label="Keep the account">
                <X />
              </Button>
            </div>
          ) : (
            <div className="flex gap-1">
              <Button variant="ghost" size="icon-xs" onClick={onEdit} aria-label={`Edit ${account.name}`}>
                <Pencil />
              </Button>
              <Button variant="ghost" size="icon-xs" onClick={() => setConfirming(true)} aria-label={`Delete ${account.name}`}>
                <Trash2 />
              </Button>
            </div>
          )}
        </CardAction>
      </CardHeader>
      <CardContent className="flex flex-col gap-1">
        <span className={cn("text-2xl font-semibold tracking-tight tabular-nums", account.balance < 0 && "text-red-600 dark:text-red-400")}>
          {formatMoney(account.balance, account.currency)}
        </span>
        {account.credit_limit ? (
          <span className="text-xs text-muted-foreground tabular-nums">{creditLine(account.balance, account.credit_limit, account.currency)}</span>
        ) : null}
        {externalId && (
          <span className="truncate font-mono text-xs text-muted-foreground" title={externalId}>
            id …{externalId.slice(-6)}
          </span>
        )}
        {confirming && <p className="text-xs text-destructive">Deletes the account and hides its operations.</p>}
        {error && <p className="text-xs text-destructive">{error}</p>}
      </CardContent>
      <CardFooter className="gap-2">
        <Button
          variant="outline"
          size="sm"
          onClick={() => {
            setSelectedAccountId(account.id);
            navigate({ to: "/" });
          }}
        >
          Operations
          <ArrowRight />
        </Button>
        {importBank && (
          <Link to="/import" search={{ bank: importBank, account: account.id }} className={buttonVariants({ variant: "ghost", size: "sm" })}>
            <FileSpreadsheet />
            Import
          </Link>
        )}
      </CardFooter>
    </Card>
  );
}

/** Name, Monobank account id, balance and credit limit; the bank stays */
function EditAccountCard({ account, onDone, onCancel }: { account: Account; onDone: () => void; onCancel: () => void }) {
  const [name, setName] = useState(account.name);
  const [externalId, setExternalId] = useState(account.external_id || account.monobank_account_id || "");
  const [balance, setBalance] = useState((account.balance / 100).toFixed(2));
  const [limit, setLimit] = useState(((account.credit_limit ?? 0) / 100).toFixed(2));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const save = async (e: React.FormEvent) => {
    e.preventDefault();
    const minor = parseMoney(balance);
    const creditLimit = parseMoney(limit);
    if (minor === null || creditLimit === null || creditLimit < 0) {
      setError("Balance and limit must be numbers, the limit 0 or more");
      return;
    }
    setBusy(true);
    setError(null);
    const response = await apiFetch(`/accounts/${account.id}`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name: name.trim(), external_id: externalId.trim(), balance: minor, credit_limit: creditLimit }),
    });
    setBusy(false);
    if (!response.ok) {
      setError(await apiError(response));
      return;
    }
    onDone();
  };

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center gap-3">
          <BankBadge bank={account.bank} />
          <CardTitle>Edit account</CardTitle>
        </div>
      </CardHeader>
      <CardContent>
        <form onSubmit={save} className="flex flex-col gap-3">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor={`name-${account.id}`}>Name</Label>
            <Input id={`name-${account.id}`} required maxLength={100} value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          {account.bank === 1 && (
            <div className="flex flex-col gap-1.5">
              <Label htmlFor={`ext-${account.id}`}>Monobank account id</Label>
              <Input id={`ext-${account.id}`} className="font-mono" value={externalId} onChange={(e) => setExternalId(e.target.value)} />
            </div>
          )}
          <div className="flex flex-col gap-1.5">
            <Label htmlFor={`bal-${account.id}`}>Own balance, {getCurrencySymbol(account.currency)} (negative = owed)</Label>
            <Input id={`bal-${account.id}`} inputMode="decimal" value={balance} onChange={(e) => setBalance(e.target.value)} />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor={`limit-${account.id}`}>Credit limit, {getCurrencySymbol(account.currency)} (0 for a debit card)</Label>
            <Input id={`limit-${account.id}`} inputMode="decimal" value={limit} onChange={(e) => setLimit(e.target.value)} />
          </div>
          <div className="flex items-center gap-2">
            <Button type="submit" size="sm" disabled={busy || !name.trim()}>
              {busy ? <Loader2 className="animate-spin" /> : <Check />}
              Save
            </Button>
            <Button type="button" variant="ghost" size="sm" onClick={onCancel}>
              Cancel
            </Button>
            {error && <span className="text-xs text-destructive">{error}</span>}
          </div>
        </form>
      </CardContent>
    </Card>
  );
}

function AddAccountCard({ onCreated }: { onCreated: () => void }) {
  const [name, setName] = useState("");
  const [bank, setBank] = useState<BankId>(1);
  const [currency, setCurrency] = useState(980);
  const [externalId, setExternalId] = useState("");
  const [balance, setBalance] = useState("");
  const [limit, setLimit] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const bankItems = BANK_IDS.map((b) => ({ value: String(b), label: BANKS[b].name }));
  const currencyItems = CURRENCIES.map((c) => ({ value: String(c.value), label: c.label }));

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    const minor = parseMoney(balance);
    const creditLimit = parseMoney(limit);
    if (minor === null || creditLimit === null || creditLimit < 0) {
      setError("Balance and limit must be numbers, the limit 0 or more");
      return;
    }
    setBusy(true);
    setError(null);
    const response = await apiFetch("/accounts", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name: name.trim(), bank, currency, external_id: externalId.trim(), balance: minor, credit_limit: creditLimit }),
    });
    setBusy(false);
    if (!response.ok) {
      setError(await apiError(response));
      return;
    }
    onCreated();
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>New account</CardTitle>
        <CardDescription>
          {bank === 1
            ? "Monobank: the account id (from the Monobank API client-info) lets the app fetch statements and match webhooks."
            : bank === 2
              ? "PrivatBank: after creating it, import statement .xlsx files."
              : bank === 5
                ? "Власний Рахунок: after creating it, import the «Виписка по рахунку» .xlsx; it sets the credit limit and balance."
                : "Track cash or another bank by hand."}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={submit} className="grid gap-3 sm:grid-cols-2">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="acc-name">Name</Label>
            <Input id="acc-name" required maxLength={100} value={name} onChange={(e) => setName(e.target.value)} placeholder="Black card" />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label>Bank</Label>
            <Select items={bankItems} value={String(bank)} onValueChange={(v) => v && setBank(Number(v) as BankId)}>
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {bankItems.map((b) => (
                  <SelectItem key={b.value} value={b.value}>
                    {b.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="flex flex-col gap-1.5">
            <Label>Currency</Label>
            <Select items={currencyItems} value={String(currency)} onValueChange={(v) => v && setCurrency(Number(v))}>
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {currencyItems.map((c) => (
                  <SelectItem key={c.value} value={c.value}>
                    {c.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="acc-balance">Own balance now (negative = owed)</Label>
            <Input id="acc-balance" inputMode="decimal" value={balance} onChange={(e) => setBalance(e.target.value)} placeholder="0.00" />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="acc-limit">Credit limit</Label>
            <Input id="acc-limit" inputMode="decimal" value={limit} onChange={(e) => setLimit(e.target.value)} placeholder="0 for a debit card" />
          </div>
          {bank === 1 && (
            <div className="flex flex-col gap-1.5 sm:col-span-2">
              <Label htmlFor="acc-ext">Monobank account id</Label>
              <Input id="acc-ext" value={externalId} onChange={(e) => setExternalId(e.target.value)} className="font-mono" placeholder="e.g. kKGVoZuHWzqVoZuH" />
            </div>
          )}
          <div className="flex items-center gap-3 sm:col-span-2">
            <Button type="submit" disabled={busy || !name.trim()}>
              {busy ? <Loader2 className="animate-spin" /> : <Plus />}
              Create
            </Button>
            {error && <span className="text-sm text-destructive">{error}</span>}
            {bank === 1 && !externalId && <Badge variant="outline">without the id, month import is off</Badge>}
          </div>
        </form>
      </CardContent>
    </Card>
  );
}
