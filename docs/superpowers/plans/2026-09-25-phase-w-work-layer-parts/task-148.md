### Task 148: intake forms with typed, required fields; the Entra-protected Hub web form

W5, stream WH; no seam. Depends on task 147 (`intake.Route`, `IntakeDefaults`, and the
`formDefaults` stub this task fills) and **task 80's browser session** (the Entra auth-code
session outside `/v1`, a proposed Hub-contract addition **not merged until the `feat/hub` owner
has reviewed it**). Estimate 0.375 (3 h). Closes, jointly with task 170 (the form editor):
**L-27** (*"Intake forms (§3.5) with typed, required fields"*) and **L-139** (*"A Hub-hosted form
behind Entra (OD-6)"*); serves **L-28** (task 147's two defaults) and **L-233** (the Hub serves
only intake forms and read-only pages). Every `server/` and `store/` path is **UNVERIFIED until
merge** (rule H1); task 80's part is not written at this writing, so every name of its session
below is **UNVERIFIED** (gap 2).

A business owner who wants something from the team should not need the desktop app, a licence
or a tracker account. They open a link, sign in with the same Entra account they use for mail,
fill in a short form the team designed, and read *We have it.* This task gives a team admin forms
with typed fields, some required, and gives the Hub one browser page per form that renders it and
takes the submission. Every submission becomes a request through task 146's `createRequest`, routed
by task 147's rules, in one transaction.

Seven decisions the scope left open, made here.

1. **Every form has one `quote` field, and the Hub puts it there.** `POST …/forms` refuses a form
   whose `fields` lacks exactly one field with id `quote`, kind `longtext`, `required: true`
   (`fields`, 400). The editor (task 170) creates it and cannot delete it; its label is the team's
   (*"What do you need?"*). Its value is the request's `quote`, `text·H`, byte for byte (task 146's
   `ValidateQuote`); every other field goes to `answers`.
2. **A form needs an id prefix, and the skeleton has none.** Forms take **`fm-`** (collides with
   none of the Hub's, TM-05's or the Work prefixes listed at skeleton `:185-187` and 05 §3.6). This
   is a change to task 02's `workids.go` and its `TestWorkPrefixesAreDisjoint` (gap 1).
3. **Types are checked on the server, by kind, and nothing is coerced.** `text` is one line, 1-500
   characters; `longtext` 1 byte to 16 KiB; `select` exactly one of its options; `date` is
   `YYYY-MM-DD` and a real date (`tomorrow`, `26/09/2026` and `2026-02-30` are refused); `number`
   is `^-?[0-9]{1,15}(\.[0-9]{1,6})?$`, stored as the text given (no float round trip); `person` is
   the email of a user in the org, stored as their `u-` id. An empty optional field is absent from
   `answers`, never `""`. The browser's `type="date"` is a convenience; the server is the check.
