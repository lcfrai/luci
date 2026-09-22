// desktop/src/api/index.ts
//
// The app-wide client, constructed once (contract §4.1). Components and the store import
// `api` from here and never call createClient themselves: a second client is a second base
// URL and a second timeout, and in dev one of them is always the wrong one. Tests build
// their own with createClient({ fetch, webSocket }) and never touch this singleton.

export * from "./client";

import { createClient } from "./client";

export const api = createClient();
