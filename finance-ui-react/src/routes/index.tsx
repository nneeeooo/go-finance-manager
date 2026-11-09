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
    <div className="flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto">
      <FinanceChart transactions={transactions} />

      <div className="min-h-96 flex-1 overflow-x-auto rounded-lg border">
        <div className="h-full min-w-[46rem]">
          <TransactionsTable transactions={transactions} />
        </div>
      </div>
    </div>
  );
}
