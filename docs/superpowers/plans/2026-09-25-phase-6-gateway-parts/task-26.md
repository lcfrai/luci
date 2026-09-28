### Task 26: The AU-residency canary

**The gateway's own config is already right, and the residency promise is still false.** LiteLLM's `/v1/messages/count_tokens` ignores a deployment's custom `api_base` and calls `api.anthropic.com` with the deployment's own key: BerriAI/litellm **#31149** (*"[Bug]: Anthropic `/v1/messages/count_tokens` ignores custom `api_base` despite repeated production reports"*, **OPEN**, opened 2026-06-24) and **#42073** (*"[Bug]: Proxy `/v1/messages/count_tokens` sends third-party `api_key` to `api.anthropic.com` when deployment uses custom `api_base`"*, **OPEN**, opened 2026-09-20), both read from the GitHub API on 2026-09-28 and recorded at `docs/superpowers/planning/2026-09-28-oss-adoption/06-platform.md:516-556`. The mechanism named there is `AnthropicTokenCounter.count_tokens()` not passing the custom `api_base` to the count-tokens handler, so it defaults to `https://api.anthropic.com/v1/messages/count_tokens`. **Claude Code calls that endpoint on the hot path of every turn.** So a project classified `au-only` under OD-24 sends message content to Global, silently, while every line of `litellm.config.yaml` says `bedrock/…` in `ap-southeast-2`. This task is OD-33's first half (`docs/superpowers/planning/2026-09-23-owner-decisions.md:203-222`, on the main tree's branch; this worktree's copy stops at OD-29).

**A config assertion cannot catch this, so this task asserts egress.** Task 02's tests already read the rendered config, and they pass while the leak is live — the config is not what is wrong, the routing under it is. The proof therefore has to be *"nothing arrived at `api.anthropic.com`"*, measured at the place a request would arrive, **by the port and scheme it would arrive on**: the leak is `https://api.anthropic.com/…`, so TCP **443** wrapped in TLS. Locally that place exists: a **leak sentinel**, `leaksentinel`, a connection recorder this task writes, listening on `:443` (and `:80`) under the Docker network alias `api.anthropic.com` on the internal backend network. It records every TCP connection it accepts, with the server name from the TLS ClientHello, and completes no handshake: the attempt is the egress, so the attempt is the evidence, and no certificate, test CA or trust store is involved. It is **not** task 16's `fakeanthropic`, which is a plaintext HTTP server on `:4900` (`task-16.md:23`): a leak dialling 443 would find nothing listening there, be refused, and read as *zero requests* — a canary that passes on a leaking image, the one failure this part exists to prevent (review C1).

**It fails today, and that is the point.** Nothing in phase 6 is built yet, so `residency_canary.sh` written now exits **2, `CANARY NOT PROVEN`**, naming the first piece that is missing. It stays red through the build and turns green in task 23. Three exit codes, and only one of them is a pass:

| Exit | Meaning | Who sees it |
|---|---|---|
| `0` | `CANARY PASS` — every endpoint the product uses was driven **and served** on an AU-only project, and the sentinel recorded no connection but the control's | task 23, and every LiteLLM bump after it |
| `1` | `CANARY LEAK` — the sentinel recorded a connection the control did not make, or the gatekeeper answered `2xx` on AU's `count_tokens`. The image leaks, or the rule regressed | anyone; this is the loud failure OD-33 asks for |
| `2` | `CANARY NOT PROVEN` — the stack, the sentinel, the tested image or the positive control is missing or unreachable, or the AU drive was not served | today, and any run where the harness itself is broken |

**A canary that cannot see a leak is worse than none, so it proves itself first, on the leak's own path.** Before it trusts a count of zero, the script runs a **positive control** from inside the `litellm` container — the container that leaks, through its own resolver and its own network — with the image's own `python3`: open `api.anthropic.com:443` and begin TLS with the server name `api.anthropic.com`. That is the name, the port and the scheme the leaked call uses, so the sentinel must record that connection, matched by the control's source port. If it does not, the alias is not intercepting what the leak would dial, and the run is `NOT PROVEN` (2), never a pass. A control on any other port, scheme or container proves a path the leak does not take: the first draft's control posted `http://…` (port 80) from another container, while the sentinel listened on 4900 and the leak dials 443 — three paths, none of them the leak's (review C1). Every "zero requests" assertion in this repository that has no positive control **on the path it measures** is one DNS change away from being vacuous.

