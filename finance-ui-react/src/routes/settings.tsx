import { createFileRoute } from "@tanstack/react-router";
import { useCallback, useEffect, useState } from "react";
import { Check, Copy, Loader2, LogOut, Plus, ShieldCheck, Trash2, X } from "lucide-react";

import { LoginRequired, PageHeader } from "@/components/PageHeader";
import { useTheme } from "@/components/ThemeProvider";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { API_BASE_URL, apiError, apiFetch } from "@/lib/api";
import { useAuthStore } from "@/stores/auth-store";
import { useDashboardStore } from "@/stores/dashboard-store";
import type { AllowedEmail, Role, User } from "@/types";

export const Route = createFileRoute("/settings")({
  component: SettingsPage,
});

const THEMES = [
  { value: "system", label: "System" },
  { value: "dark", label: "Dark" },
  { value: "light", label: "Light" },
];
const ROLES = [
  { value: "regular", label: "Regular" },
  { value: "admin", label: "Admin" },
];

function SettingsPage() {
  const user = useAuthStore((state) => state.user);
  const initialized = useAuthStore((state) => state.initialized);
  const [me, setMe] = useState<User | null>(null);

  useEffect(() => {
    if (!initialized || !user) return;
    apiFetch("/users/me").then(async (r) => r.ok && setMe(await r.json()));
  }, [initialized, user]);

  if (!initialized) return null;
  if (!user) return <LoginRequired />;

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto pb-4">
      <PageHeader title="Settings" description="Your profile, appearance and access" />
      <div className="grid gap-4 lg:grid-cols-2">
        <ProfileCard me={me} />
        <AppearanceCard />
        <WebhookCard />
      </div>
      {me?.role === "admin" && <AllowListCard me={me} />}
    </div>
  );
}

function ProfileCard({ me }: { me: User | null }) {
  const logout = useAuthStore((state) => state.logout);
  const resetTransactions = useDashboardStore((state) => state.resetTransactions);

  return (
    <Card>
      <CardHeader>
        <CardTitle>Profile</CardTitle>
        <CardDescription className="flex flex-wrap items-center gap-2">
          {me ? me.email : <Loader2 className="size-3 animate-spin" />}
          {me && <Badge variant={me.role === "admin" ? "default" : "secondary"}>{me.role}</Badge>}
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        <Button
          variant="ghost"
          className="self-start"
          onClick={() => {
            resetTransactions();
            void logout();
          }}
        >
          <LogOut />
          Sign out
        </Button>
      </CardContent>
    </Card>
  );
}

