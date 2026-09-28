### Task 26: The AU-residency canary

**The gateway's own config is already right, and the residency promise is still false.** LiteLLM's `/v1/messages/count_tokens` ignores a deployment's custom `api_base` and calls `api.anthropic.com` with the deployment's own key: BerriAI/litellm **#31149** (*"[Bug]: Anthropic `/v1/messages/count_tokens` ignores custom `api_base` despite repeated production reports"*, **OPEN**, opened 2026-06-24) and **#42073** (*"[Bug]: Proxy `/v1/messages/count_tokens` sends third-party `api_key` to `api.anthropic.com` when deployment uses custom `api_base`"*, **OPEN**, opened 2026-09-20), both read from the GitHub API on 2026-09-28 and recorded at `docs/superpowers/planning/2026-09-28-oss-adoption/06-platform.md:516-556`. The mechanism named there is `AnthropicTokenCounter.count_tokens()` not passing the custom `api_base` to the count-tokens handler, so it defaults to `https://api.anthropic.com/v1/messages/count_tokens`. **Claude Code calls that endpoint on the hot path of every turn.** So a project classified `au-only` under OD-24 sends message content to Global, silently, while every line of `litellm.config.yaml` says `bedrock/…` in `ap-southeast-2`. This task is OD-33's first half (`docs/superpowers/planning/2026-09-23-owner-decisions.md:203-222`, on the main tree's branch; this worktree's copy stops at OD-29).

**A config assertion cannot catch this, so this task asserts egress.** Task 02's tests already read the rendered config, and they pass while the leak is live — the config is not what is wrong, the routing under it is. The proof therefore has to be *"nothing arrived at `api.anthropic.com`"*, measured at the place a request would arrive. Locally that place exists: a **leak sentinel**, one more copy of task 16's `fakeanthropic`, carrying the Docker network alias `api.anthropic.com` on the internal backend network, with task 16's `GET /_captured` as the evidence.

**It fails today, and that is the point.** Nothing in phase 6 is built yet, so `residency_canary.sh` written now exits **2, `CANARY NOT PROVEN`**, naming the first piece that is missing. It stays red through the build and turns green in task 23. Three exit codes, and only one of them is a pass:

| Exit | Meaning | Who sees it |
|---|---|---|
| `0` | `CANARY PASS` — every endpoint the product uses was driven on an AU-only project and the sentinel recorded nothing | task 23, and every LiteLLM bump after it |
| `1` | `CANARY LEAK` — the sentinel recorded a request. The image leaks, or the rule regressed | anyone; this is the loud failure OD-33 asks for |
| `2` | `CANARY NOT PROVEN` — the stack, the sentinel or the positive control is missing or unreachable | today, and any run where the harness itself is broken |

**A canary that cannot see a leak is worse than none, so it proves itself first.** Before it trusts a count of zero, the script runs a **positive control**: a `curlimages/curl` container on the backend network posts to `http://api.anthropic.com/v1/messages`, and the sentinel must record it. If it does not, the alias is not intercepting and the run is `NOT PROVEN` (2), never a pass. Every "zero requests" assertion in this repository that has no positive control is one DNS change away from being vacuous.