**What the product does about the leak: it does not serve the endpoint on AU.** The protocol documentation is explicit that this costs nothing structural — *"Token-counting endpoints are the only optional ones: when they're absent, Claude Code falls back to a character-based estimate of context usage"*, and in the same table, *"No error: Claude Code falls back to a character-based estimate, so `/context` shows approximate counts"* (https://code.claude.com/docs/en/llm-gateway-protocol, read 2026-09-28, quoted at `…/2026-09-28-oss-adoption/06-platform.md:1057-1090`). So on the **AU** route the gatekeeper answers `404` with problem code `path_not_allowed`, the answer task 11 already writes for a path it does not forward, and the CLI degrades to an approximate context meter. On the **Global** route `count_tokens` is still forwarded: the upstream the leak reaches is the upstream Global means, exact counts are worth having, and there is no residency question. The leak becomes structurally impossible instead of waiting on an upstream fix that has not come in three months.

**Decisions this part makes:**
- **(a) The rule is data in one package, not an `if` in the proxy.** `internal/inference/residency` holds the forwardable paths per route and the upstream hosts per route. Task 11's allowlist consults it. A rule spread across a proxy handler cannot be unit-tested in wave 1, and this rule has to exist before the thing it constrains.
- **(b) `residency` imports nothing outside the standard library — not even task 01.** It takes a route as the wire string (`"global"`, `"au"`, task 01's JSON values), so this task can be written and merged in wave 1 without waiting for `dataclass`, and task 11 passes `string(route)`. One test pins the two literals and names task 01 as their owner, so a rename there fails here.
- **(c) An unknown route refuses everything** (P6-D5). `""`, `"AU"`, `"bedrock"` and anything else are not "probably Global".
- **(d) Paths match exactly, never by prefix.** `/v1/messages/count_tokens/x` is not `/v1/messages/count_tokens`, and a prefix match would forward `/v1/messages/count_tokens` wherever `/v1/messages` is allowed — which is the leak, re-introduced by a helper.
- **(e) No configuration switch re-enables `count_tokens` on AU.** A flag that turns the residency promise off at run time is a flag someone sets while debugging. When P17 reads PASS on a pinned LiteLLM version — no connection after a passed control — the constant changes in a commit, and the canary's `count_tokens` expectation (step 5, item 6 (ii) and (iv)) changes in the same commit: it then requires the endpoint served on AU and still reads the sentinel. The canary run that passes with forwarding on is recorded beside that commit (seam S6.14). Until then a `2xx` there is a leak by definition, so no run can pass with forwarding on by accident.
- **(f) The canary drives the product's endpoints through the gatekeeper, and measures at the sentinel** — not at LiteLLM's logs, which the leak bypasses (the call is made by LiteLLM's own HTTP client, to a host our config never names).
- **(g) The AU fixture project is the stack's own** (`deploy/local/gateway/projects.json`, task 17's seed: one Global, one AU). The canary does not invent a project; a fixture that only the canary knows about is a fixture nothing else keeps honest.
- **(h) The sentinel records connections, not requests, and terminates no TLS** (review C1). Terminating TLS for `api.anthropic.com` would need a test CA that LiteLLM's HTTP client trusts; whether it honours `SSL_CERT_FILE` or the system store is **UNVERIFIED**, and a client that refuses our certificate aborts the handshake before any request exists — so a request-level record reads zero on exactly the leak it failed to answer. Every accepted connection is recorded, whatever follows; the record is written when the ClientHello read returns or its 2 s deadline passes, and its `at` is the moment of `accept`. The cost: a record carries no path, header or key, so *what* leaked is read from what the run drove, which is why the canary drives one endpoint set on one project and P17 drives one endpoint alone.
- **(i) The sentinel is its own binary, not a mode of task 16's fake** (review I1). `fakeanthropic` speaks the Messages API in plaintext and refuses any `-label` but `global` and `au` (`task-16.md:460`, pinned by its flag table at `:378`). A recorder that reads at most a ClientHello shares none of its code, and a flag that turned the fake into a TLS listener would put an untested second job into a binary three other tasks depend on. `leaksentinel` is standard library only, so it lands in wave 1 beside the rule.
- **(j) The sentinel exists in fake mode only.** In anthropic mode LiteLLM's Global group calls the **real** `api.anthropic.com`, and the alias would shadow it. The service therefore lives in its own compose file, `compose.residency.yaml`, which `init.sh` adds to `COMPOSE_FILE` in fake mode and never in anthropic mode — the mechanism that already adds `compose.anthropic.yaml` (`task-17.md:497-507`) — and the canary refuses to run without it.

**Files:**
- `control-plane/internal/inference/residency/residency.go` — the rule: `Forward`, `ForwardablePaths`, `Upstreams`, `LeakHost` (new)
- `control-plane/internal/inference/residency/residency_test.go` — seven tests, each killing a named mutant (new)
- `control-plane/cmd/leaksentinel/main.go` — the connection recorder: the listeners, the ClientHello read, the ring, `GET /_connections` (new)
- `control-plane/cmd/leaksentinel/main_test.go` — seven tests, each killing a named mutant (new)
- `deploy/local/gateway/residency_canary.sh` — the egress assertion, three exit codes, the positive control on the leak's own path (new)
- `docs/verification/2026-09-28-phase-6-residency-canary.md` — one row per run: date, LiteLLM image digest, verdict, who ran it (new)
- Amended, not written here, each named in "Amendments this task carries" below: `task-11.md`, `task-17.md`, `task-18.md`, `task-23.md`.

**Interfaces:**
- Produces:
  - `residency.Forward(route, path string) (bool, string)` — `true, ""` to forward; `false, wire.CodePathNotAllowed` to refuse. The second value is the problem code the caller writes, never a sentence.
  - `residency.ForwardablePaths(route string) []string` — the allowlist task 11's `AllowedPaths` becomes, per route.
  - `residency.Upstreams` — `map[string][]string`: `"global"` → `{"api.anthropic.com"}`, `"au"` → `{"bedrock-runtime.ap-southeast-2.amazonaws.com", "bedrock-runtime.ap-southeast-4.amazonaws.com"}`. Read by task 19's NetworkPolicy and ask P8-3's allowlist, so one list drives the code and the firewall.
  - `residency.LeakHost` — `"api.anthropic.com"`, the host the canary watches and the one host AU must never reach.
  - the binary `leaksentinel`: flags `-listen` (comma-separated, default `:443,:80`), `-status` (default `:4902`), `-hello-timeout` (default `2s`); `GET /_connections` on the status address → `{"started": <RFC 3339>, "listening": [<addr>…], "connections": [record…]}`, oldest first, at most 50, a record being `{"seq", "at", "peer" (ip:port), "port" (the local port it arrived on), "tls" (a ClientHello was read), "sni"}`. `seq` never resets while the process lives; `started` changes when it restarts; the status address is never recorded. In local mode it is the service `leak-sentinel`, reached as `http://leak-sentinel:4902` from the backend network and dialled as `api.anthropic.com` by anything on it.
  - `deploy/local/gateway/residency_canary.sh`, exit `0`/`1`/`2`, and one evidence line on stdout for the verification document.
- Consumes: nothing in wave 1. Its proof mode consumes task 17's stack in fake mode (with this task's sentinel in it, by amendment) and its AU fixture project, task 16's `fakeanthropic-au` (whose `fake au reply` proves the AU group served the drive), task 12's gatekeeper, keysmith's mint in task 17 step 7's JSON, and task 27's `images/litellm/IMAGE_ID` — all of which it names and none of which it waits for.
- Consumed by: task 11 (the allowlist), task 17 (builds `leaksentinel` into `images/gateway` and runs it, by amendment), task 18 (P17), task 19 (`Upstreams` as the egress policy's source), task 23 (step: run the canary, paste the line), and every LiteLLM bump after this phase.

**Amendments this task carries.** Each is a change to another part's text, written here so one reader sees the whole rule; the executor of this task makes them, because a rule split across five parts that nobody owns is the state this defect was found in.

| Part | The amendment | Why it is here and not there |
|---|---|---|
| `task-11.md` | `AllowedPaths` becomes `residency.ForwardablePaths(string(decision.Route))`, so `/v1/messages/count_tokens` is `404 path_not_allowed` on AU and forwarded on Global. Two tests are added to its list: an AU decision on `count_tokens` is `404` and makes **no** upstream call; a Global decision on the same path is forwarded (kills a rule that refuses everywhere, which would cost Global its exact counts for nothing) | Task 11's allowlist is one constant today. Leaving the rule there alone would make it an `if` in a proxy handler, which decision (a) refuses |
| `task-17.md` | **The sentinel, in a fourth compose file** (decision (j)): `deploy/local/gateway/compose.residency.yaml`, complete in step 7 — one service, `leak-sentinel`, from the same `images/gateway` image as the others, running `/usr/local/bin/leaksentinel -listen :443,:80 -status :4902`, on `backend` **only**, with the alias `api.anthropic.com` there and `sysctls: net.ipv4.ip_unprivileged_port_start: "0"`, so its non-root user can bind 443 and 80 (U27). `init.sh`'s default becomes `files="compose.yaml:compose.residency.yaml"` (`task-17.md:497`); the anthropic branch (`:503`) is unchanged, so anthropic mode has no sentinel. `images/gateway` carries a fourth host-built binary, `leaksentinel`. **Every place in task 17 that a seventh service and a fourth binary break** (review M1): the prose at `:3` (the service list), `:7` (the backend's members), `:14` ("three binaries"), `:21` ("six services"), `:27` ("three host-built binaries"); the backend's DNS names at `:34` and the image's binaries at `:40`; *Consumes* at `:41`; the service table at `:46-53` (a `leak-sentinel` row); `compose_test.sh`'s copy list at `:100-101` and its fake render at `:126` (both gain `compose.residency.yaml`); `want_nets` at `:178-180` (`"leak-sentinel": ["backend"]`); the Dockerfile's comment and `COPY` at `:421` and `:426`; the build loop at `:565`, its expected output and "six services `running`" at `:570`, and "lists six services" at `:511`. `compose_test.sh` gains **four checks**, so its `PASS` count at `:228` rises by four: in fake mode `leak-sentinel` exists and runs `leaksentinel` with `:443` among its `-listen` addresses (kills review C1's defect, a sentinel on another binary or port, before any run); its `backend` aliases are exactly `[api.anthropic.com]` (kills a lost alias, which the canary would otherwise meet only at run time); in anthropic mode it is absent (kills the alias shadowing the real API); `init.sh`'s fake-mode `.env` names `compose.residency.yaml` and its anthropic-mode `.env` does not. "Publishes no port" needs no new check: the published-ports check at `:152-153` already fails on any port but the gatekeeper's and keysmith's | The alias has to sit on the network LiteLLM egresses from, which is task 17's to declare. A canary that stands up its own compose file proves something about a stack nobody ships |
| `task-18.md` | **P17**: does the pinned image leak? In the probe client, which already runs inside `litellm` (`task-18.md:7`): run the canary's positive control first (the `python3` connect to `api.anthropic.com:443` with that server name, found on the sentinel by its source port); take the high-water `seq` from `http://leak-sentinel:4902/_connections`; drive `POST /v1/messages/count_tokens` straight at LiteLLM on an `au/` group; read the sentinel again after 15 s. A connection above the mark is the defect **present** (the expected reading on `v1.102.1`, U26); none after a passed control is PASS; none after a failed control is UNKNOWN. It is the one run in which the sentinel meets the real leaking handler, so it is also the canary's proof on real traffic. P17's result is what task 26's constant is set against, and the probe record names the image ID and source commit (task 27). Task 18's counts move with it: the probe table (`:19-35`) gains a row, *"Seventeen summary lines"* (`:812`) becomes eighteen, the results table's *"P01 to P16"* (`:833`) runs to P17, and *"Sixteen probes"* (`:880`) becomes seventeen | Task 18 is where "what the pinned image actually does" is measured. The canary asserts the product's behaviour; P17 measures upstream's, and the two answers differ |
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

Seven more in `cmd/leaksentinel/main_test.go`, same shape. Each builds the sentinel with `newSentinel` and a `200ms` hello timeout, serves it on `127.0.0.1:0`, and polls its ring for up to 2 s, because a record is written when the ClientHello read returns.

| # | Test | Mutant killed |
|---|---|---|
| S1 | `TestATLSClientHelloIsRecordedWithItsServerName` — `tls.Dial` with `ServerName: "api.anthropic.com"` fails (no handshake completes) and the ring holds one record: `tls` true, `sni` `api.anthropic.com`, `port` the listener's | Recording only a completed handshake or a parsed request — review C1 itself, where the leak the sentinel could not answer read as zero; and a dropped server name |
| S2 | `TestAConnectionThatSendsNothingIsRecorded` — a bare TCP dial, closed at once, is one record with `tls` false | Recording inside `GetConfigForClient` only: a client that gives up before its ClientHello goes unseen |
| S3 | `TestAPlaintextRequestIsRecordedAndNothingOfItKept` — `POST /v1/messages/count_tokens` with `x-api-key: CANARY-KEY` and a body carrying `CANARY` is one record, `tls` false, and the `/_connections` JSON contains none of `CANARY`, `count_tokens`, `x-api-key` | Keeping the first bytes "for debugging": a leaked call carries the deployment's own key (#42073), so a sentinel that kept bytes would write a real key into a developer's log (global constraint 3) |
| S4 | `TestTheStatusEndpointIsNeverRecorded` — three `GET /_connections` leave `connections` empty | The canary's own reads counted as connections, so every run reads `LEAK` |
| S5 | `TestTheRingIsBoundedAndSeqNeverRestarts` — 60 connections leave 50 records, `seq` 11 to 60 | An unbounded ring, or a `seq` that restarts when the ring wraps, so a high-water mark reads old records as new or new ones as old |
| S6 | `TestStartedAndListeningAreReported` — two reads carry the same `started` and the configured `listening` list | A sentinel that cannot say where it listens or that it restarted: the canary could not tell one on `:443` from one on nothing, or see a restart reset `seq` under its mark |
| S7 | `TestFlagsRefuseASentinelThatRecordsNothing` — `newSentinel` refuses `-listen ""`, `-listen ","`, a `-listen` address equal to `-status`, and `-hello-timeout 0` | A sentinel started on no port, which records nothing and so reads as a pass |

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

- [ ] **Step 4: Write the sentinel, and its seven tests**

`control-plane/cmd/leaksentinel/main.go`, standard library only, about 145 lines with its comments. The record is taken for every accepted connection, after at most one ClientHello read; nothing the client sends is kept but the server name (decision (h)):

```go
// Command leaksentinel records every connection made to it and serves none
// (phase 6 task 26, OD-33). Local mode runs it, in fake mode only, under the
// Docker network alias api.anthropic.com on the internal backend network, so a
// connection here is something on that network dialling Global.
//
// It records a connection, not a request. It reads at most a TLS ClientHello,
// keeps its server name, presents no certificate and completes no handshake,
// so no trust store is involved and a client that would refuse our certificate
// is still seen. It keeps no byte the client sent beyond that name.
package main

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const ringMax = 50 // records kept, as task 16's /_captured keeps

type record struct {
	Seq  int       `json:"seq"`
	At   time.Time `json:"at"`
	Peer string    `json:"peer"` // ip:port; the canary finds its control by the port
	Port int       `json:"port"` // the local port it arrived on
	TLS  bool      `json:"tls"`  // a ClientHello was read
	SNI  string    `json:"sni"`  // its server name, "" if none
}

type sentinel struct {
	listen  []string
	hello   time.Duration
	started time.Time
	mu      sync.Mutex
	seq     int
	ring    []record
}

var errNoHandshake = errors.New("leaksentinel completes no handshake")

func newSentinel(listen, status string, hello time.Duration) (*sentinel, error) {
	var addrs []string
	for _, a := range strings.Split(listen, ",") {
		if a = strings.TrimSpace(a); a == status && a != "" {
			return nil, fmt.Errorf("-listen %s is the status address, which is never recorded", a)
		} else if a != "" {
			addrs = append(addrs, a)
		}
	}
	if len(addrs) == 0 {
		return nil, errors.New("-listen names no address: a sentinel on nothing records nothing")
	}
	if hello <= 0 {
		return nil, errors.New("-hello-timeout must be positive")
	}
	return &sentinel{listen: addrs, hello: hello, started: time.Now().UTC()}, nil
}

// serve accepts until ln closes. Every accepted connection is recorded.
func (s *sentinel) serve(ln net.Listener) {
	port := ln.Addr().(*net.TCPAddr).Port
	for {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		go s.handle(c, port)
	}
}

func (s *sentinel) handle(c net.Conn, port int) {
	defer c.Close()
	r := record{At: time.Now().UTC(), Peer: c.RemoteAddr().String(), Port: port}
	_ = c.SetDeadline(time.Now().Add(s.hello))
	cfg := &tls.Config{GetConfigForClient: func(h *tls.ClientHelloInfo) (*tls.Config, error) {
		r.TLS, r.SNI = true, h.ServerName
		return nil, errNoHandshake // an alert, never a certificate
	}}
	_ = tls.Server(c, cfg).Handshake() // always fails; the record is the point
	s.add(r)
}

func (s *sentinel) add(r record) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	r.Seq = s.seq
	if s.ring = append(s.ring, r); len(s.ring) > ringMax {
		s.ring = s.ring[len(s.ring)-ringMax:]
	}
}

func (s *sentinel) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /_connections", func(w http.ResponseWriter, _ *http.Request) {
		s.mu.Lock()
		body := map[string]any{"started": s.started, "listening": s.listen,
			"connections": append([]record{}, s.ring...)}
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(body)
	})
	return mux
}

func main() {
	listen := flag.String("listen", ":443,:80", "comma-separated addresses whose connections are recorded")
	status := flag.String("status", ":4902", "address of GET /_connections, never recorded")
	hello := flag.Duration("hello-timeout", 2*time.Second, "how long to wait for a ClientHello")
	flag.Parse()
	s, err := newSentinel(*listen, *status, *hello)
	if err != nil {
		fmt.Fprintln(os.Stderr, "leaksentinel:", err)
		os.Exit(2)
	}
	for _, a := range s.listen {
		ln, err := net.Listen("tcp", a)
		if err != nil { // a sentinel missing one of its ports must not start
			log.Fatalf("leaksentinel: listen %s: %v", a, err)
		}
		go s.serve(ln)
	}
	log.Printf("leaksentinel: recording %v, status on %s", s.listen, *status)
	log.Fatal(http.ListenAndServe(*status, s.handler()))
}
```

`GetConfigForClient` is how the standard library hands a server the ClientHello before it chooses a certificate; returning an error there ends the handshake with an alert, so the sentinel never needs a key pair. A plaintext client fails earlier, in the record layer, and is recorded with `tls` false. It logs one line at start and nothing per connection. Write `main_test.go` from the S1–S7 rows above, run `go test ./cmd/leaksentinel/`, then break each property in turn and confirm its test fails.

- [ ] **Step 5: Write `deploy/local/gateway/residency_canary.sh`**

`bash`, `set -Eeuo pipefail`, run from `deploy/local/gateway`, no argument required, one optional `-v`; JSON is read with `python3`, standard library only, as task 17's `compose_test.sh` reads it. In order, and it stops at the first thing it cannot prove:

1. **Preconditions, or exit 2.** Fake mode: `PANACEA_APP_GATEWAY_LOCAL_UPSTREAM` is unset or `fake`, and `.env`'s `COMPOSE_FILE` names `compose.residency.yaml` (decision (j)). `docker compose ps` shows `litellm`, `gatekeeper`, `keysmith` and `leak-sentinel` running. `state/projects.json` holds a project whose `dataClass.residency` is `au-only` (the seed's `local-au`). `images/litellm/IMAGE_ID` exists and equals the running `litellm` container's image (`docker inspect -f '{{.Image}}'`), so the verdict names the image task 27 tested. The **names** of `litellm`'s environment variables — listed inside the container with `env | cut -d= -f1`, so no value leaves it — include no `http_proxy`, `https_proxy` or `all_proxy` in either case: with one set, the leak would dial the proxy and not the sentinel, and a zero would be vacuous (Docker Desktop can inject them from its own proxy settings, U28). The sentinel answers (item 2) and its `listening` includes `:443`. The gatekeeper answers `HEAD /api/hello`. Each missing piece is named in the `CANARY NOT PROVEN` line, in this order, so today's run — no `init.sh` has ever written `.env` — says *".env not found: task 17's init.sh has not run"* rather than *"no leak"*.
2. **Where the sentinel is read, and the first mark.** The sentinel publishes no port and the backend is `internal: true`, so the host cannot reach it (review I2). Every read runs where it is reachable: inside `litellm`, as `docker compose exec -T litellm python3 -` — the way task 18's probe client already runs (`task-18.md:7`) — fetching `http://leak-sentinel:4902/_connections` with proxies disabled. Keep the highest `seq` (mark A) and `started`. Every later read must return the same `started`, or the run is `NOT PROVEN` (*"leak-sentinel restarted during the run"*), because a restart resets `seq` under the mark. The ring keeps 50 and never resets, so counts are deltas and nothing is cleared between runs.
3. **The positive control, or exit 2.** In the same container, `python3` opens `socket.create_connection(("api.anthropic.com", 443), 5)`, prints its own source port, and starts TLS with `server_hostname="api.anthropic.com"`; the handshake fails, as it must. Poll the sentinel for up to 10 s for the record whose `peer` ends in that port, with `port` 443, `tls` true and `sni` `api.anthropic.com`. Any **other** record above mark A is exit 1 at once — nothing was driven yet, so something on the backend dialled Global unprompted — and that is decided first, because a leak must never read as `NOT PROVEN`. Then, if the control's record did not arrive, or the connect itself failed, the alias does not intercept what the leak would dial, and **every later count of zero would be vacuous**: exit 2, *"positive control did not reach the sentinel"*. Record the control's round trip; item 6's window is ten times it or 15 s, whichever is longer.
4. **The second mark** (B), the highest `seq` once the control's record is in, so the control is never counted as the leak.
5. **Drive every endpoint the product uses, on the AU project, from where a sandbox stands.** A token is bound to the address it was minted for, and the gatekeeper refuses it from the host (`task-17.md:590-591`), so the drive runs from a `curlimages/curl` container on `panacea-app-gateway`, `gw-canary`, pinned as task 17 step 7's is, minted through keysmith's `127.0.0.1:4100` with the local bearer, in task 17 step 7's mint JSON (`task-17.md:577-585`) with `projectId` the AU project and `podAddr` that container's address. Then, through `http://gatekeeper.panacea-gateway.svc.cluster.local:4000`: `POST /v1/messages` (streaming and not), `POST /v1/messages/count_tokens`, `GET /v1/models`, `HEAD /api/hello`. Every status and body marker is kept; nothing exits here, because a leaked call may also be a failed one, and a leak must never read as `NOT PROVEN`.
6. **Read the sentinel after the window, then decide, in this order.** (i) Any record above mark B: exit 1, `CANARY LEAK`, printing each record's `seq`, `port`, `tls`, `sni` and `peer` — all the sentinel holds. (ii) `count_tokens` answered `2xx`: exit 1 — while `residency` refuses it on AU, the product forwarded the endpoint the defect lives in, whatever the sentinel saw (decision (e) says how this expectation changes, and only with the constant). (iii) Either `/v1/messages` call did not answer `200` carrying `fake au reply` (task 16's label): exit 2, *"the AU drive was not served"* — the AU group was never exercised, so the zero is vacuous. (iv) `count_tokens` answered anything but `404 path_not_allowed`: exit 2, the rule was never exercised. (v) Otherwise exit 0: the drive was served, the rule answered, and nothing dialled Global.
7. **One evidence line on stdout, whatever the verdict**, for the verification document, printed by an `EXIT` trap that also revokes the canary's session and removes `gw-canary`, and exits with the status it was given: `CANARY <PASS|LEAK|NOT PROVEN> litellm=<image id> project=<id> endpoints=<n> connections=<n> control=<ms> at=<RFC 3339> why="<the first thing it could not prove, or the first leaked record; empty on PASS>"`. It is the only line that starts `CANARY `; item 6's records print on lines of their own before it.

