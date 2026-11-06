import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { formatMoney } from "@/lib/banks";
import type { Account } from "@/types";

/** Choose one of the given accounts; shows the name and balance */
export function AccountPicker({
  accounts,
  value,
  onChange,
  placeholder = "Choose an account",
}: {
  accounts: Account[];
  value: string;
  onChange: (id: string) => void;
  placeholder?: string;
}) {
  const items = accounts.map((a) => ({ value: a.id, label: `${a.name} · ${formatMoney(a.balance, a.currency)}` }));
  return (
    <Select items={items} value={value || null} onValueChange={(v) => v && onChange(v as string)} disabled={accounts.length === 0}>
      <SelectTrigger className="w-full">
        <SelectValue placeholder={accounts.length ? placeholder : "No accounts of this bank yet"} />
      </SelectTrigger>
      <SelectContent>
        {items.map((i) => (
          <SelectItem key={i.value} value={i.value}>
            {i.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