4. **The page is a plain HTML form, and the session is task 80's.** No script, no client-side
   validation, no `/v1` call from the page. `GET /p/forms/{formId}` renders; `POST
   /p/forms/{formId}` takes `application/x-www-form-urlencoded`, field names `f.<id>` plus `csrf`.
   The CSRF token is the session's (task 80 mints one per session); the POST compares it in
   constant time and a missing or wrong one is 403 before the body is read further. The page's
   cookie is `HttpOnly`, `SameSite=Strict` and accepted by no `/v1` route (task 80's rule, which
   this task's test re-proves on one `/v1` route).
5. **Who may submit: any signed-in person of the org.** A requester need not be in the team, so a
   form is open to every `N` principal of §5.3 (a person of the org in no team) as well as to the
   team's own; a principal of another org gets 404 `form_not_found`, the same bytes as a form that
   does not exist. The requester is always `{kind: "person", userId}` of the session, which is
   what lets task 147 decide member or non-member from the Hub's own table.
6. **The confirmation promises nothing but being heard.** It shows the request's id and the words
   *"We have it. You will hear by email when it ships."* The scope's text was *"You will hear here
   when it ships"*; task 149 tells a form requester **by mail** (its scope: *"for `form`, a mail to
   the signed-in requester"*), so *here* would promise a page that does not exist. No date, no
   estimate, no percentage (`OD-27`); a test holds it.
7. **A closed form stops taking submissions and keeps its requests.** `PATCH {open: false}` closes
   it: `GET` renders *This form is closed* with the team's name and `POST` is 410 `form_closed`.
   A form is never deleted, since requests name it. An edit to `fields` applies to later
   submissions only; a request's `answers` keep the field ids they were given, and the Triage view
   (task 170) labels an answer whose field was removed by its id.

#### Files

- `control-plane/internal/hub/server/work_forms.go` (new; the File map's row, skeleton `:254`): the
  four API routes, the two page handlers, and `formDefaults`'s body.
- `control-plane/internal/hub/store/work_forms.go` (new).
- `control-plane/internal/hub/server/templates/form.html` (new): `html/template`, no script, two
  named templates, `form` and `done`, embedded with `//go:embed`.
- `control-plane/internal/hub/server/work_forms_integration_test.go` (new, `-tags integration`).
- ⚑ `control-plane/internal/hub/server/work_pages.go` (task 80): two lines mounting the page routes
  on its browser session; nothing else changes.
- ⚑ `control-plane/internal/hub/server/work_intake_rules.go` (task 147): `formDefaults`'s stub body
  replaced; `checkRulesInTeam` gains the form-id check.
- ⚑ `control-plane/internal/hub/hubapi/work_intake.go` (task 143), ⚑ `hubapi/workids.go` (task 02,
  `fm-`), ⚑ `hubapi/workevents.go` (task 13), ⚑ `server/work_matrix_rows_integration_test.go`.

#### The table this task needs from task 145's migration (requirement; complete)

```sql
-- ── task 148 (in task 145's NNNN_work_w5.sql): intake forms ─────────────────────────
CREATE TABLE work_intake_forms (
  id          text        PRIMARY KEY CHECK (id ~ '^fm-[0-9a-f]{16}$'),
  org_id      text        NOT NULL REFERENCES orgs (id),
  team_id     text        NOT NULL,
  title       text        NOT NULL CHECK (char_length(title) BETWEEN 1 AND 120),
  intro       text        NOT NULL DEFAULT '' CHECK (octet_length(intro) <= 4096),
  fields      jsonb       NOT NULL CHECK (jsonb_typeof(fields) = 'array' AND jsonb_array_length(fields) BETWEEN 1 AND 30),
  defaults    jsonb       NOT NULL DEFAULT '{"member":{},"nonMember":{}}'::jsonb,
  open        boolean     NOT NULL DEFAULT true,
  version     integer     NOT NULL DEFAULT 1 CHECK (version >= 1),
  created_at  timestamptz NOT NULL,
  created_by  text        NOT NULL,
  updated_at  timestamptz NOT NULL,
  updated_by  text        NOT NULL,
  FOREIGN KEY (org_id, team_id)    REFERENCES teams (org_id, id),
  FOREIGN KEY (org_id, created_by) REFERENCES users (org_id, id),
  FOREIGN KEY (org_id, updated_by) REFERENCES users (org_id, id)
);
CREATE INDEX work_intake_forms_team ON work_intake_forms (team_id, created_at DESC, id DESC);
ALTER TABLE work_intake_forms ENABLE ROW LEVEL SECURITY;
ALTER TABLE work_intake_forms FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON work_intake_forms TO hub_app
  USING (org_id = current_setting('hub.org_id', true))
  WITH CHECK (org_id = current_setting('hub.org_id', true));
GRANT SELECT, INSERT ON work_intake_forms TO hub_app;
GRANT UPDATE (title, intro, fields, defaults, open, version, updated_at, updated_by) ON work_intake_forms TO hub_app;

-- task 146's work_requests.form_id names a form of the same org.
ALTER TABLE work_requests ADD CONSTRAINT work_requests_form_exists
  FOREIGN KEY (form_id) REFERENCES work_intake_forms (id);
```

A form's title, intro and labels are the team's own words about what it wants to hear, not a
requester's, so erase does not reach them (as it does not reach task 147's rules). `version`
counts edits so a `PATCH` can be optimistic (`baseVersion`, 409 `form_moved`), as task 147's PUT
is; it is not a history (old fields are not kept), and the scope asks for none.

#### `hubapi/work_intake.go`, the additions (requirement on task 143; complete)

```go
// ── Task 148: intake forms (SW.21) ──────────────────────────────────────────

const PrefixForm = "fm-" // decision 2; task 02's workids.go gains it

type FormFieldKind string

const (
	FieldText     FormFieldKind = "text"
	FieldLongtext FormFieldKind = "longtext"
	FieldSelect   FormFieldKind = "select"
	FieldDate     FormFieldKind = "date"
	FieldNumber   FormFieldKind = "number"
	FieldPerson   FormFieldKind = "person"
)

var FormFieldKinds = []FormFieldKind{FieldText, FieldLongtext, FieldSelect, FieldDate, FieldNumber, FieldPerson}

const QuoteFieldID = "quote" // decision 1

type FormField struct {
	ID       string        `json:"id"`    // ^[a-z][a-z0-9_]{0,39}$, unique in the form
	Label    string        `json:"label"` // 1-120 characters
	Help     string        `json:"help,omitempty"` // 0-300 characters
	Kind     FormFieldKind `json:"kind"`
	Required bool          `json:"required"`
	Options  []string      `json:"options,omitempty"` // select only: 1-50, each 1-120, distinct
}

type IntakeForm struct {
	ID        string         `json:"id"`
	TeamID    string         `json:"teamId"`
	Title     string         `json:"title"`
	Intro     string         `json:"intro"`
	Fields    []FormField    `json:"fields"`
	Defaults  IntakeDefaults `json:"defaults"` // task 147's type (L-28)
	Open      bool           `json:"open"`
	Version   int            `json:"version"`
	PageURL   string         `json:"pageUrl"` // https://<hub>/p/forms/fm-…, the link a team shares
	CreatedAt string         `json:"createdAt"`
	UpdatedAt string         `json:"updatedAt"`
	UpdatedBy User           `json:"updatedBy"`
}

type CreateFormRequest struct {
	Title    string         `json:"title"`
	Intro    string         `json:"intro"`
	Fields   []FormField    `json:"fields"`
	Defaults IntakeDefaults `json:"defaults"`
}

// PatchFormRequest: a nil field is unchanged. BaseVersion is required.
type PatchFormRequest struct {
	BaseVersion int             `json:"baseVersion"`
	Title       *string         `json:"title,omitempty"`
	Intro       *string         `json:"intro,omitempty"`
	Fields      *[]FormField    `json:"fields,omitempty"`
	Defaults    *IntakeDefaults `json:"defaults,omitempty"`
	Open        *bool           `json:"open,omitempty"`
}

const (
	MaxFormFields       = 30
	MaxFormTextChars    = 500
	MaxFieldLabelChars  = 120
	MaxFieldHelpChars   = 300
	MaxFieldOptions     = 50
	MaxFormIntroBytes   = 4096
	EventFormCreated    = "form.created"
	EventFormUpdated    = "form.updated"
)

// FormData: the form's id travels as the event's subject; the version and
// whether it is open are all the feed says. Titles and labels stay in the table.
type FormData struct {
	Version int64 `json:"version"`
	Open    bool  `json:"open"`
}
// Appended to WorkEventKinds:
//	{EventFormCreated, FormData{}, ScopeTeam, VisibleToReaders},
//	{EventFormUpdated, FormData{}, ScopeTeam, VisibleToReaders},
```

`form.` joins task 13's prefix list. A form's submissions are `request.created` events (task 146)
with `channel: form`; the form's own kinds only say it was made or changed. `PageURL` is built
from `PANACEA_APP_HUB_PUBLIC_URL`, task 80's variable for its page links (**UNVERIFIED** name
until task 80 lands).

