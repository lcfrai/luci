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
