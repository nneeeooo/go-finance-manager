import { SignInCard } from "@/components/SignInCard";
import type { ReactNode } from "react";

/** Page title row: heading and one-line description on the left, the page's main action on the right */
export function PageHeader({ title, description, action }: { title: string; description?: ReactNode; action?: ReactNode }) {
  return (
    <div className="flex flex-wrap items-end justify-between gap-3">
      <div className="flex min-w-0 flex-col gap-1">
        <h1 className="text-xl font-semibold tracking-tight">{title}</h1>
        {description && <p className="text-sm text-muted-foreground">{description}</p>}
      </div>
      {action}
    </div>
  );
}

/** Shown on pages that need a login */
export function LoginRequired() {
  return <SignInCard />;
}