#### The checks (`hubapi/work_forms.go`, new, pure, complete)

```go
package hubapi

var fieldIDRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,39}$`)
var numberRe = regexp.MustCompile(`^-?[0-9]{1,15}(\.[0-9]{1,6})?$`)

// ValidateFormFields: 1-30 fields, ids distinct, exactly one quote field that
// is a required longtext (decision 1), and each field's own shape.
func ValidateFormFields(fs []FormField) *FieldError {
	if len(fs) == 0 || len(fs) > MaxFormFields {
		return &FieldError{"fields", ErrBadEnvelope}
	}
	seen, quotes := map[string]bool{}, 0
	for i, f := range fs {
		at := func(s string) *FieldError { return &FieldError{fmt.Sprintf("fields[%d].%s", i, s), ErrBadEnvelope} }
		n := utf8.RuneCountInString
		switch {
		case !fieldIDRe.MatchString(f.ID) || seen[f.ID]:
			return at("id")
		case strings.TrimSpace(f.Label) != f.Label || n(f.Label) == 0 || n(f.Label) > MaxFieldLabelChars:
			return at("label")
		case n(f.Help) > MaxFieldHelpChars:
			return at("help")
		case !slices.Contains(FormFieldKinds, f.Kind):
			return at("kind")
		case (f.Kind == FieldSelect) != (len(f.Options) > 0) || len(f.Options) > MaxFieldOptions || !distinctOptions(f.Options):
			return at("options")
		}
		if f.ID == QuoteFieldID {
			if f.Kind != FieldLongtext || !f.Required {
				return at("kind")
			}
			quotes++
		}
		seen[f.ID] = true
	}
	if quotes != 1 {
		return &FieldError{"fields", ErrBadEnvelope}
	}
	return nil
}

