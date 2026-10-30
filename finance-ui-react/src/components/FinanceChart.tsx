import { useMemo, useState } from "react";
import { useDashboardStore, type BankFilter } from "@/stores/dashboard-store";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectLabel,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import FinanceSummary from "./FinanceSummary";
import ActivityChart from "./ActivityChart";
import CategoryChart from "./CategoryChart";
import type { Transaction, ChartMode } from "@/types";
import {
  BANK_ITEMS,
  MONTH_ITEMS,
  PERIOD_MODE_ITEMS,
  calculateActivityChartData,
  calculateCategoryBreakdown,
  filterTransactionsBySelection,
  filterTransactionsByPeriod,
  getAvailableYears,
  getPeriodDescription,
  type PeriodFilter,
} from "@/lib/chart-utils";

type FinanceChartProps = {
  transactions: Transaction[];
};

export function FinanceChart({ transactions }: FinanceChartProps) {
  const bank = useDashboardStore((state) => state.bank);
  const setBank = useDashboardStore((state) => state.setBank);
  const selectedAccountId = useDashboardStore((state) => state.selectedAccountId);
  const setSelectedAccountId = useDashboardStore((state) => state.setSelectedAccountId);
  const accounts = useDashboardStore((state) => state.accounts);

  const [activeTab, setActiveTab] = useState<string>("activity");
  const [mode, setMode] = useState<ChartMode>("all");
  const [selectedYear, setSelectedYear] = useState<number>(() => new Date().getFullYear());
  const [selectedMonth, setSelectedMonth] = useState<number>(() => new Date().getMonth());

  const years = useMemo(() => getAvailableYears(transactions), [transactions]);

  // If selected year is not in transactions, update it to latest available
  const effectiveYear = useMemo(() => {
    return years.includes(selectedYear) ? selectedYear : (years[0] ?? new Date().getFullYear());
  }, [years, selectedYear]);

  const periodFilter: PeriodFilter = useMemo(
    () => ({
      mode,
      year: effectiveYear,
      month: selectedMonth,
    }),
    [mode, effectiveYear, selectedMonth],
  );

  const accountItems = useMemo(() => {
    const base = [{ value: "all", label: "All Accounts" }];
    const filteredAccounts =
      bank === "all"
        ? accounts
        : accounts.filter((a) => (bank === "monobank" ? a.bank === 1 : a.bank === 2));

    return [
      ...base,
      ...filteredAccounts.map((a) => ({
        value: a.id,
        label: a.name,
      })),
    ];
  }, [accounts, bank]);

  const selectedFilteredTransactions = useMemo(
    () => filterTransactionsBySelection(transactions, bank, selectedAccountId),
    [transactions, bank, selectedAccountId],
  );

  const activityData = useMemo(
    () => calculateActivityChartData(selectedFilteredTransactions, periodFilter),
    [selectedFilteredTransactions, periodFilter],
  );

  const categoryData = useMemo(() => {
    const periodFiltered = filterTransactionsByPeriod(
      selectedFilteredTransactions,
      periodFilter,
    );
    return calculateCategoryBreakdown(periodFiltered);
  }, [selectedFilteredTransactions, periodFilter]);

  const periodDescription = getPeriodDescription(periodFilter);

  return (
    <div className="rounded-lg border bg-card text-card-foreground shadow-xs">
      <FinanceSummary transactions={transactions} />

      <div className="p-4 sm:p-6">
        <Tabs
          value={activeTab}
          onValueChange={(val) => {
            if (val) setActiveTab(val as string);
          }}
          className="flex flex-col gap-4"
        >
          {/* Header controls */}
          <div className="flex flex-col justify-between gap-4 lg:flex-row lg:items-center">
            <div className="flex flex-col gap-1">
              <div className="flex items-center gap-3">
                <TabsList>
                  <TabsTrigger value="activity">Cash Flow</TabsTrigger>
                  <TabsTrigger value="categories">Categories</TabsTrigger>
                </TabsList>
              </div>
              <p className="text-xs text-muted-foreground">{periodDescription}</p>
            </div>

            {/* Filters bar */}
            <div className="flex flex-wrap items-center gap-2">
              {/* Bank filter */}
              <Select
                items={BANK_ITEMS}
                value={bank}
                onValueChange={(value) => {
                  if (value) setBank(value as BankFilter);
                }}
              >
                <SelectTrigger className="h-9 min-w-32">
                  <SelectValue placeholder="Bank" />
                </SelectTrigger>
                <SelectContent>
                  <SelectGroup>
                    <SelectLabel>Bank</SelectLabel>
                    {BANK_ITEMS.map((item) => (
                      <SelectItem key={item.value} value={item.value}>
                        {item.label}
                      </SelectItem>
                    ))}
                  </SelectGroup>
                </SelectContent>
              </Select>

              {/* Account filter */}
              <Select
                items={accountItems}
                value={selectedAccountId}
                onValueChange={(value) => {
                  if (value) setSelectedAccountId(value as string);
                }}
              >
                <SelectTrigger className="h-9 min-w-44">
                  <SelectValue placeholder="Account" />
                </SelectTrigger>
                <SelectContent>
                  <SelectGroup>
                    <SelectLabel>Account</SelectLabel>
                    {accountItems.map((item) => (
                      <SelectItem key={item.value} value={item.value}>
                        {item.label}
                      </SelectItem>
                    ))}
                  </SelectGroup>
                </SelectContent>
              </Select>

              {/* Mode filter */}
              <Select
                items={PERIOD_MODE_ITEMS}
                value={mode}
                onValueChange={(value) => {
                  if (value) setMode(value as ChartMode);
                }}
              >
                <SelectTrigger className="h-9 min-w-28">
                  <SelectValue placeholder="Period" />
                </SelectTrigger>
                <SelectContent>
                  <SelectGroup>
                    <SelectLabel>Period</SelectLabel>
                    {PERIOD_MODE_ITEMS.map((item) => (
                      <SelectItem key={item.value} value={item.value}>
                        {item.label}
                      </SelectItem>
                    ))}
                  </SelectGroup>
                </SelectContent>
              </Select>

              {/* Year filter (Quarters, Months, Days) */}
              {mode !== "all" && (
                <Select
                  items={years.map((y) => ({ label: String(y), value: String(y) }))}
                  value={String(effectiveYear)}
                  onValueChange={(value) => {
                    if (value) setSelectedYear(Number(value));
                  }}
                >
                  <SelectTrigger className="h-9 min-w-24">
                    <SelectValue placeholder="Year" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      <SelectLabel>Year</SelectLabel>
                      {years.map((year) => (
                        <SelectItem key={year} value={String(year)}>
                          {year}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              )}

              {/* Month filter (Days mode) */}
              {mode === "days" && (
                <Select
                  items={MONTH_ITEMS}
                  value={String(selectedMonth)}
                  onValueChange={(value) => {
                    if (value) setSelectedMonth(Number(value));
                  }}
                >
                  <SelectTrigger className="h-9 min-w-32">
                    <SelectValue placeholder="Month" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      <SelectLabel>Month</SelectLabel>
                      {MONTH_ITEMS.map((month) => (
                        <SelectItem key={month.value} value={month.value}>
                          {month.label}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              )}
            </div>
          </div>

          {/* Tab contents */}
          <TabsContent value="activity" className="mt-2 outline-none">
            <ActivityChart data={activityData} />
          </TabsContent>

          <TabsContent value="categories" className="mt-2 outline-none">
            <CategoryChart
              items={categoryData.items}
              totalExpense={categoryData.totalExpense}
            />
          </TabsContent>
        </Tabs>
      </div>
    </div>
  );
}

export default FinanceChart;
