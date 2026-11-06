import { useAuthStore } from "@/stores/auth-store";

export const API_BASE_URL: string =
  (import.meta.env.VITE_API_BASE_URL as string | undefined) ??
  "http://localhost:8080/api/v1";

/** fetch() against the API with the access token; on 401 it refreshes the token once and retries. */
export async function apiFetch(path: string, init: RequestInit = {}): Promise<Response> {
  const send = (token: string | null) =>
    fetch(`${API_BASE_URL}${path}`, {
      ...init,
      headers: { ...init.headers, ...(token ? { Authorization: `Bearer ${token}` } : {}) },
    });

  let response = await send(useAuthStore.getState().accessToken);
  if (response.status === 401) {
    await useAuthStore.getState().refreshAccessToken();
    const token = useAuthStore.getState().accessToken;
    if (token) response = await send(token);
  }
  return response;
}

/** The API's error text ({"error": "..."}), else the HTTP status */
export async function apiError(response: Response): Promise<string> {
  const data = await response.json().catch(() => null);
  return data?.error ?? `Request failed (${response.status})`;
}