func distinctOptions(os []string) bool {
	seen := map[string]bool{}
	for _, o := range os {
		if o == "" || strings.TrimSpace(o) != o || utf8.RuneCountInString(o) > MaxFieldLabelChars || seen[o] {
			return false
		}
		seen[o] = true
	}
	return true
}

// ErrFieldRequired and ErrFieldType are the two answers a submitter can get
// wrong; the page names the field with the code (form_field_required or
// form_field_invalid) and its label.
var (
	ErrFieldRequired = errors.New("hubapi: a required field is empty")
	ErrFieldType     = errors.New("hubapi: a field's value is not of its kind")
)

// CheckAnswers checks a submission against the form, in the form's field
// order, and splits it into the quote and the other answers. A person field
// is returned as the email given; the server resolves it to a u- id.
func CheckAnswers(form IntakeForm, in map[string]string) (quote string, answers map[string]string, fe *FieldError) {
	for _, k := range slices.Sorted(maps.Keys(in)) { // a stray field is refused, never stored
		if !slices.ContainsFunc(form.Fields, func(f FormField) bool { return f.ID == k }) {
			return "", nil, &FieldError{k, ErrBadEnvelope}
		}
	}
	answers = map[string]string{}
	for _, f := range form.Fields {
		v, given := in[f.ID]
		if f.Kind != FieldLongtext {
			v = strings.TrimSpace(v) // one-line kinds; a longtext is kept byte for byte
		}
		if !given || v == "" {
			if f.Required {
				return "", nil, &FieldError{f.ID, ErrFieldRequired}
			}
			continue // absent, never "" (decision 3)
		}
		if !valueFits(f, v) {
			return "", nil, &FieldError{f.ID, ErrFieldType}
		}
		if f.ID == QuoteFieldID {
			quote = v
		} else {
			answers[f.ID] = v
		}
	}
	return quote, answers, nil
}

