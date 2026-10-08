import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, it, vi } from "vitest";
import { ApiError, type ReviewedContextDocument, type SessionReviewedContext } from "../../api/client";
import type { ImplementationContext, Session } from "../../store/types";
import { aSession } from "../../test/fixtures";
import { ReviewedContextNote } from "./ReviewedContextNote";

const state = vi.hoisted(() => ({ scope: "account-A:workspace-A", canRead: true, loading: false, error: "", get: vi.fn() }));
vi.mock("../../api", () => ({ api: { getSessionReviewedContext: state.get } }));
vi.mock("../../hooks/useProjectDraft", () => ({ useProjectDraftScope: () => state.scope }));
vi.mock("../../hooks/useProjectAccess", () => ({ useProjectAccess: () => ({ canRead: state.canRead, loading: state.loading, error: state.error, message: state.loading ? "Checking project permissions…" : "This project is not available with your current team permissions." }) }));

const context: ImplementationContext = { projectId: "p-1", worktreeId: "w-1", briefRevision: 2, planRevision: 3, specificationRevision: 4, designPlanRevision: 5, briefDigest: "brief", planDigest: "plan", specificationDigest: "definition", designPlanDigest: "design", taskIds: ["task-A"], taskTitles: ["Build the original garden"], requirementIds: ["R-A"], sourceCommit: "", sourceDigest: "source", sourceDirty: true };
const session = aSession({ id: "s-aaaaaaaa", projectId: "p-1", worktreeId: "w-1", title: "Garden attempt A", implementationContext: context, reviewedContextDelivery: { status: "captured", version: 1, digest: "a".repeat(64) } });
function response(kind: ReviewedContextDocument, value: unknown = { purpose: `Original ${kind}`, revision: 2 }, source: Session = session): SessionReviewedContext {
  return { sessionId: source.id, projectId: source.projectId, worktreeId: source.worktreeId, version: 1, digest: source.reviewedContextDelivery!.digest!, implementationContext: source.implementationContext!, document: { kind, sha256: "b".repeat(64), bytes: 100, value } };
}
function pending<T>() { let resolve!: (value: T) => void; const promise = new Promise<T>(done => { resolve = done; }); return { promise, resolve }; }
beforeEach(() => { state.scope = "account-A:workspace-A"; state.canRead = true; state.loading = false; state.error = ""; state.get.mockReset().mockImplementation(async (_id, kind) => response(kind)); });
const open = async () => { await userEvent.click(screen.getByRole("button", { name: "View reviewed context" })); return screen.findByRole("dialog", { name: "Reviewed context" }); };

it("reads four original documents only after deliberate actions and renders their content inertly", async () => {
  const authored = '<img src="https://example.invalid/private.png" onerror="alert(1)">';
  state.get.mockImplementation(async (_id, kind) => response(kind, { purpose: `Original ${kind}`, requirements: [{ id: "R-A", title: authored, acceptance: "Keep the original acceptance" }], revision: 2 }));
  render(<ReviewedContextNote session={session} canInspect />);
  expect(state.get).not.toHaveBeenCalled();
  const dialog = await open();
  await within(dialog).findByText("Original definition");
  expect(within(dialog).getByText("Brief 2 · Definition 4 · Design 5 · Plan 3")).not.toBeNull();
  expect(within(dialog).getByText("Build the original garden", { exact: false })).not.toBeNull();
  expect(within(dialog).getByText(authored)).not.toBeNull();
  expect(dialog.querySelector("img, iframe, a, textarea, input, [contenteditable=true]")).toBeNull();
  for (const [kind, label] of [["brief", "Brief"], ["design", "Design handoff"], ["plan", "Implementation plan"]] as const) {
    await userEvent.selectOptions(within(dialog).getByLabelText("Captured document"), label);
    await within(dialog).findByText(`Original ${kind}`);
  }
  expect(state.get.mock.calls).toEqual([[session.id, "definition"], [session.id, "brief"], [session.id, "design"], [session.id, "plan"]]);
  await userEvent.click(within(dialog).getByText("Recorded revision details"));
  await within(dialog).findByText("These identify this session's original captured document. Later amendments and current approvals are separate.");
  expect(within(dialog).queryByText(/Restoring a draft/)).toBeNull();
});

