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
   | mail intake: `IntakeSettings`, `PutIntakeSettingsRequest`, `RecordReleaseIssuesRequest` (the scope's `IntakeAddress`) | 149 | `task-149.md:274-317` |
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

#### `hubapi/codes.go` ⚑, appended (complete)

```go
// W5 (task 143). Appended after W4's; each status in the file's comment style.
const (
	CodeRequestClosed            = "request_closed"              // 409, details.state
	CodeRequestErased            = "request_erased"              // 409 (task 146 decision 7)
	CodeFormFieldRequired        = "form_field_required"         // 400, details.field
	CodeTrackerExists            = "tracker_exists"              // 409
	CodePrincipalCannotMoveState = "principal_cannot_move_state" // 403, details.actorKind
	CodeWebhookURLInvalid        = "webhook_url_invalid"         // 422
	CodeWebhookNotFound          = "webhook_not_found"           // 404 (task 156 gap 2)
	CodeAppNotApproved           = "app_not_approved"            // 403
	CodeKeyRevoked               = "key_revoked"                 // 401
	CodeConnectorNotAllowed      = "connector_not_allowed"       // 403
	CodeHarnessNotAllowed        = "harness_not_allowed"         // 403, details.harness
	CodeLoopOutsideGrid          = "loop_outside_grid"           // 403, details.class
)
```

`connector_url_invalid` is task 161's, appended by its own block (`task-161.md:415`); task 153's
three codes are sync-run codes on `SyncError`, never an HTTP status (decision 7).

#### `work_intake.go`, continued: the status map (complete)

```go
// WorkIntakeCodeStatus is read by the fake (144) and the server (146-167) when
// they write a problem, so a code's status is written once (decision 9).
var WorkIntakeCodeStatus = map[string]int{
	CodeRequestClosed: 409, CodeRequestErased: 409, CodeFormFieldRequired: 400,
	CodeTrackerExists: 409, CodePrincipalCannotMoveState: 403, CodeWebhookURLInvalid: 422,
	CodeWebhookNotFound: 404, CodeAppNotApproved: 403, CodeKeyRevoked: 401,
	CodeConnectorNotAllowed: 403, CodeHarnessNotAllowed: 403, CodeLoopOutsideGrid: 403,
}
```

#### `hubclient/work.go` ⚑: `Work` embeds `WorkIntake` (complete)

```go
// Work is W1's WorkIssues, W2's WorkPlan, W3's WorkAgents, W4's WorkRoadmap
// and W5's WorkIntake (task 143; the first four as tasks 04, 73, 99 and 114
// leave it, task-114.md:800). Embedding keeps each slice's methods in one place.
type Work interface {
	WorkIssues
	WorkPlan
	WorkAgents
	WorkRoadmap
	WorkIntake
}

// WorkIntake is SW.21's method set: one method per route. Host webhook ingress
// is not here: the host calls the Hub (task 154). Route tasks may add methods.
type WorkIntake interface {
	// requests (146)
	CreateRequest(ctx context.Context, teamID string, req hubapi.CreateRequestRequest) (hubapi.Request, error)
	ListRequests(ctx context.Context, teamID string, page hubapi.PageRequest) (hubapi.Page[hubapi.Request], error)
	Request(ctx context.Context, requestID string) (hubapi.Request, error)
	AcceptRequest(ctx context.Context, requestID string, req hubapi.AcceptRequestRequest) (hubapi.AcceptRequestResponse, error)
	LinkRequest(ctx context.Context, requestID string, req hubapi.LinkRequestRequest) (hubapi.Request, error)
	DeclineRequest(ctx context.Context, requestID string, req hubapi.DeclineRequestRequest) (hubapi.Request, error)
	MarkImportant(ctx context.Context, requestID string, req hubapi.SetImportantRequest) (hubapi.Request, error)
	EraseRequest(ctx context.Context, requestID string, req hubapi.EraseRequest) (hubapi.RequestEraseResponse, error)
	IssueRequesters(ctx context.Context, issueID string) ([]hubapi.IssueRequester, error)
	RequestMessages(ctx context.Context, requestID string) ([]hubapi.RequestMessage, error)
	// rules, forms and the address (147-149)
	ListIntakeRules(ctx context.Context, teamID string) (hubapi.IntakeRuleSet, error)
	PutIntakeRules(ctx context.Context, teamID string, req hubapi.PutIntakeRulesRequest) (hubapi.IntakeRuleSet, error)
	CreateForm(ctx context.Context, teamID string, req hubapi.CreateFormRequest) (hubapi.IntakeForm, error)
	ListForms(ctx context.Context, teamID string) ([]hubapi.IntakeForm, error)
	GetForm(ctx context.Context, formID string) (hubapi.IntakeForm, error)
	PatchForm(ctx context.Context, formID string, req hubapi.PatchFormRequest) (hubapi.IntakeForm, error)
	IntakeSettings(ctx context.Context, teamID string) (hubapi.IntakeSettings, error)
	PutIntakeSettings(ctx context.Context, teamID string, req hubapi.PutIntakeSettingsRequest) (hubapi.IntakeSettings, error)
	RecordReleaseIssues(ctx context.Context, releaseID string, req hubapi.RecordReleaseIssuesRequest) error
	// tracker sync (150-153); ApplySyncBatch only as the connection's principal
	ConnectTracker(ctx context.Context, projectID string, req hubapi.ConnectTrackerRequest) (hubapi.TrackerConnection, error)
	ListTrackers(ctx context.Context, teamID string) ([]hubapi.TrackerConnection, error)
	DisconnectTracker(ctx context.Context, connectionID string) (hubapi.TrackerConnection, error)
	ApplySyncBatch(ctx context.Context, connectionID string, batch hubapi.SyncBatch) (hubapi.SyncRun, error)
	ListSyncRuns(ctx context.Context, connectionID string, page hubapi.PageRequest) (hubapi.Page[hubapi.SyncRun], error)
	// outside PRs (154-155)
	ListOutsidePRLinks(ctx context.Context, issueID string) ([]hubapi.OutsidePRLink, error)
	CreateHostHook(ctx context.Context, projectID string, host hubapi.HostKind) (hubapi.CreatedHostHook, error)
	// webhooks (156)
	CreateWebhook(ctx context.Context, teamID string, req hubapi.CreateWebhookRequest) (hubapi.CreatedWebhook, error)
	ListWebhooks(ctx context.Context, teamID string) ([]hubapi.Webhook, error)
	DisableWebhook(ctx context.Context, webhookID string) (hubapi.Webhook, error)
	RotateWebhookSecret(ctx context.Context, webhookID string) (hubapi.CreatedWebhook, error)
	ListDeliveries(ctx context.Context, webhookID string, page hubapi.PageRequest) (hubapi.Page[hubapi.WebhookDelivery], error)
	Redeliver(ctx context.Context, deliveryID string) (hubapi.WebhookDelivery, error)
	// Teams (157)
	BindTeams(ctx context.Context, teamID string, b hubapi.TeamsBinding) (hubapi.TeamsBinding, error)
	ListTeamsBindings(ctx context.Context, teamID string) ([]hubapi.TeamsBinding, error)
	UnbindTeams(ctx context.Context, teamID string, subject hubapi.TeamsSubject) error
	// apps and keys (159-160)
	RegisterApp(ctx context.Context, m hubapi.AppManifest) (hubapi.App, error)
	ListApps(ctx context.Context) ([]hubapi.App, error)
	DecideApp(ctx context.Context, appID string, req hubapi.DecideAppRequest) (hubapi.App, error)
	CreateAPIKey(ctx context.Context, teamID *string, req hubapi.CreateAPIKeyRequest) (hubapi.CreatedAPIKey, error)
	ListAPIKeys(ctx context.Context, teamID *string) ([]hubapi.APIKey, error)
	RevokeAPIKey(ctx context.Context, keyID string) (hubapi.APIKey, error)
	// MCP connectors (161; the types are task 161's block)
	ListMCPConnectors(ctx context.Context) ([]hubapi.MCPConnector, error)
	PutMCPConnector(ctx context.Context, req hubapi.PutConnectorRequest) (hubapi.MCPConnector, error)
	DeleteMCPConnector(ctx context.Context, url string) error
	// loops (162-163)
	CreateLoop(ctx context.Context, teamID string, req hubapi.CreateLoopRequest) (hubapi.Loop, error)
	PutLoopVersion(ctx context.Context, loopID string, body hubapi.LoopVersionBody) (hubapi.LoopVersion, error)
	ListLoopVersions(ctx context.Context, loopID string) ([]hubapi.LoopVersion, error)
	ListLoopRuns(ctx context.Context, loopID string, page hubapi.PageRequest) (hubapi.Page[hubapi.LoopRun], error)
	RunLoopNow(ctx context.Context, loopID string) (hubapi.LoopRun, error)
	// harness and skills (166)
	GetTeamAgentPolicy(ctx context.Context, teamID string) (hubapi.TeamAgentPolicy, error)
	PutTeamAgentPolicy(ctx context.Context, teamID string, p hubapi.TeamAgentPolicy) (hubapi.TeamAgentPolicy, error)
}
```

`CreateAPIKey` and `ListAPIKeys` take `teamID` as `nil` for a personal key and a `tm-` id for a
team key, so one method serves task 160's two routes. `PutConnectorRequest` is the Go name for task
169's `PutConnectorBody` (`task-169.md:349`); if task 161's block names it otherwise, 161 wins.
`PageRequest`, `Page[T]` and `EraseRequest` are Phase H's and task 04's (`types.go` at `3df9fab8`;
`task-04.md:229-232`). **Fifty-one methods**, counted in the block.

#### `hubclient/fake_work_intake_stub.go` and `client_work_intake_stub.go` (complete)

```go
package hubclient

// fake_work_intake_stub.go: *Fake satisfies WorkIntake until task 144 adds the
// real rules; task 144 deletes this file in the commit that adds them. Every
// method is one line of the same shape; two are shown and the other
// forty-nine follow the same pattern, in WorkIntake's order.
func (f *Fake) CreateRequest(context.Context, string, hubapi.CreateRequestRequest) (hubapi.Request, error) {
	return hubapi.Request{}, workNotInFakeYet
}
func (f *Fake) UnbindTeams(context.Context, string, hubapi.TeamsSubject) error { return workNotInFakeYet }
```

```go
package hubclient

// client_work_intake_stub.go: the HTTP client answers ErrNotConfigured for
// every WorkIntake method until task 168 builds them over HTTP and deletes
// this file (task 04's client idiom, task-04.md:138-147).
func (c *Client) CreateRequest(context.Context, string, hubapi.CreateRequestRequest) (hubapi.Request, error) {
	return hubapi.Request{}, ErrNotConfigured
}
func (c *Client) UnbindTeams(context.Context, string, hubapi.TeamsSubject) error { return ErrNotConfigured }
```

Both files import `context` and `…/internal/hub/hubapi` and hold one method per `WorkIntake`
method, written by hand from the interface as task 114's are (`task-114.md:902-956`); the
compile-time assertions in `hubclient/work_intake_test.go` are the check that none is missing:

```go
package hubclient

var (
	_ WorkIntake = (*Fake)(nil)
	_ WorkIntake = (*Client)(nil)
	_ Hub        = (*Fake)(nil)
)
```

#### The routes (SW.21), one per method; each route task registers its own

Where a route part exists, its path is taken from it (cited); the rest are **proposed** here for
their route task, which may rename them (the method set does not change).

| Methods | Routes | From |
|---|---|---|
| `CreateRequest`, `ListRequests`, `Request`, `AcceptRequest`, `LinkRequest`, `DeclineRequest`, `MarkImportant`, `EraseRequest`, `IssueRequesters` | `POST`/`GET /v1/teams/{teamId}/requests`, `GET /v1/requests/{requestId}`, `POST …/accept`, `…/link`, `…/decline`, `PUT …/important`, `POST …/erase`, `GET /v1/issues/{issueId}/requests` | `task-146.md` (routes table) |
| `RequestMessages` | `GET /v1/requests/{requestId}/messages` | proposed (157) |
| `ListIntakeRules`, `PutIntakeRules` | `GET`/`PUT /v1/teams/{teamId}/intake-rules` | `task-147.md` |
| `CreateForm`, `ListForms`, `GetForm`, `PatchForm` | `POST`/`GET /v1/teams/{teamId}/forms`, `GET`/`PATCH /v1/forms/{formId}` | `task-148.md` |
| `IntakeSettings`, `PutIntakeSettings`, `RecordReleaseIssues` | `GET`/`PUT /v1/teams/{teamId}/intake-address`, `PUT /v1/releases/{releaseId}/issues` | `task-149.md:447-449`, `:291` |
| `ConnectTracker`, `ListTrackers`, `DisconnectTracker`, `ApplySyncBatch`, `ListSyncRuns` | `POST /v1/projects/{projectId}/trackers`, `GET /v1/teams/{teamId}/trackers`, `POST /v1/trackers/{connectionId}/disconnect`, `POST …/batches`, `GET …/runs` | proposed (150) |
| `ListOutsidePRLinks`, `CreateHostHook` | `GET /v1/issues/{issueId}/outside-links`, `POST /v1/projects/{projectId}/host-hooks` | proposed (154) |
| `CreateWebhook` … `Redeliver` | `POST`/`GET /v1/teams/{teamId}/webhooks`, `POST /v1/webhooks/{webhookId}/disable`, `…/rotate`, `GET …/deliveries`, `POST /v1/webhook-deliveries/{deliveryId}/redeliver` | `task-156.md` |
| `BindTeams`, `ListTeamsBindings`, `UnbindTeams` | `PUT`/`GET /v1/teams/{teamId}/teams-bindings`, `POST …/teams-bindings/remove` | `task-157.md` |
| `RegisterApp`, `ListApps`, `DecideApp` | `POST`/`GET /v1/apps`, `POST /v1/apps/{appId}/decide` | proposed (159) |
| `CreateAPIKey`, `ListAPIKeys`, `RevokeAPIKey` | `POST`/`GET /v1/me/api-keys` and `/v1/teams/{teamId}/api-keys`, `POST /v1/api-keys/{keyId}/revoke` | proposed (160) |
| `ListMCPConnectors`, `PutMCPConnector`, `DeleteMCPConnector` | `GET`/`PUT /v1/work/mcp-connectors`, `DELETE …?url=` | `task-161.md:585-587` |
| `CreateLoop` … `RunLoopNow` | `POST /v1/teams/{teamId}/loops`, `POST /v1/loops/{loopId}/versions`, `GET …/versions`, `GET …/runs`, `POST …/run-now` | proposed (162-163) |
| `GetTeamAgentPolicy`, `PutTeamAgentPolicy` | `GET`/`PUT /v1/teams/{teamId}/agent-policy` | proposed (166) |

Every write is `POST`, `PUT`, `PATCH` or `DELETE`; no `GET` has a side effect (contract §10a). The
host's inbound webhook (`POST /v1/hooks/{host}/{projectId}`, task 154) has no method: the host
calls it, never `hubclient`.

#### Tests first — `hubapi/work_intake_test.go` and `hubclient/work_intake_test.go`

The goldens, one per record, under `hubapi/testdata/work-intake/`: `request`, `issue-requester`,
`request-message`, `intake-rule-set`, `intake-form`, `intake-settings`, `tracker-connection`
(one per source, three files), `external-link`, `sync-run`, `outside-pr-link`,
`outside-commit-link`, `host-hook`, `webhook`, `webhook-delivery`, `teams-binding`, `app`,
`api-key`, `mcp-connector`, `loop`, `loop-version`, `loop-run`, `team-agent-policy`: twenty-four.
Task 169's desktop golden test reads these files by path (`task-169.md:28-30`).

```go
package hubapi

func TestW5RecordsRoundTripTheirGoldens(t *testing.T) { // kills "field renamed or retagged"
	for name, v := range map[string]any{
		"request": &Request{}, "issue-requester": &IssueRequester{}, "request-message": &RequestMessage{},
		"intake-rule-set": &IntakeRuleSet{}, "intake-form": &IntakeForm{}, "intake-settings": &IntakeSettings{},
		"tracker-connection-azure_boards": &TrackerConnection{}, "tracker-connection-github": &TrackerConnection{},
		"tracker-connection-jira": &TrackerConnection{}, "external-link": &ExternalLink{}, "sync-run": &SyncRun{},
		"outside-pr-link": &OutsidePRLink{}, "outside-commit-link": &OutsideCommitLink{}, "host-hook": &HostHook{},
		"webhook": &Webhook{}, "webhook-delivery": &WebhookDelivery{}, "teams-binding": &TeamsBinding{},
		"app": &App{}, "api-key": &APIKey{}, "mcp-connector": &MCPConnector{}, "loop": &Loop{},
		"loop-version": &LoopVersion{}, "loop-run": &LoopRun{}, "team-agent-policy": &TeamAgentPolicy{},
	} {
		raw := mustRead(t, "testdata/work-intake/"+name+".json")
		mustUnmarshalStrict(t, raw, v) // DisallowUnknownFields
		if got := mustMarshal(t, v); !bytes.Equal(got, bytes.TrimSpace(raw)) {
			t.Fatalf("%s: round trip differs:\n%s\n%s", name, got, raw)
		}
	}
}
```

`IntakeSettings` is task 149's name for the scope's `IntakeAddress` (`task-149.md:277-284`); the
scope's `formId` on the address is not in it, so a mail request takes no form's defaults (gap 4).

```go
func TestAnAgentQuoteFailsValidation(t *testing.T) { // kills "an agent-authored quote"
	if ValidateQuote(AuthoredText{Text: "please add SSO", Author: AuthorAgent}) == nil {
		t.Fatal("a quote with author A passed")
	}
}

func TestTrackerModeHasNoStateMode(t *testing.T) { // kills "a mode that syncs states", 05 §5.4 W5
	for _, m := range []TrackerMode{"in", "in_and_comments"} {
		if !ValidTrackerMode(m) {
			t.Fatalf("%s refused", m)
		}
	}
	for _, m := range []TrackerMode{"two_way", "states", "in_and_states", ""} {
		if ValidTrackerMode(m) {
			t.Fatalf("%s accepted", m)
		}
	}
}

func TestAWebhookURLMustBeHTTPS(t *testing.T) { // kills "plain-http delivery"
	for _, u := range []string{"http://example.com/h", "https://user:pw@example.com/h", "ftp://x", "https:///nohost"} {
		if ValidateCreateWebhook(CreateWebhookRequest{URL: u, Kinds: []string{"issue.created"}}, []string{"issue.created"}) == nil {
			t.Fatalf("%s accepted", u)
		}
	}
}

func TestOnlyCreatedTypesCarryASecret(t *testing.T) { // kills "the secret listed again"
	carriers := map[string]bool{"CreatedAPIKey": true, "CreatedWebhook": true, "CreatedHostHook": true}
	for _, v := range w5Types() {
		rt := reflect.TypeOf(v)
		for i := 0; i < rt.NumField(); i++ {
			if n := strings.ToLower(rt.Field(i).Name); strings.Contains(n, "secret") && n != "secretversion" && !carriers[rt.Name()] {
				t.Fatalf("%s.%s holds a secret", rt.Name(), rt.Field(i).Name)
			}
		}
	}
}

func TestNoW5TypePredicts(t *testing.T) { // kills "a prediction field", OD-27
	banned := []string{"percent", "progress", "eta", "forecast", "remaining"}
	for _, v := range w5Types() {
		rt := reflect.TypeOf(v)
		for i := 0; i < rt.NumField(); i++ {
			tag := strings.ToLower(strings.Split(rt.Field(i).Tag.Get("json"), ",")[0])
			for _, b := range banned {
				if strings.Contains(tag, b) {
					t.Fatalf("%s.%s (%q) reads as a prediction", rt.Name(), rt.Field(i).Name, tag)
				}
			}
		}
	}
}

func TestW5PrefixesAreDisjoint(t *testing.T) { // kills "prefix collision"
	seen := map[string]bool{}
	for _, p := range append(append(append([]string{}, hubPrefixes...), controlPlanePrefixes...),
		append(append(phaseTPrefixes, WorkPrefixes...), append(WorkIntakePrefixes, "gd")...)...) {
		if seen[p] {
			t.Fatalf("prefix %q declared twice", p)
		}
		seen[p] = true
	}
}

func TestImportBatchAcceptsW5Sources(t *testing.T) { // kills "SW.6 not extended"
	for _, s := range []string{ImportSourceJira, ImportSourceAzureBoards, ImportSourceGitHub} {
		if err := ValidateImportBatch(ImportBatch{Source: s, Items: []ImportItem{sampleImportItem()}}); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
	}
}

func TestEachW5CodeMapsToItsStatus(t *testing.T) { // kills "wrong status"
	want := map[string]int{"request_closed": 409, "request_erased": 409, "form_field_required": 400,
		"tracker_exists": 409, "principal_cannot_move_state": 403, "webhook_url_invalid": 422,
		"webhook_not_found": 404, "app_not_approved": 403, "key_revoked": 401,
		"connector_not_allowed": 403, "harness_not_allowed": 403, "loop_outside_grid": 403}
	if !reflect.DeepEqual(want, WorkIntakeCodeStatus) {
		t.Fatalf("got %v", WorkIntakeCodeStatus)
	}
}
```

`w5Types()` lists a zero value of every struct declared in `work_intake.go` (the adopted blocks
included), and a companion test parses the file with `go/parser` and fails if a declared struct is
missing from the list, so a new type cannot skip the two reflection tests. `hubPrefixes`,
`controlPlanePrefixes` and `phaseTPrefixes` are task 02's test lists (`TestWorkPrefixesAreDisjoint`,
**UNVERIFIED** names); `ErrBadEnvelope` wrapping means `ValidateCreateWebhook`'s URL failure
still `errors.Is` the envelope error. Loop, key and manifest validation each get a table test of
their refusals (a schedule with `eventKinds`, a key for 366 days, a manifest without `work.read`,
a policy with a branch name for a commit).

