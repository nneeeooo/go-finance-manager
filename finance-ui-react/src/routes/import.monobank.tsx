import { createFileRoute, Link } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { AlertCircle, Ban, CheckCircle2, Clock, CloudDownload, CreditCard, Loader2, Search, Square } from "lucide-react";

import { BankBadge } from "@/components/BankBadge";
import { LoginRequired, PageHeader } from "@/components/PageHeader";
import { Badge } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { apiError, apiFetch } from "@/lib/api";
import { formatMoney } from "@/lib/banks";
import { cn } from "@/lib/utils";
import { useAuthStore } from "@/stores/auth-store";
import { useDashboardStore } from "@/stores/dashboard-store";

export const Route = createFileRoute("/import/monobank")({
  validateSearch: (search: Record<string, unknown>): { account?: string } =>
    typeof search.account === "string" ? { account: search.account } : {},
  component: MonobankImport,
});

type Job = {
  account_id: string;
  status: "running" | "done" | "error" | "cancelled";
  months: string[];
  months_done: number;
  current?: string;
  fetched: number;
  created: number;
  next_call_at?: string;
  error?: string;
  started_at: string;
  finished_at?: string;
};

// One card account as Monobank lists it, with the user's account it is already linked to
type MonoAccount = {
  id: string;
  type: string;
  currency: number;
  balance: number;
  credit_limit: number;
  masked_pan: string[];
  iban: string;
  linked_account_id?: string;
};

const CARD_TYPES: Record<string, string> = {
  black: "Black card",
  white: "White card",
  platinum: "Platinum card",
  iron: "Iron card",
  fop: "FOP account",
  yellow: "Yellow card",
  eAid: "eSupport card",
  madeInUkraine: "Made in Ukraine card",
};
const cardName = (a: MonoAccount) => {
  const pan = a.masked_pan[0];
  return `${CARD_TYPES[a.type] ?? a.type}${pan ? ` ·${pan.slice(-4)}` : ""}`;
};

