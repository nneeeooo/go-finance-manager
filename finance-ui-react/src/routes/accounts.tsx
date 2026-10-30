import { API_BASE_URL } from "@/lib/api";
import { createFileRoute } from "@tanstack/react-router";
import { useEffect, useState } from "react";

import { useAuthStore } from "@/stores/auth-store";

export const Route = createFileRoute("/accounts")({
  component: RouteComponent,
});

type Account = {
  id: string;
  bank: string;
  name: string;
  currency: string;
};

function RouteComponent() {
  const accessToken = useAuthStore((state) => state.accessToken);
  const initialized = useAuthStore((state) => state.initialized);

  const [accounts, setAccounts] = useState<Account[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!initialized || !accessToken) {
      return;
    }

    const fetchAccounts = async () => {
      setLoading(true);
      setError(null);

      try {
        const response = await fetch(`${API_BASE_URL}/accounts`, {
          headers: {
            Authorization: `Bearer ${accessToken}`,
          },
        });

        if (!response.ok) {
          throw new Error("Failed to fetch accounts");
        }

        const data: Account[] = await response.json();

        setAccounts(data);
      } catch (error) {
        setError(
          error instanceof Error ? error.message : "Something went wrong",
        );
      } finally {
        setLoading(false);
      }
    };

    fetchAccounts().catch(console.error);
  }, [initialized, accessToken]);

  if (loading) {
    return <div>Loading accounts...</div>;
  }

  if (!accessToken) {
    return <div>You need to log in.</div>;
  }

  if (error) {
    return <div>{error}</div>;
  }

  return (
    <div>
      <h1>Accounts</h1>

      {accounts.length === 0 ? (
        <p>No accounts found.</p>
      ) : (
        <ul>
          {accounts.map((account) => (
            <li key={account.id}>
              {account.name} — {account.bank} — {account.currency}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
