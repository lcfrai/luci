import { useCallback, useEffect, useRef, useState } from "react";
import { useShallow } from "zustand/react/shallow";
import { projectBranchesApi, type BranchCatalogue, type BranchResult } from "../../api/projectBranches";
import { errorText } from "../../api/product";
import { ApiError } from "../../api/client";
import { useCockpit } from "../../store/store";
import type { BranchManagerTarget } from "../../store/slices/uiSlice";
import { WorkflowDialog } from "../common/WorkflowDialog";
import { Button } from "../fluid/components/button";

export function BranchManager() {
  const target = useCockpit(state => state.branchManager);
  return target ? <ProjectBranchManager key={`${target.projectId}:${target.action}:${target.branchId ?? ""}`} target={target} /> : null;
}

export function ProjectBranchManager({ target }: { target: BranchManagerTarget }) {
  const { projectId, action } = target;
  const alive = useRef(true), flight = useRef(false), generation = useRef(0);
  const intent = useRef<{ id: string; branchId: string; commit: string; name?: string; targetId?: string; targetCommit?: string } | null>(null);
  const [catalogue, setCatalogue] = useState<BranchCatalogue | null>(null);
  const [name, setName] = useState("");
  const [branchId, setBranchId] = useState(target.branchId ?? "");
  const [pending, setPending] = useState("loading"), [error, setError] = useState("");
  // A lost reply must be reconciled with the same durable operation, never retried blindly.
  const [unresolved, setUnresolved] = useState<string | null>(null);
  const close = useCockpit(state => state.closeBranchManager);
  const open = useCockpit(state => state.openBranchManager);
  const drafts = useCockpit(useShallow(state => Object.values(state.unsavedProjectDrafts).filter(draft => draft.projectId === projectId)));
  const contextDrafts = drafts.filter(draft => draft.purpose !== "verification-setup");
  const setupDrafts = drafts.filter(draft => draft.purpose === "verification-setup");
  const load = useCallback(async () => {
    const ticket = ++generation.current;
    const value = await projectBranchesApi.catalogue(projectId);
    if (!alive.current || ticket !== generation.current) return;
    if (value.projectId !== projectId || !Array.isArray(value.branches) || value.branches.some(branch => branch.projectId !== projectId))
      throw new Error("Luci could not verify the branch list. Refresh before changing source.");
    setCatalogue(value);
    setBranchId(current => current || value.branches.find(branch => branch.canSwitch)?.id || "");
  }, [projectId]);
  useEffect(() => {
    alive.current = true;
    void load().catch(cause => { if (alive.current) setError(errorText(cause)); }).finally(() => { if (alive.current) setPending(""); });
    return () => { alive.current = false; generation.current++; };
  }, [load]);
  const selected = catalogue?.branches.find(branch => branch.id === branchId);
  const running = catalogue?.operation?.status === "running";
  const busy = !!pending;
  const validName = /^[A-Za-z0-9][A-Za-z0-9._/-]{0,199}$/.test(name.trim()) && !name.includes("..") && !name.endsWith("/") && !name.endsWith(".") && !name.split("/").some(part => !part || part.startsWith(".") || part.endsWith(".lock"));
  const allowed = !!catalogue && catalogue.clean && /^[a-f0-9]{40}$/.test(catalogue.commit) && !!catalogue.currentBranchId && !drafts.length && !unresolved && !running &&
    (action === "create" ? catalogue.canCreate && validName && !catalogue.branches.some(branch => branch.name === name.trim()) : catalogue.canSwitch && !!selected?.canSwitch);

  async function accept(result: BranchResult, operationId: string) {
    const operation = result.operation;
    if (!operation || operation.id !== operationId || operation.projectId !== projectId || operation.action !== action)
      throw new Error("Luci could not verify the branch operation. Check its state before continuing.");
    const expected = intent.current;
    if (!expected || expected.id !== operationId || operation.sourceBranchId !== expected.branchId || operation.sourceCommit !== expected.commit ||
      (action === "create" ? operation.branch !== expected.name : operation.branchId !== expected.targetId || operation.targetCommit !== expected.targetCommit))
      throw new Error("This result does not match the requested branch change. Check the project before continuing.");
    if (operation.status !== "succeeded") {
      setUnresolved(operation.status === "failed" ? null : operationId);
      setError(operation.message || "The branch has not changed. Check the operation before continuing.");
      await load();
      return;
    }
    const { branch, worktree, commit } = result;
    if (!branch || !worktree || !commit || branch.projectId !== projectId || worktree.projectId !== projectId ||
      branch.id !== worktree.branchId || !worktree.isPrimary || branch.id !== operation.branchId || branch.name !== operation.branch || commit !== operation.commit)
      throw new Error("Luci could not verify the checked-out branch. Check its state before continuing.");
    const current = await projectBranchesApi.catalogue(projectId);
    if (!alive.current) return;
    if (current.projectId !== projectId || current.currentBranchId !== branch.id || current.commit !== commit ||
      !Array.isArray(current.branches) || current.branches.some(item => item.projectId !== projectId) ||
      !current.branches.some(item => item.id === branch.id && item.name === branch.name && item.isHead && item.commit === commit))
      throw new Error("The project changed after checkout. Check its current state before continuing.");
    const state = useCockpit.getState();
    state.acceptBranchChange(projectId, current.branches, worktree);
    state.pushToast(`Now working on ${branch.name}.`);
  }
  async function submit() {
    if (!allowed || busy || flight.current || !catalogue) return;
    flight.current = true; setPending(action); setError(""); generation.current++;
    const operationId = `branch-${crypto.randomUUID()}`;
    setUnresolved(operationId);
    const expected = { expectedBranchId: catalogue.currentBranchId, expectedCommit: catalogue.commit, operationId };
    intent.current = { id: operationId, branchId: catalogue.currentBranchId, commit: catalogue.commit,
      ...(action === "create" ? { name: name.trim() } : { targetId: branchId, targetCommit: selected!.commit }) };
    try {
      const result = action === "create" ? await projectBranchesApi.create(projectId, { ...expected, name: name.trim() }) :
        await projectBranchesApi.switch(projectId, { ...expected, branchId, expectedTargetCommit: selected!.commit });
      if (alive.current) await accept(result, operationId);
    } catch (cause) {
      if (alive.current) {
        if (cause instanceof ApiError && [400, 401, 403, 404, 409].includes(cause.status)) setUnresolved(null);
        setError(errorText(cause));
      }
    }
    finally { flight.current = false; if (alive.current) setPending(""); }
  }
  async function check() {
    if (busy || flight.current) return;
    flight.current = true; setPending("check"); setError("");
    try {
      const operationId = unresolved;
      if (operationId) await accept(await projectBranchesApi.operation(projectId, operationId), operationId);
      else await load();
    } catch (cause) {
      if (alive.current && cause instanceof ApiError && cause.status === 404 && unresolved) {
        // The catalogue's read gate must also answer: a still-running checkout
        // refuses that read, so a premature missing receipt never unlocks retry.
        try { await load(); if (alive.current) { setUnresolved(null); setError("No branch operation was started. Review the current state before trying again."); } }
        catch (readError) { if (alive.current) setError(errorText(readError)); }
      } else if (alive.current) setError(errorText(cause));
    }
    finally { flight.current = false; if (alive.current) setPending(""); }
  }
  return <WorkflowDialog title={action === "create" ? "Create a branch" : "Switch branch"}
    description={action === "create" ? "Give this change its own branch. Luci creates it from the current saved commit and opens it in your project." : "Open an existing branch in your project. Its files, definition and design will become the current source."}
    busy={busy} onClose={close}>
    {catalogue && <div className="workflow-facts"><span>Current: {catalogue.currentBranch || "No branch"}</span><code>{catalogue.commit?.slice(0, 8) || "No saved commit"}</code></div>}
    {error && <p className="workflow-error" role="alert">{error}</p>}
    {pending === "loading" && <p role="status">Reading your project…</p>}
    {!!drafts.length && <div className="workflow-alert">
      {!!contextDrafts.length && <p>Save your {contextDrafts.map(draft => draft.label).join(" and ")} edits before changing branches.</p>}
      {!!setupDrafts.length && <p>{setupDrafts.every(draft => draft.retained)
        ? "Your verification setup is kept in this browser. Close this dialog, change phase and choose Keep drafts and continue, or discard the setup before changing branches."
        : "Your verification setup is not retained in this browser. Keep editing to copy the command text you need, then discard the setup before changing branches."}</p>}
      <Button variant="secondary" disabled={busy} onClick={close}>Keep editing</Button>
    </div>}
    {catalogue?.message && <p className="workflow-alert">{catalogue.message}</p>}
    {catalogue && (!catalogue.clean || !catalogue.commit) && <Button variant="secondary" disabled={busy || !!unresolved || !!drafts.length} onClick={() => { close(); useCockpit.getState().setLens("merge"); }}>Open Git to commit changes</Button>}
    <form className="workflow-form" onSubmit={event => { event.preventDefault(); void submit(); }}>
      {action === "create" ? <label>Branch name<input required autoFocus maxLength={200} autoComplete="off" spellCheck={false} placeholder="feature/my-change" value={name} disabled={busy || !!unresolved} onChange={event => setName(event.target.value)} /><small>Use a short name without spaces, such as feature/my-change.</small></label> : <label>Branch<select value={branchId} disabled={busy || !!unresolved} onChange={event => setBranchId(event.target.value)}><option value="">Choose a branch</option>{catalogue?.branches.map(branch => <option key={branch.id} value={branch.id} disabled={!branch.canSwitch}>{branch.name}{branch.id === catalogue.currentBranchId ? " · current" : branch.checkedOut ? " · open elsewhere" : ""}</option>)}</select>{selected?.message && <small>{selected.message}</small>}</label>}
      <small>Commit changed files and stop active sessions first. Project data, secrets and published releases remain with the workspace.</small>
      <div className="workflow-actions">
        <Button variant="ghost" disabled={busy} onClick={close}>Cancel</Button>
        {action === "switch" && <Button variant="ghost" disabled={busy || !!unresolved} onClick={() => open({ projectId, action: "create" })}>New branch</Button>}
        <Button variant="secondary" loading={pending === "check"} disabled={busy} onClick={() => void check()}>{unresolved || running ? "Check branch state" : "Refresh branches"}</Button>
        <Button type="submit" loading={pending === action} disabled={busy || !allowed}>{action === "create" ? "Create and open branch" : "Switch branch"}</Button>
      </div>
    </form>
  </WorkflowDialog>;
}