const MAX_MONTHS = 24;
const ym = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}`;
const monthLabel = (s: string) => {
  const [y, m] = s.split("-").map(Number);
  const label = new Date(y, m - 1, 1).toLocaleDateString("uk-UA", { month: "long", year: "numeric" });
  return label.charAt(0).toUpperCase() + label.slice(1); // "Листопад 2026 р.", not "Р."
};
const monthsBetween = (from: string, to: string) => {
  const [fy, fm] = from.split("-").map(Number);
  const [ty, tm] = to.split("-").map(Number);
  return (ty - fy) * 12 + (tm - fm) + 1;
};

function MonobankImport() {
  const { account: preselected } = Route.useSearch();
  const accessToken = useAuthStore((state) => state.accessToken);
  const initialized = useAuthStore((state) => state.initialized);
  const localAccounts = useDashboardStore((state) => state.accounts);
  const fetchAccounts = useDashboardStore((state) => state.fetchAccounts);
  const resetTransactions = useDashboardStore((state) => state.resetTransactions);

  const now = new Date();
  const [token, setToken] = useState("");
  const [cards, setCards] = useState<MonoAccount[] | null>(null);
  const [cardId, setCardId] = useState("");
  const [from, setFrom] = useState(ym(new Date(now.getFullYear(), now.getMonth() - 2, 1)));
  const [to, setTo] = useState(ym(now));
  const [job, setJob] = useState<Job | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState<"cards" | "start" | "cancel" | null>(null);
  const [clock, setClock] = useState(() => Date.now()); // ticks the countdown while a fetch runs

  const card = cards?.find((c) => c.id === cardId);
  const count = monthsBetween(from, to);
  const running = job?.status === "running";
  const localName = (id?: string) => localAccounts.find((a) => a.id === id)?.name;

  useEffect(() => {
    if (initialized && accessToken) fetchAccounts().catch(console.error);
  }, [initialized, accessToken, fetchAccounts]);

  // The last fetch since the server started (it may still be running after a reload)
  useEffect(() => {
    if (!initialized || !accessToken) return;
    apiFetch("/imports/monobank").then(async (r) => r.status === 200 && setJob(await r.json()));
  }, [initialized, accessToken]);

  // Poll while it runs; refresh accounts and the dashboard when it ends
  useEffect(() => {
    if (!running) return;
    const poll = setInterval(async () => {
      const r = await apiFetch("/imports/monobank");
      if (r.status !== 200) return;
      const next: Job = await r.json();
      setJob(next);
      if (next.status !== "running") {
        resetTransactions(); // clears the accounts too, so load them after
        fetchAccounts().catch(console.error);
      }
    }, 2000);
    const timer = setInterval(() => setClock(Date.now()), 1000);
    return () => {
      clearInterval(poll);
      clearInterval(timer);
    };
  }, [running, fetchAccounts, resetTransactions]);

  const loadCards = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy("cards");
    setError(null);
    const response = await apiFetch("/imports/monobank/accounts", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ token: token.trim() }),
    });
    setBusy(null);
    if (!response.ok) {
      setError(await apiError(response));
      return;
    }
    const list: MonoAccount[] = await response.json();
    setCards(list);
    // The account the Accounts page sent us to, else the only card
    const pick = list.find((c) => preselected && c.linked_account_id === preselected) ?? (list.length === 1 ? list[0] : undefined);
    setCardId(pick?.id ?? "");
  };

  const start = async () => {
    if (!card) return;
    setBusy("start");
    setError(null);
    // A card not yet among the user's accounts becomes one first
    let accountId = card.linked_account_id;
    if (!accountId) {
      const created = await apiFetch("/accounts", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ bank: 1, name: `Monobank ${cardName(card)}`, currency: card.currency, balance: card.balance, external_id: card.id }),
      });
      if (!created.ok) {
        setBusy(null);
        setError(await apiError(created));
        return;
      }
      accountId = ((await created.json()) as { id: string }).id;
      setCards((list) => list?.map((c) => (c.id === card.id ? { ...c, linked_account_id: accountId } : c)) ?? null);
      fetchAccounts().catch(console.error);
    }
    const response = await apiFetch("/imports/monobank", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ account_id: accountId, token: token.trim(), from, to }),
    });
    setBusy(null);
    if (!response.ok) {
      setError(await apiError(response));
      return;
    }
    setJob(await response.json());
  };

  const cancel = async () => {
    setBusy("cancel");
    const response = await apiFetch("/imports/monobank", { method: "DELETE" });
    setBusy(null);
    if (response.ok) {
      setJob(await response.json());
      resetTransactions(); // clears the accounts too, so load them after
      fetchAccounts().catch(console.error);
    } else if (response.status !== 404) {
      setError(await apiError(response));
    }
  };

  if (!initialized) return null;
  if (!accessToken) return <LoginRequired />;

  const wait = job?.next_call_at ? Math.max(0, Math.ceil((Date.parse(job.next_call_at) - clock) / 1000)) : 0;

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto pb-4">
      <PageHeader
        title="Fetch Monobank history"
        description="Loads statements month by month. Monobank allows one request a minute, so a month takes about a minute."
      />

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <BankBadge bank={1} className="size-7" />
            1. Your Monobank cards
          </CardTitle>
          <CardDescription>
            Get a personal token on <a href="https://api.monobank.ua/" target="_blank" rel="noreferrer" className="underline">api.monobank.ua</a>.
            It is used for this page only: the server does not save it.
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <form onSubmit={loadCards} className="flex flex-wrap items-end gap-2">
            <div className="flex min-w-64 flex-1 flex-col gap-1.5">
              <Label htmlFor="token">Monobank API token</Label>
              <Input
                id="token"
                type="password"
                autoComplete="off"
                value={token}
                onChange={(e) => setToken(e.target.value)}
                placeholder="Empty uses the server token (admins)"
              />
            </div>
            <Button type="submit" variant="outline" disabled={busy !== null || running}>
              {busy === "cards" ? <Loader2 className="animate-spin" /> : <Search />}
              Show cards
            </Button>
          </form>

          {cards && cards.length === 0 && <p className="text-sm text-muted-foreground">This token has no card accounts.</p>}
          {cards && cards.length > 0 && (
            <div role="radiogroup" aria-label="Card to fetch" className="grid gap-2 sm:grid-cols-2">
              {cards.map((c) => (
                <button
                  key={c.id}
                  type="button"
                  role="radio"
                  aria-checked={c.id === cardId}
                  disabled={running}
                  onClick={() => setCardId(c.id)}
                  className={cn(
                    "flex items-start gap-3 rounded-lg border p-3 text-left transition-colors hover:bg-muted/50 disabled:opacity-60",
                    c.id === cardId && "border-primary ring-1 ring-primary",
                  )}
                >
                  <CreditCard className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
                  <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                    <span className="font-medium">{cardName(c)}</span>
                    <span className="tabular-nums text-sm">{formatMoney(c.balance, c.currency)}</span>
                    <span className="truncate text-xs text-muted-foreground">
                      {c.linked_account_id ? `Saved as “${localName(c.linked_account_id) ?? "your account"}”` : "New: will be added to your accounts"}
                    </span>
                  </span>
                </button>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      <Card className={cn(!card && "opacity-60")}>
        <CardHeader>
          <CardTitle>2. Months to fetch</CardTitle>
          <CardDescription>
            Already saved operations are updated, not duplicated. Live purchases arrive by webhook, so this is for history.
          </CardDescription>
        </CardHeader>
        <CardContent className="grid gap-4 sm:grid-cols-2">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="from">From</Label>
            <Input id="from" type="month" required max={to} value={from} onChange={(e) => setFrom(e.target.value)} />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="to">To</Label>
            <Input id="to" type="month" required min={from} max={ym(now)} value={to} onChange={(e) => setTo(e.target.value)} />
          </div>
          <div className="flex flex-wrap items-center gap-3 sm:col-span-2">
            <Button onClick={start} disabled={busy !== null || running || !card || count < 1 || count > MAX_MONTHS}>
              {busy === "start" ? <Loader2 className="animate-spin" /> : <CloudDownload />}
              Fetch {count > 0 ? `${count} ${count === 1 ? "month" : "months"}` : ""}
            </Button>
            {count > MAX_MONTHS && <span className="text-sm text-destructive">At most {MAX_MONTHS} months at once</span>}
            {count >= 1 && count <= MAX_MONTHS && <span className="text-sm text-muted-foreground">about {count} min</span>}
          </div>
        </CardContent>
      </Card>

      {error && (
        <p className="flex items-center gap-2 text-sm text-destructive">
          <AlertCircle className="size-4" />
          {error}
        </p>
      )}

      {job && (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              {job.status === "running" && <Loader2 className="size-4 animate-spin" />}
              {job.status === "done" && <CheckCircle2 className="size-4 text-emerald-600 dark:text-emerald-400" />}
              {job.status === "error" && <AlertCircle className="size-4 text-destructive" />}
              {job.status === "cancelled" && <Ban className="size-4 text-muted-foreground" />}
              {{ running: "Fetching…", done: "Done", error: "Stopped by an error", cancelled: "Cancelled" }[job.status]}
              {localName(job.account_id) && <span className="font-normal text-muted-foreground">· {localName(job.account_id)}</span>}
            </CardTitle>
            <CardDescription>
              {job.fetched} operations fetched, {job.created} new
              {job.status === "running" && job.current && ` · now ${monthLabel(job.current)}`}
              {job.status === "running" && wait > 0 && (
                <span className="ml-2 inline-flex items-center gap-1">
                  <Clock className="size-3" />
                  next request in {wait} s
                </span>
              )}
            </CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col gap-3">
            <div className="h-2 overflow-hidden rounded-full bg-muted">
              <div
                className={cn("h-full rounded-full transition-all", job.status === "error" ? "bg-destructive" : "bg-primary")}
                style={{ width: `${(job.months_done / Math.max(1, job.months.length)) * 100}%` }}
              />
            </div>
            <div className="flex flex-wrap gap-1.5">
              {job.months.map((m, i) => (
                <Badge
                  key={m}
                  variant={i < job.months_done ? "default" : m === job.current ? "secondary" : "outline"}
                >
                  {monthLabel(m)}
                </Badge>
              ))}
            </div>
            {job.error && <p className="text-sm text-destructive">{job.error}</p>}
            {job.status === "running" && (
              <Button variant="outline" size="sm" className="self-start" onClick={cancel} disabled={busy === "cancel"}>
                {busy === "cancel" ? <Loader2 className="animate-spin" /> : <Square />}
                Cancel
              </Button>
            )}
            {job.status !== "running" && job.created > 0 && (
              <Link to="/" className={cn(buttonVariants({ variant: "outline", size: "sm" }), "self-start")}>
                Open dashboard
              </Link>
            )}
          </CardContent>
        </Card>
      )}
    </div>
  );
}