**What the product does about the leak: it does not serve the endpoint on AU.** The protocol documentation is explicit that this costs nothing structural — *"Token-counting endpoints are the only optional ones: when they're absent, Claude Code falls back to a character-based estimate of context usage"*, and in the same table, *"No error: Claude Code falls back to a character-based estimate, so `/context` shows approximate counts"* (https://code.claude.com/docs/en/llm-gateway-protocol, read 2026-09-28, quoted at `…/2026-09-28-oss-adoption/06-platform.md:1057-1090`). So on the **AU** route the gatekeeper answers `404` with problem code `path_not_allowed`, the answer task 11 already writes for a path it does not forward, and the CLI degrades to an approximate context meter. On the **Global** route `count_tokens` is still forwarded: the upstream the leak reaches is the upstream Global means, exact counts are worth having, and there is no residency question. The leak becomes structurally impossible instead of waiting on an upstream fix that has not come in three months.

**Decisions this part makes:**
- **(a) The rule is data in one package, not an `if` in the proxy.** `internal/inference/residency` holds the forwardable paths per route and the upstream hosts per route. Task 11's allowlist consults it. A rule spread across a proxy handler cannot be unit-tested in wave 1, and this rule has to exist before the thing it constrains.
- **(b) `residency` imports nothing outside the standard library — not even task 01.** It takes a route as the wire string (`"global"`, `"au"`, task 01's JSON values), so this task can be written and merged in wave 1 without waiting for `dataclass`, and task 11 passes `string(route)`. One test pins the two literals and names task 01 as their owner, so a rename there fails here.
- **(c) An unknown route refuses everything** (P6-D5). `""`, `"AU"`, `"bedrock"` and anything else are not "probably Global".
- **(d) Paths match exactly, never by prefix.** `/v1/messages/count_tokens/x` is not `/v1/messages/count_tokens`, and a prefix match would forward `/v1/messages/count_tokens` wherever `/v1/messages` is allowed — which is the leak, re-introduced by a helper.
- **(e) No configuration switch re-enables `count_tokens` on AU.** A flag that turns the residency promise off at run time is a flag someone sets while debugging. When a pinned LiteLLM version passes the canary with forwarding on, the constant changes in a commit, with the canary run recorded beside it (seam S6.14).
- **(f) The canary drives the product's endpoints through the gatekeeper, and measures at the sentinel** — not at LiteLLM's logs, which the leak bypasses (the call is made by LiteLLM's own HTTP client, to a host our config never names).
- **(g) The AU fixture project is the stack's own** (`deploy/local/gateway/projects.json`, task 17's seed: one Global, one AU). The canary does not invent a project; a fixture that only the canary knows about is a fixture nothing else keeps honest.

**Files:**
- `control-plane/internal/inference/residency/residency.go` — the rule: `Forward`, `ForwardablePaths`, `Upstreams`, `LeakHost` (new)
- `control-plane/internal/inference/residency/residency_test.go` — seven tests, each killing a named mutant (new)
- `deploy/local/gateway/residency_canary.sh` — the egress assertion, three exit codes, the positive control (new)
- `docs/verification/2026-09-28-phase-6-residency-canary.md` — one row per run: date, LiteLLM image digest, verdict, who ran it (new)
- Amended, not written here, each named in "Amendments this task carries" below: `task-11.md`, `task-17.md`, `task-18.md`, `task-23.md`.

**Interfaces:**
- Produces:
  - `residency.Forward(route, path string) (bool, string)` — `true, ""` to forward; `false, wire.CodePathNotAllowed` to refuse. The second value is the problem code the caller writes, never a sentence.
  - `residency.ForwardablePaths(route string) []string` — the allowlist task 11's `AllowedPaths` becomes, per route.
  - `residency.Upstreams` — `map[string][]string`: `"global"` → `{"api.anthropic.com"}`, `"au"` → `{"bedrock-runtime.ap-southeast-2.amazonaws.com", "bedrock-runtime.ap-southeast-4.amazonaws.com"}`. Read by task 19's NetworkPolicy and ask P8-3's allowlist, so one list drives the code and the firewall.
  - `residency.LeakHost` — `"api.anthropic.com"`, the host the canary watches and the one host AU must never reach.
  - `deploy/local/gateway/residency_canary.sh`, exit `0`/`1`/`2`, and one evidence line on stdout for the verification document.
- Consumes: nothing in wave 1. Its proof mode consumes task 16's `fakeanthropic` and `/_captured`, task 17's stack and its AU fixture project, task 12's gatekeeper and task 15a's mint path — all of which it names and none of which it waits for.
- Consumed by: task 11 (the allowlist), task 18 (P17), task 19 (`Upstreams` as the egress policy's source), task 23 (step: run the canary, paste the line), and every LiteLLM bump after this phase.

**Amendments this task carries.** Each is a change to another part's text, written here so one reader sees the whole rule; the executor of this task makes them, because a rule split across five parts that nobody owns is the state this defect was found in.

| Part | The amendment | Why it is here and not there |
|---|---|---|
| `task-11.md` | `AllowedPaths` becomes `residency.ForwardablePaths(string(decision.Route))`, so `/v1/messages/count_tokens` is `404 path_not_allowed` on AU and forwarded on Global. Two tests are added to its list: an AU decision on `count_tokens` is `404` and makes **no** upstream call; a Global decision on the same path is forwarded (kills a rule that refuses everywhere, which would cost Global its exact counts for nothing) | Task 11's allowlist is one constant today. Leaving the rule there alone would make it an `if` in a proxy handler, which decision (a) refuses |
| `task-17.md` | A third fake, `leak-sentinel`, on `panacea-app-gateway-backend` **only**, with the network alias `api.anthropic.com`, from the same `images/gateway` image as the others (`fakeanthropic -label leak`). Its `compose_test.sh` gains: the alias exists, the sentinel publishes no port, and it is on no other network | The alias has to sit on the network LiteLLM egresses from, which is task 17's to declare. A canary that stands up its own compose file proves something about a stack nobody ships |
| `task-18.md` | **P17**: does the pinned image leak? Drive `POST /v1/messages/count_tokens` at LiteLLM directly on an `au/` group, then read the sentinel's `/_captured`. A captured request is the defect **present** (the expected result on `v1.102.1`); nothing captured is UNKNOWN until the positive control passes, and PASS only with it. P17's result is what task 26's constant is set against, and the probe record names the digest | Task 18 is where "what the pinned image actually does" is measured. The canary asserts the product's behaviour; P17 measures upstream's, and the two answers differ |
| `task-23.md` | A twelfth item: run `residency_canary.sh`, paste its evidence line into the record, and stop on anything but `0`. Item 12 is a **blocker**, not a partial pass | The end-to-end record is the one place the whole stack is up with a real CLI |

**Tests, and the mutant each kills.** Seven in `residency_test.go`, one mutant each, in the shape of task 10's file (a `// Mutant killed:` comment above each function).

| # | Test | Mutant killed |
|---|---|---|
| 1 | `TestCountTokensIsRefusedOnAU` — `Forward("au", "/v1/messages/count_tokens")` is `false, wire.CodePathNotAllowed` | Forwarding it on both routes: the leak, written into our own rule |
| 2 | `TestCountTokensIsForwardedOnGlobal` — the same path on `"global"` is `true, ""` | Refusing it everywhere. Global loses exact counts and nobody notices the AU rule is doing nothing |
| 3 | `TestMessagesIsForwardedOnBothRoutes` — `/v1/messages` is `true, ""` on both | A rule that refuses the product's only required endpoint |
| 4 | `TestAnUnknownRouteForwardsNothing` — `""`, `"AU"`, `"bedrock"`, `"Global"` each refuse every path | `default: return true` — P6-D5 in one line |
| 5 | `TestPathsMatchExactly` — `/v1/messages/count_tokens/x`, `/v1/messages/count_tokens/`, `/V1/Messages` and a path still carrying `?x=1` all refuse on AU | A `strings.HasPrefix` helper, which forwards `count_tokens` wherever `/v1/messages` is allowed — the leak re-introduced by a convenience |
| 6 | `TestUpstreamsAreDisjointAndAUNeverNamesTheLeakHost` — `Upstreams["au"]` does not contain `LeakHost`, `Upstreams["global"]` does, and neither list is empty | An AU egress list that keeps `api.anthropic.com` "just in case", which makes task 19's NetworkPolicy and P8-3's allowlist agree with the bug |
| 7 | `TestTheRouteStringsAreTaskOnesOwn` — the two literals equal `string(dataclass.RouteGlobal)` and `string(dataclass.RouteAU)`, read in the **test** file, which may import task 01 | Decision (b)'s own risk: the package imports nothing, so a rename in task 01 would otherwise pass here and refuse every request in production |

- [ ] **Step 1: Preflight**

```bash
test -d control-plane/internal/inference || echo "task 02 has not landed; mkdir -p it"
grep -n 'CodePathNotAllowed' control-plane/internal/inference/wire/wire.go   # task 03's constant, or absent in wave 1
grep -rn 'api.anthropic.com' deploy/local/gateway/ docs/superpowers/plans/2026-09-25-phase-6-gateway-parts/task-17.md
go version   # expect go1.23.x, the module's directive
```

Nothing here is a blocker. In wave 1 `wire` does not exist yet, so the second grep is empty and step 2's constant is the literal `"path_not_allowed"` with a comment naming `wire.CodePathNotAllowed` as its owner; task 03 lands the constant and the literal is checked against it by test 7's sibling (one line, same file). The package still imports nothing (decision (b)).

- [ ] **Step 2: Write the rule**

`control-plane/internal/inference/residency/residency.go`, standard library only, about 60 lines:

```go
// Package residency holds the one rule that decides what may leave for which
// upstream, per inference route. It is data, not an if in a proxy handler.
//
// It exists because LiteLLM's /v1/messages/count_tokens ignores a deployment's
// custom api_base and calls api.anthropic.com with the deployment's own key
// (BerriAI/litellm #31149, open since 2026-06-24, and #42073, both open on
// 2026-09-28). Claude Code calls that endpoint on the hot path, so forwarding
// it on the AU route would send message content to Global whatever the config
// says. Until a pinned image passes deploy/local/gateway/residency_canary.sh
// with AU forwarding on, AU does not serve it (seam S6.14).
package residency

const (
	RouteGlobal = "global" // dataclass.RouteGlobal owns these two strings;
	RouteAU     = "au"     // residency_test.go pins them to it.

	// LeakHost is the one host an AU request must never reach, and the host
	// the canary's sentinel answers on.
	LeakHost = "api.anthropic.com"

	codePathNotAllowed = "path_not_allowed" // wire.CodePathNotAllowed owns it.
)

var forwardable = map[string][]string{
	RouteGlobal: {"/v1/messages", "/v1/messages/count_tokens"},
	RouteAU:     {"/v1/messages"},
}

// Upstreams is the per-route egress list. Task 19's NetworkPolicy and ask
// P8-3's firewall allowlist read it, so one list drives the code and the wire.
var Upstreams = map[string][]string{
	RouteGlobal: {LeakHost},
	RouteAU: {
		"bedrock-runtime.ap-southeast-2.amazonaws.com",
		"bedrock-runtime.ap-southeast-4.amazonaws.com",
	},
}

// Forward reports whether path may be forwarded on route. The second value is
// the problem code the caller writes when it may not; it is never a sentence.
// An unknown route forwards nothing (P6-D5).
func Forward(route, path string) (bool, string) {
	for _, p := range forwardable[route] {
		if p == path { // exact, never a prefix: /v1/messages/count_tokens is
			return true, "" // not under /v1/messages for this purpose.
		}
	}
	return false, codePathNotAllowed
}

// ForwardablePaths is the allowlist task 11's proxy consults, per route.
func ForwardablePaths(route string) []string { … a copy, never the slice … }
```

`ForwardablePaths` returns a copy: a caller that sorts or appends to the returned slice would otherwise edit the rule at run time.

- [ ] **Step 3: Write the seven tests**

`residency_test.go`, package `residency`, one function per row of the table above, each under a `// Mutant killed:` comment naming the row. Test 5 is a table of paths; test 4 a table of routes; test 7 is the only one that imports `.../internal/dataclass`, and its failure message says *"task 01 renamed a route; residency's literals and task 11's call both move"*. Run `go test ./internal/inference/residency/` and then, by hand, break each rule in turn and confirm exactly the named test fails — a mutant that kills two tests is fine; a mutant that kills none is a missing test.

- [ ] **Step 4: Write `deploy/local/gateway/residency_canary.sh`**

`bash`, `set -euo pipefail`, no argument required, one optional `-v`. In order, and it stops at the first thing it cannot prove:

1. **Preconditions, or exit 2.** `docker compose ps` shows `litellm`, `gatekeeper`, `keysmith` and `leak-sentinel` running; `state/projects.json` holds a project whose `dataClass.residency` is `au-only`; the gatekeeper answers `HEAD /api/hello`. Each missing piece is named in the `CANARY NOT PROVEN` line, so today's run says *"leak-sentinel not running"* rather than *"no leak"*.
2. **The high-water mark.** `GET http://leak-sentinel:4900/_captured` and keep the highest `seq`. The sentinel is task 16's binary, which keeps the last 50 requests and has no reset, so the count is taken as a delta and nothing needs clearing between runs.
3. **The positive control, or exit 2.** `docker run --rm --network panacea-app-gateway-backend curlimages/curl -s -o /dev/null -w '%{http_code}' -X POST http://api.anthropic.com/v1/messages -d '{}'`, then poll `_captured` for a `seq` above the mark. If nothing arrives, the alias is not intercepting, and **every later count of zero would be vacuous**: exit 2, `positive control did not reach the sentinel`. Record the observed round trip; step 6's poll is ten times it or 15 s, whichever is longer.
4. **A second high-water mark**, taken after the control, so the control's own request is never counted as the leak.
5. **Drive every endpoint the product uses, on the AU project.** Mint through keysmith with the local bearer, then through the gatekeeper: `POST /v1/messages` (streaming and not), `POST /v1/messages/count_tokens`, `GET /v1/models`, `HEAD /api/hello`. The `count_tokens` call must answer `404 path_not_allowed` — that is the rule working — and a `200` there is a **LEAK** verdict on its own, before the sentinel is read, because the product forwarded the endpoint the defect lives in.
6. **Read the sentinel after the poll window.** Any `seq` above mark two is exit 1, `CANARY LEAK`, printing the captured method, path and header names (never a body — the sentinel has none). Zero, after the full window, is exit 0.
7. **One evidence line on stdout, whatever the verdict**, for the verification document: `CANARY <PASS|LEAK|NOT PROVEN> litellm=<digest> project=<id> endpoints=<n> captured=<n> control=<ms> at=<RFC 3339>`.

The script never calls a real provider: the only hosts it names are `gatekeeper`, `keysmith`, `leak-sentinel` and the alias, all inside the compose networks, and step 1 refuses to run if `PANACEA_APP_GATEWAY_LOCAL_UPSTREAM` is `anthropic` (task 17's real-upstream mode), because the sentinel's alias would then shadow the real host and the run would prove nothing about either.

- [ ] **Step 5: Run it today, and record `NOT PROVEN`**

With no phase 6 code in the tree, step 1 fails at the first check. Run it, paste the line into `docs/verification/2026-09-28-phase-6-residency-canary.md` as the first row, dated, verdict `NOT PROVEN`, with *"no phase 6 code exists; the canary is written before the thing it watches"* in the note column. That row is this task's own evidence that the canary fails loudly by default rather than passing vacuously — which is the property OD-33 asks for, and the property a canary written after the code almost never has.

- [ ] **Step 6: Make the four amendments**

Edit `task-11.md`, `task-17.md`, `task-18.md` and `task-23.md` exactly as the amendments table says, each with a one-line note naming task 26 and OD-33 as the source. Do not restate the rule in any of them: each points at `residency` for the rule and states only its own half.

- [ ] **Step 7: Verify and commit**

```bash
go test ./internal/inference/residency/ > /tmp/p6t26.out 2>&1; echo "exit=$?"; tail -5 /tmp/p6t26.out
bash -n deploy/local/gateway/residency_canary.sh > /tmp/p6t26b.out 2>&1; echo "exit=$?"
bash deploy/local/gateway/residency_canary.sh > /tmp/p6t26c.out 2>&1; echo "exit=$?"   # expect 2 today
grep -c 'CANARY NOT PROVEN' /tmp/p6t26c.out          # expect 1
grep -rn 'count_tokens' docs/superpowers/plans/2026-09-25-phase-6-gateway-parts/task-11.md   # the amendment landed
git add control-plane/internal/inference/residency deploy/local/gateway/residency_canary.sh docs/verification/2026-09-28-phase-6-residency-canary.md
git commit -F <message file> -- control-plane/internal/inference/residency deploy/local/gateway/residency_canary.sh docs/verification/2026-09-28-phase-6-residency-canary.md docs/superpowers/plans/2026-09-25-phase-6-gateway-parts/task-11.md docs/superpowers/plans/2026-09-25-phase-6-gateway-parts/task-17.md docs/superpowers/plans/2026-09-25-phase-6-gateway-parts/task-18.md docs/superpowers/plans/2026-09-25-phase-6-gateway-parts/task-23.md
```

Exit codes are read from a file, never through a pipe. Message: `feat(gateway): AU serves no count_tokens, and a canary that fails until it is proven (phase 6 task 26, OD-33)`, a body naming issues #31149 and #42073 with their state on 2026-09-28, and the attribution line.

**Acceptance:** `go test ./internal/inference/residency/` passes and each of the seven mutants kills its named test; `residency_canary.sh` exits **2** with `CANARY NOT PROVEN` naming the first missing piece; the verification document holds that run as its first row; the four amendments are in the four part files; no file outside the commit's pathspec changed.

**What this task does not claim:**

- **That the leak is fixed upstream.** #31149 and #42073 were both open on 2026-09-28. This task routes around the defect on AU and watches for it everywhere; it does not wait for a fix, and it does not assert one will come.
- **That the Global route is free of it.** On Global the leaked call reaches the upstream Global already means, so the residency question does not arise — but the *key* the leaked call carries is LiteLLM's own deployment key rather than the session's virtual key (#42073), so a Global `count_tokens` is **not attributed** to the session, the person or the project. Nothing in this phase corrects that; it is gap 3.
- **That the canary covers the cloud.** It measures inside one Docker network. The cloud equivalent is an egress assertion at phase 8's firewall, gap 2.
- **That a zero count proves absence for all time.** It proves no request arrived in this run's window, on the endpoints the script drives, against the digest it names. An endpoint the product starts calling and this script does not drive is invisible to it, which is why the endpoint list lives beside task 11's allowlist and not in the script alone.
- **That refusing `count_tokens` is free.** `/context` on an AU project shows an approximate figure. The docs call that a fallback, not an error (`06-platform.md:1057-1090`); nobody has watched a real session do it, which task 23's item 12 is the first chance to.

**Gaps against the skeleton, for the orchestrator:**

1. **The skeleton's path allowlist is one list, not two.** `task-11.md` and the skeleton's task 11 scope both name `/v1/messages` and `/v1/messages/count_tokens` unconditionally; the amendment makes it per route. Anyone reading the skeleton's paragraph without this part will re-introduce the leak.
2. **No cloud canary.** Task 24's proving scripts have nothing equivalent; the natural home is a P07 that captures an AU sandbox's egress at the firewall (ask P8-3 owns the allowlist). Proposed, not built: **task 24 gap** for the orchestrator to place, since this task does not edit task 24.
3. **Attribution of a Global `count_tokens` is lost** while #42073 is open (the leaked call carries the deployment key, so LiteLLM files no spend against the session's key). Task 09's rollup cannot see it. Owner-visible; no id proposed here.
4. **`wire.CodePathNotAllowed` is referenced by literal in wave 1.** Decision (b) keeps the package dependency-free; the cost is one literal that task 03 owns. If the orchestrator would rather `residency` import `wire`, the task moves from wave 1 to wave 2 and test 7 grows a second assertion.
