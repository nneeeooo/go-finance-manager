import { createFileRoute, useNavigate } from "@tanstack/react-router";

import { MonobankImport } from "@/components/imports/MonobankImport";
import { StatementImport } from "@/components/imports/StatementImport";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

type Bank = "privatbank" | "monobank" | "vstbank";
const BANKS_ON_PAGE: Bank[] = ["privatbank", "monobank", "vstbank"];

export const Route = createFileRoute("/import")({
  validateSearch: (search: Record<string, unknown>): { bank?: Bank; account?: string } => ({
    bank: BANKS_ON_PAGE.find((b) => b === search.bank),
    account: typeof search.account === "string" ? search.account : undefined,
  }),
  component: Import,
});

/** One import page: a tab per bank */
function Import() {
  const { bank = "privatbank", account } = Route.useSearch();
  const navigate = useNavigate({ from: Route.fullPath });

  return (
    <Tabs
      value={bank}
      onValueChange={(value) => navigate({ search: { bank: value as Bank }, replace: true })}
      className="min-h-0 flex-1"
    >
      <TabsList>
        <TabsTrigger value="privatbank">PrivatBank</TabsTrigger>
        <TabsTrigger value="monobank">Monobank</TabsTrigger>
        <TabsTrigger value="vstbank">Власний Рахунок</TabsTrigger>
      </TabsList>
      <TabsContent value="privatbank" className="flex min-h-0 flex-1 flex-col">
        <StatementImport bank={2} preselected={account} />
      </TabsContent>
      <TabsContent value="monobank" className="flex min-h-0 flex-1 flex-col">
        <MonobankImport preselected={account} />
      </TabsContent>
      <TabsContent value="vstbank" className="flex min-h-0 flex-1 flex-col">
        <StatementImport bank={5} preselected={account} />
      </TabsContent>
    </Tabs>
  );
}