function AppearanceCard() {
  const { theme, setTheme } = useTheme();
  return (
    <Card>
      <CardHeader>
        <CardTitle>Appearance</CardTitle>
        <CardDescription>Saved in this browser</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-1.5">
        <Label>Theme</Label>
        <Select items={THEMES} value={theme} onValueChange={(v) => v && setTheme(v as typeof theme)}>
          <SelectTrigger className="w-48">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {THEMES.map((t) => (
              <SelectItem key={t.value} value={t.value}>
                {t.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </CardContent>
    </Card>
  );
}

function WebhookCard() {
  const [copied, setCopied] = useState(false);
  const url = new URL(`${API_BASE_URL}/webhooks/monobank`, window.location.origin).toString();
  return (
    <Card className="lg:col-span-2">
      <CardHeader>
        <CardTitle>Monobank live updates</CardTitle>
        <CardDescription>
          Register this URL as your Monobank webhook (api.monobank.ua → personal/webhook) and every purchase arrives within
          seconds. If the server has MONOBANK_WEBHOOK_SECRET, add <code>?token=&lt;secret&gt;</code>.
        </CardDescription>
      </CardHeader>
      <CardContent className="flex gap-2">
        <Input readOnly value={url} className="font-mono text-xs" onFocus={(e) => e.target.select()} />
        <Button
          variant="outline"
          onClick={() => {
            navigator.clipboard?.writeText(url);
            setCopied(true);
            setTimeout(() => setCopied(false), 1500);
          }}
          aria-label="Copy webhook URL"
        >
          {copied ? <Check /> : <Copy />}
        </Button>
      </CardContent>
    </Card>
  );
}

/** Admin only: who may register and with which role */
function AllowListCard({ me }: { me: User }) {
  const [list, setList] = useState<AllowedEmail[] | null>(null);
  const [email, setEmail] = useState("");
  const [role, setRole] = useState<Role>("regular");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [confirming, setConfirming] = useState<string | null>(null);

  const run = useCallback(async (request: Promise<Response>) => {
    setBusy(true);
    setError(null);
    const response = await request;
    setBusy(false);
    if (!response.ok) {
      setError(await apiError(response));
      return false;
    }
    setList(await response.json());
    return true;
  }, []);

  useEffect(() => {
    apiFetch("/admin/allowed-emails").then(async (r) => (r.ok ? setList(await r.json()) : setError(await apiError(r))));
  }, []);

  const json = (method: string, body: object) => ({ method, headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
  const add = async (e: React.FormEvent) => {
    e.preventDefault();
    if (await run(apiFetch("/admin/allowed-emails", json("POST", { email: email.trim(), role })))) {
      setEmail("");
      setRole("regular");
    }
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <ShieldCheck className="size-4" />
          Access
        </CardTitle>
        <CardDescription>
          Only these emails can register, with the role shown. Removing someone ends their sessions; their accounts and
          operations stay, and inviting them again restores them.
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-3">
        {list === null ? (
          <Loader2 className="size-4 animate-spin text-muted-foreground" />
        ) : (
          <ul className="flex flex-col divide-y rounded-lg border">
            {list.map((entry) => {
              const self = entry.email.toLowerCase() === me.email.toLowerCase();
              return (
                <li key={entry.id} className="flex flex-wrap items-center gap-2 px-3 py-2">
                  <span className="min-w-0 flex-1 truncate">{entry.email}</span>
                  {self && <Badge variant="outline">you</Badge>}
                  <Badge variant={entry.registered ? "secondary" : "outline"}>{entry.registered ? "registered" : "invited"}</Badge>
                  <Select
                    items={ROLES}
                    value={entry.role}
                    disabled={self || busy}
                    onValueChange={(v) => v && v !== entry.role && run(apiFetch(`/admin/allowed-emails/${entry.id}`, json("PATCH", { role: v })))}
                  >
                    <SelectTrigger size="sm" className="w-28" aria-label={`Role of ${entry.email}`}>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {ROLES.map((r) => (
                        <SelectItem key={r.value} value={r.value}>
                          {r.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  {!self &&
                    (confirming === entry.id ? (
                      <div className="flex gap-1">
                        <Button
                          variant="destructive"
                          size="xs"
                          disabled={busy}
                          onClick={() => {
                            setConfirming(null);
                            run(apiFetch(`/admin/allowed-emails/${entry.id}`, { method: "DELETE" }));
                          }}
                        >
                          <Trash2 />
                          Remove
                        </Button>
                        <Button variant="ghost" size="icon-xs" onClick={() => setConfirming(null)} aria-label="Keep">
                          <X />
                        </Button>
                      </div>
                    ) : (
                      <Button variant="ghost" size="icon-xs" onClick={() => setConfirming(entry.id)} aria-label={`Remove ${entry.email}`}>
                        <Trash2 />
                      </Button>
                    ))}
                </li>
              );
            })}
          </ul>
        )}
        <form onSubmit={add} className="flex flex-wrap gap-2">
          <Input type="email" required placeholder="email@example.com" aria-label="Email to invite" value={email} onChange={(e) => setEmail(e.target.value)} className="min-w-56 flex-1" />
          <Select items={ROLES} value={role} onValueChange={(v) => v && setRole(v as Role)}>
            <SelectTrigger className="w-32" aria-label="Role">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {ROLES.map((r) => (
                <SelectItem key={r.value} value={r.value}>
                  {r.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Button type="submit" disabled={busy || !email.trim()}>
            {busy ? <Loader2 className="animate-spin" /> : <Plus />}
            Invite
          </Button>
        </form>
        {error && <p className="text-sm text-destructive">{error}</p>}
      </CardContent>
    </Card>
  );
}