it("rejects a late previous document response and never substitutes current context after unavailable capture", async () => {
  const first = pending<SessionReviewedContext>(); state.get.mockReturnValueOnce(first.promise);
  render(<ReviewedContextNote session={session} canInspect />); const dialog = await open();
  await userEvent.selectOptions(within(dialog).getByLabelText("Captured document"), "Design handoff");
  await within(dialog).findByText("Original design");
  await act(async () => first.resolve(response("definition", { purpose: "Late old selection" })));
  expect(within(dialog).queryByText("Late old selection")).toBeNull();
  state.get.mockRejectedValueOnce(new ApiError("conflict", "The retained captured context is unavailable.", 409));
  await userEvent.selectOptions(within(dialog).getByLabelText("Captured document"), "Implementation plan");
  await within(dialog).findByRole("alert");
  expect(within(dialog).getByText("No current project document has been substituted.")).not.toBeNull();
  expect(within(dialog).queryByText("Original design")).toBeNull();
  await userEvent.click(within(dialog).getByRole("button", { name: "Retry captured document" }));
  await within(dialog).findByText("Original plan");
});

it.each(["session", "workspace", "permission"])("clears the open inspector and fences a late read when %s identity changes", async change => {
  const first = pending<SessionReviewedContext>(); state.get.mockReturnValueOnce(first.promise);
  const mounted = render(<ReviewedContextNote session={session} canInspect />); await open();
  if (change === "workspace") state.scope = "account-B:workspace-B";
  if (change === "permission") state.canRead = false;
  const next = change === "session" ? { ...session, id: "s-bbbbbbbb", title: "Other attempt" } : session;
  mounted.rerender(<ReviewedContextNote session={next} canInspect={state.canRead} />);
  expect(screen.queryByRole("dialog")).toBeNull();
  await act(async () => first.resolve(response("definition", { purpose: "Private old document" })));
  expect(screen.queryByText("Private old document")).toBeNull();
  expect(state.get).toHaveBeenCalledOnce();
});

it("waits for confirmed read access, clears displayed data on denial, and never needs execution access", async () => {
  state.loading = true;
  const mounted = render(<ReviewedContextNote session={session} canInspect />); const dialog = await open();
  expect(within(dialog).getByText("Checking project permissions…")).not.toBeNull();
  expect(state.get).not.toHaveBeenCalled();
  state.loading = false; mounted.rerender(<ReviewedContextNote session={session} canInspect />);
  await within(dialog).findByText("Original definition");
  state.canRead = false; mounted.rerender(<ReviewedContextNote session={session} canInspect />);
  expect(within(dialog).queryByText("Original definition")).toBeNull();
  expect(within(dialog).queryByRole("region", { name: "Captured session scope" })).toBeNull();
  expect(state.get).toHaveBeenCalledOnce();
});

it("preserves the viewed document through permission rechecks without automatically reading it again", async () => {
  const mounted = render(<ReviewedContextNote session={session} canInspect />); const dialog = await open();
  const content = await within(dialog).findByText("Original definition");
  state.loading = true; mounted.rerender(<ReviewedContextNote session={session} canInspect readPending />);
  expect(screen.getByRole("dialog", { name: "Reviewed context" })).toBe(dialog);
  expect(within(dialog).getByText("Original definition")).toBe(content);
  expect((within(dialog).getByLabelText("Captured document") as HTMLSelectElement).disabled).toBe(true);
  state.loading = false; mounted.rerender(<ReviewedContextNote session={session} canInspect />);
  await waitFor(() => expect((within(dialog).getByLabelText("Captured document") as HTMLSelectElement).disabled).toBe(false));
  expect(within(dialog).getByText("Original definition")).toBe(content);
  expect(state.get).toHaveBeenCalledOnce();
  await userEvent.selectOptions(within(dialog).getByLabelText("Captured document"), "Brief");
  await within(dialog).findByText("Original brief");
  expect(state.get.mock.calls).toEqual([[session.id, "definition"], [session.id, "brief"]]);
});

it.each(["sessionId", "digest", "document"])("refuses a response with mismatched %s", async field => {
  const value = response("definition");
  if (field === "sessionId") value.sessionId = "s-foreign";
  if (field === "digest") value.digest = "c".repeat(64);
  if (field === "document") value.document.kind = "plan";
  state.get.mockResolvedValue(value);
  render(<ReviewedContextNote session={session} canInspect />); const dialog = await open();
  await within(dialog).findByRole("alert");
  expect(within(dialog).queryByText("Original definition")).toBeNull();
});