func valueFits(f FormField, v string) bool {
	if !utf8.ValidString(v) {
		return false
	}
	switch f.Kind {
	case FieldText:
		return !strings.ContainsAny(v, "\r\n") && utf8.RuneCountInString(v) <= MaxFormTextChars
	case FieldLongtext:
		return len(v) <= MaxRequestQuoteBytes // task 146's cap, the description's
	case FieldSelect:
		return slices.Contains(f.Options, v)
	case FieldDate:
		t, err := time.Parse(time.DateOnly, v)
		return err == nil && t.Format(time.DateOnly) == v // 2026-02-30 is refused
	case FieldNumber:
		return numberRe.MatchString(v)
	case FieldPerson:
		a, err := mail.ParseAddress(v)
		return err == nil && a.Name == "" && a.Address == v
	}
	return false
}
```

`slices.Sorted(maps.Keys(…))` and `time.DateOnly` need Go 1.23 and 1.20; the module is `go 1.23`
(skeleton `:175`), so both hold. `time.Parse` already refuses day 30 of February (`day out of
range`); the round-trip comparison is the second layer and also refuses a value with a zone or
time appended. The file imports `errors`, `fmt`, `maps`, `net/mail`, `regexp`, `slices`,
`strings`, `time` and `unicode/utf8`, all standard library.

`CheckAnswers` returns the **first** failure in the form's field order, so the page names one
field at a time from the top, which is where a person reads from. The page re-renders every value
given (escaped) so nothing typed is lost.

#### The store (`store/work_forms.go`, signatures complete)

```go
package store

func InsertForm(ctx context.Context, tx pgx.Tx, f hubapi.IntakeForm, org, by string, at time.Time) error

// FormByID reads a form under row-level security alone: the page handler
// needs a form of the org whatever the caller's teams (decision 5).
func FormByID(ctx context.Context, q Querier, id string) (hubapi.IntakeForm, bool, error)

func ListForms(ctx context.Context, q Querier, teamID string, pr hubapi.PageRequest) ([]hubapi.IntakeForm, *string, error)

// UpdateForm writes the patched form as version base+1:
//   UPDATE work_intake_forms SET …, version = version + 1 WHERE id = $1 AND version = $2
// Zero rows is ErrFormMoved.
func UpdateForm(ctx context.Context, tx pgx.Tx, f hubapi.IntakeForm, base int, by string, at time.Time) error

var ErrFormMoved = errors.New("store: the form moved on")

// UserByEmail resolves a person field: a user of the caller's org whose
// address is email, compared case-insensitively. found is false otherwise.
func UserByEmail(ctx context.Context, q Querier, email string) (userID string, found bool, err error)
```

`UserByEmail` reads the Hub's `users` table; that it carries a mail column is **UNVERIFIED** until
Hub task 05 lands (task 149 needs the same read to turn a sender into a `person`).

#### The routes (`server/work_forms.go`)

```go
package server

//go:embed templates/form.html
var formHTML string

var formPages = template.Must(template.New("form").Parse(formHTML)) // html/template: escaped by context

func (s *Server) registerWorkForms() {
	s.mux.HandleFunc("POST /v1/teams/{teamId}/forms", s.postForm) // team admin; 201
	s.mux.HandleFunc("GET /v1/teams/{teamId}/forms", s.listForms) // viewer
	s.mux.HandleFunc("GET /v1/forms/{formId}", s.getForm)         // viewer of the form's team
	s.mux.HandleFunc("PATCH /v1/forms/{formId}", s.patchForm)     // team admin; baseVersion
}

// In task 80's work_pages.go, on its browser session (two lines; UNVERIFIED names):
//	pages.HandleFunc("GET /p/forms/{formId}", s.formPage)
//	pages.HandleFunc("POST /p/forms/{formId}", s.submitForm)

