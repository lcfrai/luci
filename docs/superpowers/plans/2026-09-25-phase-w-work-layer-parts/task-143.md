### Task 143: `hubapi` and `hubclient.WorkIntake`: the W5 wire types and methods

**Slice:** W5. **Size:** 3 focused hours (an estimate, not a measurement). **Stream:** WH.
**dependsOn:** 97 (W2's proof), and through it tasks 02-04 (the Work ids, `work.go`,
`hubclient/work.go`), 15 (SW.6's import) and 114 (W4's wire, the model this part follows); a
sync-merge of `feat/hub` into `feat/work-layer` first (rule H1: additive only). **Seam:** defines
**SW.21**, the W5 wire: every W5 record, request body, code and the `WorkIntake` method set,
consumed by tasks 144-173. After this task SW.21 grows only by added fields, methods or routes.

W5 has twelve route tasks (146-167) and five desktop tasks (169-173). Nine of them were written
before this part and each stated, complete, the wire it needs (the table under decision 1). This
part **collects** those blocks into one file, writes the types no part has written, rules the
conflicts between them, and fixes the method set, the codes and the goldens once, so the fake
(144), the migration (145) and the HTTP client (168) chase one shape.

**Anchors, re-read on 2026-09-26 in the Hub worktree at `feat/hub` `3df9fab8`** (under
`control-plane/internal/hub/`): `hubapi/codes.go` (67 lines; the codes as constants with their
status in a trailing comment, `:20-24` the style; no status map), `hubapi/keys.go`, `types.go`
(`User`, `Page`), and the contract `docs/superpowers/contracts/2026-09-23-hub-contract.md`: `:103-112`
(the ten Hub prefixes and the control plane's five), `:2083-2085` (*"No Microsoft Graph, no client
secret, no certificate, no storage account, no Key Vault access in v1"*).

**Read in this branch's parts, not yet code, so UNVERIFIED until merge:** task 02's prefix block
(`task-02.md:138-163`: prefixes without a dash, `PrefixRequest = "rq"`, `PrefixLoop = "lp"`,
`WorkPrefixes`), task 03's import sources (`task-03.md:242-243`), task 15's `ImportSources`
(`task-15.md:175-177`: *"W5 (task 150) appends"*), task 04's `Work` interface (`task-04.md:153-233`)
and task 114's embed pattern and stubs (`task-114.md:35-75`, `:798-956`).

#### Decisions the scope left open, made here

1. **The route owner's Go wins, and is taken as written** (task 169's own rule, `task-169.md:18-24`).
   Six parts already hold complete Go for `work_intake.go`; this part places each block in the file,
   in task order, and writes nothing over it:

   | Block | Owner | Source in this plan |
   |---|---|---|
   | requests: `Request`, `Requester`, `RequestChannel`, `RequestState`, `RuleHit`, `IntakeRoute`, the four bodies, `RequestEraseResponse`, `IssueRequester`, `ValidateRequester`, `ValidateQuote` | 146 | `task-146.md:188-367` |
   | intake rules: `IntakeMatch`, `IntakeRule`, `IntakeRuleSet`, `PutIntakeRulesRequest`, `IntakeDefaults` | 147 | `task-147.md:114-172`, validation `:288-…` |
   | forms: `FormFieldKind`, `FormField`, `IntakeForm`, `CreateFormRequest`, `PatchFormRequest`, `ValidateFormFields`, `CheckAnswers` | 148 | `task-148.md:117-198`, `:207-…` |
   | external comments: `ExternalComment`, `CommentExternal` | 153 | `task-153.md:100-…` |
   | MCP connectors: `MCPConnector`, keyed by URL | 161 | `task-161.md:389-426` |
   | the event kinds and data of each | 146-148, 153 | each block's `workevents.go` additions |

2. **Two lines of those blocks are deleted, not kept.** `const PrefixRequest = "rq-"`
   (`task-146.md:191`) redeclares task 02's `PrefixRequest = "rq"` and would not compile;
   `const PrefixForm = "fm-"` (`task-148.md:120`) breaks task 02's no-dash style. Both prefixes are
   declared once, in `workids.go`, below.
