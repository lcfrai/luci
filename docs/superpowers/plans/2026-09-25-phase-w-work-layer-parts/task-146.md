### Task 146: requests `rq-`: the requester, the verbatim quote as `text·H`, *important*, accept or link in `triage`, erase

W5, stream WH; no seam of its own (it consumes **SW.21**, task 143's W5 wire, and **SW.3**, task
145's guards). Depends on task 14 (the erase pattern this follows), task 145 (the W5 migration that
holds `work_requests`) and Hub task H27 (the audit route and brief erase). Estimate 0.375 (3 h).
Closes, jointly with task 170 (the Triage surface): **L-132** and **L-133**; serves **L-137**,
**L-138** and **L-140** (tasks 149 and 157 use this task's store function). Every `server/` and
`store/` path is **UNVERIFIED until merge** (rule H1); `server/erase.go` is Hub task 27's file and
this task adds routes beside task 14's, never a change to them.

A **request** is how someone outside the team asks for something (05 §3.5): a business owner, a
client, a person in another team. It keeps **who asked** and **their words, verbatim**, and it
waits in the team's Triage until a person accepts it into a new issue, links it to an issue that
already exists, or declines it. After this task a member can record a request on someone's behalf,
the team can list and read its requests, flag one *important*, accept, link or decline it, an issue
lists who asked for it by name, and an org admin can erase a request's text. The form (task 148),
mail (task 149) and Teams (task 157) paths all create through the one store function this task
writes, so a request is the same record whichever door it came in by.

**Task 143 and task 145 are not written at this writing** (gap 1 in *Gaps*). This part therefore
states, complete, the wire types it needs from `hubapi/work_intake.go` (SW.21) and the table it
needs from `NNNN_work_w5.sql`. They are **requirements on those two parts**: when they land, their
names win and this part follows them, as rule H1 has a WH task follow a moved Hub file.

Eight decisions the scope left open, made here.

1. **One request, one outcome.** A request is `open` until exactly one of *accept*, *link* or
   *decline* closes it, and then it is `accepted`, `linked` or `declined` for good. A second close
   of any kind is 409 `request_closed`, naming the state it is in. A requester who asks for two
   things sends two requests; a person who wants one ask to feed two issues accepts it into one and
   relates the second (task 10's relations). This is what makes the test *"a second accept is
   `request_closed`"* a rule and not a race: the close is one `UPDATE … WHERE state = 'open'`
   inside the org's write lock, and zero rows is the 409.
2. **The quote becomes the issue's description, and the request stays out of the agent's
   context.** The scope pins accept to *"revision 1 whose description is the quote, `author: "H"`"*,
   and 05 §3.5 says *"its text never enters an agent's managed block"*. Task 19 renders an issue's
   `H` description into the managed block (`task-19.md:10-26`), so these two meet. They are
   reconciled like this: by accepting, a **person** adopts the words as the issue's description,
   and the accept dialog says so (task 170: *"The description will be these words. Agents that
   start from this issue read them."*); what never enters is the **request record**: its id,
   requester, email, channel, answers and rule. The test that kills *"request text reaches the
   agent"* checks exactly that. Whether the owner wants the stricter reading, an empty
   description with the quote reachable only as data through `panacea issue show`, is **owner
   question Q-146** in *Gaps*; the change would be one line in `acceptRequest` and one test.
3. **Link writes no revision and moves nothing.** It sets the request's `issue_id` and appends
   `request.linked`. The issue gains a requester in `GET /v1/issues/{issueId}/requests` and nothing
   else: no revision, no state, no `issue.updated`. A person who wants the requester's words in the
   issue pastes them into a revision, which is then their act.
4. **Accept is task 08's create, not a copy of it.** It builds the revision through
   `hubapi.IssueRevisionDigest`, checks `serves` with task 08's `checkServes`, takes the state from
   the team's policy (`defaultState`, task 11) and the thread state from task 08's
   `threadStateOf`, exactly as `importOne` does (`task-15.md:420-486`). With no serve the issue is
   `not_tied` with reason **`support`** (task 03's closed list, `task-03.md:171-177`): an ask from
   outside the team is the case `support` exists for, and the accepter can re-tie it later.
5. **The project must be the request's team's.** A `projectId` outside the request's team is 400
   `field: projectId`, whether or not the project exists and whoever can see it, so the answer is
   no oracle for another team's projects. Rules (task 147) route only inside the team by the same
   check.
6. **Requests are team-scoped events.** Every `request.*` kind is `ScopeTeam` and
   `VisibleToReaders` in task 13's registry, with `projectId` null: an open request belongs to the
   team's Triage, not to a project. `request.accepted` and `request.linked` carry the `issueId`,
   and the issue's own `issue.created` (accept only) is task 13's, unchanged.
7. **Erase is not a state.** It blanks the text and sets `erased_at`; an erased request keeps its
   state and `issue_id`, so a triage count and an issue's requester list still add up, reading
   *erased* where the name was. Accept, link and decline of an erased open request are 409
   `request_erased`: an issue made from nothing is not an accept. `important` may still be set.
8. **`channel_ref` survives erase.** The `Message-ID` (task 149) or Teams message id (task 157) is
   an identifier, not a person's words, and keeping it is what stops a mail retry from re-creating
   a request an admin erased. It is capped at 998 bytes (the RFC 5322 line limit) and never shown
   in the feed.

#### Files

- `control-plane/internal/hub/store/work_requests.go` (new): the create, read, close, flag and
  erase statements.
- `control-plane/internal/hub/server/work_requests.go` (new): the routes, and `createRequest`, the
  one server function the form, mail and Teams paths call.
- `control-plane/internal/hub/server/work_requests_integration_test.go` (new, `-tags integration`).
- `control-plane/internal/hub/hubapi/work_intake_test.go` (new if task 143 has not made it; else
  appended): the pure validation rows.
- ⚑ `control-plane/internal/hub/server/erase.go` (Hub task 27's file, task 14's two routes): one
  route added beside them.
- ⚑ `control-plane/internal/hub/server/work_emit.go` (task 13; task 14's `emitWorkErase`): the
  erase kinds it accepts gain `request.erased`.
- ⚑ `control-plane/internal/hub/hubapi/workevents.go` (task 13): the six `request.*` kinds.
- ⚑ `control-plane/internal/hub/server/work_matrix_rows_integration_test.go` (tasks 07, 145): the
  task's rows under key `"146"`, and `"146"` appended to `workMatrixLive`.

Read before starting, each at the line cited: task 03 (`AuthoredText`, `NotTiedReason`,
`task-03.md:171-177`), task 07's row helpers (`task-07.md:376-392`), task 08's create path and
`checkServes` (`task-08.md:253-263`, `:413-416`), task 13's registry and `emitWork`
(`task-13.md:250-270`, `:332-339`), task 14's erase (`task-14.md:184-264`) and task 15's
`importOne` (`task-15.md:420-486`).

#### The table this task needs from task 145's migration (requirement; complete)

```sql
-- ── task 146 (in task 145's NNNN_work_w5.sql): requests from outside the team ──────
CREATE TABLE work_requests (
  id                 text PRIMARY KEY CHECK (id ~ '^rq-[0-9a-f]{16}$'),
  org_id             text NOT NULL REFERENCES orgs (id),
  team_id            text NOT NULL,
  channel            text NOT NULL CHECK (channel IN ('member', 'form', 'email', 'teams')),
  channel_ref        text NOT NULL DEFAULT '' CHECK (octet_length(channel_ref) <= 998),
  requester_kind     text NOT NULL CHECK (requester_kind IN ('person', 'email', 'teams')),
  requester_user_id  text,
  requester_email    text NOT NULL DEFAULT '' CHECK (octet_length(requester_email) <= 320),
  requester_name     text NOT NULL DEFAULT '' CHECK (octet_length(requester_name) <= 1024),
  quote              text NOT NULL CHECK (octet_length(quote) <= 16384),
  quote_trimmed      integer NOT NULL DEFAULT 0 CHECK (quote_trimmed >= 0),   -- task 149
  answers            jsonb NOT NULL DEFAULT '{}'::jsonb,                       -- task 148
  form_id            text,                                                     -- task 148
  rule_id            text,                                                     -- task 147
  rule_version       integer,
  rule_author        text,
  route              jsonb NOT NULL DEFAULT '{}'::jsonb,                       -- task 147's IntakeRoute
  important          boolean NOT NULL DEFAULT false,
  state              text NOT NULL DEFAULT 'open'
                     CHECK (state IN ('open', 'accepted', 'linked', 'declined')),
  issue_id           text,
  decline_reason     text NOT NULL DEFAULT '' CHECK (octet_length(decline_reason) <= 2000),
  created_at         timestamptz NOT NULL,
  created_by         text,                             -- null when the janitor read it from mail
  closed_at          timestamptz,
  closed_by          text,
  erased_at          timestamptz,
  FOREIGN KEY (org_id, team_id)           REFERENCES teams (org_id, id),
  FOREIGN KEY (org_id, requester_user_id) REFERENCES users (org_id, id),
  FOREIGN KEY (org_id, rule_author)       REFERENCES users (org_id, id),
  FOREIGN KEY (org_id, issue_id)          REFERENCES issues (org_id, id),
  FOREIGN KEY (org_id, created_by)        REFERENCES users (org_id, id),
  FOREIGN KEY (org_id, closed_by)         REFERENCES users (org_id, id),
  CONSTRAINT work_requests_issue_iff_joined
    CHECK ((state IN ('accepted', 'linked')) = (issue_id IS NOT NULL)),
  CONSTRAINT work_requests_closed_iff_not_open
    CHECK ((state = 'open') = (closed_at IS NULL AND closed_by IS NULL)),
  CONSTRAINT work_requests_person_has_user
    CHECK ((requester_kind = 'person') = (requester_user_id IS NOT NULL)),
  CONSTRAINT work_requests_rule_whole
    CHECK ((rule_id IS NULL) = (rule_version IS NULL) AND (rule_id IS NULL) = (rule_author IS NULL))
);
CREATE UNIQUE INDEX work_requests_channel_once ON work_requests (org_id, team_id, channel, channel_ref)
  WHERE channel_ref <> '';
CREATE INDEX work_requests_team_state ON work_requests (team_id, state, created_at DESC, id DESC);
CREATE INDEX work_requests_issue ON work_requests (issue_id) WHERE issue_id IS NOT NULL;

ALTER TABLE work_requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE work_requests FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON work_requests TO hub_app
  USING (org_id = current_setting('hub.org_id', true))
  WITH CHECK (org_id = current_setting('hub.org_id', true));
GRANT SELECT, INSERT ON work_requests TO hub_app;
GRANT UPDATE (important, state, issue_id, decline_reason, closed_at, closed_by) ON work_requests TO hub_app;

-- erase_request: the brief's erase for a request (05 §3.6). Text blanked; ids,
-- state, issue_id, channel_ref and the rule kept. The only path to those columns.
CREATE FUNCTION erase_request(p_request text) RETURNS integer
LANGUAGE plpgsql SECURITY DEFINER SET search_path = hub, pg_temp AS $$
DECLARE v_org text := current_setting('hub.org_id', true); v_n integer;
BEGIN
  IF v_org IS NULL OR NOT EXISTS (SELECT 1 FROM work_requests r WHERE r.id = p_request AND r.org_id = v_org) THEN
    RAISE EXCEPTION 'hub: work: no request % in this org', p_request USING ERRCODE = 'no_data_found';
  END IF;
  UPDATE work_requests SET quote = '', answers = '{}'::jsonb, requester_email = '', requester_name = '',
                           decline_reason = '', erased_at = now()
   WHERE id = p_request AND erased_at IS NULL;
  GET DIAGNOSTICS v_n = ROW_COUNT;
  RETURN v_n;
END $$;
REVOKE ALL ON FUNCTION erase_request(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION erase_request(text) TO hub_app;
```

`requester_user_id` is **kept** by erase: it is an id, like `created_by` on a brief revision, and
the person's name is read from `users` at request time, so an erased request from a person still
reads *Ada Lovelace asked* while an erased request by email reads *erased*. Whether an owner wants
the person's link cut too is part of Q-146. `answers` holds a form's typed values (task 148), all
of them a person's words, so all of it goes. `decline_reason` is a team member's words and goes
too, as `state_reason` does on an issue (`task-14.md` decision 3). No text column is added to the
`UPDATE` grant: `quote`, `answers`, the requester's email and name are written once, by `INSERT`,
and changed only by `erase_request`, which is what task 145's grant test must hold. The 16 KiB cap
on `quote` is the second layer behind `hubapi.MaxRequestQuoteBytes`, which **equals** task 02's
`MaxIssueDescriptionBytes` (`task-02.md:211`): accept copies the quote into a description byte for
byte, so a quote the description would refuse must never be stored.

#### `hubapi/work_intake.go`, the part this task needs (requirement on task 143; complete)

```go
// ── Task 146: requests (SW.21) ──────────────────────────────────────────────

const PrefixRequest = "rq-" // task 02's workids.go already lists it (skeleton :185-187)

type RequestChannel string

const (
	ChannelMember RequestChannel = "member" // a team member recorded it on someone's behalf
	ChannelForm   RequestChannel = "form"   // task 148
	ChannelEmail  RequestChannel = "email"  // task 149
	ChannelTeams  RequestChannel = "teams"  // task 157
)

var RequestChannels = []RequestChannel{ChannelMember, ChannelForm, ChannelEmail, ChannelTeams}

type RequesterKind string

const (
	RequesterPerson RequesterKind = "person" // an org user: userId set
	RequesterEmail  RequesterKind = "email"  // an address, perhaps outside the tenant
	RequesterTeams  RequesterKind = "teams"  // a Teams sender with no org user (task 157)
)

// Requester is who asked. DisplayName is read from users for a person, and is
// the stored name otherwise; "" once erased (Erased on the Request says why).
type Requester struct {
	Kind        RequesterKind `json:"kind"`
	UserID      string        `json:"userId,omitempty"`
	Email       string        `json:"email,omitempty"`
	DisplayName string        `json:"displayName"`
}

type RequestState string

const (
	RequestOpen     RequestState = "open"
	RequestAccepted RequestState = "accepted"
	RequestLinked   RequestState = "linked"
	RequestDeclined RequestState = "declined"
)

var RequestStates = []RequestState{RequestOpen, RequestAccepted, RequestLinked, RequestDeclined}

// RuleHit names the intake rule that routed a request (task 147); nil when no
// rule matched and the form's defaults stood.
type RuleHit struct {
	RuleID      string `json:"ruleId"`
	RuleVersion int    `json:"ruleVersion"`
	RuleAuthor  string `json:"ruleAuthor"` // a u- id
}

// IntakeRoute is where a request is suggested to go inside its team: the
// accept dialog's defaults (task 170). An empty field suggests nothing. Task
// 147 fills it from the form's defaults and the first matching rule.
type IntakeRoute struct {
	ProjectID string   `json:"projectId,omitempty"`
	Priority  Priority `json:"priority,omitempty"`
	LabelIDs  []string `json:"labelIds,omitempty"`
	Important bool     `json:"important,omitempty"`
}

// Request is a request as every route answers it. Quote, Answers and the
// requester's email and name are "" or empty once Erased.
type Request struct {
	ID            string            `json:"id"`
	TeamID        string            `json:"teamId"`
	Channel       RequestChannel    `json:"channel"`
	Requester     Requester         `json:"requester"`
	Quote         AuthoredText      `json:"quote"`         // author is always "H"
	QuoteTrimmed  int               `json:"quoteTrimmed"`  // lines cut by task 149; 0 otherwise
	Answers       map[string]string `json:"answers"`       // task 148's typed fields, as text
	FormID        string            `json:"formId,omitempty"`
	Rule          *RuleHit          `json:"rule"`
	Route         IntakeRoute       `json:"route"`
	Important     bool              `json:"important"`
	State         RequestState      `json:"state"`
	IssueID       string            `json:"issueId,omitempty"`
	DeclineReason string            `json:"declineReason,omitempty"`
	CreatedAt     string            `json:"createdAt"`
	CreatedBy     *User             `json:"createdBy"` // the Hub's User; null when the janitor read it from mail
	ClosedAt      string            `json:"closedAt,omitempty"`
	ClosedBy      *User             `json:"closedBy,omitempty"`
	Erased        bool              `json:"erased"`
}

// CreateRequestRequest is POST /v1/teams/{teamId}/requests: a member records
// an ask on someone's behalf. channel is always "member" on this route; the
// form, mail and Teams paths build a store.RequestRow directly.
type CreateRequestRequest struct {
	Requester Requester    `json:"requester"`
	Quote     AuthoredText `json:"quote"`
	Important bool         `json:"important"`
}

type AcceptRequestRequest struct {
	ProjectID string  `json:"projectId"`
	Title     string  `json:"title"`  // the accepter's words, author "H"
	Serves    []Serve `json:"serves"` // optional; empty is not_tied · support
}

type AcceptRequestResponse struct {
	Request Request  `json:"request"`
	Issue   IssueRef `json:"issue"`
}

type LinkRequestRequest struct {
	IssueID string `json:"issueId"`
}

type DeclineRequestRequest struct {
	Reason string `json:"reason"` // 1-500 characters, trimmed
}

type SetImportantRequest struct {
	Important bool `json:"important"`
}

// RequestEraseResponse is POST /v1/requests/{requestId}/erase's 200.
// DerivedIssueIDs names the issue accepted from the request, so the admin can
// erase it too through task 14's route; nothing cascades (decision in Steps).
type RequestEraseResponse struct {
	RequestID       string   `json:"requestId"`
	Erased          int      `json:"erased"` // 1 the first time, 0 after
	ErasedAt        string   `json:"erasedAt"`
	DerivedIssueIDs []string `json:"derivedIssueIds"`
}

// IssueRequester is one row of GET /v1/issues/{issueId}/requests: who asked,
// by name (05 §1 L-135's reason), never their words.
type IssueRequester struct {
	RequestID string         `json:"requestId"`
	Requester Requester      `json:"requester"`
	Channel   RequestChannel `json:"channel"`
	Important bool           `json:"important"`
	State     RequestState   `json:"state"` // accepted or linked
	CreatedAt string         `json:"createdAt"`
	Erased    bool           `json:"erased"`
}

const (
	MaxRequestQuoteBytes   = MaxIssueDescriptionBytes // 16 KiB: accept copies it byte for byte
	MaxRequesterNameChars  = 200
	MaxRequesterEmailBytes = 320 // RFC 5321's path limit
	MaxDeclineReasonChars  = 500
)

// ValidateRequester checks the shape of who asked, before anything is read.
// A person's display name is the Hub's, so the client may not send one.
func ValidateRequester(r Requester) *FieldError {
	switch r.Kind {
	case RequesterPerson:
		if !strings.HasPrefix(r.UserID, "u-") || len(r.UserID) > 64 || r.Email != "" || r.DisplayName != "" {
			return &FieldError{"requester", ErrBadEnvelope}
		}
	case RequesterEmail:
		if r.UserID != "" || !validMailbox(r.Email) ||
			utf8.RuneCountInString(r.DisplayName) > MaxRequesterNameChars {
			return &FieldError{"requester", ErrBadEnvelope}
		}
	default: // "teams" is task 157's, never a client's
		return &FieldError{"requester.kind", ErrBadEnvelope}
	}
	return nil
}

// ValidateQuote: the requester's words, author "H", 1 byte to 16 KiB, valid
// UTF-8, kept byte for byte (never trimmed, never normalised).
func ValidateQuote(q AuthoredText) *FieldError {
	if q.Author != AuthorHuman || q.Text == "" || len(q.Text) > MaxRequestQuoteBytes || !utf8.ValidString(q.Text) {
		return &FieldError{"quote", ErrBadEnvelope}
	}
	return nil
}

// validMailbox is net/mail's parse plus the length cap, with no display name.
func validMailbox(s string) bool {
	a, err := mail.ParseAddress(s)
	return err == nil && a.Name == "" && a.Address == s && len(s) <= MaxRequesterEmailBytes
}
```

`FieldError` and `ErrBadEnvelope` are task 03's (`task-03.md:464`) and `AuthorHuman` task 02's.
The user-id check is only a shape check (the Hub's `u-`, hub-contract `:105-112` as cited in 05
§3.6); the store resolves the id against `users` in the caller's org and answers 400
`field: requester` when it is not there, so the prefix test is never the authority. The file
imports `net/mail`, `strings` and `unicode/utf8`, all standard library. `ValidateQuote` is the only
check on the quote's text: an ask may be a page of pasted email, may start with whitespace and may
hold a word a title check would refuse, and the requester's words are kept as they gave them. A
mail body longer than 16 KiB after the reply-marker cut is task 149's to handle (it cuts at a line
boundary and counts the lines in `quoteTrimmed`); this task refuses, never cuts.

#### `hubapi/workevents.go`, the additions (task 13's registry; complete)

```go
const (
	EventRequestCreated   = "request.created"
	EventRequestAccepted  = "request.accepted"
	EventRequestLinked    = "request.linked"
	EventRequestDeclined  = "request.declined"
	EventRequestImportant = "request.important"
	EventRequestErased    = "request.erased"
)

// RequestCreatedData never holds the requester's email, name or words. The
// rule fields are "" and 0 until task 147 routes; task 147 changes no type.
type RequestCreatedData struct {
	Channel       string `json:"channel"`
	RequesterKind string `json:"requesterKind"`
	RuleID        string `json:"ruleId"`
	RuleVersion   int64  `json:"ruleVersion"`
	RuleAuthor    string `json:"ruleAuthor"`
	Important     bool   `json:"important"`
}
type RequestAcceptedData struct {
	IssueID string `json:"issueId"`
}
type RequestLinkedData struct {
	IssueID string `json:"issueId"`
}
type RequestDeclinedData struct{} // the reason is a person's words: stored on the row, never here
type RequestImportantData struct {
	Important bool `json:"important"`
}
type RequestErasedData struct{} // emitWorkErase adds "reason", which the feed strips

// Appended to WorkEventKinds (task 13's SW.5 registry), after task 143's other W5 kinds:
//	{EventRequestCreated, RequestCreatedData{}, ScopeTeam, VisibleToReaders},
//	{EventRequestAccepted, RequestAcceptedData{}, ScopeTeam, VisibleToReaders},
//	{EventRequestLinked, RequestLinkedData{}, ScopeTeam, VisibleToReaders},
//	{EventRequestDeclined, RequestDeclinedData{}, ScopeTeam, VisibleToReaders},
//	{EventRequestImportant, RequestImportantData{}, ScopeTeam, VisibleToReaders},
//	{EventRequestErased, RequestErasedData{}, ScopeTeam, VisibleToReaders},
```

Task 13's `TestTheHubWritesOnlyDeclaredWorkKinds` matches Work kinds by prefix (`task-13.md:524`);
`request.` is added to its prefix list in this task, so a writer that invents `request.opened` is
caught. Its W1 count assertion (`len(WorkEventKinds) != 18`, `task-13.md:481`) is W1's; task 143
owns the W5 count, and this task adds six to it. `emitWorkErase` (`task-14.md:257-261`) accepts
`request.erased` as a third erase kind, and Hub task 26's projection strips `reason` from it as it
does from `issue.erased`.

#### The store (`store/work_requests.go`, signatures complete)

```go
package store

// RequestRow is one work_requests row. The server fills it; the store writes
// what it is given and checks nothing a CHECK constraint already checks.
type RequestRow struct {
	ID, OrgID, TeamID  string
	Channel            string
	ChannelRef         string // "" for member and form; Message-ID or Teams id otherwise
	RequesterKind      string
	RequesterUserID    string // "" unless kind is person
	RequesterEmail     string
	RequesterName      string // stored for email and teams; "" for a person (read from users)
	Quote              string
	QuoteTrimmed       int
	Answers            map[string]string
	FormID             string
	RuleID             string // "" when no rule matched (task 147)
	RuleVersion        int
	RuleAuthor         string
	Route              hubapi.IntakeRoute
	Important          bool
	State              string
	IssueID            string
	DeclineReason      string
	CreatedAt          time.Time
	CreatedBy          string // "" when the janitor read it from mail
	ClosedAt           *time.Time
	ClosedBy           string
	ErasedAt           *time.Time
}

// InsertRequest writes a new open request. A 23505 on work_requests_channel_once
// is ErrRequestSeen: the same Message-ID or Teams message, delivered again.
func InsertRequest(ctx context.Context, tx pgx.Tx, r RequestRow) error

var ErrRequestSeen = errors.New("store: this channel reference already made a request")

// RequestByChannelRef finds the request an earlier delivery made, for the
// caller that wants to answer "already have it" (tasks 149, 157).
func RequestByChannelRef(ctx context.Context, q Querier, teamID, channel, ref string) (RequestRow, bool, error)

// LockRequest: SELECT … FROM work_requests WHERE id = $1 FOR UPDATE. Row-level
// security scopes it to the org; the caller checks the team role.
func LockRequest(ctx context.Context, tx pgx.Tx, id string) (RequestRow, bool, error)
func RequestByID(ctx context.Context, q Querier, id string) (RequestRow, bool, error)

// ListRequests reads one team's requests, newest first, by state when state
// is not "": ORDER BY created_at DESC, id DESC, keyset-paged as task 08's list.
func ListRequests(ctx context.Context, q Querier, teamID, state string, pr hubapi.PageRequest) (rows []RequestRow, next *string, err error)

// CloseRequest moves an open request to accepted, linked or declined:
//   UPDATE work_requests SET state = $2, issue_id = nullif($3, ''),
//          decline_reason = $4, closed_at = $5, closed_by = $6
//    WHERE id = $1 AND state = 'open' AND erased_at IS NULL
// Zero rows is ErrRequestClosed; the caller has already told an erased one apart.
func CloseRequest(ctx context.Context, tx pgx.Tx, id, state, issueID, reason, by string, at time.Time) error

var ErrRequestClosed = errors.New("store: the request is no longer open")

// SetRequestImportant: UPDATE … SET important = $2 WHERE id = $1 AND important <> $2.
// changed is false when it already held the value; the handler then emits nothing.
func SetRequestImportant(ctx context.Context, tx pgx.Tx, id string, important bool) (changed bool, err error)

// RequestsOfIssue lists the requests joined to one issue, oldest first, for
// GET /v1/issues/{issueId}/requests and for task 149's shipped pass.
func RequestsOfIssue(ctx context.Context, q Querier, issueID string) ([]RequestRow, error)

// EraseRequest: SELECT hub.erase_request($1). P0002 is ErrNotInOrg (task 14's).
func EraseRequest(ctx context.Context, tx pgx.Tx, id string) (n int, erasedAt time.Time, err error)
```

The display name of a `person` requester is read by a join on `users` in every read, so a rename
in Entra shows everywhere at once and erase need not reach it. A requester who is no longer in the
org reads as the Hub reads any departed user (`hub-contract` §5's user row; its exact wording is
**UNVERIFIED** until Hub task 05 lands).

#### The routes (`server/work_requests.go`; `server/erase.go` ⚑)

```go
package server

func (s *Server) registerWorkRequests() {
	s.mux.HandleFunc("POST /v1/teams/{teamId}/requests", s.postRequest)            // member
	s.mux.HandleFunc("GET /v1/teams/{teamId}/requests", s.listRequests)            // viewer; ?state=
	s.mux.HandleFunc("GET /v1/requests/{requestId}", s.getRequest)                 // viewer
	s.mux.HandleFunc("POST /v1/requests/{requestId}/accept", s.acceptRequest)      // member
	s.mux.HandleFunc("POST /v1/requests/{requestId}/link", s.linkRequest)          // member
	s.mux.HandleFunc("POST /v1/requests/{requestId}/decline", s.declineRequest)    // member
	s.mux.HandleFunc("PUT /v1/requests/{requestId}/important", s.putImportant)     // member
	s.mux.HandleFunc("GET /v1/issues/{issueId}/requests", s.issueRequests)         // viewer
}

// In Hub task 27's registerErase, beside task 14's two routes, which are not changed:
//	mux.HandleFunc("POST /v1/requests/{requestId}/erase", s.eraseRequest)        // org admin

// createRequest is the one way a request is made. The member route, the form
// (148), the mail pass (149) and Teams (157) all call it inside their own
// write transaction. It resolves a person requester in the org, calls the
// router (task 147 replaces routeRequest's body; this task's body returns the
// team's Triage with no rule), inserts, and appends request.created.
func (s *Server) createRequest(ctx context.Context, tx *Tx, r store.RequestRow) (store.RequestRow, error) {
	if r.RequesterKind == string(hubapi.RequesterPerson) {
		ok, err := store.UserInOrg(ctx, tx, r.RequesterUserID) // H05's read; UNVERIFIED name
		if err != nil {
			return r, err
		}
		if !ok {
			return r, Refuse(http.StatusBadRequest, "bad_request", map[string]any{"field": "requester"})
		}
	}
	r.ID, r.OrgID, r.State, r.CreatedAt = ids.Wide(hubapi.PrefixRequest), tx.P.OrgID, string(hubapi.RequestOpen), tx.Now
	if err := s.routeRequest(ctx, tx, &r); err != nil { // task 147
		return r, err
	}
	if err := store.InsertRequest(ctx, tx, r); err != nil {
		return r, err // ErrRequestSeen reaches the channel's caller, which answers "already have it"
	}
	return r, emitWork(tx, hubapi.EventRequestCreated, r.TeamID, "", r.ID, hubapi.RequestCreatedData{
		Channel: r.Channel, RequesterKind: r.RequesterKind, RuleID: r.RuleID,
		RuleVersion: int64(r.RuleVersion), RuleAuthor: r.RuleAuthor, Important: r.Important})
}

// routeRequest, this task's body: no rule, no suggestion. Task 147 replaces it.
func (s *Server) routeRequest(ctx context.Context, tx *Tx, r *store.RequestRow) error { return nil }

// requestFor finds a request under row-level security and checks the caller's
// role in its team. Not in the org, or in a team the caller is not in, is 404
// request_not_found; in the team below want is 403 forbidden.
func requestFor(ctx context.Context, tx *Tx, id string, want hubapi.Role, lock bool) (store.RequestRow, error)
```

`emitWork` with `projectID ""` is task 13's rule for a `ScopeTeam` kind (`task-13.md:337-338`), so
a request event reaches every reader of the team through `/v1/changes` and names only ids.
`Refuse`, `Require`, `Decode`, `Forbidden`, `s.Read` and `s.Write` are Hub task 14's and 15's, as
task 15 cites them (`task-15.md:414-418`). `store.UserInOrg` stands for whichever read Hub task 05
gives for *this user id is a member of this org*; its name is **UNVERIFIED** until that lands.

```go
// acceptOne makes the issue a request becomes (decisions 2 and 4), inside the
// accept handler's s.Write, after requestFor(member, lock) and the project check.
func (s *Server) acceptOne(ctx context.Context, tx *Tx, rq store.RequestRow, projectID string,
	in hubapi.AcceptRequestRequest) (hubapi.IssueRef, error) {
	pol, err := store.TeamWorkPolicy(ctx, tx, rq.TeamID) // task 11's read; UNVERIFIED name
	if err != nil {
		return hubapi.IssueRef{}, err
	}
	content := hubapi.IssueRevisionContent{
		Title:       hubapi.AuthoredText{Text: in.Title, Author: hubapi.AuthorHuman},
		Description: hubapi.AuthoredText{Text: rq.Quote, Author: hubapi.AuthorHuman}, // byte for byte
		Serves:      orEmpty(in.Serves), Implements: []string{}}
	if err := hubapi.ValidateIssueRevisionContent(content); err != nil {
		return hubapi.IssueRef{}, Refuse(http.StatusBadRequest, "bad_request", fieldOf(err, "title"))
	}
	if err := checkServes(ctx, tx, projectID, content.Serves, "serves"); err != nil { // task 08
		return hubapi.IssueRef{}, err
	}
	n, err := store.NextIssueNumber(ctx, tx, tx.P.OrgID, projectID)
	if err != nil {
		return hubapi.IssueRef{}, err
	}
	id := ids.Wide(hubapi.PrefixIssue)
	head := store.IssueRow{ID: id, OrgID: tx.P.OrgID, ProjectID: projectID, TeamID: rq.TeamID, Number: n,
		State: pol.DefaultState, Priority: routedPriority(rq.Route), Source: hubapi.SourceIntake,
		CreatedAt: tx.Now, UpdatedAt: tx.Now, CreatedBy: tx.P.UserID}
	if len(content.Serves) == 0 {
		head.NotTiedReason = hubapi.NotTiedSupport
	}
	env := hubapi.IssueRevisionEnvelope{V: 1, Kind: "issue-revision", IssueID: id, Revision: 1,
		ProjectID: projectID, Content: content}
	digest, canonical, err := hubapi.IssueRevisionDigest(env)
	if err != nil {
		return hubapi.IssueRef{}, err
	}
	if err := store.InsertIssue(ctx, tx, head); err != nil {
		return hubapi.IssueRef{}, err
	}
	if err := store.InsertIssueRevision(ctx, tx, store.RevisionRow{OrgID: tx.P.OrgID, IssueID: id,
		ProjectID: projectID, Revision: 1, Canonical: canonical, Digest: digest, WrittenAt: tx.Now,
		WrittenBy: tx.P.UserID, Serves: content.Serves}); err != nil {
		return hubapi.IssueRef{}, err
	}
	if err := store.MoveIssueHead(ctx, tx, id, 1, tx.Now); err != nil {
		return hubapi.IssueRef{}, err
	}
	if err := store.SetHeadTitle(ctx, tx, id, in.Title); err != nil { // task 14
		return hubapi.IssueRef{}, err
	}
	if err := emitWork(tx, hubapi.EventIssueCreated, rq.TeamID, projectID, id, hubapi.IssueCreatedData{
		Number: int64(n), Key: hubapi.IssueKey{IssueID: id, Revision: 1}.String(), Digest: digest.String(),
		State: string(pol.DefaultState), Source: string(hubapi.SourceIntake)}); err != nil {
		return hubapi.IssueRef{}, err
	}
	return hubapi.IssueRef{IssueID: id, Number: n}, nil
}
```

`orEmpty` turns a nil `serves` into `[]hubapi.Serve{}` so the canonical bytes say `[]`, as task 15
does literally. `routedPriority` returns the route's suggested priority (task 147) or `none`.
`fieldOf` maps task 02's content error to its JSON field (`title`, since the description is the
quote and `ValidateQuote` already passed). **Labels suggested by a rule are not attached here**:
the accept dialog shows them and the accepter adds them after, through task 11's label route, so
every label on the issue is a person's act. Task 08's create also subscribes the creator (task 13's
rule); accept calls the same subscribe step, so the accepter follows the issue they made. The
requester is not subscribed: they are told of *shipped* by task 149, not of every change.

**Each handler, in order.** Every write is inside `s.Write` (the org's write lock); every read
inside `s.Read`.

- `postRequest`: `Require(team, member)` → 404 `team_not_found` / 403; `Decode[CreateRequestRequest]`;
  `ValidateRequester`, `ValidateQuote` → 400 with the field; `createRequest` with `channel:
  member`, `created_by` the caller; 201 `Request`.
- `listRequests`: `Require(team, viewer)`; `state` is `""` or one of `RequestStates`, else 400
  `field: state`; `ListRequests`; 200 `Page[Request]` (the Hub's page shape).
- `getRequest`: `requestFor(viewer)`; 200 `Request`.
- `acceptRequest`: `requestFor(member, lock)`; erased → 409 `request_erased`; not open → 409
  `request_closed` `{state}`; `Decode[AcceptRequestRequest]`; the project is not the request's
  team's → 400 `field: projectId` (decision 5); `acceptOne`; `CloseRequest(accepted, issueId)`;
  emit `request.accepted {issueId}`; 200 `AcceptRequestResponse`.
- `linkRequest`: `requestFor(member, lock)`; the same two 409s; `Decode[LinkRequestRequest]`;
  task 08's `readableIssue(member, lock=false)` on `issueId` → its 404; the issue's team is not the
  request's → 400 `field: issueId`; `CloseRequest(linked, issueId)`; emit `request.linked`; 200
  `Request`. **Nothing is written to the issue** (decision 3).
- `declineRequest`: `requestFor(member, lock)`; the two 409s; `reason` trimmed, 1-500 characters,
  else 400 `field: reason`; `CloseRequest(declined, "", reason)`; emit `request.declined {}`; 200.
- `putImportant`: `requestFor(member, lock)`; `SetRequestImportant`; emit `request.important` only
  when it changed; 200 `Request`. Allowed on a closed or erased request: the flag is the team's
  note on the ask, and an accepted issue's requester list shows it.
- `issueRequests`: `readableIssue(viewer)`; `RequestsOfIssue`; 200 `{items: [IssueRequester]}`.
- `eraseRequest`: not an org admin → `Forbidden("org_admin")` (before anything is read, as task 14
  does, `task-14.md:245-251`); `Decode[EraseRequest]`; `reason` 1-500 → else 400; `RequestByID`
  under row-level security alone, no team check → 404 `request_not_found`; `EraseRequest`;
  `emitWorkErase(tx, request.erased, team, "", id, RequestErasedData{}, reason)`, on a second erase
  too; 200 `RequestEraseResponse` with `derivedIssueIds` = `[issue_id]` when the state is
  `accepted`, else `[]`.

**No cascade, a choice recorded for the owner.** An issue accepted from a request holds the quote
in its revision 1. Erasing the request does not erase the issue: the issue is the team's record,
has its own revisions, comments, trailers and perhaps a release, and task 14's erase is a
separate, separately audited act by the same admin. The response names the issue so the admin's
next click is obvious (task 173's admin surface shows *also erase EP-142?*). If the owner wants the
erase to take the derived issue with it, it is task 14's `EraseIssue` called in the same
transaction, one more count in the response, and this decision flips; it is **Q-146b** in *Gaps*.
A **linked** issue holds none of the requester's words (decision 3), so it is not named.

#### Wire shapes

```jsonc
// POST /v1/teams/tm-…/requests                        member; 201
{ "requester": { "kind": "email", "email": "owner@client.example", "displayName": "R. Owner" },
  "quote": { "text": "Can the upload resume after a dropped connection?", "author": "H" },
  "important": false }
// → 201
{ "id": "rq-5c1e…", "teamId": "tm-…", "channel": "member",
  "requester": { "kind": "email", "email": "owner@client.example", "displayName": "R. Owner" },
  "quote": { "text": "Can the upload resume after a dropped connection?", "author": "H" },
  "quoteTrimmed": 0, "answers": {}, "rule": null, "route": {}, "important": false,
  "state": "open", "createdAt": "2026-09-26T09:14:00Z", "createdBy": { "id": "u-…", … }, "erased": false }

// POST /v1/requests/rq-5c1e…/accept                   member; 200
{ "projectId": "pj-…", "title": "Resume uploads after a dropped connection", "serves": [] }
// → 200
{ "request": { "id": "rq-5c1e…", "state": "accepted", "issueId": "is-9a0b…", "closedAt": "…", … },
  "issue": { "issueId": "is-9a0b…", "number": 143, "externalId": "" } }

// the change feed for the accept: ids only, never the quote, the email or the name
{ "kind": "issue.created",    "teamId": "tm-…", "projectId": "pj-…", "subject": "is-9a0b…", "data": { … } }
{ "kind": "request.accepted", "teamId": "tm-…", "projectId": null,   "subject": "rq-5c1e…",
  "data": { "issueId": "is-9a0b…" } }

// GET /v1/issues/is-9a0b…/requests                     viewer; 200
{ "items": [ { "requestId": "rq-5c1e…", "channel": "member", "important": false, "state": "accepted",
               "requester": { "kind": "email", "email": "owner@client.example", "displayName": "R. Owner" },
               "createdAt": "…", "erased": false } ] }

// POST /v1/requests/rq-5c1e…/erase                     org admin; 200
{ "reason": "Client asked for removal, matter 4471" }
// → 200
{ "requestId": "rq-5c1e…", "erased": 1, "erasedAt": "…", "derivedIssueIds": ["is-9a0b…"] }

// refusals: nothing is written by any of them
409 request_closed    { "state": "accepted" }    // a second accept, link or decline
409 request_erased                               // accept, link or decline of an erased open request
400 bad_request       { "field": "projectId" }   // a project outside the request's team
400 bad_request       { "field": "quote" }       // empty, author "A", over 16 KiB, not UTF-8
400 bad_request       { "field": "requester" }   // a person id not in the org; an address with a name part
403 forbidden         { "required": "member" }   // a viewer's accept, link, decline or flag
403 forbidden         { "required": "org_admin" } // any team role's erase
404 request_not_found                            // another team's request, or another org's
```

The feed lines show the event envelope's fields as task 13 names them; the envelope's exact key
names are the Hub's (`hub-contract` §8), **UNVERIFIED** here beyond `kind` and `data`.

#### Tests to write first, part 1 — `hubapi/work_intake_test.go` (pure)

| Test | Holds | Mutant it kills |
|---|---|---|
| `TestValidateQuoteKeepsTheBytes` | `"  Resume?\n"` passes as given; `""`, author `A`, 16 KiB + 1 and `"\xff"` each fail on `quote` | a trimming validator (the quote paraphrased by whitespace); a cap above the description's |
| `TestTheQuoteFitsADescription` | `MaxRequestQuoteBytes == MaxIssueDescriptionBytes` | the two caps drifting, so an accept of a stored quote fails |
| `TestValidateRequesterPerKind` | a person with an email or a name fails; an email with `"R <r@x>"` fails; `teams` from a client fails on `requester.kind` | a client naming a person (the Hub's name is the truth); a Teams sender forged over the API |
| `TestRequestEventsCarryNoText` | every field of the six `Request*Data` types is named in the list `channel, requesterKind, ruleId, ruleVersion, ruleAuthor, important, issueId` | a later edit adding `quote`, `email` or `displayName` to an event |
| `TestRequestKindsAreTeamScoped` | the six kinds are in `WorkEventKinds` with `ScopeTeam` | a kind missing from the registry (`emitWork` would 500) or scoped to a project nobody names |

#### Tests to write first, part 2 — `server/work_requests_integration_test.go` (`//go:build integration`)

Against the compose stack, through task 17's `devissuer` fixture. The canary is
`CANARY-rq-7f3e0d`, placed in the quote, the requester's name and a decline reason.

| Test | Holds | Mutant it kills |
|---|---|---|
| `TestTheQuoteNeverReachesTheFeed` | create, accept, link, decline and erase requests carrying the canary; every `/v1/changes` page for a team member, from 0 to the end, holds no canary byte, and no event `data` key is `quote`, `email` or `displayName` | the quote, the address or the name in the feed |
| `TestAcceptWritesTheQuoteVerbatim` | accept → one new issue; `GET …/revisions/1`'s description equals the stored quote byte for byte (a quote with leading spaces and a CRLF), `author: "H"`, `writtenBy` the accepter; the head reads `source: intake` and the team's `defaultState` | the quote paraphrased, trimmed or normalised; authored as `A`; written as the requester |
| `TestASecondAcceptIsRequestClosed` | accept, then accept again → 409 `request_closed {state: accepted}`; the project has one new issue; two goroutines accepting at once → one 200, one 409 | two issues from one request |
| `TestLinkLeavesTheIssueAlone` | link to an issue at revision 3 → the issue's revision count, head, state and `updatedAt` are unchanged, and no `issue.*` event is appended | link rewrites the issue |
| `TestAServeTiesTheAcceptedIssue` | accept with a valid serve → `threadState: tied`; with none → `not_tied · support`; with a serve naming another project's criterion → task 08's 400 on `serves` and no issue, and the request still `open` | a serve unchecked; a failed accept that closes the request anyway |
| `TestAProjectOfAnotherTeamIsRefused` | accept into another team's project, and into a project id that does not exist → both 400 `field: projectId`, identical bodies | cross-team accept; an oracle for another team's project ids |
| `TestErasedRequestsLeaveNoText` | erase; then every `text`, `varchar` and `jsonb` column of `hub.work_requests`, read from `information_schema.columns`, holds no canary, and `answers` is `{}` | a denormalised copy left, or a column added later and forgotten |
| `TestEraseNamesTheDerivedIssue` | erase an accepted request → `derivedIssueIds` is its issue; the issue's revision 1 still holds the quote and is not erased; a second erase → `erased: 0`, a second `request.erased` event | a silent cascade; a second erase unaudited |
| `TestAnErasedRequestCannotBeAccepted` | erase an open request; accept → 409 `request_erased`; `important` → 200 | an issue made from an erased (empty) quote |
| `TestTheReplayStaysErased` | create through `createRequest` with channel `email`, ref `<m1@x>`; erase; `createRequest` with the same ref → `ErrRequestSeen`, and the row stays erased | a re-delivery restoring erased words |
| `TestAnIssueListsItsRequestersByName` | two requests linked to one issue → `GET …/requests` lists both, oldest first, with names and no `quote` key | the list missing, or carrying the words |
| `TestTheManagedBlockHasNoRequestField` | accept a request with requester `R. Owner`, email `owner@client.example`; render task 19's issue section for the issue; the bytes hold none of `rq-`, the email, the name or the channel | request text reaching the agent (decision 2's line) |

`TestTheManagedBlockHasNoRequestField` runs task 19's renderer from `control-plane/internal/claudemd`
on the issue head it reads back over `hubclient`, so it lives in task 19's package's integration
tests if the Hub's boundary rule (skeleton `:179-180`) forbids the import here; either way it runs
in this task's commit.

#### The matrix rows (`server/work_matrix_rows_integration_test.go` ⚑, key `"146"`)

```go
"146": {
	wwrite("POST /v1/teams/{teamId}/requests", "POST", "/v1/teams/{TeamID}/requests", 201, "team_not_found",
		func(*fixture) any {
			return map[string]any{"requester": map[string]any{"kind": "email", "email": "r@client.example",
				"displayName": "R"}, "quote": h("Resume uploads?"), "important": false}
		}),
	wread("GET /v1/teams/{teamId}/requests", "/v1/teams/{TeamID}/requests", "team_not_found"),
	wread("GET /v1/requests/{requestId}", "/v1/requests/{RequestID}", "request_not_found"),
	wwrite("POST /v1/requests/{requestId}/accept", "POST", "/v1/requests/{FreshRequestID}/accept", 200, "request_not_found",
		func(f *fixture) any { return map[string]any{"projectId": f.ProjectID, "title": "Resume uploads", "serves": []any{}} }),
	wwrite("POST /v1/requests/{requestId}/link", "POST", "/v1/requests/{FreshRequestID}/link", 200, "request_not_found",
		func(f *fixture) any { return map[string]any{"issueId": f.IssueID} }),
	wwrite("POST /v1/requests/{requestId}/decline", "POST", "/v1/requests/{FreshRequestID}/decline", 200, "request_not_found",
		func(*fixture) any { return map[string]any{"reason": "Out of scope"} }),
	wwrite("PUT /v1/requests/{requestId}/important", "PUT", "/v1/requests/{RequestID}/important", 200, "request_not_found",
		func(*fixture) any { return map[string]any{"important": true} }),
	wread("GET /v1/issues/{issueId}/requests", "/v1/issues/{IssueID}/requests", "issue_not_found"),
	werase("POST /v1/requests/{requestId}/erase", "/v1/requests/{RequestID}/erase"),
},
```

`{FreshRequestID}` is a new fixture placeholder: an **open** request made per column, since accept,
link and decline close the one they touch and the matrix runs each column in turn. The fixture
gains `RequestID` and `FreshRequestID` (task 07's `fixture`; how it mints a per-column value is
**UNVERIFIED** until H16b lands, and if it cannot, the three rows each create their own request in
`body`). The erase row is written as task 14's are (`task-07.md:460-461`). `werase` sends `empty`
(`task-07.md:388-391`), and every Work erase requires a `reason`, so the `O` column would read 400,
not 200: a defect in task 07's helper shared by task 14's two rows and this one (gap 3). The fix
is one line in task 07, `body: func(*fixture) any { return map[string]any{"reason": "matrix"} }`,
and this row needs no change when it lands.
