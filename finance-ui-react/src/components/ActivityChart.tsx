import { useMemo } from "react";
import {
  Bar,
  BarChart,
  CartesianGrid,
  Rectangle,
  XAxis,
  YAxis,
} from "recharts";
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from "@/components/ui/chart";
import {
  formatCurrency,
  type ActivityPoint,
} from "@/lib/chart-utils";

type ActivityChartProps = {
  data: ActivityPoint[];
};

const chartConfig = {
  total: {
    label: "Net total",
    color: "var(--chart-2)",
  },
  income: {
    label: "Income",
    color: "var(--chart-1)",
  },
  expense: {
    label: "Expense",
    color: "var(--destructive)",
  },
} satisfies ChartConfig;

export function ActivityChart({ data }: ActivityChartProps) {
  const chartMax = useMemo(() => {
    if (data.length === 0) return 100;
    const maxVal = Math.max(...data.map((entry) => Math.abs(entry.total)), 1);
    return Math.max(Math.ceil(maxVal / 100) * 100, 100);
  }, [data]);

  if (data.length === 0) {
    return (
      <div className="flex h-64 w-full items-center justify-center text-sm text-muted-foreground">
        No transaction activity for selected period
      </div>
    );
  }

  return (
    <ChartContainer config={chartConfig} className="h-64 w-full">
      <BarChart accessibilityLayer data={data}>
        <CartesianGrid vertical={false} strokeDasharray="3 3" />
        <XAxis
          dataKey="displayLabel"
          tickLine={false}
          axisLine={false}
          tickMargin={8}
        />
        <YAxis
          type="number"
          domain={[-chartMax, chartMax]}
          tickLine={false}
          axisLine={false}
          tickFormatter={(val) => `${val.toLocaleString()} ₴`}
        />
        <ChartTooltip
          cursor={false}
          content={
            <ChartTooltipContent
              formatter={(value, name) => (
                <div className="flex items-center justify-between gap-2">
                  <span className="text-muted-foreground">
                    {name === "total" ? "Net flow" : String(name)}:
                  </span>
                  <span className="font-mono font-medium">
                    {formatCurrency(Number(value))} ₴
                  </span>
                </div>
              )}
            />
          }
        />
        <Bar
          dataKey="total"
          radius={4}
          shape={(props) => {
            const isNegative = Number(props.value) < 0;
            return (
              <Rectangle
                {...props}
                fill={isNegative ? "var(--destructive)" : "var(--chart-2)"}
                radius={4}
              />
            );
          }}
        />
      </BarChart>
    </ChartContainer>
  );
}

export default ActivityChart;
