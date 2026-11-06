import { BANKS, type BankId } from "@/lib/banks";
import { cn } from "@/lib/utils";

/** Round bank mark: Monobank black/white, PrivatBank green, cash amber */
export function BankBadge({ bank, className }: { bank: number; className?: string }) {
  const b = BANKS[bank as BankId] ?? BANKS[4];
  return (
    <span
      className={cn(
        "inline-flex size-9 shrink-0 items-center justify-center rounded-full text-[10px] font-bold tracking-tight ring-1",
        b.className,
        className,
      )}
      title={b.name}
      aria-label={b.name}
    >
      {b.short}
    </span>
  );
}
