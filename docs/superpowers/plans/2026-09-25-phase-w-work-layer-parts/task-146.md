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
  quote              text NOT NULL CHECK (octet_length(quote) <= 65536),
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
and changed only by `erase_request`, which is what task 145's grant test must hold. The 64 KiB cap
on `quote` is the second layer behind `hubapi.MaxRequestQuoteBytes`.

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
	MaxRequestQuoteBytes   = 64 << 10 // the table's CHECK is the second layer
	MaxRequesterNameChars  = 200
	MaxRequesterEmailBytes = 320 // RFC 5321's path limit
	MaxDeclineReasonChars  = 500
)

// ValidateRequester checks the shape of who asked, before anything is read.
// A person's display name is the Hub's, so the client may not send one.
func ValidateRequester(r Requester) *FieldError {
	switch r.Kind {
	case RequesterPerson:
		if !ValidID(PrefixUser, r.UserID) || r.Email != "" || r.DisplayName != "" {
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

// ValidateQuote: the requester's words, author "H", 1 byte to 64 KiB, valid
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

`FieldError`, `ErrBadEnvelope`, `AuthorHuman` and `ValidID` are task 03's and task 02's
(`task-03.md:464`); `PrefixUser` is the Hub's `u-` (hub-contract `:105-112`, as cited in 05 §3.6).
The file imports `net/mail` and `unicode/utf8`, both standard library. `ValidateQuote` is the only
check on the quote's text: an ask may be 40 KiB of pasted email, may start with whitespace and may
hold a word a title check would refuse, and the requester's words are kept as they gave them.

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
