// desktop/src/api/index.ts
//
// The app-wide client, constructed once (contract §4.1). Components and the store import
// `api` from here and never call createClient themselves: a second client is a second base
// URL and a second timeout, and in dev one of them is always the wrong one. Tests build
// their own with createClient({ fetch, webSocket }) and never touch this singleton.

export * from "./client";

import { createClient } from "./client";

/** Constructed at module load, so `resolveBaseUrl` runs once and the base URL is fixed for the
 *  life of the process. That is correct for the design — one client, one base URL — and the
 *  value cannot change under a running app anyway, since it comes from a build-time env var.
 *  If task 24 ever needs it changeable at runtime, this is the line to revisit: it becomes a
 *  mutable module binding or a setter, and every consumer of `api` keeps its import unchanged. */
export const api = createClient();
