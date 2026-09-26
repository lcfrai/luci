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