**Three exit codes, and no fourth** (review I2). `set -E` and `trap '…; exit 2' ERR` make any command that fails where no item expected it — a `curl` that cannot resolve, an `exec` into a stopped container — exit 2 naming its line, never `curl`'s own status. Every failure an item does expect is tested inside an `if` before its exit code is chosen.

The script never calls a real provider. The hosts it names are the gatekeeper (its alias from `gw-canary`, `127.0.0.1:4000` from the host), keysmith's `127.0.0.1:4100`, `leak-sentinel`, and `api.anthropic.com` — which, inside the backend network, **is** the sentinel, and which has no route out from there if the alias ever failed (the control would then read `NOT PROVEN`). Item 1 refuses anthropic mode, where there is no sentinel and the name is the real API.

- [ ] **Step 6: Run it today, and record `NOT PROVEN`**

With no phase 6 code in the tree, item 1 fails at the first check. Run it, paste the line into `docs/verification/2026-09-28-phase-6-residency-canary.md` as the first row, dated, verdict `NOT PROVEN`, with *"no phase 6 code exists; the canary is written before the thing it watches"* in the note column. That row is this task's own evidence that the canary fails loudly by default rather than passing vacuously — which is the property OD-33 asks for, and the property a canary written after the code almost never has.

- [ ] **Step 7: Make the four amendments**

