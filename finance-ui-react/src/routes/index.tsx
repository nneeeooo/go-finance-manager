import FinanceChart from "@/components/FinanceChart";
import TransactionsTable from "@/components/TransactionsTable";
import { createFileRoute } from "@tanstack/react-router";
import { useEffect } from "react";
import { useDashboardStore } from "@/stores/dashboard-store";
import { LoginRequired } from "@/components/PageHeader";
import { useAuthStore } from "@/stores/auth-store";

export const Route = createFileRoute("/")({
  component: Home,
});

function Home() {
  const { transactions, fetchTransactions } = useDashboardStore();
  const { user, initialized } = useAuthStore();

  useEffect(() => {
    if (!initialized || !user) {
      return;
    }

    fetchTransactions().catch(console.error);
  }, [initialized, user, fetchTransactions]);

  if (!initialized) {
    return <div>Loading...</div>;
  }

  if (!user) {
    return <LoginRequired />;
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
