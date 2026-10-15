import { ApiError } from "./client";
import { errorText } from "./product";

export type CodexConnectionAction = "save" | "start" | "status" | "other";

// A failed transport gives no evidence about the device grant or mutation result.
export function codexConnectionError(cause: unknown, action: CodexConnectionAction): string {
  if (!(cause instanceof ApiError) || (cause.code !== "network_unavailable" && cause.code !== "timeout")) return errorText(cause);
  const service = cause.code === "timeout"
    ? "Luci’s workspace service did not respond in time."
    : "Luci’s workspace service could not be reached.";
  if (action === "start") return `${service} The sign-in request is unconfirmed. Refresh your sessions before requesting another device code.`;
  if (action === "save") return `${service} Saving this sign-in is unconfirmed. Keep the session and refresh Codex connection before trying Save again.`;
  return `${service} Codex connection status is unavailable. Reconnect and refresh the connection; this does not show whether device sign-in succeeded.`;
}
