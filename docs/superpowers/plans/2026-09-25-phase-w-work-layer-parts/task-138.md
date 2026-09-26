### Task 138: assists, part 3: natural-language filters as editable chips

W4, control plane and desktop, about 2.5 hours (an estimate, not a measurement). Depends on task
91 (the view editor, *Save as view*, the replica's filter evaluation) and task 136 (the assist
client, the kill switch refusal `assists_off`, the route by the project's data class). **Neither
part is on this tree on 2026-09-26**, nor is task 83's (the desktop `ViewFilter`) or task 72's (the
Go one): every name this part takes from them is **UNVERIFIED**, listed in the table below, and
their parts win at build. The names from task 136 are the ones task 139 already assumed
(`task-139.md:79-89`), so the two parts rename together. Reads task 20's `expectNoPrediction`.

Closes **L-100** (05 §1 G, `05-design.md:234`: *"Parsed through the gateway and shown as editable
filter chips before they apply"*).

**What the reader sees** (05 §1 G L-100, §2.4; AX Principle 7):

- **The list header gains a bar**, *Filter in words*, above task 91's filter groups. Priya types
  *"bugs assigned to me updated last week"* and presses Enter.
- **The words become chips, pending.** *label is Bug* · *assignee is Priya* · *updated last week
  (21 Sep – 27 Sep)*. Pending chips have a dashed ring and the list below **does not change**. The
  typed text stays in the bar, above the chips, so the person can compare what they said with what
  was read.
- **Each chip edits and removes.** Clicking a chip opens its value in place: the values of that
  field from the team's own vocabulary, or the list of relative periods for a date. `×` removes it.
- **What was not used is said, not hidden.** A line under the chips: *"Not used: no label called
  'perf'."* One sentence per dropped phrase, each naming why.
- ***Apply* makes it an ordinary filter.** The chips become task 91's filter group, the list
  changes, and *Save as view* (task 91) keeps it. *Discard* clears the chips and leaves the text.
- **Failure keeps the words.** *"Could not read that as a filter."* with the text still in the bar
  and the list as it was; with the kill switch on, *"Filters in words are off for this org."* The
  ordinary filter groups keep working in both cases.

**Decisions made here, so nobody re-derives them:**

1. **The prompt carries the team's vocabulary and the person's words, nothing else.** State names,
   label names, member display names, cycle names and feature names of the team, each with a short
   opaque key (`s1`, `l1`, `m1`, `c1`, `f1`). No issue title, description, comment or brief text is
   read to build it. The control plane logs neither the words nor the reply.
2. **The model answers in names; code answers in ids.** The reply is `{op, clauses [{field,
   values?, within?, period?, phrase}]}`. Every value is looked up in the vocabulary by exact,
   case-insensitive name (or by its key); a value with no match is dropped and named in `dropped`
   (*"no label called 'perf'"*), and a clause left with no value is dropped whole.
3. **One flat group.** The bar yields one AND or OR group of clauses; nesting (task 91's L-99) is
   done after *Apply*, in the editor. A reply with nested groups is refused as a whole
   (*"Could not read that as a filter"*), not flattened, so no grouping is invented.
4. **Dates are relative, always.** A date clause (`updated`, `created`) carries exactly one of
   `within` (an ISO-8601 duration `P<n>D` or `P<n>W`, at most 365 days) or `period` (`today`,
   `yesterday`, `this_week`, `last_week`, `this_month`, `last_month`). A clause carrying any other
   key (`on`, `before`, `after`, `from`, `to`, `date`) is dropped with *"dates must be relative"*.
   The desktop's `resolve.ts` turns a relative clause into bounds **in the person's time zone** at
   each evaluation, so a saved view of *last week* stays *last week*.
5. **Weeks run Monday to Sunday**, the ISO week, in the person's zone; the bounds are half-open,
   `[Monday 00:00, next Monday 00:00)`. `within` is a rolling span of exact instants back from now.
6. **The route is the team's strictest project.** The vocabulary includes feature names, which
   belong to projects; the call goes on an `au-only` project's route when any project of the team
   is `au-only` (OD-19), and on the team's first project's route otherwise.
7. **The fields are the eight a person filters by most**: `state`, `label`, `assignee`, `priority`,
   `cycle`, `feature`, `updated`, `created`. Any other field is dropped with *"cannot filter by
   '<field>' in words yet"*; the editor still offers every field task 91 built.
8. **The words are 1 to 300 characters.** Longer or empty is `400 bad_request` (`httpx`'s existing
   code, `control-plane/internal/httpx/problem.go:46`), with no call to the gateway.
