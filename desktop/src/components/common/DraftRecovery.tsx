import { useState } from "react";
import type { ProjectDraftState } from "../../hooks/useProjectDraft";
import { mergeProjectDraft } from "../../utils/projectDraftMerge";
import { Button } from "../fluid/components/button";
import { DraftDocumentReview, draftFieldLabel, DraftReviewDisclosure } from "./DraftDocumentReview";
import "./draft-recovery.css";

export function DraftRecovery<T>({ draft, label, base, saved, merge = false, busy, readOnly, onRestore, onDiscard }: {
  draft: ProjectDraftState<T>; label: string; base: string | null; saved: T | null; busy?: boolean; readOnly?: boolean;
  merge?: boolean;
  onRestore(value: T): void; onDiscard?(): void;
}) {
  const [reviewed, setReviewed] = useState("");
  const [choices, setChoices] = useState<{ reviewKey: string; values: Record<string, "draft" | "saved"> }>({ reviewKey: "", values: {} });
  const pending = draft.pending, changed = !!pending && !!base && pending.base !== base;
  // Any new saved value or recovered draft invalidates previous choices and consent.
  const reviewKey = pending ? JSON.stringify([base, pending.updatedAt, pending.value, pending.original, saved]) : "";
  const selected = choices.reviewKey === reviewKey ? choices.values : {};
  const combined = merge && changed && saved && pending ? mergeProjectDraft(pending.original, pending.value, saved, selected) : null;
  return <div className="draft-recovery">
    {draft.error && <p className="journey-warning" role={draft.error.startsWith("Draft recovery is unavailable") ? "status" : "alert"}>{draft.error}</p>}
    {pending ? <section aria-label={`${label} draft recovery`} className="draft-recovery-panel">
      <strong>Recover your {label.toLowerCase()} draft</strong>
      <p>Unsaved edits from {new Date(pending.updatedAt).toLocaleString()} are available in this browser. The saved project has not been changed.</p>
      {changed && <p className="journey-warning">The saved version changed after this draft began. Review both versions before continuing; saving will still check the current revision.</p>}
      {combined && <>
        <p>{pending.original ? "Edits to different fields are combined with the saved version. Where both versions changed the same content, choose what to keep." : "This older browser draft does not include its original saved version. Choose what to keep for each difference so another person's work is not silently replaced."} You can edit the recovered draft before saving it.</p>
        {combined.conflicts.map(conflict => {
          const name = draftFieldLabel(conflict.path, [pending.original, saved, pending.value]);
          return <fieldset className="draft-conflict" key={conflict.key} disabled={busy || readOnly}>
            <legend>{name}</legend>
            <div className="draft-comparison">
              <div className="draft-choice" data-selected={selected[conflict.key] === "saved" || undefined}><label><input type="radio" name={`recovery-${label}-${conflict.key}`} checked={selected[conflict.key] === "saved"} onChange={() => { setChoices({ reviewKey, values: { ...selected, [conflict.key]: "saved" } }); setReviewed(""); }} />Keep saved version for {name}</label><DraftDocumentReview value={conflict.saved} path={conflict.path} documents={[saved, pending.value, pending.original]} absent={conflict.baseline === undefined ? "Not present in the saved version" : "Removed in the saved version"} /></div>
              <div className="draft-choice" data-selected={selected[conflict.key] === "draft" || undefined}><label><input type="radio" name={`recovery-${label}-${conflict.key}`} checked={selected[conflict.key] === "draft"} onChange={() => { setChoices({ reviewKey, values: { ...selected, [conflict.key]: "draft" } }); setReviewed(""); }} />Use my draft for {name}</label><DraftDocumentReview value={conflict.draft} path={conflict.path} documents={[pending.value, saved, pending.original]} absent={conflict.baseline === undefined ? "Not present in your draft" : "Removed in your draft"} /></div>
            </div>
            {pending.original && <DraftReviewDisclosure title={`Before these edits: ${name}`}>{() => <DraftDocumentReview value={conflict.baseline} path={conflict.path} documents={[pending.original]} absent="Not present before these edits" />}</DraftReviewDisclosure>}
          </fieldset>;
        })}
        <p role="status">{combined.unresolved ? `${combined.unresolved} overlapping ${combined.unresolved === 1 ? "change needs" : "changes need"} your choice.` : "Recovery choices are ready. The saved project stays unchanged until you save the recovered draft."}</p>
        <DraftReviewDisclosure title="Review combined draft">{() => <>
          {combined.unresolved > 0 && <p className="journey-warning">Until you make every choice, this preview shows the saved version for undecided differences.</p>}
          <DraftDocumentReview value={combined.value} documents={[combined.value]} />
          <DraftReviewDisclosure title="Combined draft technical values" className="draft-technical">{() => <pre className="draft-result">{JSON.stringify(combined.value, null, 2)}</pre>}</DraftReviewDisclosure>
        </>}</DraftReviewDisclosure>
      </>}
      <DraftReviewDisclosure title="Compare draft with saved version">{() => <>
        <div className="draft-comparison"><div><h3>Saved version</h3>{merge ? <DraftDocumentReview value={saved} documents={[saved]} /> : <pre>{JSON.stringify(saved, null, 2)}</pre>}</div><div><h3>Your draft</h3>{merge ? <DraftDocumentReview value={pending.value} documents={[pending.value]} /> : <pre>{JSON.stringify(pending.value, null, 2)}</pre>}</div></div>
        {merge && <DraftReviewDisclosure title="Original technical values" className="draft-technical">{() => <div className="draft-comparison"><div><h3>Saved version values</h3><pre>{JSON.stringify(saved, null, 2)}</pre></div><div><h3>Your draft values</h3><pre>{JSON.stringify(pending.value, null, 2)}</pre></div></div>}</DraftReviewDisclosure>}
      </>}</DraftReviewDisclosure>
      {changed && <label className="setup-check"><input type="checkbox" disabled={busy || readOnly || !!combined?.unresolved} checked={reviewed === reviewKey} onChange={event => setReviewed(event.target.checked ? reviewKey : "")} />I reviewed the saved version and {combined ? "these recovery choices." : "want to continue with this draft."}</label>}
      <div className="journey-actions"><Button type="button" disabled={busy || readOnly || !base || !!combined?.unresolved || (changed && reviewed !== reviewKey)} onClick={() => { const restored = draft.take(); if (restored) onRestore(combined ? combined.value : restored.value); }}>Restore draft</Button><Button type="button" variant="secondary" disabled={busy} onClick={() => { draft.clear(); onDiscard?.(); }}>Discard browser draft</Button></div>
    </section> : draft.retained ? <p className="journey-muted" role="status">{draft.restored ? "Recovered draft" : "Unsaved changes"} · kept in this browser for up to 30 days. Save to share with your project.</p> : null}
  </div>;
}
