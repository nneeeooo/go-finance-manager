import { Link, Outlet, createRootRoute } from "@tanstack/react-router";
import { ThemeProvider } from "../components/ThemeProvider";
import { Button } from "@/components/ui/button";
import { useAuthStore } from "@/stores/auth-store";
import { useEffect } from "react";

export const Route = createRootRoute({
  component: Root,
});

function Root() {
  const refreshAccessToken = useAuthStore((state) => state.refreshAccessToken);

  useEffect(() => {
    refreshAccessToken();
  }, [refreshAccessToken]);

  return (
    <ThemeProvider defaultTheme="dark" storageKey="vite-ui-theme">
      <div className="w-dvw h-dvh p-4 flex flex-col gap-2">
        <div className="flex gap-2">
          <Button>
            <Link to="/">Home</Link>
          </Button>

          <Button>
            <Link to="/accounts">Accounts</Link>
          </Button>

          <Button>
            <Link to="/user">User</Link>
          </Button>
        </div>
        <Outlet />
      </div>
    </ThemeProvider>
  );
}