Edit `task-11.md`, `task-17.md`, `task-18.md` and `task-23.md` exactly as the amendments table says, each with a one-line note naming task 26 and OD-33 as the source. Do not restate the rule in any of them: each points at `residency` for the rule and states only its own half. Task 17's new file, complete — it repeats the `x-gateway` keys because a YAML anchor does not cross files:

```yaml
# Phase 6 local mode, fake upstream only: the leak sentinel (task 26, OD-33).
# It answers as api.anthropic.com on the internal backend network and records
# every connection made to it. Fake mode's config names no such host, so any
# connection is something on the backend dialling Global.
# init.sh adds this file in fake mode only: in anthropic mode the alias would
# shadow the real API that LiteLLM's Global group calls.
services:
  leak-sentinel:
    image: panacea-app/gateway:local
    build:
      context: ../../..
      dockerfile: images/gateway/Dockerfile
    user: "65532:65532"
    read_only: true
    restart: unless-stopped
    command: ["/usr/local/bin/leaksentinel", "-listen", ":443,:80", "-status", ":4902"]
    sysctls:
      net.ipv4.ip_unprivileged_port_start: "0" # the non-root user binds 443 and 80
    networks:
      backend:
        aliases: [api.anthropic.com]
```

- [ ] **Step 8: Verify and commit**

