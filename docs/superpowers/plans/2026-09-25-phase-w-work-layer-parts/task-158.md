### Task 158: the public API: published docs; TypeScript and Go clients generated from the contract

W5, stream WH. Depends on task 143 (SW.21, the W5 wire types the document describes) and task 156
(SW.22, the webhook payload and signature the README documents). Documents the rate-limit headers
of task 160 and the app and key credentials of tasks 159 and 160, which land after this task: the
README's sections for them are generated from their types when those tasks merge, and the drift
test (below) fails until they are regenerated. Hub anchors read in `feat/hub` at `3df9fab8`:
`server.Patterns()` (H-plan `docs/superpowers/plans/2026-09-23-phase-h-hub.md:212`), the contract
(`hub-contract:N`). `server/` is **planned, not on `feat/hub`**, and **UNVERIFIED until merge**.

The Hub's API already is the public one (05 §1 L-248: *"the same API is the public one"*). What is
missing is a document a stranger can read and clients they can use. This task generates both from
the code, so they cannot drift: the routes from the real route table, the shapes from the `hubapi`
types, the roles from the authorisation matrix, and a test that fails when the checked-in document
differs from what the code generates.

**The one thing this task must not get wrong** is publishing a route the code does not have, or
missing one it does. The route list is never typed by hand: it is `server.Patterns()`, filtered to
`/v1`, and a test fails on any pattern the document lacks.

Five decisions the scope left open, made here.

