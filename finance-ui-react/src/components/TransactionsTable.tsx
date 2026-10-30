import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import currencyCodes from "currency-codes";
import mcc from "mcc";
import { Badge } from "./ui/badge";
import { useDashboardStore } from "@/stores/dashboard-store";
import { useMemo, useRef } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";

import { filterTransactionsBySelection } from "@/lib/chart-utils";

type Transaction = {
  id: string;
  account_id: string;
  bank_id: number;
  external_id?: string;
  amount: number;
  currency: number;
  mcc: number;
  category?: string;
  description: string;
  occurred_at: string;
};

type TransactionsTableProps = {
  transactions: Transaction[];
};

function TransactionsTable({ transactions }: TransactionsTableProps) {
  const bank = useDashboardStore((state) => state.bank);
  const selectedAccountId = useDashboardStore((state) => state.selectedAccountId);

  const parentRef = useRef<HTMLDivElement>(null);

  const filteredTransactions = useMemo(
    () => filterTransactionsBySelection(transactions, bank, selectedAccountId),
    [transactions, bank, selectedAccountId],
  );

  const rowVirtualizer = useVirtualizer({
    count: filteredTransactions.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 40,
    overscan: 10,
  });

  function formatDate(dateString: string) {
    return new Date(dateString).toLocaleDateString("uk-UA");
  }

  function formatTime(dateString: string) {
    return new Date(dateString).toLocaleTimeString("uk-UA", {
      hour12: false,
    });
  }

  return (
    <div className="h-full flex flex-col">
      <Table className="w-full table-fixed">
        <TableHeader className="block bg-background border-b rounded-t-lg">
          <TableRow className="rounded-t-lg grid grid-cols-[minmax(0,3fr)_minmax(0,4fr)_6rem_5rem_5rem_4rem_5rem_5rem] *:flex *:items-center">
            <TableHead>Category</TableHead>
            <TableHead>Description</TableHead>
            <TableHead>Bank</TableHead>
            <TableHead>Date</TableHead>
            <TableHead>Time</TableHead>
            <TableHead>MCC</TableHead>
            <TableHead className="text-right">Amount</TableHead>
            <TableHead>Currency</TableHead>
          </TableRow>
        </TableHeader>
      </Table>

      <div ref={parentRef} className="min-h-0 flex-1 overflow-auto">
        <Table className="w-full table-fixed">
          <TableBody
            className="relative block"
            style={{
              height: `${rowVirtualizer.getTotalSize()}px`,
            }}
          >
            {rowVirtualizer.getVirtualItems().map((virtualRow) => {
              const transaction = filteredTransactions[virtualRow.index];

              return (
                <TableRow
                  className="grid grid-cols-[minmax(0,3fr)_minmax(0,4fr)_6rem_5rem_5rem_4rem_5rem_5rem]"
                  key={transaction.id}
                  ref={rowVirtualizer.measureElement}
                  data-index={virtualRow.index}
                  style={{
                    position: "absolute",
                    top: 0,
                    left: 0,
                    width: "100%",
                    transform: `translateY(${virtualRow.start}px)`,
                  }}
                >
                  <TableCell>
                    <div className="truncate">
                      {transaction.category ||
                        mcc.get(transaction.mcc)?.irs_description ||
                        "Unknown"}
                    </div>
                  </TableCell>

                  <TableCell>
                    <div className="truncate">{transaction.description}</div>
                  </TableCell>

                  <TableCell>
                    <Badge
                      variant="secondary"
                      className={
                        transaction.bank_id === 2 ? "bg-green-700" : ""
                      }
                    >
                      {transaction.bank_id === 2 ? "privatbank" : "monobank"}
                    </Badge>
                  </TableCell>

                  <TableCell className="whitespace-nowrap text-right">
                    {formatDate(transaction.occurred_at)}
                  </TableCell>

                  <TableCell className="whitespace-nowrap text-right">
                    {formatTime(transaction.occurred_at)}
                  </TableCell>

                  <TableCell className="text-center">
                    {transaction.mcc}
                  </TableCell>

                  <TableCell className="whitespace-nowrap text-right">
                    {(transaction.amount / 100).toFixed(2)}
                  </TableCell>

                  <TableCell className="text-center">
                    {currencyCodes.number(String(transaction.currency))?.code ??
                      transaction.currency}
                  </TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      </div>
    </div>
  );
}

export default TransactionsTable;
