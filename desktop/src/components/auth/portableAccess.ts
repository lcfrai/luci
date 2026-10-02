/** The portable launcher's owner capability is confined to its same-origin local API. */
export function portableOriginAllowed(
  href: string,
  native: boolean,
  apiBase?: string,
): boolean {
  if (native) return false;
  try {
    const page = new URL(href);
    if (
      !["http:", "https:"].includes(page.protocol) ||
      !["127.0.0.1", "localhost", "[::1]"].includes(page.hostname)
    )
      return false;
    return !apiBase || new URL(apiBase).origin === page.origin;
  } catch {
    return false;
  }
}
export function portableAccessAvailable(): boolean {
  return portableOriginAllowed(
    window.location.href,
    "__TAURI_INTERNALS__" in window,
    window.__LUCI_API__ || import.meta.env.VITE_LUCI_API,
  );
}
export class PortableAccessError extends Error {
  constructor(
    readonly status: number,
    message: string,
  ) {
    super(message);
  }
}
type LocalPath =
  | "/v1/auth/local-owner"
  | "/v1/auth/local-account"
  | "/v1/auth/local-owner/recover";
export async function portableRequest<T>(
  proof: string,
  path: LocalPath,
  body?: unknown,
): Promise<T> {
  if (!portableAccessAvailable() || !/^[A-Za-z0-9_-]{43}$/.test(proof))
    throw new PortableAccessError(
      403,
      "Open Luci with its portable launcher on this computer to manage a local account.",
    );
  try {
    const response = await fetch(`${window.location.origin}${path}`, {
      method: body === undefined ? "GET" : "POST",
      headers: {
        "X-Luci-Local-Access-Token": proof,
        ...(body === undefined ? {} : { "Content-Type": "application/json" }),
      },
      body: body === undefined ? undefined : JSON.stringify(body),
      signal: AbortSignal.timeout(15_000),
      redirect: "error",
      cache: "no-store",
      credentials: "omit",
    });
    const value = await response.json();
    if (!response.ok)
      throw new PortableAccessError(
        response.status,
        typeof value?.message === "string"
          ? value.message
          : "Local account access was refused. Open Luci with its portable launcher again.",
      );
    return value as T;
  } catch (cause) {
    if (cause instanceof PortableAccessError) throw cause;
    throw new PortableAccessError(
      0,
      "Luci’s local service is unavailable. Check the portable launcher and retry the connection.",
    );
  }
}
