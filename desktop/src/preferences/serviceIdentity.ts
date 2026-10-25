// Preferences are not authority. Only the native-owned service may replace its
// ephemeral transport address with its already selected, stable state directory.
let nativeService: { apiUrl: string; stateDir: string } | undefined;

export function bindNativePreferenceIdentity(apiUrl: string, stateDir: string | null | undefined): void {
  nativeService = undefined;
  if (!stateDir) return; // An external LUCI_API override has no owned state directory.
  try {
    const url = new URL(apiUrl);
    if (url.protocol !== "http:" || url.hostname !== "127.0.0.1" || !url.port ||
        url.pathname !== "/" || url.search || url.hash || url.username || url.password) return;
    nativeService = { apiUrl: url.origin, stateDir };
  } catch { /* Keep the complete service URL when native identity is unavailable. */ }
}

export function preferenceServiceIdentity(baseUrl: string): string {
  const service = baseUrl.replace(/\/v1\/teams\/[^/]+\/workspace$/, "");
  if (nativeService && service.replace(/\/$/, "") === nativeService.apiUrl) {
    return JSON.stringify(["luci.native-state", nativeService.stateDir]);
  }
  return service;
}
