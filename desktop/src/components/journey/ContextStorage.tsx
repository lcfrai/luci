import { useEffect, useRef, useState } from "react";
import { productApi, errorText } from "../../api/product";
import { projectContextApi, type ProjectContext, type ContextArchivePage } from "../../api/projectContext";
import { useProjectAccess } from "../../hooks/useProjectAccess";
import { useProjectDraftScope } from "../../hooks/useProjectDraft";
import { Button } from "../fluid/components/button";
import { WorkflowDialog } from "../common/WorkflowDialog";
import { ContextHistory } from "./ContextHistory";
import "./context-storage.css";

const bytes = (value: number) => value < 1024 * 1024 ? `${Math.ceil(value / 1024)} KiB` : `${(value / (1024 * 1024)).toFixed(1)} MiB`;
const exportStatus = { current: "Up to date", missing: "Missing", changed: "Changed outside the editor", unavailable: "Cannot read file" };
function download(blob: Blob, name: string) {
  const url = URL.createObjectURL(blob), link = document.createElement("a");
  link.href = url; link.download = name; document.body.appendChild(link); link.click(); link.remove();
  setTimeout(() => URL.revokeObjectURL(url), 30_000);
}
export function ContextStorage({ projectId }: { projectId: string }) {
  const scope = useProjectDraftScope(projectId);
  return <ProjectContextStorage key={`${scope}:${projectId}`} projectId={projectId} />;
}
function ProjectContextStorage({ projectId }: { projectId: string }) {
  const [open, setOpen] = useState(false);
  return <details className="context-storage" onToggle={event => setOpen(event.currentTarget.open)}>
    <summary>Context storage and history</summary>
    {open && <ContextStorageContent projectId={projectId} />}
  </details>;
}
function ContextStorageContent({ projectId }: { projectId: string }) {
  const access = useProjectAccess(projectId);
  const { canRead, canEdit } = access;
  const [context, setContext] = useState<ProjectContext | null>(null), [error, setError] = useState("");
  const [busy, setBusy] = useState(""), [notice, setNotice] = useState("");
  const [repairOpen, setRepairOpen] = useState(false), [backupDigest, setBackupDigest] = useState("");
  const [acknowledged, setAcknowledged] = useState(false);
  const [page, setPage] = useState<ContextArchivePage | null>(null), [previous, setPrevious] = useState<string[]>([]);
  const alive = useRef(true), working = useRef(false);
  function verify(value: ProjectContext) {
    if (value.projectId !== projectId) throw new Error("This context belongs to another project. Refresh the current project.");
    return value;
  }
  async function run(label: string, action: () => Promise<void>) {
    if (working.current) return;
    working.current = true; setBusy(label); setError(""); setNotice("");
    try { await action(); } catch (cause) { if (alive.current) setError(errorText(cause)); }
    finally { working.current = false; if (alive.current) setBusy(""); }
  }
  function apply(value: ProjectContext) {
    verify(value);
    if (!alive.current) return;
    setContext(value); setBackupDigest(""); setAcknowledged(false); setPage(null); setPrevious([]);
  }
  async function refresh() { if (!canRead) return; await run("Refreshing context…", async () => apply(await projectContextApi.get(projectId))); }
  useEffect(() => {
    alive.current = true; void refresh();
    return () => { alive.current = false; };
  }, [projectId, canRead || !!context]);
  async function archive(cursor: string, direction: "first" | "next" | "previous" = "next") {
    if (!context || !cursor) return;
    const selected = context;
    await run("Reading archived revisions…", async () => {
      const value = await projectContextApi.archive(projectId, cursor);
      if (!alive.current) return;
      // Imported snapshots retain their original project ID; stable context ID
      // and the exact reachable digest are the archive's ownership boundary.
      if (value.digest !== cursor || !value.chunk || value.chunk.contextId !== selected.contextId)
        throw new Error("This archive page does not match the selected project context. Refresh before trying again.");
      setPrevious(direction === "first" ? [] : direction === "previous" ? previous.slice(0, -1) : page ? [...previous, page.digest] : []); setPage(value);
    });
  }
  const changed = context?.exports.filter(item => item.status !== "current") ?? [];
  const backedUp = !!context && backupDigest === context.expectedDigest;
  const removableArchive = !!context && (context.archiveChunks > 0 || context.unusedArchiveChunks > 0) && !context.archiveError;
  const archiveAvailable = !!context?.archiveHead && !context.archiveError;
  return <section aria-label="Project context storage">
    <p>Saved project context travels with your source. Generated files keep people and agents on the same definition, design and plan.</p>
    {access.message && <p role="status" className="journey-muted">{access.message}</p>}
    {error && <p className="journey-error" role="alert">{error}</p>}
    {notice && <p className="journey-success" role="status">{notice}</p>}
    {busy && <p role="status">{busy}</p>}
    <Button type="button" variant="secondary" disabled={!!busy} onClick={() => void refresh()}>Refresh context storage</Button>
    {context && canRead && <>
      {context.imported && <p className="journey-muted">This project carries imported context. Its historical review records do not grant current approvals; review the current definition and design in this workspace.</p>}
      <h3>Files shared with agents</h3>
      <ul className="context-export-list">{context.exports.map(item => <li key={item.path}><code>{item.path}</code><span>{exportStatus[item.status]}</span></li>)}</ul>
      {!!changed.length && <Button type="button" variant="secondary" disabled={!canEdit || !!busy} onClick={() => setRepairOpen(true)}>Review export repair</Button>}
      <h3>Saved revision archive</h3>
      <p>{context.archiveRevisions} archived revisions · {context.recentRevisions} recent revisions · {bytes(context.archiveBytes + context.unusedArchiveBytes)} of {bytes(context.archiveBytesLimit)} used.</p>
      <p className="journey-muted">{context.archiveChunks + context.unusedArchiveChunks} of {context.archiveChunksLimit} archive files. Older revisions are kept until you explicitly remove them. Current documents and recent revisions remain available.</p>
      {context.archiveError && <p className="journey-error" role="alert">{context.archiveError}</p>}
      {archiveAvailable && <Button type="button" variant="secondary" disabled={!!busy} onClick={() => void archive(context.archiveHead, "first")}>Browse archived revisions</Button>}
      {page?.chunk && <section className="context-archive" aria-label="Archived project revisions">
        <p className="journey-muted">These are historical snapshots. They do not grant current approval.</p>
        {([ ["Brief", page.chunk.briefRevisions ?? []], ["Sketch", page.chunk.designRevisions ?? []], ["Definition", page.chunk.specificationRevisions ?? []], ["Design handoff", page.chunk.designPlanRevisions ?? []] ] as const).map(([label, items]) => items.length > 0 && <div key={label}><h4>{label}</h4><ul>{items.map((item, index) => <li key={`${item.revision}:${index}`}>Revision {item.revision}{item.updatedAt ? ` · ${new Date(item.updatedAt).toLocaleString()}` : ""}{"purpose" in item && item.purpose ? ` · ${item.purpose}` : ""}</li>)}</ul></div>)}
        <div className="journey-actions"><Button type="button" variant="secondary" disabled={!!busy || !previous.length} onClick={() => void archive(previous[previous.length - 1], "previous")}>Newer archive page</Button><Button type="button" variant="secondary" disabled={!!busy || !page.nextCursor} onClick={() => void archive(page.nextCursor)}>Older archive page</Button><Button type="button" variant="tertiary" disabled={!!busy} onClick={() => download(new Blob([JSON.stringify(page.chunk, null, 2)], { type: "application/json" }), `luci-context-${page.digest}.json`)}>Download these revisions</Button></div>
      </section>}
      {context.unusedArchiveChunks > 0 && <p className="journey-warning">{context.unusedArchiveChunks} unused archive files from an interrupted save or cleanup use another {bytes(context.unusedArchiveBytes)}. Back them up before removing them below.</p>}
      {removableArchive && <details className="context-prune"><summary>Free archive space</summary>
        <p>Download a project backup before removing archived snapshots. Keep that file private: it includes your source and Git history. Save any browser drafts first to include those edits. Runtime databases and volumes need separate backups.</p>
        <Button type="button" variant="secondary" disabled={!canEdit || !!busy} onClick={() => void run("Preparing context backup…", async () => {
          if (!canEdit) return;
          const selectedDigest = context.expectedDigest;
          setBackupDigest(""); setAcknowledged(false);
          const blob = await productApi.backupProject(projectId);
          if (!alive.current) return;
          const current = verify(await projectContextApi.get(projectId));
          if (!alive.current) return;
          download(blob, `luci-${projectId}-context-backup.zip`);
          apply(current);
          if (current.expectedDigest !== selectedDigest) { setNotice("The saved context changed while preparing the backup. Download a new backup before removing archived revisions."); return; }
          setBackupDigest(selectedDigest); setNotice("Backup download started. Confirm that you saved the file before removing archived revisions.");
        })}>Download backup before removal</Button>
        <label className="setup-check"><input type="checkbox" disabled={!canEdit || !!busy || !backedUp} checked={acknowledged} onChange={event => setAcknowledged(event.target.checked)} />I saved the backup and want to remove archived revisions from this project.</label>
        <Button type="button" variant="secondary" disabled={!canEdit || !!busy || !backedUp || !acknowledged} onClick={() => void run("Removing archived revisions…", async () => {
          if (!canEdit || !backedUp || !acknowledged) return;
          const value = await projectContextApi.prune(projectId, context.expectedDigest);
          if (!alive.current) return;
          apply(value); setNotice("Archived revisions removed. Current documents and recent revisions are retained.");
        })}>Remove archived revisions</Button>
      </details>}
      <ContextHistory projectId={projectId} />
    </>}
    {repairOpen && context && canEdit && <WorkflowDialog title="Repair shared context files?" busy={!!busy} description="Regenerate these files from the saved project context. Edits made directly to these generated files will be replaced. This keeps current revisions and approvals unchanged." onClose={() => { if (!busy) setRepairOpen(false); }}>
      <ul>{changed.map(item => <li key={item.path}><code>{item.path}</code> · {exportStatus[item.status]}</li>)}</ul>
      <div className="journey-actions"><Button type="button" variant="secondary" disabled={!!busy} onClick={() => setRepairOpen(false)}>Keep existing files</Button><Button type="button" disabled={!!busy} onClick={() => void run("Repairing shared context files…", async () => {
        if (!canEdit) return;
        const value = await projectContextApi.repair(projectId, context.expectedDigest);
        if (!alive.current) return;
        apply(value); setRepairOpen(false); setNotice("Shared context files now match the saved project.");
      })}>Repair context exports</Button></div>
    </WorkflowDialog>}
  </section>;
}
