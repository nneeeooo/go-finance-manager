import { useAuthStore } from "@/stores/auth-store";

// Same origin as the UI (Vite proxies /api and /auth to the API in dev, nginx in production), so the HttpOnly
// fin_session cookie travels with every request and no token is ever visible to JS.
export const API_BASE_URL: string = (import.meta.env.VITE_API_BASE_URL as string | undefined) ?? "/api/v1";

/** fetch() against the API with the session cookie. X-Requested-With is the CSRF header the API wants on unsafe methods. */
export async function apiFetch(path: string, init: RequestInit = {}): Promise<Response> {
  const response = await fetch(`${API_BASE_URL}${path}`, {
    ...init,
    credentials: "include",
    headers: { ...init.headers, "X-Requested-With": "fetch" },
  });
  // The session ended (expired, or auth revoked it): back to the signed-out view
  if (response.status === 401) useAuthStore.getState().setUser(null);
  return response;
}

/** Full-page, same-tab navigation to the SSO login; the API sends the browser back to `returnTo` afterwards. */
export function startLogin(returnTo: string = window.location.pathname + window.location.search) {
  window.location.assign(`/auth/login?return_to=${encodeURIComponent(returnTo)}`);
}

/** The API's error text ({"error": "..."}), else the HTTP status */
export async function apiError(response: Response): Promise<string> {
  const data = await response.json().catch(() => null);
  return data?.error ?? `Request failed (${response.status})`;
}
