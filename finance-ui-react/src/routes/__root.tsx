import { Link, Outlet, createRootRoute } from "@tanstack/react-router";
import type { ReactNode } from "react";
import { CalendarClock, FileSpreadsheet, Landmark, LayoutDashboard, Settings, UserRound, Wallet } from "lucide-react";
import { ThemeProvider } from "../components/ThemeProvider";
import { buttonVariants } from "@/components/ui/button";
import { useAuthStore } from "@/stores/auth-store";
import { useEffect } from "react";

export const Route = createRootRoute({
  component: Root,
});

function NavLink({ to, label, children }: { to: string; label: string; children: ReactNode }) {
  return (
    <Link
      to={to}
      aria-label={label}
      activeOptions={{ exact: to === "/" }}
      className={buttonVariants({ variant: "ghost", size: "sm" })}
      activeProps={{ className: buttonVariants({ variant: "secondary", size: "sm" }) }}
    >
      {children}
    </Link>
  );
}

function Root() {
  const loadUser = useAuthStore((state) => state.loadUser);

  useEffect(() => {
    loadUser();
  }, [loadUser]);

  return (
    <ThemeProvider defaultTheme="dark" storageKey="vite-ui-theme">
      <div className="flex h-dvh w-dvw flex-col gap-3 p-2 sm:p-4">
        <nav className="flex flex-wrap items-center gap-1 border-b pb-3">
          <NavLink to="/" label="Dashboard">
            <LayoutDashboard />
            <span className="hidden sm:inline">Dashboard</span>
          </NavLink>
          <NavLink to="/accounts" label="Accounts">
            <Wallet />
            <span className="hidden sm:inline">Accounts</span>
          </NavLink>
          <NavLink to="/import" label="Import">
            <FileSpreadsheet />
            <span className="hidden sm:inline">Import</span>
          </NavLink>
          <NavLink to="/loans" label="Loans">
            <Landmark />
            <span className="hidden sm:inline">Loans</span>
          </NavLink>
          <NavLink to="/grace" label="Grace period">
            <CalendarClock />
            <span className="hidden sm:inline">Grace period</span>
          </NavLink>
          <span className="ml-auto flex gap-1">
            <NavLink to="/settings" label="Settings">
              <Settings />
              <span className="hidden sm:inline">Settings</span>
            </NavLink>
            <NavLink to="/user" label="User">
              <UserRound />
              <span className="hidden sm:inline">User</span>
            </NavLink>
          </span>
        </nav>
        <Outlet />
      </div>
    </ThemeProvider>
  );
}