// formDefaults replaces task 147's stub. A form of another team never reaches
// here: createRequest's caller passes the form's own team.
func (s *Server) formDefaults(ctx context.Context, tx *Tx, formID string) (*hubapi.IntakeDefaults, error) {
	f, ok, err := store.FormByID(ctx, tx, formID)
	if err != nil || !ok {
		return nil, err
	}
	return &f.Defaults, nil
}
```

`postForm` and `patchForm` run, in `s.Write`: `Decode`; the team role (`admin`, else 403; not in
the team 404 `team_not_found` or `form_not_found`); `ValidateFormFields`; the title trimmed 1-120
characters and the intro at most 4 KiB; **the defaults checked as a rule's route is**, by task
147's `checkRulesInTeam` logic for one route (`defaults.member.projectId`,
`defaults.nonMember.labelIds[k]`); insert or `UpdateForm` (409 `form_moved {version}`); emit
`form.created` or `form.updated {version, open}`; 201 or 200 with the form. `getForm` and
`listForms` are reads for any role in the form's team.

#### The page handlers (`server/work_forms.go`, complete)

```go
const pageCSP = "default-src 'none'; form-action 'self'; style-src 'self'; frame-ancestors 'none'; base-uri 'none'"

func pageHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Content-Security-Policy", pageCSP)
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("Cache-Control", "no-store")
}

type formView struct {
	Form   hubapi.IntakeForm
	Team   string            // the team's display name
	CSRF   string            // the session's token (task 80)
	Values map[string]string // what was typed, re-rendered escaped on a refusal
	Error  *pageError        // nil on first render
	Closed bool
}
type pageError struct{ Code, FieldID, Label string }

