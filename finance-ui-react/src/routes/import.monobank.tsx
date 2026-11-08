import { createFileRoute, Link } from "@tanstack/react-router";
import { useEffect, useState, type ReactNode } from "react";
import { endOfDay, format, startOfDay, startOfMonth, subMonths } from "date-fns";
import { uk } from "date-fns/locale";
import type { DateRange } from "react-day-picker";
import {
  AlertCircle,
  Ban,
  CalendarIcon,
  CheckCircle2,
  Clock,
  CloudDownload,
  CreditCard,
  History,
  Loader2,
  RefreshCw,
  Search,
  Square,
} from "lucide-react";

import { BankBadge } from "@/components/BankBadge";
import { LoginRequired, PageHeader } from "@/components/PageHeader";
import { Badge } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import { Calendar } from "@/components/ui/calendar";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { apiError, apiFetch } from "@/lib/api";
import { creditLine, formatMoney } from "@/lib/banks";
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
  windows: { from: number; to: number }[]; // unix seconds, newest first
  windows_done: number;
  calls: number;
  fetched: number;
  created: number;
  newest_at?: string; // the newest operation fetched
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
  balance: number; // own money (Monobank's minus the credit limit), negative = owed
  credit_limit: number;
  masked_pan: string[];
  iban: string;
  linked_account_id?: string;
  newest_saved_at?: string; // the linked account's newest saved operation
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

// Monobank: one statement call a minute, each for at most 31 days + 1 hour; the API cuts ranges the same way
const WINDOW = 2682000;
const MAX_WINDOWS = 24;
const UPDATE_OVERLAP = 3 * 86400; // an update also re-reads 3 days before the newest saved operation (holds)
const FIRST_MONTH = new Date(2017, 10, 1); // Monobank launched in November 2017
const callsFor = (from: number, to: number) => (to < from ? 0 : Math.ceil((to - from + 1) / (WINDOW + 1)));
/** Where an update starts, as the API computes it: never less than one full window */
const updateFrom = (newest: string | undefined, now: number) =>
  Math.min(now - WINDOW, newest ? Date.parse(newest) / 1000 - UPDATE_OVERLAP : Infinity);
const longDay = (d: Date) => format(d, "d MMM yyyy", { locale: uk });
const day = (unix: number) => format(unix * 1000, "dd.MM.yy");
const unix = (d: Date) => Math.floor(d.getTime() / 1000);

/** A radio card, like the card picker */
function Choice({ checked, onSelect, icon, title, children }: { checked: boolean; onSelect: () => void; icon: ReactNode; title: string; children: ReactNode }) {
  return (
    <button
      type="button"
      role="radio"
      aria-checked={checked}
      onClick={onSelect}
      className={cn(
        "flex items-start gap-3 rounded-lg border p-3 text-left transition-colors hover:bg-muted/50 [&_svg]:mt-0.5 [&_svg]:size-4 [&_svg]:shrink-0 [&_svg]:text-muted-foreground",
        checked && "border-primary ring-1 ring-primary",
      )}
    >
      {icon}
      <span className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="font-medium">{title}</span>
        <span className="text-xs text-muted-foreground">{children}</span>
      </span>
    </button>
  );
}

