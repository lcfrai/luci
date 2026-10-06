import { useState } from "react";
import type { ProjectDraftState } from "../../hooks/useProjectDraft";
import { Button } from "../fluid/components/button";
import "./draft-recovery.css";

export function DraftRecovery<T>({ draft, label, base, saved, busy, readOnly, onRestore, onDiscard }: {
  draft: ProjectDraftState<T>; label: string; base: string | null; saved: T | null; busy?: boolean; readOnly?: boolean;
  onRestore(value: T): void; onDiscard?(): void;
}) {
  const [reviewed, setReviewed] = useState("");
  const pending = draft.pending, changed = !!pending && !!base && pending.base !== base;
  const reviewKey = JSON.stringify([base, pending?.updatedAt]);
  return <div className="draft-recovery">
    {draft.error && <p className="journey-warning" role={draft.error.startsWith("Draft recovery is unavailable") ? "status" : "alert"}>{draft.error}</p>}
    {pending ? <section aria-label={`${label} draft recovery`} className="draft-recovery-panel">
      <strong>Recover your {label.toLowerCase()} draft</strong>
      <p>Unsaved edits from {new Date(pending.updatedAt).toLocaleString()} are available in this browser. The saved project has not been changed.</p>
      {changed && <p className="journey-warning">The saved version changed after this draft began. Review both versions before continuing; saving will still check the current revision.</p>}
      <details><summary>Compare draft with saved version</summary><div className="draft-comparison"><div><h3>Saved version</h3><pre>{JSON.stringify(saved, null, 2)}</pre></div><div><h3>Your draft</h3><pre>{JSON.stringify(pending.value, null, 2)}</pre></div></div></details>
      {changed && <label className="setup-check"><input type="checkbox" checked={reviewed === reviewKey} onChange={event => setReviewed(event.target.checked ? reviewKey : "")} />I reviewed the saved version and want to continue with this draft.</label>}
      <div className="journey-actions"><Button type="button" disabled={busy || readOnly || !base || (changed && reviewed !== reviewKey)} onClick={() => { const restored = draft.take(); if (restored) onRestore(restored.value); }}>Restore draft</Button><Button type="button" variant="secondary" disabled={busy} onClick={() => { draft.clear(); onDiscard?.(); }}>Discard browser draft</Button></div>
    </section> : draft.retained ? <p className="journey-muted" role="status">{draft.restored ? "Recovered draft" : "Unsaved changes"} · kept in this browser for up to 30 days. Save to share with your project.</p> : null}
  </div>;
}