// submitForm takes one submission. The session is task 80's: sess.P is the
// signed-in principal, sess.CSRF its token. Every refusal writes nothing.
func (s *Server) submitForm(w http.ResponseWriter, r *http.Request, sess *PageSession) {
	pageHeaders(w)
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "The form could not be read.", http.StatusBadRequest)
		return
	}
	if subtle.ConstantTimeCompare([]byte(r.PostForm.Get("csrf")), []byte(sess.CSRF)) != 1 {
		http.Error(w, "This form has expired. Open the link again.", http.StatusForbidden)
		return
	}
	in := map[string]string{}
	for k, vs := range r.PostForm {
		if id, ok := strings.CutPrefix(k, "f."); ok && len(vs) == 1 {
			in[id] = vs[0]
		} else if k != "csrf" {
			http.Error(w, "The form could not be read.", http.StatusBadRequest) // a repeated or stray name
			return
		}
	}
	s.PageWrite(w, r, sess, func(tx *Tx) (int, string, any, error) { // task 80's page twin of s.Write
		ctx := r.Context()
		f, ok, err := store.FormByID(ctx, tx, r.PathValue("formId"))
		if err != nil || !ok {
			return http.StatusNotFound, "", nil, err // another org's form reads as none (decision 5)
		}
		if !f.Open {
			return http.StatusGone, "form", formView{Form: f, Closed: true}, nil
		}
		quote, answers, fe := hubapi.CheckAnswers(f, in)
		if fe == nil {
			fe, err = s.resolvePeople(ctx, tx, f, answers) // person fields: email → u- id
		}
		if err != nil || fe != nil {
			return http.StatusBadRequest, "form", formView{Form: f, CSRF: sess.CSRF, Values: in,
				Error: errorFor(f, fe)}, err
		}
		rq, err := s.createRequest(ctx, tx, store.RequestRow{TeamID: f.TeamID, Channel: string(hubapi.ChannelForm),
			RequesterKind: string(hubapi.RequesterPerson), RequesterUserID: tx.P.UserID, Quote: quote,
			Answers: answers, FormID: f.ID, CreatedBy: tx.P.UserID})
		if err != nil {
			return 0, "", nil, err
		}
		return http.StatusOK, "done", map[string]string{"RequestID": rq.ID, "Title": f.Title}, nil
	})
}
```

`PageSession` and `PageWrite` stand for task 80's session and its write wrapper (the org's write
lock, `hub.org_id` set, the principal on `tx.P`, the named template rendered with the status, and a
500 page with no detail on an error); both names are **UNVERIFIED** until task 80's part lands. A
`/v1` route never sees `PageSession`: the page mux is a separate `http.ServeMux` mounted at `/p/`
(task 80's rule), which is what `TestThePageCookieIsNotAnAPIToken` proves from the outside.
`errorFor` maps `ErrFieldRequired` to `form_field_required` and `ErrFieldType` or a stray id to
`form_field_invalid`, with the field's label. `formPage` (`GET`) renders `form` with a fresh view,
or `form` with `Closed` and status 410, or 404 for a form that is not in the org.

**`frame-ancestors 'none'` and `base-uri 'none'` are added to the scope's CSP.** `default-src` does
not govern either: without the first, another site can frame the form and trick a signed-in person
into submitting it (the `SameSite=Strict` cookie already stops the submit; this stops the framing);
without the second, an injected `<base>` could redirect the form's relative action. Both cost
nothing for a page with no frames and no base.

#### `server/templates/form.html` (complete)

```html
{{define "form"}}<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Form.Title}}</title><link rel="stylesheet" href="/p/static/page.css"></head>
<body class="page"><main class="page-main">
  <p class="page-team">{{.Team}}</p>
  <h1>{{.Form.Title}}</h1>
  {{if .Closed}}<p>This form is closed.</p>{{else}}
  {{with .Form.Intro}}<p class="page-intro">{{.}}</p>{{end}}
  {{with .Error}}<p class="page-error" role="alert" id="err">{{.Label}}: {{if eq .Code "form_field_required"}}this is required.{{else}}this is not a valid answer.{{end}}
    <span class="page-code">{{.Code}}</span></p>{{end}}
  <form method="post" action="" accept-charset="utf-8">
    <input type="hidden" name="csrf" value="{{.CSRF}}">
    {{$v := .Values}}{{$e := .Error}}
    {{range .Form.Fields}}<div class="page-field">
      <label for="f-{{.ID}}">{{.Label}}{{if .Required}} <span class="page-req">required</span>{{end}}</label>
      {{with .Help}}<p class="page-help">{{.}}</p>{{end}}
      {{if eq .Kind "longtext"}}<textarea id="f-{{.ID}}" name="f.{{.ID}}" rows="6"{{if .Required}} required{{end}}>{{index $v .ID}}</textarea>
      {{else if eq .Kind "select"}}<select id="f-{{.ID}}" name="f.{{.ID}}"{{if .Required}} required{{end}}>
        <option value=""></option>{{$cur := index $v .ID}}{{range .Options}}<option{{if eq . $cur}} selected{{end}}>{{.}}</option>{{end}}</select>
      {{else if eq .Kind "date"}}<input id="f-{{.ID}}" name="f.{{.ID}}" type="date" value="{{index $v .ID}}"{{if .Required}} required{{end}}>
      {{else if eq .Kind "person"}}<input id="f-{{.ID}}" name="f.{{.ID}}" type="email" autocomplete="off" value="{{index $v .ID}}" placeholder="name@kpmg.com.au"{{if .Required}} required{{end}}>
      {{else}}<input id="f-{{.ID}}" name="f.{{.ID}}" type="text"{{if eq .Kind "number"}} inputmode="decimal"{{end}} value="{{index $v .ID}}"{{if .Required}} required{{end}}>{{end}}
    </div>{{end}}
    <button type="submit" class="page-submit">Send</button>
  </form>{{end}}
</main></body></html>{{end}}

{{define "done"}}<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>{{.Title}}</title><link rel="stylesheet" href="/p/static/page.css"></head>
<body class="page"><main class="page-main">
  <h1>We have it.</h1>
  <p>You will hear by email when it ships.</p>
  <p class="page-fact">Request <code>{{.RequestID}}</code></p>
