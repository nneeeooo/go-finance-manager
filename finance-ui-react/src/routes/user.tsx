import { createFileRoute } from "@tanstack/react-router";

import { SignInCard } from "@/components/SignInCard";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { useAuthStore } from "@/stores/auth-store";
import { useDashboardStore } from "@/stores/dashboard-store";

export const Route = createFileRoute("/user")({
  component: RouteComponent,
});

function RouteComponent() {
  const user = useAuthStore((state) => state.user);
  const initialized = useAuthStore((state) => state.initialized);
  const logout = useAuthStore((state) => state.logout);
  const resetTransactions = useDashboardStore((state) => state.resetTransactions);

  if (!initialized) return <></>;
  if (!user) return <SignInCard />;

  return (
    <div className="flex items-center justify-center p-4">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle>You're signed in</CardTitle>
          <CardDescription>{user.email}</CardDescription>
        </CardHeader>

        <CardContent>
          <Button
            className="w-full"
            variant="outline"
            onClick={() => {
              resetTransactions();
              void logout();
            }}
          >
            Log out
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}
