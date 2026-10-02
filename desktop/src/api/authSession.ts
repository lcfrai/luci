const TOKEN_KEY = "luci.control-plane.session";
export const AUTH_EXPIRED = "luci-auth-expired";
export function authToken(): string | null {
  try {
    return window.sessionStorage.getItem(TOKEN_KEY);
  } catch {
    return null;
  }
}
export function setAuthToken(token: string | null): void {
  if (token) window.sessionStorage.setItem(TOKEN_KEY, token);
  else window.sessionStorage.removeItem(TOKEN_KEY);
}
export function authHeaders(
  headers?: Record<string, string>,
): Record<string, string> | undefined {
  const token = authToken();
  return token ? { ...headers, Authorization: `Bearer ${token}` } : headers;
}
export function expireAuth(): void {
  if (!authToken()) return;
  setAuthToken(null);
  window.dispatchEvent(new Event(AUTH_EXPIRED));
}