</main></body></html>{{end}}
```

`/p/static/page.css` is task 80's stylesheet for its read-only page (Commit Mono for the request
id in `code`, Segoe UI Variable for the text, `OD-28`; **UNVERIFIED** path until task 80 lands).
`action=""` posts to the page's own URL, which `form-action 'self'` allows. The browser's
`required` attribute is a courtesy that a person can bypass; `CheckAnswers` is the check. A
`number` field is `type="text"` with `inputmode="decimal"`, not `type="number"`: a browser's number
input rewrites `1,5` by locale and drops what it cannot parse, and the requester's value should
reach the server as typed. The error names its code in a `page-code` span so the requester can
quote it to the team.

#### Wire shapes

```jsonc
// POST /v1/teams/tm-…/forms                              team admin; 201
{ "title": "Ask the payments team", "intro": "Tell us what you need and why.",
  "fields": [
    { "id": "quote", "label": "What do you need?", "kind": "longtext", "required": true },
    { "id": "needed_by", "label": "Is there a date it matters by?", "kind": "date", "required": false },
    { "id": "area", "label": "Area", "kind": "select", "required": true, "options": ["Uploads", "Billing"] } ],
  "defaults": { "member": { "priority": "medium" }, "nonMember": { "priority": "low", "important": false } } }
// → 201 { "id": "fm-2b7c…", "version": 1, "open": true, "pageUrl": "https://hub…/p/forms/fm-2b7c…", … }

// POST /p/forms/fm-2b7c…      (application/x-www-form-urlencoded, the session cookie)
//   csrf=…&f.quote=Can+uploads+resume%3F&f.area=Uploads
// → 200 text/html: "We have it." … "Request rq-5c1e…"
// the request it made (GET /v1/requests/rq-5c1e…):
{ "channel": "form", "formId": "fm-2b7c…", "requester": { "kind": "person", "userId": "u-…", "displayName": "…" },
  "quote": { "text": "Can uploads resume?", "author": "H" }, "answers": { "area": "Uploads" },
  "route": { "priority": "low" }, "rule": null, … }

// refusals
400 bad_request   { "field": "fields" }            // no quote field, or two
400 bad_request   { "field": "fields[0].kind" }    // quote as text, or not required
400 bad_request   { "field": "defaults.member.projectId" } // another team's project
409 form_moved    { "version": "3" }               // PATCH on a stale baseVersion
// the page: 400 re-rendered with "Area: this is required. form_field_required"; 403 no csrf;
//           410 closed; 404 another org's form or none
```

#### Tests to write first, part 1 — `hubapi/work_forms_test.go` (pure)

| Test | Holds | Mutant it kills |
|---|---|---|
| `TestAFormNeedsOneQuoteField` | no `quote` → `fields`; two → `fields`; `quote` as `text` or optional → `fields[i].kind` | a request with no verbatim quote |
| `TestFieldShapes` | an id `Area`, a duplicate id, a padded label, a select with no options, a text with options → each its `fields[i].…` | a field shape unchecked |
| `TestCheckAnswersNamesTheFirstMissing` | two required fields empty → `FieldError{first id, ErrFieldRequired}` in form order | partial request; the wrong field named |
| `TestDatesAreRealDates` | `2026-09-26` passes; `tomorrow`, `26/09/2026`, `2026-02-30`, `2026-09-26T00:00:00Z` fail | types unchecked on the server |
| `TestNumbersAreKeptAsTyped` | `-12.5` passes and is stored `-12.5`; `1e3`, `1,5`, `NaN`, 16 digits fail | a float round trip; exponent accepted |
| `TestSelectIsOneOfItsOptions` | `Uploads` passes; `uploads` and `Other` fail | a select taking free text |
| `TestTheQuoteIsKeptByteForByte` | a longtext of `"  line\r\n"` comes back as given; a text field is trimmed | the quote trimmed |
| `TestAStrayFieldIsRefused` | `in` with `member: "true"` → `FieldError{"member", …}` | a stray answer stored (and membership read from the client, task 147) |