function MonobankImport() {
  const { account: preselected } = Route.useSearch();
  const user = useAuthStore((state) => state.user);
  const initialized = useAuthStore((state) => state.initialized);
  const localAccounts = useDashboardStore((state) => state.accounts);
  const fetchAccounts = useDashboardStore((state) => state.fetchAccounts);
  const resetTransactions = useDashboardStore((state) => state.resetTransactions);

  const [today] = useState(() => new Date());
  const [token, setToken] = useState("");
  const [cards, setCards] = useState<MonoAccount[] | null>(null);
  const [cardId, setCardId] = useState("");
  const [mode, setMode] = useState<"update" | "range">("update");
  const [range, setRange] = useState<DateRange | undefined>(() => ({ from: startOfMonth(subMonths(today, 2)), to: today }));
  const [pickerOpen, setPickerOpen] = useState(false);
  const [job, setJob] = useState<Job | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState<"cards" | "start" | "cancel" | null>(null);
  const [clock, setClock] = useState(() => Date.now()); // ticks the countdown while a fetch runs

  const card = cards?.find((c) => c.id === cardId);
  const running = job?.status === "running";
  const localName = (id?: string) => localAccounts.find((a) => a.id === id)?.name;

  const now = unix(today);
  const updateStart = updateFrom(card?.newest_saved_at, now);
  const count =
    mode === "update"
      ? callsFor(updateStart, now)
      : range?.from && range.to
        ? callsFor(unix(startOfDay(range.from)), Math.min(unix(endOfDay(range.to)), now))
        : 0;

  useEffect(() => {
    if (initialized && user) fetchAccounts().catch(console.error);
  }, [initialized, user, fetchAccounts]);

  // The last fetch since the server started (it may still be running after a reload)
  useEffect(() => {
    if (!initialized || !user) return;
    apiFetch("/imports/monobank").then(async (r) => r.status === 200 && setJob(await r.json()));
  }, [initialized, user]);

  // Poll while it runs; refresh accounts and the dashboard when it ends
  useEffect(() => {
    if (!running) return;
    const poll = setInterval(async () => {
      const r = await apiFetch("/imports/monobank");
      if (r.status !== 200) return;
      const next: Job = await r.json();
      setJob(next);
      if (next.status !== "running") {
        // The card's "New operations" now starts from what this fetch saved
        const newest = next.newest_at;
        if (newest)
          setCards((list) =>
            list?.map((c) =>
              c.linked_account_id === next.account_id && (!c.newest_saved_at || Date.parse(newest) > Date.parse(c.newest_saved_at))
                ? { ...c, newest_saved_at: newest }
                : c,
            ) ?? null,
          );
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
        body: JSON.stringify({ bank: 1, name: `Monobank ${cardName(card)}`, currency: card.currency, balance: card.balance, credit_limit: card.credit_limit, external_id: card.id }),
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
    // No dates: the API updates since the newest saved operation
    const days = mode === "range" && range?.from && range.to ? { from: format(range.from, "yyyy-MM-dd"), to: format(range.to, "yyyy-MM-dd") } : {};
    const response = await apiFetch("/imports/monobank", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ account_id: accountId, token: token.trim(), ...days }),
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
  if (!user) return <LoginRequired />;

  const wait = job?.next_call_at ? Math.max(0, Math.ceil((Date.parse(job.next_call_at) - clock) / 1000)) : 0;
  const newest = card?.newest_saved_at ? new Date(card.newest_saved_at) : null;

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto pb-4">
      <PageHeader
        title="Fetch Monobank history"
        description="Pick a card, then fetch its new operations or a date range. Monobank allows one call a minute, 31 days each."
      />

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <BankBadge bank={1} className="size-7" />
            1. Card
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
                    <span className={cn("tabular-nums text-sm", c.balance < 0 && "text-red-600 dark:text-red-400")}>{formatMoney(c.balance, c.currency)}</span>
                    {c.credit_limit > 0 && <span className="text-xs text-muted-foreground tabular-nums">{creditLine(c.balance, c.credit_limit, c.currency)}</span>}
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

      <Card className={cn(!card && "pointer-events-none opacity-60")} aria-disabled={!card}>
        <CardHeader>
          <CardTitle>2. What to fetch</CardTitle>
          <CardDescription>
            {card ? `For ${cardName(card)}.` : "Pick a card first."} Saved operations are updated, never duplicated.
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <div role="radiogroup" aria-label="What to fetch" className="grid gap-2 sm:grid-cols-2">
            <Choice checked={mode === "update"} onSelect={() => setMode("update")} icon={<RefreshCw />} title="New operations">
              {newest
                ? `Newest saved: ${longDay(newest)}. Fetches from ${longDay(new Date(updateStart * 1000))}, so pending holds are refreshed.`
                : "Nothing saved yet: fetches the last 31 days."}
            </Choice>
            <Choice checked={mode === "range"} onSelect={() => setMode("range")} icon={<History />} title="Date range">
              Older history, or a period again. Up to about 2 years at once.
            </Choice>
          </div>

          {mode === "range" && (
            <div className="flex flex-col gap-1.5">
              <Label>Period</Label>
              <Popover open={pickerOpen} onOpenChange={setPickerOpen}>
                <PopoverTrigger render={<Button variant="outline" className="w-full justify-start font-normal sm:w-80" />}>
                  <CalendarIcon />
                  {range?.from ? (range.to ? `${longDay(range.from)} – ${longDay(range.to)}` : `${longDay(range.from)} – …`) : "Pick the dates"}
                </PopoverTrigger>
                <PopoverContent className="w-auto p-0" align="start">
                  <Calendar
                    mode="range"
                    numberOfMonths={2}
                    captionLayout="dropdown"
                    locale={uk}
                    selected={range}
                    onSelect={setRange}
                    defaultMonth={range?.from ? subMonths(range.to ?? range.from, 1) : subMonths(today, 1)}
                    startMonth={FIRST_MONTH}
                    endMonth={today}
                    disabled={{ after: today }}
                  />
                </PopoverContent>
              </Popover>
            </div>
          )}

          <div className="flex flex-wrap items-center gap-3 border-t pt-4">
            <Button onClick={start} disabled={busy !== null || running || !card || count < 1 || count > MAX_WINDOWS}>
              {busy === "start" ? <Loader2 className="animate-spin" /> : <CloudDownload />}
              Fetch
            </Button>
            {count > MAX_WINDOWS ? (
              <span className="text-sm text-destructive">Too long for one fetch: pick about 2 years at most</span>
            ) : count === 0 ? (
              <span className="text-sm text-muted-foreground">Pick the start and end dates</span>
            ) : (
              <span className="text-sm text-muted-foreground">
                {count === 1 ? "1 call to Monobank, a few seconds" : `${count} calls to Monobank, about ${count - 1} min`}
              </span>
            )}
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
              {job.fetched} operations fetched, {job.created} new · {job.calls} {job.calls === 1 ? "call" : "calls"} to Monobank
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
                style={{ width: `${(job.windows_done / Math.max(1, job.windows.length)) * 100}%` }}
              />
            </div>
            <div className="flex flex-wrap gap-1.5">
              {job.windows.map((w, i) => (
                <Badge
                  key={w.to}
                  variant={i < job.windows_done ? "default" : running && i === job.windows_done ? "secondary" : "outline"}
                  className="tabular-nums"
                >
                  {day(w.from)} – {day(w.to)}
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