it.each([undefined, { status: "legacy-live" as const }, { status: "captured" as const, version: 1, digest: "" }])("does not offer an inspector for legacy or malformed recorded delivery", receipt => {
  render(<ReviewedContextNote session={{ ...session, reviewedContextDelivery: receipt }} canInspect />);
  expect(screen.queryByRole("button", { name: "View reviewed context" })).toBeNull();
  expect(screen.getByRole("note", { name: "Agent context" }).textContent).toContain("no verified captured copy is recorded");
  expect(state.get).not.toHaveBeenCalled();
});

it("returns keyboard focus to its trigger after close without changing an unrelated draft", async () => {
  render(<><label>Unsaved task<input defaultValue="Keep my changes" /></label><ReviewedContextNote session={session} canInspect /></>);
  const trigger = screen.getByRole("button", { name: "View reviewed context" }); trigger.focus();
  await userEvent.keyboard("{Enter}"); const dialog = await screen.findByRole("dialog", { name: "Reviewed context" });
  await within(dialog).findByText("Original definition");
  await userEvent.keyboard("{Escape}");
  await waitFor(() => expect(document.activeElement).toBe(trigger));
  expect((screen.getByLabelText("Unsaved task") as HTMLInputElement).value).toBe("Keep my changes");
});

function versionTwoSession(): Session {
  return {...session, implementationContext: {...context, dependencyContextDigest: 'd'.repeat(64)}, reviewedContextDelivery: {...session.reviewedContextDelivery!, version: 2}} as Session;
}
it('recognizes version 2 captured inputs and reads the exact dependency-bound original document', async () => {
  const selected=versionTwoSession();
  state.get.mockImplementation(async(_id,kind)=>({...response(kind,{purpose:'Version 2 original definition'},selected),version:2}));
  render(<ReviewedContextNote session={selected} canInspect/>);
  expect(screen.getByRole('note',{name:'Agent context'}).textContent).toContain('Reviewed inputs were captured at start');
  const dialog=await open();await within(dialog).findByText('Version 2 original definition');
  expect(state.get).toHaveBeenCalledExactlyOnceWith(selected.id,'definition');
});
it.each(['wrong-version','missing-dependency','wrong-dependency'])('refuses version 2 %s rather than showing unbound context', async change => {
  const selected=versionTwoSession();
  const value={...response('definition',{purpose:'Unbound private document'},selected),version:change==='wrong-version'?1:2};
  if(change==='missing-dependency')value.implementationContext=context;
  if(change==='wrong-dependency')value.implementationContext={...selected.implementationContext!,dependencyContextDigest:'e'.repeat(64)} as ImplementationContext;
  state.get.mockResolvedValue(value);render(<ReviewedContextNote session={selected} canInspect/>);
  const dialog=await open();await within(dialog).findByRole('alert');expect(within(dialog).queryByText('Unbound private document')).toBeNull();
});
it.each([undefined,'','invalid'])('does not offer version 2 inspection without a valid saved dependency digest: %j', dependencyContextDigest => {
  const selected=versionTwoSession();selected.implementationContext={...context,dependencyContextDigest} as ImplementationContext;
  render(<ReviewedContextNote session={selected} canInspect/>);expect(screen.queryByRole('button',{name:'View reviewed context'})).toBeNull();expect(state.get).not.toHaveBeenCalled();
});
it('rejects a dependency-bearing response for a version 1 session',async()=>{
  state.get.mockResolvedValue({...response('definition',{purpose:'Wrong version content'}),implementationContext:{...context,dependencyContextDigest:'d'.repeat(64)}});
  render(<ReviewedContextNote session={session} canInspect/>);const dialog=await open();await within(dialog).findByRole('alert');expect(screen.queryByText('Wrong version content')).toBeNull();
});
it('closes a version 2 inspection when the saved dependency binding changes',async()=>{
  const selected=versionTwoSession();state.get.mockImplementation(async(_id,kind)=>({...response(kind,undefined,selected),version:2}));
  const view=render(<ReviewedContextNote session={selected} canInspect/>);await open();await screen.findByText('Original definition');
  view.rerender(<ReviewedContextNote session={{...selected,implementationContext:{...selected.implementationContext!,dependencyContextDigest:'e'.repeat(64)} as ImplementationContext}} canInspect/>);
  expect(screen.queryByRole('dialog')).toBeNull();
});