| Test | Mutant it kills |
|---|---|
| `TestW5RecordsRoundTripTheirGoldens` | a field renamed or retagged |
| `TestAnAgentQuoteFailsValidation` | an agent-authored quote |
| `TestTrackerModeHasNoStateMode` | a mode that syncs states |
| `TestAWebhookURLMustBeHTTPS` | plain-http delivery; credentials in a URL |
| `TestOnlyCreatedTypesCarryASecret` | the secret listed again |
| `TestNoW5TypePredicts` | a prediction field (`OD-27`) |
| `TestW5PrefixesAreDisjoint` | prefix collision |
| `TestImportBatchAcceptsW5Sources` | SW.6 not extended |
| `TestEachW5CodeMapsToItsStatus` | wrong status |
| `var _ WorkIntake = (*Fake)(nil)` and the `Work` embed | `Work` without `WorkIntake`; a missing stub |

#### Steps

- [ ] **Step 1: sync-merge `feat/hub`** (the orchestrator's merge, rule H1) and read tasks 02-04,
  15, 114 and 146-161 on the sync base. Take their merged names; a rename is a find-and-replace in
  this part, never a new type. Re-check the prefixes against every list.
- [ ] **Step 2: the tests first.** Write both test files and the twenty-four goldens (from the
  blocks' own example JSON where a part gives one, `task-146.md:698` for a request); run
  `go test ./control-plane/internal/hub/hubapi/ -run 'W5|Agent|Tracker|Webhook|Secret|Predicts' -count=1`
  and see it fail to compile.
- [ ] **Step 3: `work_intake.go`**: the adopted blocks in task order (146, 147, 148, 149, 153,
  161), with decision 2's two lines removed, then the types above, validation and the status map.
- [ ] **Step 4: `workids.go`, `work.go` and `codes.go` appends**; then `hubclient/work.go`, the
  two stubs and the assertions.
- [ ] **Step 5: run the acceptance commands**; commit each file by path, each message ending with
  the `Co-Authored-By` line; announce SW.21 frozen to 144-173 in the skeleton's *Start lines*.

#### Acceptance

```bash
go test ./control-plane/internal/hub/hubapi/ -count=1 -v -run 'W5|AgentQuote|TrackerMode|WebhookURL|Secret|Predicts|ImportBatch'
# expect: --- PASS for the nine tests above; ok
go test ./control-plane/internal/hub/hubclient/ -count=1
# expect: ok (the assertions compile)
go vet ./control-plane/internal/hub/...
# expect: no output
ls control-plane/internal/hub/hubapi/testdata/work-intake/*.json | wc -l
# expect: 24
grep -c "rq-\"\|fm-\"" control-plane/internal/hub/hubapi/work_intake.go
# expect: 0 (decision 2's two lines are gone)
```

#### What this task does NOT claim

- That the route table accepts these rows: task 168's end to end does.
- That Entra issues the integration or app principals this names (**UNVERIFIED**, tasks 150, 159).
- That Key Vault holds W5's secrets: the contract gives the Hub no Key Vault access in v1
  (`hub-contract.md:2083-2085`); task 145 carries that as a proposed contract addition.
- That the proposed routes are final: their route tasks may rename them.
- Any parity row: this task carries the W5 rows 144-173 close.

#### Gaps found while writing this part

1. **Task 169's desktop mirror follows 143's scope, not the route owners** for `Request` (channel
   `tracker`, `answers` as a list, `projectId`, `channelRef`), `IntakeRule` and `IntakeForm`;
   tasks 146-148's Go differs, and decision 1 makes theirs win. Task 169's golden test will fail
   until it re-mirrors; it should also add `UpstreamChange.to` and `SyncError.retryAt`.
2. **Tasks 146 and 148 redeclare prefixes** (`task-146.md:191`, `task-148.md:120`); decision 2
   deletes both lines, and those parts' *Files* lists should drop the `workids.go` touch.
3. **Tasks 150, 154, 159, 160, 162, 163 and 166 have no part on this tree** (2026-09-26): their
   types here mirror task 169's TS and their routes are proposed.
4. **`IntakeSettings` has no `formId`** (`task-149.md:277-284`) though the scope's `IntakeAddress`
   had one: a mail request cannot take a form's member or non-member defaults.
5. **The skeleton's task-table row for 143** says *"the W5 wire types"* but the scope names
   `HarnessAllow`, `TeamSkill`, `mc-` and `lr-`, all superseded here (decisions 4-6).


