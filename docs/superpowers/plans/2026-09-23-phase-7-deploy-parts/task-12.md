### Task 12: Drill-in routes — a run, a release's scans, why an environment is down, two live streams

**Half:** 7a. **Hours:** 2.5. **Depends on:** 03, 08.

What the lens opens when the user drills into one thing. Every route here is **on demand** — the
watcher never calls them — and the two streams are opened for **one thing at a time** (plan 7P-14),
because two live connections per visible environment would break the one-heartbeat spirit
(survey §2.3).

**Files:**
- `control-plane/internal/deploy/panacea/stream.go` (new) — `Client.Stream`: parses Panacea's `data: <json>` frames into `deploy.StreamEvent`
- `control-plane/internal/deploy/panacea/stream_test.go` (new)
- `control-plane/internal/api/deploy_drill.go` (new) — seven routes (five reads, two streams) and the relay
- `control-plane/internal/api/deploy_drill_test.go` (new)
- `control-plane/internal/api/routes_audit_test.go` (modified) — seven `GET` lines
- `control-plane/internal/api/api.go` (modified) — `a.registerDeployDrill(guarded)` in `New`; no new `Deps` field: the handlers read `Deps.DeployConnection` (task 05, whose `Current()` is the `deploy.Reader`, `Stream` included) and `Deps.DeployBinding` (task 06, for the Panacea project)

#### Panacea's two stream formats — what the connector parses

Both write `data: <one JSON object>\n\n` and flush (`handlers/readiness_stream.go:195`,
`handlers/logs_stream.go:118-141`).

```go
// readinessFrameDTO mirrors handlers.readinessEvent (handlers/readiness_stream.go:67-82).
type readinessFrameDTO struct {
	Environment       string `json:"environment"`
	LiveReadyReplicas *int   `json:"liveReadyReplicas"`
	Measured          bool   `json:"measured"`
	CrashReason       string `json:"crashReason"`
	RestartCount      *int   `json:"restartCount"`
	OOMKilled         bool   `json:"oomKilled"`
	ExitCode          *int   `json:"exitCode"`
}

// logFrameDTO mirrors the pipeline-log frames (handlers/logs_stream.go:48-49, :123-141).
// A frame with "error" and no "task" is the in-stream refusal of :118.
type logFrameDTO struct {
	Task  string `json:"task"`
	Line  string `json:"line"`
	Error string `json:"error"`
}
```

`Client.Stream` opens `GET P/pipelines/{run}/logs/stream` (`routes.go:2283`) or `GET
P/environments/{env}/readiness/stream` (`:2286`) with the bearer and **no per-attempt timeout** (a
stream is long), then an idle timeout: 60 s without a byte closes it with `KindUnavailable`. A frame
that does not parse is skipped and counted; ten in a row end the stream as `KindUnavailable` (the
format moved). `Measured` is authoritative: `measured:false` → `Ready: nil` whatever the count says.

#### Control-plane routes — complete wire

`{P}` is `/v1/projects/{projectId}/deploy`. Environment and run names are Panacea's, `[a-z0-9.-]`,
safe as path segments; each is validated against that set before use (else `400 bad_request`).

```
GET {P}/runs/{run}
  200 RunView                         (task 08's shape; a live read, for a run the watcher is not tracking)
  404 deploy_not_found

GET {P}/scans?environment=feat-x&deploymentId=engagement-portal-build-x7k2p
  — environment is REQUIRED as a key (absent → 400 bad_request "environment is required"), and its
    value is the deployment row's "environment" verbatim, validated against [a-z0-9.-]*. It is
    passed to Scans(ctx, p, environment, deploymentId) unchanged: Panacea reads a missing env as
    production (review C3). An empty value is passed as empty; see the note below.
  200 { "deploymentId": "…", "environment": "feat-x",
        "overall": "FAIL" | "INCOMPLETE" | "WARN" | "PASS" | "SKIPPED",
        "blocking":   [ { "scan": "trivy", "reason": "…" } ],
        "incomplete": [ { "scan": "dast",  "reason": "…" } ],
        "scans": [ { "name": "trivy", "status": "FAIL", "mode": "enforce", "counts": { "critical": 2 },
                     "summary": "…", "skipReason": "", "errorReason": "", "blocking": true,
                     "lastScanAt": "…", "findingsSource": "live" } ],
        "detailsRetained": true | false | null,
        "detailsSource": "live" | "snapshot" | "mixed" | "unverified" | "none" | null,
        "updatedAt": "…" }
  — "mode" is shown, never changed: Ship sends no override (brief B2).
  — An empty "environment" is legal at the route because a Panacea row without the
    panacea.io/environment label reads back with "" (clients/kubernetes.go:5681, :5751); Panacea
    then answers with production's report, as its own portal does (SecurityScanReview.tsx:437-440).
    The route stays a thin pass-through, but the desktop never sends "" (task 17 ScanList, re-review
    b03 M-1): it would draw production's verdict as the release's own. Settled from source that ""
    is legacy-only: every current writer sets a non-empty environment label — environmentForBranch
    answers "prod", the sanitised name or "dev" (clients/pipeline.go:2195-2253); the webhook's
    TargetEnvironment answers "preview", "prod" or branchEnvName with a "dev" fallback
    (webhook-receiver/models/event.go:116-148); advisory runs fall back to "unknown"
    (pipeline.go:842). A production row carries "prod", never "".

GET {P}/environments/{env}/pods
  200 { "pods": [ { "name": "…", "phase": "Running", "ready": false, "restarts": 7,
                    "reason": "", "message": "" } ] }

GET {P}/environments/{env}/events
  200 { "events": [ { "type": "Warning", "reason": "BackOff", "object": "Pod/app-7c9d…",
                      "message": "…", "lastSeen": "…" | null, "count": 12 } ] }

GET {P}/environments/{env}/diagnosis
  200 { "health": "down", "summary": "…", "source": "rules" | "rules+ai",
        "findings": [ { "problem": "…", "likelyCause": "…", "suggestedFix": "…", "evidence": ["…"],
                        "severity": "critical", "owner": "app" } ] }

GET {P}/runs/{run}/logs/stream
GET {P}/environments/{env}/readiness/stream
  200 text/event-stream; Cache-Control: no-store; X-Accel-Buffering: no
      OUR frames, never Panacea's:
        event: log        data: {"step":"build-kaniko","line":"…"}
        event: readiness  data: {"measured":true,"ready":0,"crashReason":"CrashLoopBackOff","restarts":7,"oomKilled":false,"exitCode":1}
        event: problem    data: {"message":"pipeline run not found: …"}
        event: end        data: {}
  409 conflict        — never; a second stream REPLACES the first (see below)
```

