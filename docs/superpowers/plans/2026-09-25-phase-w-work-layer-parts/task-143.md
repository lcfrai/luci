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