```bash
go test ./internal/inference/residency/ ./cmd/leaksentinel/ > /tmp/p6t26.out 2>&1; echo "exit=$?"; tail -5 /tmp/p6t26.out
bash -n deploy/local/gateway/residency_canary.sh > /tmp/p6t26b.out 2>&1; echo "exit=$?"
bash deploy/local/gateway/residency_canary.sh > /tmp/p6t26c.out 2>&1; echo "exit=$?"   # expect 2 today
grep -c 'CANARY NOT PROVEN' /tmp/p6t26c.out          # expect 1
grep -rn 'count_tokens' docs/superpowers/plans/2026-09-25-phase-6-gateway-parts/task-11.md   # the amendment landed
grep -c 'compose.residency.yaml' docs/superpowers/plans/2026-09-25-phase-6-gateway-parts/task-17.md   # expect at least 3: files, init.sh, compose_test
git add control-plane/internal/inference/residency control-plane/cmd/leaksentinel deploy/local/gateway/residency_canary.sh docs/verification/2026-09-28-phase-6-residency-canary.md
git commit -F <message file> -- control-plane/internal/inference/residency control-plane/cmd/leaksentinel deploy/local/gateway/residency_canary.sh docs/verification/2026-09-28-phase-6-residency-canary.md docs/superpowers/plans/2026-09-25-phase-6-gateway-parts/task-11.md docs/superpowers/plans/2026-09-25-phase-6-gateway-parts/task-17.md docs/superpowers/plans/2026-09-25-phase-6-gateway-parts/task-18.md docs/superpowers/plans/2026-09-25-phase-6-gateway-parts/task-23.md
```

