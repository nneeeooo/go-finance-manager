import { useMemo } from "react";
import { Cell, Pie, PieChart } from "recharts";
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from "@/components/ui/chart";
import {
  formatCurrency,
  type CategoryPoint,
} from "@/lib/chart-utils";

type CategoryChartProps = {
  items: CategoryPoint[];
  totalExpense: number;
};

export function CategoryChart({ items, totalExpense }: CategoryChartProps) {
  const chartConfig = useMemo(() => {
    const config: ChartConfig = {
      expense: {
        label: "Expense",
      },
    };
    items.forEach((item, index) => {
      config[item.id] = {
        label: item.name,
        color: `var(--chart-${(index % 5) + 1})`,
      };
    });
    return config;
  }, [items]);

  if (items.length === 0 || totalExpense === 0) {
    return (
      <div className="flex h-72 w-full flex-col items-center justify-center gap-1 text-muted-foreground">
        <p className="text-sm font-medium">No expense records found</p>
        <p className="text-xs">Try selecting another bank or period</p>
      </div>
    );
  }

  return (
    <div className="grid grid-cols-1 items-center gap-6 lg:grid-cols-[1fr_320px]">
      {/* Donut Chart */}
      <div className="relative flex items-center justify-center">
        <ChartContainer
          config={chartConfig}
          className="mx-auto aspect-square h-64 w-full max-w-[280px]"
        >
          <PieChart>
            <ChartTooltip
              cursor={false}
              content={
                <ChartTooltipContent
                  hideLabel
                  formatter={(_, __, entry) => {
                    const payload = entry.payload as CategoryPoint;
                    return (
                      <div className="flex flex-col gap-1">
                        <span className="font-medium">{payload.name}</span>
                        <div className="flex items-center justify-between gap-4 font-mono text-xs">
                          <span>{formatCurrency(payload.amount)} ₴</span>
                          <span className="text-muted-foreground">
                            {payload.percentage.toFixed(1)}%
                          </span>
                        </div>
                      </div>
                    );
                  }}
                />
              }
            />
            <Pie
              data={items}
              dataKey="amount"
              nameKey="name"
              innerRadius={70}
              outerRadius={100}
              paddingAngle={2}
              stroke="transparent"
            >
              {items.map((entry) => (
                <Cell key={entry.id} fill={entry.fill} />
              ))}
            </Pie>
          </PieChart>
        </ChartContainer>

        {/* Center label */}
        <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center text-center">
          <span className="text-xl font-bold tracking-tight">
            {formatCurrency(totalExpense)} ₴
          </span>
          <span className="text-xs text-muted-foreground">Total expenses</span>
        </div>
      </div>

      {/* Top Categories Breakdown List */}
      <div className="flex max-h-64 flex-col gap-2 overflow-y-auto pr-1">
        <div className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
          Top Categories ({items.length})
        </div>
        <div className="flex flex-col gap-1.5 divide-y divide-border/40">
          {items.map((item) => (
            <div
              key={item.id}
              className="flex items-center justify-between gap-2 pt-1.5 text-xs"
            >
              <div className="flex min-w-0 items-center gap-2">
                <span
                  className="size-2.5 shrink-0 rounded-full"
                  style={{ backgroundColor: item.fill }}
                />
                <span className="truncate font-medium text-foreground" title={item.name}>
                  {item.name}
                </span>
                <span className="text-[10px] text-muted-foreground">
                  ({item.count})
                </span>
              </div>
              <div className="flex shrink-0 items-center gap-2 font-mono">
                <span>{formatCurrency(item.amount)} ₴</span>
                <span className="inline-block w-12 text-right text-[11px] text-muted-foreground">
                  {item.percentage.toFixed(1)}%
                </span>
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

export default CategoryChart;
