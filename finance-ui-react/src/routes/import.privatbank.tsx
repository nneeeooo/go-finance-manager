import { createFileRoute, Link } from "@tanstack/react-router";
import { useEffect, useMemo, useState } from "react";
import { CheckCircle2, FileSpreadsheet, Loader2, Upload } from "lucide-react";

import { AccountPicker } from "@/components/AccountPicker";
import { BankBadge } from "@/components/BankBadge";
import { LoginRequired, PageHeader } from "@/components/PageHeader";
import { Button, buttonVariants } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { apiError, apiFetch } from "@/lib/api";
import { formatMoney } from "@/lib/banks";
import { cn } from "@/lib/utils";
import { useAuthStore } from "@/stores/auth-store";
import { useDashboardStore } from "@/stores/dashboard-store";

export const Route = createFileRoute("/import/privatbank")({
  validateSearch: (search: Record<string, unknown>): { account?: string } =>
    typeof search.account === "string" ? { account: search.account } : {},
  component: PrivatbankImport,
});

type Row = { date: string; category: string; description: string; amount: number; currency: string };
type Result = { total: number; new: number; dry_run: boolean; from: string; to: string; preview: Row[] };

const CURRENCY_NUMBER: Record<string, number> = { UAH: 980, USD: 840, EUR: 978, PLN: 985 };
const day = (iso: string) => new Date(iso).toLocaleDateString("uk-UA");

function PrivatbankImport() {
  const { account: preselected } = Route.useSearch();
  const user = useAuthStore((state) => state.user);
  const initialized = useAuthStore((state) => state.initialized);
  const allAccounts = useDashboardStore((state) => state.accounts);
  const accounts = useMemo(() => allAccounts.filter((a) => a.bank === 2), [allAccounts]);
  const fetchAccounts = useDashboardStore((state) => state.fetchAccounts);
  const resetTransactions = useDashboardStore((state) => state.resetTransactions);

  const [accountId, setAccountId] = useState(preselected ?? "");
  const [files, setFiles] = useState<File[]>([]);
  const [preview, setPreview] = useState<Result | null>(null);
  const [imported, setImported] = useState<Result | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (initialized && user) fetchAccounts().catch(console.error);
  }, [initialized, user, fetchAccounts]);

  // A new file or account starts over
  const restart = () => {
    setPreview(null);
    setImported(null);
    setError(null);
  };

  const send = async (dryRun: boolean) => {
    if (files.length === 0 || !accountId) return;
    const form = new FormData();
    form.append("account_id", accountId);
    for (const f of files) form.append("file", f); // overlapping statements count each operation once
    setBusy(true);
    setError(null);
    const response = await apiFetch(`/imports/privatbank${dryRun ? "?dry_run=true" : ""}`, { method: "POST", body: form });
    setBusy(false);
    if (!response.ok) {
      setError(await apiError(response));
      return;
    }
    const result: Result = await response.json();
    if (dryRun) {
      setPreview(result);
    } else {
      setImported(result);
      resetTransactions(); // the dashboard reloads with the new operations; this clears the accounts too
      fetchAccounts().catch(console.error);
    }
  };

  if (!initialized) return null;
  if (!user) return <LoginRequired />;

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto pb-4">
      <PageHeader
        title="Import PrivatBank statement"
        description="Privat24 → Card → Statement → Excel (.xlsx). Pick one or several files; operations already imported are skipped."
      />

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <BankBadge bank={2} className="size-7" />
            Statement
          </CardTitle>
          <CardDescription>Choose the account the statement belongs to, then the file.</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-4 sm:grid-cols-2">
          <div className="flex flex-col gap-1.5">
            <Label>Account</Label>
            <AccountPicker
              accounts={accounts}
              value={accountId}
              onChange={(id) => {
                setAccountId(id);
                restart();
              }}
            />
            {accounts.length === 0 && (
              <Link to="/accounts" className="text-xs text-muted-foreground underline">
                Add a PrivatBank account first
              </Link>
            )}
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="statement">Statements .xlsx</Label>
            <Input
              id="statement"
              type="file"
              multiple
              accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
              onChange={(e) => {
                setFiles(Array.from(e.target.files ?? []));
                restart();
              }}
            />
            {files.length > 1 && <span className="text-xs text-muted-foreground">{files.length} files</span>}
          </div>
          <div className="flex flex-wrap items-center gap-2 sm:col-span-2">
            <Button variant="outline" onClick={() => send(true)} disabled={busy || files.length === 0 || !accountId}>
              {busy && !preview ? <Loader2 className="animate-spin" /> : <FileSpreadsheet />}
              Check
            </Button>
            <Button onClick={() => send(false)} disabled={busy || !preview || preview.new === 0 || !!imported}>
              {busy && preview ? <Loader2 className="animate-spin" /> : <Upload />}
              {preview ? `Import ${preview.new} new` : "Import"}
            </Button>
            {error && <span className="text-sm text-destructive">{error}</span>}
          </div>
        </CardContent>
      </Card>

      {imported && (
        <Card>
          <CardContent className="flex flex-wrap items-center gap-3 text-sm">
            <CheckCircle2 className="size-5 text-emerald-600 dark:text-emerald-400" />
            <span>
              Imported {imported.new} new of {imported.total} operations ({day(imported.from)} – {day(imported.to)}).
            </span>
            <Link to="/" className={cn(buttonVariants({ variant: "outline", size: "sm" }), "ml-auto")}>
              Open dashboard
            </Link>
          </CardContent>
        </Card>
      )}

      {preview && !imported && (
        <Card>
          <CardHeader>
            <CardTitle>
              {preview.new} new of {preview.total} operations
            </CardTitle>
            <CardDescription>
              {day(preview.from)} – {day(preview.to)}
              {preview.new === 0 && " · everything is already imported"}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Date</TableHead>
                  <TableHead>Description</TableHead>
                  <TableHead>Category</TableHead>
                  <TableHead className="text-right">Amount</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {preview.preview.map((r, i) => (
                  <TableRow key={i}>
                    <TableCell className="tabular-nums text-muted-foreground">{new Date(r.date).toLocaleString("uk-UA")}</TableCell>
                    <TableCell className="max-w-64 truncate" title={r.description}>
                      {r.description}
                    </TableCell>
                    <TableCell className="text-muted-foreground">{r.category}</TableCell>
                    <TableCell className={cn("text-right tabular-nums", r.amount < 0 ? "text-red-600 dark:text-red-400" : "text-emerald-600 dark:text-emerald-400")}>
                      {formatMoney(r.amount, CURRENCY_NUMBER[r.currency] ?? 980)}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
            {preview.total > preview.preview.length && (
              <p className="mt-2 text-xs text-muted-foreground">First {preview.preview.length} of {preview.total} shown.</p>
            )}
          </CardContent>
        </Card>
      )}
    </div>
  );
}