3. **The request channels are task 146's**: `member`, `form`, `email`, `teams`. The scope's
   `tracker` is not a channel: tracker sync creates **issues** in `triage` through SW.6 (task 150),
   never requests, so a `tracker` request would have no writer.
4. **Loops follow tasks 162-163, not the scope's flat `Loop`.** A loop is a head (`Loop`) and
   append-only versions (`LoopVersion`, carrying `LoopTrigger`, the instruction and `LoopBound`);
   runs are keyed by `(loopId, run)` with no prefix (task 163's scope). So `lr-` is **not** added.
   The Go below mirrors task 169's TS (`task-169.md:195-244`) field for field.
5. **`HarnessAllow` plus `TeamSkill` become one `TeamAgentPolicy`** (task 166's name, mirrored at
   `task-169.md:310-318`); the three scope methods become `GetTeamAgentPolicy` and
   `PutTeamAgentPolicy`.
6. **Prefixes added: nine**, in task 02's style: `ir` rule, `fm` form, `tc` tracker connection,
   `xl` external link, `hl` outside-PR or commit link, `wh` webhook, `wd` delivery, `oa` app, `ak`
   key. `mc` is not added (task 161 keys a connector by its URL) and `lr` is not (decision 4).
7. **Three additive proposals from later parts are accepted here**: `UpstreamChange.To`, display
   text only (`task-151.md:1053-1056`); `SyncError.RetryAt` (`task-152.md:733-736`); the code
   `webhook_not_found` (`task-156.md:703`). The tracker run-level codes (`tracker_throttled` and
   the others, `task-151.md:1059-1061`) stay `worksync` constants and never become HTTP codes.
8. **The tracker target is raw JSON on the record, typed on the way in** (`task-151.md:1057-1058`):
   `TrackerConnection.Target` is `json.RawMessage`, and `ParseTrackerTarget` decodes it into one of
   three typed variants by `source`, the union task 169 mirrors (`task-169.md:161-164`).
9. **The codes carry a status map**, as task 114's decision 3: `WorkIntakeCodeStatus` in
   `work_intake.go`, because `codes.go` has none (checked at `3df9fab8`).
10. **Stubs keep `var _ Hub = (*Fake)(nil)` compiling**, as task 114's decision 2:
    `fake_work_intake_stub.go` (task 144 deletes it) and `client_work_intake_stub.go` (task 168
    deletes it), every method answering `workNotInFakeYet` or `ErrNotConfigured`.

#### Files

| Path (under `control-plane/internal/hub/`) | Change |
|---|---|
| `hubapi/work_intake.go` | new: the adopted blocks (decision 1) and every type below, the codes' status map, `Validate` |
| `hubapi/work_intake_test.go` | new: goldens, validation, the no-prediction and no-secret reflection tests, codes |
| `hubapi/testdata/work-intake/*.json` | new: one golden per record (twenty-four files, listed in *Tests first*) |
| `hubapi/workids.go` ⚑ (task 02) | appended: nine prefixes and `WorkIntakePrefixes` |
| `hubapi/work.go` ⚑ (task 03) | appended: three import sources |
| `hubapi/codes.go` ⚑ | appended: eleven W5 codes, in the file's style |
| `hubclient/work.go` ⚑ (tasks 04, 72, 99, 114) | `Work` embeds `WorkIntake`; the interface, complete |
| `hubclient/fake_work_intake_stub.go` | new: `*Fake` satisfies `WorkIntake` with 501s (task 144 deletes it) |
| `hubclient/client_work_intake_stub.go` | new: the HTTP client's 501s (task 168 deletes it) |
| `hubclient/work_intake_test.go` | new: the compile-time embed assertions |

#### `hubapi/workids.go` ⚑, appended (complete)

```go
// W5 prefixes (task 143, decision 6). Proposed here as additions to 05 §3.6's
// list, as task 99 did for gd-. Checked free on 2026-09-26 against the Hub's
// ten (hub-contract :103-112), the control plane's s p b w m, Phase T's twelve,
// WorkPrefixes above and task 99's gd; TestWorkPrefixesAreDisjoint re-checks.
const (
	PrefixIntakeRule    = "ir"
	PrefixForm          = "fm"
	PrefixTracker       = "tc"
	PrefixExternalLink  = "xl"
	PrefixHostLink      = "hl" // an outside PR link or an outside commit link (task 154)
	PrefixWebhook       = "wh"
	PrefixDelivery      = "wd"
	PrefixApp           = "oa"
	PrefixAPIKey        = "ak"
)

// WorkIntakePrefixes is every W5 prefix above, in declaration order.
var WorkIntakePrefixes = []string{PrefixIntakeRule, PrefixForm, PrefixTracker,
	PrefixExternalLink, PrefixHostLink, PrefixWebhook, PrefixDelivery, PrefixApp, PrefixAPIKey}
```

`rq` and `lp` are task 02's already. Task 02's `TestWorkPrefixesAreDisjoint` gains
`WorkIntakePrefixes` in its union (one line), so a W5 prefix that collides with any list is caught.

#### `hubapi/work.go` ⚑, appended (complete)

```go
// W5's import sources (task 143; task 150's engine writes through SW.6 with
// these). Task 15 fixed linear and csv and named these three as W5's.
const (
	ImportSourceJira        = "jira"
	ImportSourceAzureBoards = "azure_boards"
	ImportSourceGitHub      = "github"
)

func init() {
	ImportSources = append(ImportSources, ImportSourceJira, ImportSourceAzureBoards, ImportSourceGitHub)
}
```

`ImportSources` is task 15's `var` (`task-15.md:177`), so appending in `init` keeps task 15's
declaration untouched and its validator unchanged: `ValidateImportBatch` reads the slice.
`TestImportBatchAcceptsW5Sources` below holds that `ImportBatch{Source: "jira"}` passes.

#### `hubapi/work_intake.go`, complete: the header, actors and trackers

```go
package hubapi

import (
	"encoding/json"
	"fmt"
	"net/mail"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

// ActorKind names who wrote a Work record (task 167). The Hub's Postgres
// setting hub.work_actor_kind takes these seven words and no other (task 145).
type ActorKind string

const (
	ActorPerson      ActorKind = "person"
	ActorAgent       ActorKind = "agent"       // a person's agent: OwnerID and AgentID set
	ActorJanitor     ActorKind = "janitor"     // the Hub's own sweep; no user
	ActorIntegration ActorKind = "integration" // a tracker connection's principal (task 150)
	ActorApp         ActorKind = "app"         // an approved app acting for UserID (task 159)
	ActorKey         ActorKind = "key"         // an API key acting for UserID (task 160)
	ActorHost        ActorKind = "host"        // an outside host's webhook, mapped to UserID (task 154)
)

var ActorKinds = []ActorKind{ActorPerson, ActorAgent, ActorJanitor, ActorIntegration, ActorApp, ActorKey, ActorHost}

// Actor is ids only: no name, no email (task 156 sends it to third parties).
type Actor struct {
	Kind    ActorKind `json:"kind"`
	UserID  string    `json:"userId,omitempty"`  // the person the write is attributed to
	OwnerID string    `json:"ownerId,omitempty"` // the agent's owner
	AgentID string    `json:"agentId,omitempty"`
	AppID   string    `json:"appId,omitempty"`   // oa-…
	KeyID   string    `json:"keyId,omitempty"`   // ak-…
}

// ── Tracker sync (tasks 150-153) ───────────────────────────────────────────

// TrackerMode never names a state: sync never writes a state, a fact or done
// (05 §5.4 W5). "in" is one way into triage; "in_and_comments" adds comments.
type TrackerMode string

const (
	TrackerIn            TrackerMode = "in"
	TrackerInAndComments TrackerMode = "in_and_comments"
)

type TrackerState string

const (
	TrackerActive       TrackerState = "active"
	TrackerDisconnected TrackerState = "disconnected"
)

// TrackerConnection binds one outside project to one Hub project (task 150).
type TrackerConnection struct {
	ID          string          `json:"id"` // tc-…
	TeamID      string          `json:"teamId"`
	ProjectID   string          `json:"projectId"`
	Source      string          `json:"source"` // ImportSourceJira | ImportSourceAzureBoards | ImportSourceGitHub
	Target      json.RawMessage `json:"target"` // one of the three below, by Source (decision 8)
	Mode        TrackerMode     `json:"mode"`
	PrincipalID string          `json:"principalId"` // the integration principal; never a token
	State       TrackerState    `json:"state"`
	LastRunAt   *string         `json:"lastRunAt"`
	CreatedBy   User            `json:"createdBy"`
}

type AzureBoardsTarget struct {
	Source       string  `json:"source"` // "azure_boards"
	Organisation string  `json:"organisation"`
	Project      string  `json:"project"`
	AreaPath     *string `json:"areaPath"`
}

type GitHubTarget struct {
	Source     string `json:"source"` // "github"
	Owner      string `json:"owner"`
	Repository string `json:"repository"`
}

type JiraTarget struct {
	Source     string `json:"source"` // "jira"
	Site       string `json:"site"`   // https://<name>.atlassian.net
	ProjectKey string `json:"projectKey"`
}

// UpstreamChange is an outside edit, shown in the sync log and never applied.
// To is the tracker's own word ("Closed"), display text only (decision 7).
type UpstreamChange struct {
	ExternalID string `json:"externalId"`
	Field      string `json:"field"`
	At         string `json:"at"`
	To         string `json:"to"`
}

// ExternalLink is unique on (projectId, source, externalId): SW.6's idempotency key.
type ExternalLink struct {
	ID         string          `json:"id"` // xl-…
	Source     string          `json:"source"`
	ExternalID string          `json:"externalId"`
	IssueID    string          `json:"issueId"`
	URL        string          `json:"url"`
	LastSeenAt string          `json:"lastSeenAt"`
	Upstream   *UpstreamChange `json:"upstream"`
}

// SyncError names why one outside item was refused; RetryAt is the tracker's
// own reset time when it gave one (decision 7), never an estimate.
type SyncError struct {
	ExternalID string  `json:"externalId"`
	Code       string  `json:"code"`
	RetryAt    *string `json:"retryAt"`
}

// SyncRun is one engine pass. Counts are facts of that pass, not a total to reach.
type SyncRun struct {
	ConnectionID string      `json:"connectionId"`
	At           string      `json:"at"`
	Created      int         `json:"created"`
	Existing     int         `json:"existing"`
	CommentsIn   int         `json:"commentsIn"`
	CommentsOut  int         `json:"commentsOut"`
	Errors       []SyncError `json:"errors"` // never null
}

type ConnectTrackerRequest struct {
	Target json.RawMessage `json:"target"`
	Mode   TrackerMode     `json:"mode"`
}

// SyncBatch is what the integration principal sends in one pass (task 150).
// Items go through SW.6 (ImportItem, task 03) into triage; Comments only in
// mode in_and_comments (task 153); Changes are logged, never applied.
type SyncBatch struct {
	Items    []ImportItem      `json:"items"`
	Comments []ExternalComment `json:"comments"`
	Changes  []UpstreamChange  `json:"changes"`
}

// ── Outside PRs and commits (tasks 154-155) ────────────────────────────────

type HostKind string

const (
	HostAzureDevOps HostKind = "azure_devops"
	HostGitHub      HostKind = "github"
)

// LinkWord is how the link was made: a closing word, a non-closing mention,
// or the branch name (ep-142-slug).
type LinkWord string

const (
	WordClosing    LinkWord = "closing"
	WordNonClosing LinkWord = "non_closing"
	WordBranch     LinkWord = "branch"
)

type OutsidePRLink struct {
	ID          string   `json:"id"` // hl-…
	Host        HostKind `json:"host"`
	Repo        string   `json:"repo"`
	Number      int      `json:"number"`
	URL         string   `json:"url"`
	PRState     string   `json:"prState"` // "open" | "merged" | "closed"
	IssueID     string   `json:"issueId"`
	Word        LinkWord `json:"word"`
	MergeCommit *string  `json:"mergeCommit"`
}

// OutsideCommitLink is unique on (repo, sha, issueId); it shares the hl- prefix.
type OutsideCommitLink struct {
	ID       string   `json:"id"` // hl-…
	IssueID  string   `json:"issueId"`
	Host     HostKind `json:"host"`
	Repo     string   `json:"repo"`
	SHA      string   `json:"sha"` // 40 hex
	URL      string   `json:"url"`
	Word     LinkWord `json:"word"`
	PushedBy *string  `json:"pushedBy"` // the mapped u- id, or null when none maps
}

// HostHook registers one inbound host webhook for a project (task 154). The
// secret is returned once, in CreatedHostHook, and stored only by reference.
type HostHook struct {
	ProjectID     string   `json:"projectId"`
	Host          HostKind `json:"host"`
	RemoteURL     string   `json:"remoteUrl"`
	SecretVersion int      `json:"secretVersion"`
	CreatedBy     User     `json:"createdBy"`
}

type CreatedHostHook struct {
	Hook   HostHook `json:"hook"`
	Secret string   `json:"secret"`
}

// ── Outbound webhooks (task 156) ───────────────────────────────────────────

type Webhook struct {
	ID            string  `json:"id"` // wh-…
	TeamID        string  `json:"teamId"`
	ProjectID     *string `json:"projectId"`
	URL           string  `json:"url"`   // https only
	Kinds         []string `json:"kinds"` // SW.5 kinds only (task 156's WebhookKinds)
	State         string  `json:"state"` // "active" | "disabled"
	CreatedBy     User    `json:"createdBy"`
	SecretVersion int     `json:"secretVersion"`
}

type CreatedWebhook struct {
	Webhook Webhook `json:"webhook"`
	Secret  string  `json:"secret"`
}

type WebhookDelivery struct {
	ID            string  `json:"id"` // wd-…
	WebhookID     string  `json:"webhookId"`
	EventSeq      string  `json:"eventSeq"`
	Attempt       int     `json:"attempt"`
	Status        string  `json:"status"` // "ok" | "failed" | "pending"
	ResponseCode  *int    `json:"responseCode"`
	At            string  `json:"at"`
	NextAttemptAt *string `json:"nextAttemptAt"` // a scheduled retry time, not a forecast
}

type CreateWebhookRequest struct {
	ProjectID *string  `json:"projectId"`
	URL       string   `json:"url"`
	Kinds     []string `json:"kinds"`
}

// ── Teams (tasks 157, 165) and the request thread ──────────────────────────

// RequestMessage is one reply in a request's outside thread, append-only.
// Body is the external author's words, text·H.
type RequestMessage struct {
	RequestID  string  `json:"requestId"`
	ChannelRef string  `json:"channelRef"`
	AuthorName string  `json:"authorName"`
	UserID     *string `json:"userId"`
	Body       string  `json:"body"`
	At         string  `json:"at"`
}

type TeamsSubject struct {
	Kind string  `json:"kind"` // "intake" | "feature" | "view" | "digest"
	ID   *string `json:"id"`   // ft-… or vw-…; null for intake and digest
}

type TeamsBinding struct {
	TeamID     string       `json:"teamId"`
	Subject    TeamsSubject `json:"subject"`
	ChannelRef string       `json:"channelRef"`
}

// ── Apps and keys (tasks 159-160) ──────────────────────────────────────────

type AppManifest struct {
	Name          string   `json:"name"`
	EntraClientID string   `json:"entraClientId"`
	Publisher     string   `json:"publisher"`
	Contact       string   `json:"contact"`
	Scopes        []string `json:"scopes"`       // "work.read" | "work.write"
	RedirectURIs  []string `json:"redirectUris"` // https only
	WebhookURL    *string  `json:"webhookUrl"`
}

// App acts as a person, never alone: every call is its user's (task 159).
type App struct {
	ID           string      `json:"id"` // oa-…
	Manifest     AppManifest `json:"manifest"`
	RegisteredBy User        `json:"registeredBy"`
	State        string      `json:"state"` // "pending" | "approved" | "revoked"
	DecidedBy    *User       `json:"decidedBy"`
	DecidedAt    *string     `json:"decidedAt"`
	Reason       *string     `json:"reason"`
}

type DecideAppRequest struct {
	Approve bool   `json:"approve"`
	Reason  string `json:"reason"`
}

// APIKey holds no secret. Shown is the first 8 characters only.
type APIKey struct {
	ID         string  `json:"id"`    // ak-…
	Scope      string  `json:"scope"` // "personal" | "team"
	OwnerID    string  `json:"ownerId"`
	TeamID     *string `json:"teamId"`
	Name       string  `json:"name"`
	Shown      string  `json:"shown"`
	CreatedAt  string  `json:"createdAt"`
	ExpiresAt  string  `json:"expiresAt"` // required, at most 365 days on
	LastUsedAt *string `json:"lastUsedAt"`
	RevokedAt  *string `json:"revokedAt"`
	RevokedBy  *User   `json:"revokedBy"`
}

// CreatedAPIKey, CreatedWebhook and CreatedHostHook are the only W5 types that
// carry a secret, each returned once (TestOnlyCreatedTypesCarryASecret).
type CreatedAPIKey struct {
	Key    APIKey `json:"key"`
	Secret string `json:"secret"`
}

type CreateAPIKeyRequest struct {
	Name      string `json:"name"`
	ExpiresAt string `json:"expiresAt"`
}

const MaxKeyLifetime = 365 * 24 * time.Hour

// ── Loops (tasks 162-163, decision 4) ──────────────────────────────────────

// LoopBound: TurnsPerDay 1-24; a class absent from Levels inherits the owner's
// own grid. Levels are AX §7's words (ask_first … full) by action class.
type LoopBound struct {
	TurnsPerDay int               `json:"turnsPerDay"`
	Levels      map[string]string `json:"levels"`
}

type LoopFilter struct {
	LabelID   *string `json:"labelId"`
	FromState *string `json:"fromState"`
	ToState   *string `json:"toState"`
}

type LoopSchedule struct {
	Every string `json:"every"` // "day" | "weekday" | "week"
	At    string `json:"at"`    // HH:MM
	TZ    string `json:"tz"`    // IANA
}

type LoopTrigger struct {
	Kind       string        `json:"kind"`       // "event" | "schedule"
	EventKinds []string      `json:"eventKinds"` // [] for a schedule
	Filter     LoopFilter    `json:"filter"`
	Schedule   *LoopSchedule `json:"schedule"` // null for an event
}

// Loop is the head; its text lives in its versions.
type Loop struct {
	ID             string  `json:"id"` // lp-…
	TeamID         string  `json:"teamId"`
	ProjectID      string  `json:"projectId"`
	Owner          User    `json:"owner"`
	Name           string  `json:"name"`  // 1-80 characters
	State          string  `json:"state"` // "on" | "paused"
	PausedCause    *string `json:"pausedCause"`
	CurrentVersion int     `json:"currentVersion"`
}

// LoopVersion is append-only: an edit is version n+1.
type LoopVersion struct {
	LoopID      string       `json:"loopId"`
	Version     int          `json:"version"`
	Trigger     LoopTrigger  `json:"trigger"`
	Instruction AuthoredText `json:"instruction"` // the owner's, author "H", at most 2000 characters
	Action      string       `json:"action"`      // "turn" | "start"
	LabelID     *string      `json:"labelId"`
	Bound       LoopBound    `json:"bound"`
	SavedAt     string       `json:"savedAt"`
	SavedBy     User         `json:"savedBy"`
}

type LoopRunTrigger struct {
	Kind         string  `json:"kind"` // "event" | "schedule" | "manual"
	EventSeq     *string `json:"eventSeq"`
	ScheduledFor *string `json:"scheduledFor"`
}

type StartedLane struct {
	IssueKey  string `json:"issueKey"`
	SessionID string `json:"sessionId"`
	Branch    string `json:"branch"`
	Route     string `json:"route"`
}

// LoopRun is keyed by (loopId, run): no id prefix (task 163).
type LoopRun struct {
	LoopID   string         `json:"loopId"`
	Run      int            `json:"run"`
	Version  int            `json:"version"`
	FiredAt  string         `json:"firedAt"`
	Trigger  LoopRunTrigger `json:"trigger"`
	Outcome  string         `json:"outcome"` // queued | ran | skipped_budget | skipped_paused | refused_bound | failed
	TurnID   *string        `json:"turnId"`  // ot-… (Phase T)
	Started  []StartedLane  `json:"started"` // never null
	EndedAt  *string        `json:"endedAt"`
}

// LoopVersionBody is one version's content; the Hub stamps the rest.
type LoopVersionBody struct {
	Trigger     LoopTrigger  `json:"trigger"`
	Instruction AuthoredText `json:"instruction"`
	Action      string       `json:"action"`
	LabelID     *string      `json:"labelId"`
	Bound       LoopBound    `json:"bound"`
}

type CreateLoopRequest struct {
	Name  string          `json:"name"`
	First LoopVersionBody `json:"first"`
}

// ── Harness and skills (task 166, decision 5) ──────────────────────────────

type SkillsLibrary struct {
	RepoKey string `json:"repoKey"`
	Path    string `json:"path"`
	Commit  string `json:"commit"` // 40 hex, never a branch
}

type TeamSkills struct {
	Source  string         `json:"source"` // "image" | "library"
	Library *SkillsLibrary `json:"library"`
}

// TeamAgentPolicy: Harnesses is ordered and non-empty; unset means ["claude-code"].
type TeamAgentPolicy struct {
	TeamID    string     `json:"teamId"`
	Harnesses []string   `json:"harnesses"` // harness.Catalogue() ids
	Skills    TeamSkills `json:"skills"`
	UpdatedBy *User      `json:"updatedBy"`
	UpdatedAt *string    `json:"updatedAt"`
}
```

The harness ids are `control-plane/internal/harness/harness.go`'s catalogue (the control plane's
own code, **built**); `TeamAgentPolicy` names them and never parses or changes a harness (`OD-8`).

