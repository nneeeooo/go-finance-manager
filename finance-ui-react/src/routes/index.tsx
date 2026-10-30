import FinanceChart from "@/components/FinanceChart";
import TransactionsTable from "@/components/TransactionsTable";
import { createFileRoute } from "@tanstack/react-router";
import { useEffect } from "react";
import { useDashboardStore } from "@/stores/dashboard-store";
import { useAuthStore } from "@/stores/auth-store";

export const Route = createFileRoute("/")({
  component: Home,
});

function Home() {
  const { transactions, fetchTransactions } = useDashboardStore();
  const { accessToken, initialized } = useAuthStore();

  useEffect(() => {
    if (!initialized || !accessToken) {
      return;
    }

    fetchTransactions().catch(console.error);
  }, [initialized, accessToken, fetchTransactions]);

  if (!initialized) {
    return <div>Loading...</div>;
  }

  if (!accessToken) {
    return <div>You need to log in.</div>;
  }

  return (
    <>
      <FinanceChart transactions={transactions} />

      <div className="rounded-lg border min-h-0 flex-1">
        <TransactionsTable transactions={transactions} />
      </div>
    </>
  );
}
