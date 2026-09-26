### Task 147: intake rules, first match wins with the author on the event; form defaults for members and non-members

W5, stream WH; no seam. Depends on task 146 (`createRequest`, whose `routeRequest` this task
fills, and `hubapi.IntakeRoute`). Estimate 0.3 (2.5 h). Closes, jointly with task 170 (the rules
and form editors): **L-49** (*"Intake rules, first match wins, each with its author on the
event"*, 05 §1) and **L-28** (form defaults for members and non-members). Every `server/` and
`store/` path is **UNVERIFIED until merge** (rule H1).

When a request arrives, something has to say where it should go: which project, what priority,
which labels, whether it is important. Two things say it, in this order. The **form's defaults**
(task 148), one set for team members and one for everyone else, and then the team's **intake
rules**, an ordered list whose **first** matching rule overrides the fields it sets. The answer is
a *suggestion* on the request (`route`), which the accept dialog (task 170) fills in; no rule
creates an issue, and no rule sends a request to another team. After this task a team admin can
set the rule list, every request says which rule routed it and **whose rule it was**, and editing
the rules never rewrites a request already routed.

**Task 145 is not written at this writing** (gap 1). The table below is this task's requirement on
it, as task 146 states `work_requests`; task 145's names win when it lands.

Seven decisions the scope left open, made here.

1. **The rule list is one versioned document.** `PUT` replaces the **whole ordered list** in one
   body and writes version *n + 1*; every older version stays, readable with `?version=`. A
   request records `(ruleId, ruleVersion)`, so *"rule `vip` of version 4"* names exactly the words
   that routed it, even after version 9 deleted `vip`. This is what *"editing rules leaves
   already-routed requests as they were"* means, and why no rule row is ever updated: the table
   has an `INSERT` grant and no `UPDATE`.
2. **The PUT is optimistic.** The body carries `baseVersion`; if another admin wrote first it is
   409 `intake_rules_moved {version}`, and the editor reloads. Two admins editing at once never
   lose each other's rules silently.
3. **A rule id is the editor's, not the Hub's.** Rule ids are short slugs, `^[a-z0-9][a-z0-9-]{0,39}$`,
   unique in one version, kept by the editor when a rule moves (task 170 mints `rule-1`, `rule-2`
   …). The Hub mints no id for a rule, so no new id prefix is needed (the skeleton's list,
   `:185-187`, has none for rules). `(ruleId, ruleVersion)` is the key.
4. **A rule matches on four fields, and a rule with none is refused.** `match` holds any of
   `channel`, `formId`, `senderDomain` and `textContains`; a rule matches when **every field it
   sets** holds. A rule that sets none would match everything and make every later rule dead, so
   PUT refuses it (`rules[n].match`); the catch-all is the form's defaults. `senderDomain` matches
   the address's domain or any subdomain of it, case-insensitively (`client.example` matches
   `mail.client.example`, never `evilclient.example`). `textContains` is a case-insensitive
   substring of the quote after `strings.ToLower` on both, **never a regular expression**, so no
   rule can be made slow; its Unicode limits (`ToLower` is not full case folding) are named in
   *does not claim*.
5. **Overrides are per field.** A rule's `route` overrides only the fields it sets: a non-empty
   `projectId` or `priority`, a non-empty `labelIds` (which **replaces** the default list, never
   unions it, so a rule can say exactly which labels), and `important: true`. A rule cannot unset
   *important*; a requester flagged important by a form stays so. A rule's route with no field set
   is refused (`rules[n].route`): it would record a match that did nothing.
6. **Membership is the Hub's.** `isMember` is true exactly when the requester is a `person` whose
   user is in the request's team (`team_members`), read inside the create transaction. An `email`
   or `teams` requester is a non-member, even when the address belongs to a member (task 149
   turns an address that matches an org user into a `person` first, so a member mailing in is a
   member). Nothing in any submission body can set it; a JSON `member: true` is an unknown key and
   task 148's decoder refuses it.
7. **A rule routes only inside its team.** Every `projectId` in a rule is a project of the team
   and every `labelIds` entry a live label of the team, checked at PUT (`rules[n].route.projectId`,
   `rules[n].route.labelIds[k]`). A project archived after the PUT is still in the stored version;
   the accept dialog shows it as archived and suggests nothing, and task 146's accept refuses it as
   it refuses any project not live in the team.