#### `work_intake.go`, continued: validation (complete)

```go
// ── Validation. Each returns the first bad field; nothing is read from a store.

// badIntake: FieldError is {Field string; Err error} (canonical.go:282-286 at 3df9fab8).
func badIntake(field string) *FieldError { return &FieldError{field, ErrBadEnvelope} }

func ValidTrackerMode(m TrackerMode) bool { return m == TrackerIn || m == TrackerInAndComments }

// ParseTrackerTarget decodes raw by source; an unknown key or a source that
// disagrees with the target's own is refused (decision 8).
func ParseTrackerTarget(source string, raw json.RawMessage) (any, *FieldError) {
	dec := func(v any) *FieldError {
		d := json.NewDecoder(strings.NewReader(string(raw)))
		d.DisallowUnknownFields()
		if d.Decode(v) != nil {
			return badIntake("target")
		}
		return nil
	}
	switch source {
	case ImportSourceAzureBoards:
		var t AzureBoardsTarget
		if fe := dec(&t); fe != nil || t.Source != source || t.Organisation == "" || t.Project == "" {
			return nil, badIntake("target")
		}
		return t, nil
	case ImportSourceGitHub:
		var t GitHubTarget
		if fe := dec(&t); fe != nil || t.Source != source || t.Owner == "" || t.Repository == "" {
			return nil, badIntake("target")
		}
		return t, nil
	case ImportSourceJira:
		var t JiraTarget
		if fe := dec(&t); fe != nil || t.Source != source || !httpsURL(t.Site) || t.ProjectKey == "" {
			return nil, badIntake("target")
		}
		return t, nil
	}
	return nil, badIntake("source")
}

// httpsURL: https, a host, no user info, no fragment.
func httpsURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && u.Fragment == ""
}

// ValidateCreateWebhook: https only; kinds non-empty, distinct and each in
// allowed (task 156's hubapi.WebhookKinds()).
func ValidateCreateWebhook(r CreateWebhookRequest, allowed []string) *FieldError {
	if !httpsURL(r.URL) || len(r.URL) > 2048 {
		return &FieldError{"url", fmt.Errorf("%w: %s", ErrBadEnvelope, CodeWebhookURLInvalid)}
	}
	if len(r.Kinds) == 0 || !subsetDistinct(r.Kinds, allowed) {
		return badIntake("kinds")
	}
	return nil
}

func subsetDistinct(xs, allowed []string) bool {
	ok, seen := map[string]bool{}, map[string]bool{}
	for _, a := range allowed {
		ok[a] = true
	}
	for _, x := range xs {
		if !ok[x] || seen[x] {
			return false
		}
		seen[x] = true
	}
	return true
}

// ValidateCreateAPIKey: a name, and an expiry after now and at most 365 days on.
func ValidateCreateAPIKey(r CreateAPIKeyRequest, now time.Time) *FieldError {
	if n := utf8.RuneCountInString(strings.TrimSpace(r.Name)); n == 0 || n > 80 {
		return badIntake("name")
	}
	t, err := time.Parse(time.RFC3339, r.ExpiresAt)
	if err != nil || !t.After(now) || t.Sub(now) > MaxKeyLifetime {
		return badIntake("expiresAt")
	}
	return nil
}

// ValidateAppManifest: work.read always, work.write optionally; https redirects.
func ValidateAppManifest(m AppManifest) *FieldError {
	if strings.TrimSpace(m.Name) == "" || m.EntraClientID == "" {
		return badIntake("manifest")
	}
	if !subsetDistinct(m.Scopes, []string{"work.read", "work.write"}) || len(m.Scopes) == 0 || m.Scopes[0] != "work.read" {
		return badIntake("manifest.scopes")
	}
	for i, u := range m.RedirectURIs {
		if !httpsURL(u) {
			return badIntake(fmt.Sprintf("manifest.redirectUris[%d]", i))
		}
	}
	if m.WebhookURL != nil && !httpsURL(*m.WebhookURL) {
		return badIntake("manifest.webhookUrl")
	}
	if _, err := mail.ParseAddress(m.Contact); err != nil {
		return badIntake("manifest.contact")
	}
	return nil
}

var (
	autonomyLevels = map[string]bool{"ask_first": true, "propose_and_wait": true, "act_and_tell": true, "full": true}
	actionClasses  = map[string]bool{"read": true, "edit_own_worktree": true, "edit_outside_worktree": true,
		"run_tests": true, "install_dependency": true, "run_migration": true, "touch_infra": true,
		"commit": true, "push": true, "deploy": true}
)

// ValidateLoopVersionBody checks shape only; whether the bound fits inside the
// owner's own grid is the route's (loop_outside_grid, task 162).
func ValidateLoopVersionBody(b LoopVersionBody) *FieldError {
	t := b.Trigger
	switch {
	case t.Kind == "event" && (len(t.EventKinds) == 0 || t.Schedule != nil):
		return badIntake("trigger")
	case t.Kind == "schedule" && (len(t.EventKinds) != 0 || t.Schedule == nil):
		return badIntake("trigger")
	case t.Kind != "event" && t.Kind != "schedule":
		return badIntake("trigger.kind")
	}
	if s := t.Schedule; s != nil {
		if _, err := time.LoadLocation(s.TZ); err != nil || s.TZ == "" {
			return badIntake("trigger.schedule.tz")
		}
		if _, err := time.Parse("15:04", s.At); err != nil || (s.Every != "day" && s.Every != "weekday" && s.Every != "week") {
			return badIntake("trigger.schedule")
		}
	}
	if b.Instruction.Author != AuthorHuman || b.Instruction.Text == "" || utf8.RuneCountInString(b.Instruction.Text) > 2000 {
		return badIntake("instruction")
	}
	if b.Action != "turn" && b.Action != "start" {
		return badIntake("action")
	}
	if b.Bound.TurnsPerDay < 1 || b.Bound.TurnsPerDay > 24 {
		return badIntake("bound.turnsPerDay")
	}
	for class, level := range b.Bound.Levels {
		if !actionClasses[class] || !autonomyLevels[level] {
			return badIntake("bound.levels")
		}
	}
	return nil
}

// ValidateTeamAgentPolicy: harness ids from the catalogue, ordered, distinct;
// a library pinned to a 40-hex commit, never a branch.
func ValidateTeamAgentPolicy(p TeamAgentPolicy, catalogue []string) *FieldError {
	if len(p.Harnesses) == 0 || !subsetDistinct(p.Harnesses, catalogue) {
		return badIntake("harnesses")
	}
	switch p.Skills.Source {
	case "image":
		if p.Skills.Library != nil {
			return badIntake("skills.library")
		}
	case "library":
		l := p.Skills.Library
		if l == nil || l.RepoKey == "" || !hex40(l.Commit) {
			return badIntake("skills.library")
		}
	default:
		return badIntake("skills.source")
	}
	return nil
}

func hex40(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, c := range s {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}
```

`time.LoadLocation` needs the zone database; the Hub image carries it (**UNVERIFIED**: Phase H's
Dockerfile; if not, the file imports `time/tzdata`). The route passes `catalogue` from
`harness.Catalogue()`'s ids, so `hubapi` does not import the control plane's harness package.
