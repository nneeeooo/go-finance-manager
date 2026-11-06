import { createFileRoute, Link } from "@tanstack/react-router";
import { useEffect, useMemo, useState } from "react";
import { AlertCircle, CheckCircle2, Clock, CloudDownload, Loader2 } from "lucide-react";

import { AccountPicker } from "@/components/AccountPicker";
import { BankBadge } from "@/components/BankBadge";
import { LoginRequired, PageHeader } from "@/components/PageHeader";
import { Badge } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { apiError, apiFetch } from "@/lib/api";
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
  status: "running" | "done" | "error";
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

const MAX_MONTHS = 24;
const ym = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}`;
const monthLabel = (s: string) => {
  const [y, m] = s.split("-").map(Number);
  return new Date(y, m - 1, 1).toLocaleDateString("uk-UA", { month: "long", year: "numeric" });
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
  const allAccounts = useDashboardStore((state) => state.accounts);
  const accounts = useMemo(() => allAccounts.filter((a) => a.bank === 1), [allAccounts]);
  const fetchAccounts = useDashboardStore((state) => state.fetchAccounts);
  const resetTransactions = useDashboardStore((state) => state.resetTransactions);

  const now = new Date();
  const [accountId, setAccountId] = useState(preselected ?? "");
  const [token, setToken] = useState("");
  const [from, setFrom] = useState(ym(new Date(now.getFullYear(), now.getMonth() - 2, 1)));
  const [to, setTo] = useState(ym(now));
  const [job, setJob] = useState<Job | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [clock, setClock] = useState(() => Date.now()); // ticks the countdown while a fetch runs

  const account = accounts.find((a) => a.id === accountId);
  const count = monthsBetween(from, to);
  const running = job?.status === "running";

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
        fetchAccounts().catch(console.error);
        resetTransactions();
      }
    }, 2000);
    const timer = setInterval(() => setClock(Date.now()), 1000);
    return () => {
      clearInterval(poll);
      clearInterval(timer);
    };
  }, [running, fetchAccounts, resetTransactions]);

  const start = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    const response = await apiFetch("/imports/monobank", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ account_id: accountId, token: token.trim(), from, to }),
    });
    setBusy(false);
    if (!response.ok) {
      setError(await apiError(response));
      return;
    }
    setJob(await response.json());
    setToken(""); // not kept in the page either
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
            Months to fetch
          </CardTitle>
          <CardDescription>
            Already saved operations are updated, not duplicated. Live purchases arrive by webhook, so this is for history.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={start} className="grid gap-4 sm:grid-cols-2">
            <div className="flex flex-col gap-1.5 sm:col-span-2">
              <Label>Account</Label>
              <AccountPicker accounts={accounts} value={accountId} onChange={setAccountId} />
              {account && !(account.external_id || account.monobank_account_id) && (
                <span className="text-xs text-destructive">
                  This account has no Monobank account id. Add it with Edit on the Accounts page.
                </span>
              )}
              {accounts.length === 0 && (
                <Link to="/accounts" className="text-xs text-muted-foreground underline">
                  Add a Monobank account first
                </Link>
              )}
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="from">From</Label>
              <Input id="from" type="month" required max={to} value={from} onChange={(e) => setFrom(e.target.value)} />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="to">To</Label>
              <Input id="to" type="month" required min={from} max={ym(now)} value={to} onChange={(e) => setTo(e.target.value)} />
            </div>
            <div className="flex flex-col gap-1.5 sm:col-span-2">
              <Label htmlFor="token">Monobank API token</Label>
              <Input
                id="token"
                type="password"
                autoComplete="off"
                value={token}
                onChange={(e) => setToken(e.target.value)}
                placeholder="From api.monobank.ua; empty uses the server token (admins)"
              />
              <span className="text-xs text-muted-foreground">Used for this fetch only. Not saved by the server or this page.</span>
            </div>
            <div className="flex flex-wrap items-center gap-3 sm:col-span-2">
              <Button type="submit" disabled={busy || running || !accountId || count < 1 || count > MAX_MONTHS}>
                {busy ? <Loader2 className="animate-spin" /> : <CloudDownload />}
                Fetch {count > 0 ? `${count} ${count === 1 ? "month" : "months"}` : ""}
              </Button>
              {count > MAX_MONTHS && <span className="text-sm text-destructive">At most {MAX_MONTHS} months at once</span>}
              {count >= 1 && count <= MAX_MONTHS && <span className="text-sm text-muted-foreground">about {count} min</span>}
              {error && <span className="text-sm text-destructive">{error}</span>}
            </div>
          </form>
        </CardContent>
      </Card>

      {job && (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              {job.status === "running" && <Loader2 className="size-4 animate-spin" />}
              {job.status === "done" && <CheckCircle2 className="size-4 text-emerald-600 dark:text-emerald-400" />}
              {job.status === "error" && <AlertCircle className="size-4 text-destructive" />}
              {job.status === "running" ? "Fetching…" : job.status === "done" ? "Done" : "Stopped"}
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
                  className="capitalize"
                >
                  {monthLabel(m)}
                </Badge>
              ))}
            </div>
            {job.error && <p className="text-sm text-destructive">{job.error}</p>}
            {job.status === "done" && (
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
