import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { startLogin } from "@/lib/api";

const ERRORS: Record<string, string> = {
  not_invited: "This Google account is not invited to Finance Manager.",
  access_denied:
    "Sign-in was cancelled or denied. A Telegram account works only after the admin links it to your invite.",
  login_failed: "Sign-in failed. Try again.",
};

/** The way in: a full-page, same-tab trip through the SSO login (Google or Telegram), then back to this page */
export function SignInCard() {
  const code = new URLSearchParams(window.location.search).get("auth_error");

  return (
    <div className="flex items-center justify-center p-4">
      <Card className="w-full max-w-sm">
        <CardHeader className="space-y-1">
          <CardTitle className="text-xl text-center">Finance Manager</CardTitle>
          <CardDescription className="text-center">Sign in to continue</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          {code && <p className="text-sm text-destructive">{ERRORS[code] ?? ERRORS.login_failed}</p>}
          <Button className="w-full" onClick={() => startLogin(window.location.pathname, "google")}>
            Sign in with Google
          </Button>
          <Button variant="outline" className="w-full" onClick={() => startLogin(window.location.pathname, "telegram")}>
            Sign in with Telegram
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}