**One stream at a time.** The control plane holds at most one open relay. Opening a second closes
the first, which receives `event: end`. That is the rule, not a limit to tune: the lens drills into
one thing. The relay also closes on client disconnect (`r.Context().Done()`), after 30 minutes, and
when the connection's state leaves `connected`.

**Why the desktop can read these at all.** `EventSource` cannot send headers, and Hub task C3's local
caller key must be on every request (Hub contract §9.4). The desktop reads the stream with
`fetch` and a `ReadableStream` reader (task 17), so the origin middleware's CORS grant and the key
both apply as for any `GET`.

#### Tests to write first

| Test | Assertion | Mutant it kills |
|---|---|---|
| `TestReadinessFrameMeasuredWins` | `{"measured":false,"liveReadyReplicas":0}` → `Ready == nil` | an unmeasured tick drawn as "down" |
| `TestReadinessCrashReason` | a measured-down frame with `crashReason` → carried | dropping the one line that says why |
| `TestLogErrorFrame` | the `:118` frame → `Problem` set, stream ends | an error drawn as a log line |
| `TestUnparseableFramesEnd` | ten garbage frames → `KindUnavailable` | a stream that spins on a format change |
| `TestIdleTimeout` | no bytes for the idle window (injected 50 ms) → closed | a relay held open forever by a stalled upstream |
| `TestRelayEmitsOurFrames` | the route's body contains `event: log` and **no** `"task":` key | Panacea's shape reaching the desktop — the lift would then change the desktop |
| `TestSecondStreamReplacesFirst` | open A, open B → A receives `event: end` and closes; B streams | two live relays at once |
| `TestStreamClosesOnDisconnect` | the client cancels → the upstream request's context is cancelled | a leaked Panacea connection per drill-in |
| `TestScansShowModeUnchanged` | the response has `"mode":"enforce"` exactly as the fake returned | a mode rewritten for display |
| `TestScansPassTheEnvironment` | `GET {P}/scans?environment=feat-x&deploymentId=d-feat` → the fake's `Calls()` holds `Scans(engagement-portal,feat-x,d-feat)` and the body's `environment` is `feat-x`; the same request without the `environment` key → `400 bad_request` and the fake saw nothing (review C3) | dropping `env` between the route and the connector, which shows production's verdict as the branch release's |
| `TestEnvAndRunValidated` | `/environments/..%2Fetc/pods` and `/runs/a%20b` → 400; the fake saw nothing | a path-traversal string forwarded to Panacea |
| `TestDiagnosisSourceCarried` | `source: "rules+ai"` reaches the response | hiding that a model wrote the "likely cause" |

`catalogReadOnlyRoutes` lines: the seven `GET`s (t12:56-101), each *"On-demand relay of one Panacea read through
the connector; writes nothing."* The two stream lines add: *"Holds one upstream connection, which a
second stream replaces; still no write."*

#### Steps

- [ ] Tests first; `stream.go`; then `deploy_drill.go`; register through `routes`, in the same step
  that defines `registerDeployDrill`.
- [ ] Flush after each frame through `http.NewResponseController(w).Flush()`; check the phase 2
  origin middleware does not buffer (it wraps with headers only, `origin.go:183`ff) — if it
  does, stop and record it.
- [ ] Run; commit by pathspec.

#### Acceptance

```
go test ./control-plane/internal/deploy/panacea/ ./control-plane/internal/api/ -race -count=1
→ ok
```

#### What this task does NOT claim

- That Panacea's streams behave behind KPMG's proxy (buffering by an intermediary would batch the
  frames). Task 20's live check opens one of each.
- Application-log streaming (`GET P/logs/stream`, `routes.go:1109`). Not in the brief's drill-in;
  the diagnosis, pods and events answer "why is it broken" first.