Exit codes are read from a file, never through a pipe. Message: `feat(gateway): AU serves no count_tokens, and a canary that fails until it is proven (phase 6 task 26, OD-33)`, a body naming issues #31149 and #42073 with their state on 2026-09-28, and the attribution line.

**Acceptance:** `go test ./internal/inference/residency/ ./cmd/leaksentinel/` passes and each of the fourteen mutants (seven per file) kills its named test — S1 above all, because it is the one that proves, in wave 1 and without the stack, that a TLS connection the sentinel refuses is still recorded; `residency_canary.sh` exits **2** with `CANARY NOT PROVEN` naming the first missing piece; the verification document holds that run as its first row; the four amendments are in the four part files; no file outside the commit's pathspec changed.

**What this task does not claim:**

- **That the leak is fixed upstream.** #31149 and #42073 were both open on 2026-09-28. This task routes around the defect on AU and watches for it everywhere; it does not wait for a fix, and it does not assert one will come.
- **That the Global route is free of it.** On Global the leaked call reaches the upstream Global already means, so the residency question does not arise — but the *key* the leaked call carries is LiteLLM's own deployment key rather than the session's virtual key (#42073), so a Global `count_tokens` is **not attributed** to the session, the person or the project. Nothing in this phase corrects that; it is gap 3.
- **That the canary covers the cloud.** It measures inside one Docker network. The cloud equivalent is an egress assertion at phase 8's firewall, gap 2.
- **That the sentinel sees what leaked.** It records that something dialled `api.anthropic.com`, from which address, on which port, and with which server name — never the path, a header or the key, because it terminates no TLS (decision (h)). Which endpoint leaked is read from what the run drove; P17 drives one endpoint alone for exactly that reason.
- **That `:80` has a run-time control.** The control proves the alias and `:443`, the port the leak uses. The `:80` listener rests on S2 and S3 (every listener records a plaintext or silent connection) and on `leaksentinel` refusing to start when any `-listen` address cannot be bound, not on a connection in each run. A leak to any host other than `api.anthropic.com` meets no route out of the internal network and is not recorded at all; the firewall is where that is asserted (gap 2).
- **That a zero count proves absence for all time.** It proves no connection arrived in this run's window, on the endpoints the script drives, against the image ID it names. An endpoint the product starts calling and this script does not drive is invisible to it, which is why the endpoint list lives beside task 11's allowlist and not in the script alone.
- **That refusing `count_tokens` is free.** `/context` on an AU project shows an approximate figure. The docs call that a fallback, not an error (`06-platform.md:1057-1090`); nobody has watched a real session do it, which task 23's item 12 is the first chance to.

**Gaps against the skeleton, for the orchestrator:**

1. **The skeleton's path allowlist is one list, not two.** `task-11.md` and the skeleton's task 11 scope both name `/v1/messages` and `/v1/messages/count_tokens` unconditionally; the amendment makes it per route. Anyone reading the skeleton's paragraph without this part will re-introduce the leak.
2. **No cloud canary.** Task 24's proving scripts have nothing equivalent; the natural home is a P07 that captures an AU sandbox's egress at the firewall (ask P8-3 owns the allowlist). Proposed, not built: **task 24 gap** for the orchestrator to place, since this task does not edit task 24.
3. **Attribution of a Global `count_tokens` is lost** while #42073 is open (the leaked call carries the deployment key, so LiteLLM files no spend against the session's key). Task 09's rollup cannot see it. Owner-visible; no id proposed here.
4. **`wire.CodePathNotAllowed` is referenced by literal in wave 1.** Decision (b) keeps the package dependency-free; the cost is one literal that task 03 owns. If the orchestrator would rather `residency` import `wire`, the task moves from wave 1 to wave 2 and test 7 grows a second assertion.
