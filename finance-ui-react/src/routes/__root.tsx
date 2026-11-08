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

function NavLink({ to, children }: { to: string; children: ReactNode }) {
  return (
    <Link
      to={to}
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
      <div className="flex h-dvh w-dvw flex-col gap-3 p-4">
        <nav className="flex flex-wrap items-center gap-1 border-b pb-3">
          <NavLink to="/">
            <LayoutDashboard />
            Dashboard
          </NavLink>
          <NavLink to="/accounts">
            <Wallet />
            Accounts
          </NavLink>
          <NavLink to="/import">
            <FileSpreadsheet />
            Import
          </NavLink>
          <NavLink to="/loans">
            <Landmark />
            Loans
          </NavLink>
          <NavLink to="/grace">
            <CalendarClock />
            Grace period
          </NavLink>
          <span className="ml-auto flex gap-1">
            <NavLink to="/settings">
              <Settings />
              Settings
            </NavLink>
            <NavLink to="/user">
              <UserRound />
              User
            </NavLink>
          </span>
        </nav>
        <Outlet />
      </div>
    </ThemeProvider>
  );
}