1. **OpenAPI 3.0.3, not 3.1.** The scope says 3.1. `oapi-codegen` reads documents through
   `kin-openapi`, whose 3.1 support was incomplete when last checked (**UNVERIFIED** at this
   writing; the corporate network blocks the project's pages). `openapi-typescript` reads both.
   3.0.3 is the version both generators accept; nullability is `nullable: true`. When `oapi-codegen`
   accepts 3.1, moving is one constant and a regenerate.
2. **The route-to-type table is code, checked both ways.** A pattern does not say its body's type.
   `cmd/hubopenapi/routes.go` holds one entry per `/v1` pattern (request type, response type, success
   status, the route's own error codes); `TestEveryV1PatternIsDocumented` fails on a pattern with no
   entry and on an entry with no pattern. Adding a route without its entry fails CI.
3. **Roles come from the matrix, exported by a test.** Task 07 put the Work rows in
   `server/work_matrix_rows_integration_test.go` (its decision 7), a test file the generator cannot
   import. So `TestExportRouteRoles` (package `server_test`, `-tags integration`, no database
   needed) writes `docs/api/roles.json` from the rows under `-update` and compares it otherwise; the
   generator reads that file. The scope's `server/work_matrix_rows.go` is task 07's correction.
4. **The Go SDK is types generated, client written.** `oapi-codegen -generate types` emits the
   models; the client is ours, on `net/http`, like the TypeScript one on `fetch`. The module then
   depends on the standard library only. Formats that pull `oapi-codegen/runtime` (`date`, `uuid`)
   are never emitted: every id and time is a plain `string`, as on the wire.
5. **Golden checks compare canonical JSON, not bytes.** `oapi-codegen` orders struct fields by
   name, not by the schema's order, so a re-encode cannot match a golden byte for byte. The test
   decodes each golden into the generated type, re-encodes it, and compares both sides after
   canonicalising (keys sorted, no whitespace). A dropped, renamed or retyped field still fails.

**What is published:** every `/v1` route. **Not** `/p/…` (task 80's pages, for browsers), `/hooks/…`
(task 154's host ingress), `/teams/messages` (task 157's bot) or `/healthz` and `/readyz`. Auth, as
the README states it: a realm access token for `panacea-app-hub` (hub-contract §17, which replaces
`:36`'s Entra wording), an approved app's token acting as its person (task 159), or an API key
(task 160). A request with an `Origin` header is refused (hub-contract §5.1 step 1): the API is for
programs, not for browser pages.

**Files:**

- `control-plane/cmd/hubopenapi/main.go`, `routes.go`, `schema.go`, `readme.go`, `main_test.go` (new)
- `control-plane/internal/hub/server/openapi_roles_integration_test.go` (new): `TestExportRouteRoles`
- `docs/api/openapi.json`, `docs/api/README.md`, `docs/api/roles.json` (new, generated, checked in)
- `sdk/ts/package.json`, `sdk/ts/tsconfig.json`, `sdk/ts/src/index.ts`, `sdk/ts/src/index.test.ts`,
  `sdk/ts/src/schema.ts` (generated) (new)
- `sdk/go/go.mod`, `sdk/go/client.go`, `sdk/go/types.gen.go` (generated), `sdk/go/oapi.yaml`
  (the generator's config), `sdk/go/golden_test.go` (new)
- `go.work` ⚑: `./sdk/go` added to `use`; the `go` directive untouched
- **Not in the File map:** `cmd/hubopenapi/` and `docs/api/` (gaps to add); `sdk/{ts,go}/` is there

**Interfaces:**

- Produces: `docs/api/openapi.json` (the published contract), `docs/api/README.md`, the TypeScript
  package `@panacea-app/sdk` (not published to a registry) and the Go module
  `github.com/kpmg-au-emu/panacea-app/sdk/go`
- Consumes: `server.Patterns()` (H-plan `:212`); every `hubapi` type named in `routes.go`; task 07's
  `workMatrixRows` and H16b's `allRows` (through `roles.json`); SW.22 (task 156); task 160's
  `RateLimit-*` headers
- Consumed by: task 178 (warehouse and spreadsheet sync through the public API), task 182, and any
  outside developer. The desktop keeps `desktop/src/api/work.ts` (task 37) and does **not** switch
  to the SDK in W5

#### `cmd/hubopenapi/routes.go` (complete as a type; the entries are one per pattern)

```go
package main

// Route documents one /v1 pattern. Req and Resp are zero values of hubapi types
// (nil for no body); Codes are the route's own error codes beyond the common ones
// (400 bad_request, 401 unauthenticated, 403 forbidden, 404 by resource, 429
// rate_limited, 503 unavailable), which every route carries.
type Route struct {
	Pattern string // exactly as registered: "POST /v1/teams/{teamId}/webhooks"
	Summary string // one line, present tense, no marketing
	Req     any
	Resp    any
	OK      int      // 200 | 201 | 202 | 204
	Codes   []string // hubapi.Code* values, e.g. hubapi.CodeWebhookURLInvalid
	Query   []Param  // query parameters, e.g. after, limit, state
	Tag     string   // "briefs" | "work" | "intake" | "webhooks" | "apps and keys" | …
}

type Param struct {
	Name, Type, Doc string // Type: "string" | "integer" | "boolean"
	Required        bool
}

// routes is the table. Entries for the Hub's own routes (hub-contract §7, rows 1-45)
// come first, in the contract's row order, then Phase W's in task order. Example:
var routes = []Route{
	// … the Hub's rows …
	{Pattern: "POST /v1/teams/{teamId}/webhooks", Summary: "Register a webhook for a team's events",
		Req: hubapi.CreateWebhookRequest{}, Resp: hubapi.CreatedWebhook{}, OK: 201,
		Codes: []string{hubapi.CodeWebhookURLInvalid}, Tag: "webhooks"},
	// … one per /v1 pattern …
}
```

`hubapi.CreateWebhookRequest` (`{url, projectId?, kinds}`) is task 143's to name; if that part
names it otherwise, the entry follows it.

#### `cmd/hubopenapi/main.go` (complete)

```go
// Command hubopenapi writes docs/api/openapi.json and docs/api/README.md from the
// Hub's route table, the hubapi types and docs/api/roles.json. With -check it writes
// nothing and exits 1 when either file differs from what it would write.
package main

func main() {
	check := flag.Bool("check", false, "compare, do not write")
	root := flag.String("root", ".", "the repository root")
	flag.Parse()
	doc, readme, err := Generate(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "hubopenapi:", err)
		os.Exit(2)
	}
	for path, want := range map[string][]byte{"docs/api/openapi.json": doc, "docs/api/README.md": readme} {
		full := filepath.Join(*root, path)
		if *check {
			got, err := os.ReadFile(full)
			if err != nil || !bytes.Equal(got, want) {
				fmt.Fprintf(os.Stderr, "hubopenapi: %s is stale; run go run ./control-plane/cmd/hubopenapi\n", path)
				os.Exit(1)
			}
			continue
		}
		if err := os.WriteFile(full, want, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "hubopenapi:", err)
			os.Exit(2)
		}
	}
}

// Generate is pure given the tree: the patterns from server.RoutePatterns(), the
// entries of routes, the roles file. Output is deterministic: maps are emitted with
// sorted keys, paths in pattern order, schemas by name.
func Generate(root string) (openapiJSON, readmeMD []byte, err error)
```

**Getting the patterns without a server.** `(*Server).Patterns()` needs a `*Server`. If Hub task
08's constructor registers routes without opening a connection (**UNVERIFIED**), `Generate` calls
it with zero `Deps`. If it dials, this task proposes, as a Hub edit for the `feat/hub` owner,
`server.RoutePatterns() []string`: the same registration run against a recording mux, with no I/O.
The part assumes the proposal is needed and names it in the gaps; either form satisfies the tests.

#### `cmd/hubopenapi/schema.go`: the reflection rules (complete as rules; the code follows them)

```go
// schemaFor maps a Go type to an OpenAPI 3.0.3 Schema Object, adding named
// structs to components.schemas under their Go name (a generic Page[T] becomes
// "Page_<T>"). The rules, and nothing else:
//   string → {type: string}; bool → {type: boolean};
//   int, int32, int64 → {type: integer, format: int64}; float64 → {type: number};
//   a named string type with declared constants (hubapi's closed words, found by
//     the enums table below) → {type: string, enum: [...]};
//   *T → schemaFor(T) with nullable: true;
//   []T → {type: array, items: schemaFor(T)};
//   map[string]any → {type: object, additionalProperties: {}};
//   map[string]T → {type: object, additionalProperties: schemaFor(T)};
//   struct → {type: object, properties in field order, required: every field
//     whose json tag has no ",omitempty", additionalProperties: false for a
//     request type only (the Hub rejects unknown request fields, hub-contract:34-35)}.
//   A field tagged json:"-" is skipped. Any other kind is an error: the generator
//   fails rather than guess (an interface{} field outside map[string]any, a chan).
func schemaFor(t reflect.Type, c *components, request bool) (map[string]any, error)

// enums lists each closed-word type and its values. It is checked by a test
// against hubapi's own validators (each type's Valid() or its values list, where
// task 03 and task 143 export one), so a word added in hubapi and not here fails.
var enums = map[reflect.Type][]string{ /* hubapi.Priority: none urgent high medium low, … */ }
```

Response schemas are **not** `additionalProperties: false`: consumers must ignore unknown response
fields (`hub-contract:33-34`), so the document never forbids a field the Hub may add.

#### `docs/api/roles.json` (the shape, complete)

```jsonc
// Written by TestExportRouteRoles from the matrix rows; one entry per pattern.
// Columns are hub-contract §5.3's: V viewer, M member, A team admin, O org admin
// not in the team, N a non-member of the org's team, X another org's user.
{ "POST /v1/teams/{teamId}/webhooks": { "V": 403, "M": 403, "A": 201, "O": 404, "N": 404, "X": 404 },
  "GET /v1/issues/{issueId}":          { "V": 200, "M": 200, "A": 200, "O": 404, "N": 404, "X": 404 } }
```

The generator writes each operation's `x-panacea-roles` as the columns that answer 2xx (for the
first entry, `["A"]`) and each non-2xx column's status into `responses`. A pattern with no matrix
row is an error, so an undocumented authorisation is as loud as an undocumented route. **H16b's
Hub rows** carry `name, method, path` (task 07 quotes them as `row{name, method, path, …}`); whether
`name` is the pattern is **UNVERIFIED**. If it is not, `TestExportRouteRoles` maps Hub rows by a
`pattern` field the part proposes adding to H16b's `row` (a Hub edit, listed in the gaps).

#### `docs/api/README.md`: what the generator writes, in order

1. **Base URL, version and auth.** `https://hub.<env-domain>/v1`; additive-change rules
   (`hub-contract:32-35`); the three credentials (a realm token for `panacea-app-hub`, an approved
   app's token, a `pak_` key) and that a bearer starting `pak_` is only ever a key (task 160).
2. **Errors.** The envelope (`hub-contract` §4.2) and the table of every `hubapi.Code*` with its
   status, generated from `codes.go`.
3. **Rate limits.** Each bucket of hub-contract §11.3 plus task 160's `key` and `app` buckets, their
   defaults from the `PANACEA_APP_HUB_RATE_*` variables, and the headers `RateLimit-Limit`,
   `RateLimit-Remaining`, `RateLimit-Reset` and, on a 429, `Retry-After` and `details.scope`.
4. **The change feed and webhooks.** `GET /v1/changes` paging; SW.22's payload; the three headers;
   the signature recipe with a worked example (the known vector of task 156's
   `TestSignKnownVector`); the 5-minute window; at-least-once delivery, de-duplicate on
   `Panacea-Delivery`, order by `event.seq`; the retry schedule and the disable rule.
5. **Every route**, grouped by `Tag`: method and path, summary, roles, request and response schema
   names (linking into `openapi.json`), its own codes.
6. **What is not here** (`/p/`, `/hooks/`, `/teams/messages`, health) and why.

The README carries no prose that is not generated: sections 1, 4 and 6 are fixed text in
`readme.go`, so they change only through a reviewed code change.

#### `sdk/ts/package.json` (complete)

```json
{
  "name": "@panacea-app/sdk",
  "version": "0.1.0",
  "private": true,
  "type": "module",
  "main": "src/index.ts",
  "scripts": {
    "generate": "openapi-typescript ../../docs/api/openapi.json -o src/schema.ts",
    "check": "tsc --noEmit --strict -p tsconfig.json",
    "test": "vitest run"
  },
  "devDependencies": {
    "openapi-typescript": "7.4.4",
    "typescript": "~6.0.3",
    "vitest": "5.0.1"
  }
}
```

No `dependencies` key: the client needs nothing at run time but `fetch`. `typescript` and `vitest` are
pinned to the desktop's versions (`desktop/package.json:33`, `:35`). Licences: `openapi-typescript`
MIT, `typescript` Apache-2.0, `vitest` MIT, each a development dependency only; step 1 reads each
package's `LICENSE` from `node_modules` after install and the commit message records them.
`openapi-typescript`'s pin `7.4.4` is **UNVERIFIED** as the current release (the registry could not
be read from here); step 1 pins whatever the owner's registry mirror serves and records it.
`tsconfig.json` is `{"compilerOptions": {"target": "ES2022", "module": "ESNext",
"moduleResolution": "Bundler", "strict": true, "noEmit": true, "lib": ["ES2022", "DOM"]},
"include": ["src"]}`.

#### `sdk/ts/src/index.ts` (complete)

```ts
// @panacea-app/sdk — a thin client over the Hub's public API. Types come from
// src/schema.ts, generated from docs/api/openapi.json; this file is ours.
import type { components, paths } from "./schema";

export type Schemas = components["schemas"];

export interface ClientOptions {
  baseUrl: string;                       // "https://hub.<env-domain>"
  token: () => Promise<string>;          // a realm access token, an app's token, or a pak_ key
  fetch?: typeof fetch;                  // defaults to globalThis.fetch
}

/** The rate-limit headers of the tightest bucket that applied (task 160). */
export interface RateLimit { limit: number; remaining: number; resetSeconds: number }

export class HubError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    readonly details: Record<string, unknown>,
    readonly retryAfterSeconds: number | null,   // set on a 429
    readonly rateLimit: RateLimit | null,
  ) { super(`${status} ${code}`); }
}

export interface Result<T> { data: T; rateLimit: RateLimit | null }

function readRateLimit(h: Headers): RateLimit | null {
  const l = h.get("RateLimit-Limit"), r = h.get("RateLimit-Remaining"), s = h.get("RateLimit-Reset");
  if (l === null || r === null || s === null) return null;
  return { limit: Number(l), remaining: Number(r), resetSeconds: Number(s) };
}

export function createClient(opts: ClientOptions) {
  const f = opts.fetch ?? globalThis.fetch;
  async function call<T>(method: string, path: string, body?: unknown): Promise<Result<T>> {
    const res = await f(opts.baseUrl + path, {
      method,
      headers: { Authorization: `Bearer ${await opts.token()}`,
                 ...(body === undefined ? {} : { "Content-Type": "application/json; charset=utf-8" }) },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    const rateLimit = readRateLimit(res.headers);
    if (!res.ok) {
      const e = (await res.json().catch(() => ({}))) as { error?: string; details?: Record<string, unknown> };
      const ra = res.headers.get("Retry-After");
      throw new HubError(res.status, e.error ?? "unknown", e.details ?? {},
                         ra === null ? null : Number(ra), rateLimit);
    }
    const data = (res.status === 204 ? undefined : await res.json()) as T;
    return { data, rateLimit };
  }
  const q = (o: Record<string, string | number | undefined>) => {
    const s = new URLSearchParams();
    for (const [k, v] of Object.entries(o)) if (v !== undefined) s.set(k, String(v));
    const t = s.toString();
    return t === "" ? "" : `?${t}`;
  };
  return {
    call,
    listIssues: (projectId: string, query: { after?: string; limit?: number } = {}) =>
      call<paths["/v1/projects/{projectId}/issues"]["get"]["responses"]["200"]["content"]["application/json"]>(
        "GET", `/v1/projects/${encodeURIComponent(projectId)}/issues${q(query)}`),
    getIssue: (issueId: string) =>
      call<Schemas["IssueHead"]>("GET", `/v1/issues/${encodeURIComponent(issueId)}`),
    changes: (after: string, limit?: number) =>
      call<Schemas["ChangesPage"]>("GET", `/v1/changes${q({ after, limit })}`),
  };
}
```

`IssueHead` is task 03's type (`task-03.md:282`), named in the document by its Go name. The error
envelope is the contract's flat one, `{"error": "<code>", "message", "details"}`
(`hub-contract:218`); the test's recorded 429 uses that shape.

#### `sdk/go/go.mod`, `sdk/go/oapi.yaml` and `go.work` ⚑ (complete)

```text
module github.com/kpmg-au-emu/panacea-app/sdk/go

go 1.23
```

```yaml
# sdk/go/oapi.yaml — oapi-codegen's configuration (Apache-2.0, run at build time only:
#   go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.4.1 -config oapi.yaml ../../docs/api/openapi.json)
package: panacea
generate:
  models: true
output: types.gen.go
output-options:
  skip-prune: true
```

```text
go 1.23

use (
	./control-plane
	./sandboxd
	./sdk/go
)
```

The `go.work` change is one line in `use`; the `go` directive is untouched, and step 5 checks
`git diff go.work` shows exactly that line before the commit. `oapi-codegen` is never a
requirement of any module: it runs through `go run …@<version>`, so `sdk/go/go.mod` has no
`require` block and `go list -m all` inside `sdk/go` prints the module alone. The version pin
`v2.4.1` is **UNVERIFIED** as current; step 1 records what the proxy mirror serves.

#### `sdk/go/client.go` (complete)

```go
// Package panacea is a thin client over the Hub's public API. types.gen.go is
// generated from docs/api/openapi.json; this file is ours, standard library only.
package panacea

type Client struct {
	BaseURL string                                 // "https://hub.<env-domain>"
	Token   func(ctx context.Context) (string, error) // a realm token, an app's token, or a pak_ key
	HTTP    *http.Client                           // nil: http.DefaultClient
}

// RateLimit is the tightest bucket's headers (task 160); nil when absent.
type RateLimit struct{ Limit, Remaining, ResetSeconds int }

// Error is a non-2xx answer in the contract's flat envelope (hub-contract:218).
type Error struct {
	Status            int
	Code              string         `json:"error"`
	Message           string         `json:"message"`
	Details           map[string]any `json:"details"`
	RetryAfterSeconds int            // 0 unless a 429 carried Retry-After
	RateLimit         *RateLimit
}

func (e *Error) Error() string { return fmt.Sprintf("%d %s", e.Status, e.Code) }

// Do sends one request and decodes a 2xx body into out (nil for 204).
func (c *Client) Do(ctx context.Context, method, path string, in, out any) (*RateLimit, error) {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return nil, err
	}
	tok, err := c.Token(ctx)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	if in != nil {
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
	}
	hc := c.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	rl := readRateLimit(resp.Header)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		e := &Error{Status: resp.StatusCode, RateLimit: rl}
		_ = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(e)
		e.RetryAfterSeconds, _ = strconv.Atoi(resp.Header.Get("Retry-After"))
		return rl, e
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		return rl, nil
	}
	return rl, json.NewDecoder(resp.Body).Decode(out)
}

func readRateLimit(h http.Header) *RateLimit {
	l, e1 := strconv.Atoi(h.Get("RateLimit-Limit"))
	r, e2 := strconv.Atoi(h.Get("RateLimit-Remaining"))
	s, e3 := strconv.Atoi(h.Get("RateLimit-Reset"))
	if e1 != nil || e2 != nil || e3 != nil {
		return nil
	}
	return &RateLimit{Limit: l, Remaining: r, ResetSeconds: s}
}

// ListIssues is one named helper; every other route goes through Do with the
// generated types (types.gen.go names each schema by its Go name in hubapi).
func (c *Client) ListIssues(ctx context.Context, projectID, after string, limit int) (*Page_IssueHead, *RateLimit, error) {
	var out Page_IssueHead
	rl, err := c.Do(ctx, http.MethodGet, "/v1/projects/"+url.PathEscape(projectID)+"/issues?after="+
		url.QueryEscape(after)+"&limit="+strconv.Itoa(limit), nil, &out)
	return &out, rl, err
}
```

`Page_IssueHead` is the component name `schema.go` gives `hubapi.Page[hubapi.IssueHead]`; how
`oapi-codegen` turns `Page_IssueHead` into a Go identifier (kept, or `PageIssueHead`) is
**UNVERIFIED**, and `go build ./sdk/go/...` in the acceptance fails if the helper names it wrongly.

#### Tests to write first

`cmd/hubopenapi/main_test.go` (unit; builds the patterns with `server.RoutePatterns()`):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestCheckedInDocumentIsCurrent` | `Generate(root)` equals `docs/api/openapi.json` and `docs/api/README.md` byte for byte | stale docs |
| `TestEveryV1PatternIsDocumented` | every pattern of `RoutePatterns()` starting `/v1` (after the method) has one `routes` entry, and every entry's pattern is in `RoutePatterns()` | an undocumented route; a documented route that does not exist |
| `TestNoBrowserOrHostPathPublished` | no path in the document starts `/p/`, `/hooks/`, `/teams/` or is `/healthz` or `/readyz` | browser routes published |
| `TestEveryOperationHasARole` | every operation has a non-empty `x-panacea-roles`, read from `roles.json` | an undocumented authorisation |
| `TestSchemaRules` | a fixture struct with a pointer, a slice, `map[string]any`, an `omitempty` field and an enum type → the schema the rules give, written literally in the test; a `chan` field → an error | a nullable dropped; `required` wrong; a guessed schema |
| `TestEnumsMatchHubapi` | each `enums` entry equals its `hubapi` values list | a closed word added in `hubapi` and missing from the document |
| `TestRequestSchemasAreClosed` | every request schema has `additionalProperties: false`; no response schema does | unknown request fields documented as accepted; response fields forbidden |

`server/openapi_roles_integration_test.go` (`//go:build integration`; no database):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestExportRouteRoles` | with `-update`, writes `roles.json` from `workMatrixRows` and H16b's rows; without, the file equals what it would write | roles drifting from the matrix |

`sdk/go/golden_test.go` and `sdk/ts/src/index.test.ts`:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestGoldensRoundTrip` (Go) | each `control-plane/internal/hub/hubapi/testdata/work-*/*.json` decodes into the generated type named by its directory, re-encodes, and equals the golden after canonicalising both | SDK type drift |
| `TestSDKHasNoDependencies` (Go) | `go list -m all` in `sdk/go` prints one line | a runtime dependency slipped in |
| `lists issues as typed rows` (TS) | a recorded `200` page through a fake `fetch` → `data.items[0].alias` is `"EP-142"`; `rateLimit` read from the three headers | untyped rows; headers dropped |
| `surfaces Retry-After on a 429` (TS) | a recorded 429 `{"error":"rate_limited","message":"…","details":{"scope":"key"}}` with `Retry-After: 3` → `HubError` with `code "rate_limited"`, `retryAfterSeconds 3`, `details.scope "key"` | rate limits hidden from callers |
| `tsc --noEmit --strict` (TS) | the package, `schema.ts` included, compiles | TS client does not compile |

#### Steps

- [ ] **Step 1: Tools and licences.** Install `sdk/ts`'s development dependencies from the owner's
  registry mirror; read each `LICENSE`; confirm `oapi-codegen`'s version and licence from the Go
  proxy mirror; record all four in the commit message. Stop and report if any licence is not MIT,
  Apache-2.0, BSD or ISC (`OD-11`'s list, applied here too).
- [ ] **Step 2: `RoutePatterns`.** If Hub task 08's constructor dials, write the proposal for the
  `feat/hub` owner; the generator uses a local copy of the registration list until it merges,
  marked with the proposal's id so it is deleted then.
- [ ] **Step 3: The generator's tests**, then `routes.go` (every entry), `schema.go`, `readme.go`,
  `main.go`, and `TestExportRouteRoles`, until green. Generate the three `docs/api/` files.
- [ ] **Step 4: The SDKs.** `npm run generate` and `go run …oapi-codegen… -config oapi.yaml`; write
  `index.ts`, `client.go` and their tests, until green.
- [ ] **Step 5: `go.work`.** Add `./sdk/go`; `git diff go.work` shows the one line; commit by
  pathspec.

#### Acceptance

```bash
go run ./control-plane/cmd/hubopenapi -check; echo $?                                      # 0
go test ./control-plane/cmd/hubopenapi/ -race -count=1 -v 2>&1 | grep -c '^--- PASS'       # 7
go test ./control-plane/internal/hub/server/ -tags integration -run '^TestExportRouteRoles$' -count=1   # ok
go build ./sdk/go/... && go test ./sdk/go/... -count=1                                     # ok, from the workspace root
(cd sdk/ts && npm run check && npm test)                                                   # exit 0; 2 passed
grep -c '"/p/\|"/hooks/\|"/teams/' docs/api/openapi.json                                   # 0
git diff --stat go.work                                                                     # 1 file changed, 1 insertion(+)
```

Expected as commented: exit `0`, `7`, `ok`, `ok`, exit `0` with 2 passed, `0`, one insertion.

#### What this task does NOT claim

- **Not that the SDKs are published** to npm or a Go module proxy. Both live in this repository;
  publishing is an owner decision (and a registry the corporate network allows).
- **Not that the document covers routes added after W5.** `TestEveryV1PatternIsDocumented` makes a
  later plan's route fail CI until its entry is written; that is the guard, not a promise.
- **Not that an outside caller can reach `/v1` today.** hub-contract §17 accepts a realm token for
  the desktop client only; apps (task 159) and keys (task 160) are proposed Hub-contract edits.
  The README says so in its auth section until they merge.
- **Not OpenAPI 3.1** (decision 1), and not byte-for-byte SDK round-trips (decision 5).
- **Not that the desktop uses the SDK.** It keeps `desktop/src/api/work.ts` (task 37) in W5.

**Parity rows:** closes **L-249**; serves **L-248** (the API is already public) and **L-251** (the
webhook recipe is documented here).

#### Gaps found while writing this part

1. The File map has no rows for `control-plane/cmd/hubopenapi/` or `docs/api/`.
2. The scope's `server/work_matrix_rows.go` is task 07's `server/work_matrix_rows_integration_test.go`
   (task 07's decision 7), so roles reach the generator through `docs/api/roles.json`, written by a
   test.
3. `server.RoutePatterns()` (patterns without a live server) is a proposed Hub edit if Hub task 08's
   constructor does I/O; H16b's `row` may need a `pattern` field for the Hub's own rows.
4. `oapi-codegen`'s OpenAPI 3.1 support and both tool versions are **UNVERIFIED** from this network.
5. The README documents task 160's `key` and `app` buckets and headers before task 160 lands; the
   drift test forces a regenerate when it does, so task 160's part must list `docs/api/*` in its
   files.
