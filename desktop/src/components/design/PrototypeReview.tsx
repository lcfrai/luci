import { useState } from "react";
import type { DesignPlan, DesignScreen } from "../../api/designPlan";
import type { ProjectSpecification } from "../../api/definition";
import { Button } from "../fluid/components/button";

export interface OpenPrototypeSource {
  releaseId: string; commit: string; configurationRevision: number; route: string; plannedRoute?: string;
}

/** Planned routes never get to replace the verified application origin. */
export function prototypeScreenRoute(route: string): string | null {
  if (/[\\\u0000-\u001f]/.test(route)) return null;
  const value = route.trim();
  if (!value.startsWith("/") || value.startsWith("//") || /[\\\u0000-\u0020]/.test(value) || /(^|\/)[:*\[]/.test(value)) return null;
  try {
    const url = new URL(value, "https://prototype.invalid");
    if (url.origin !== "https://prototype.invalid" || url.pathname.startsWith("/.well-known/")) return null;
    return url.pathname + url.search + url.hash;
  } catch { return null; }
}

export function PrototypeReview({ plan, specification, screen, onSelectScreen, onAmend, onOpenScreen, source, width, canEdit, canOpen, busy, onNotesChange }: {
  plan: DesignPlan; specification: ProjectSpecification; screen?: DesignScreen;
  onSelectScreen(id: string): void; onAmend(id: string): void; onOpenScreen(route: string): void;
  source: OpenPrototypeSource | null; width: number; canEdit: boolean; canOpen: boolean; busy: boolean;
  onNotesChange(notes: string): void;
}) {
  const [observation, setObservation] = useState<{ base: string; heading: string; value: string } | null>(null);
  const route = screen ? prototypeScreenRoute(screen.route) : null;
  const requirements = screen ? specification.requirements.filter(item => screen.requirementIds.includes(item.id)) : [];
  return <section className="prototype-review" aria-label="Screen review">
    <header><h3>Review the planned experience</h3><p>Compare the running application with the intended behavior below. Opening a page does not verify that behavior.</p></header>
    {!screen ? <p>No screens are planned yet. Add screens and their expected states before reviewing the interface.</p> : <>
      <label className="luci-field">Screen to review<select value={screen.id} onChange={event => onSelectScreen(event.target.value)}>
        {plan.screens.map(item => <option key={item.id} value={item.id}>{item.name || "Untitled screen"}</option>)}
      </select></label>
      <p>{screen.purpose}</p>
      <p className="journey-muted">{plan.status === "approved" ? `Reviewed design ${plan.revision}` : `Design ${plan.revision} · draft expectations`} · Definition {specification.revision}</p>
      <dl className="prototype-route"><dt>Planned route</dt><dd><code>{screen.route || "No route yet"}</code></dd><dt>Designed widths</dt><dd>{screen.widths.length ? screen.widths.map(value => `${value} px`).join(", ") : "No widths yet"}</dd></dl>
      <div className="journey-actions"><Button type="button" variant="secondary" disabled={!route || !canOpen || busy} onClick={() => route && onOpenScreen(route)}>Open planned screen</Button><Button type="button" variant="secondary" disabled={!canEdit || busy} onClick={() => onAmend(screen.id)}>Amend this screen</Button></div>
      {!route && <p className="journey-muted">This screen needs a concrete route beginning with /. For routes that need an item ID, open the prototype and navigate to a real item.</p>}
      <div className="prototype-expectations"><section><h4>States and recovery to try</h4>{screen.states.some(value => value.trim()) ? <ul>{screen.states.filter(value => value.trim()).map((state, index) => <li key={index}>{state}</li>)}</ul> : <p>No states have been described yet.</p>}<p>{screen.accessibility}</p></section>
        <section><h4>Expected outcomes</h4>{requirements.length ? <ul>{requirements.map(requirement => <li key={requirement.id}><strong>{requirement.title}</strong><p>{requirement.acceptance}</p></li>)}</ul> : <p>No outcomes are linked to this screen yet.</p>}</section></div>
    </>}
    {source && <section className="prototype-observation" aria-label="Prototype observations">
      <p>Opened release <code>{source.releaseId}</code> · source <code>{source.commit.slice(0, 12)}</code> · starting at <code>{source.route}</code></p>
      <p className="journey-muted">The application may navigate after opening. These observations refer to this source, not to a later build.</p>
      {!observation ? <Button type="button" variant="secondary" disabled={!canEdit || busy} onClick={() => setObservation({ base: plan.implementationNotes, heading: `Prototype observation (human note; not verification)\nRelease: ${source.releaseId}\nSource commit: ${source.commit}\nConfiguration revision: ${source.configurationRevision}\nScreen: ${screen?.name || "Application"}${screen ? ` (${screen.id})` : ""}\nOpened route: ${source.route}\nViewport: ${width} px\nObservation or requested change:`, value: "" })}>Record an observation</Button> : <>
        <label className="luci-field">Observation or requested change<textarea autoFocus rows={4} value={observation.value} disabled={!canEdit || busy} onChange={event => {
          const value = event.target.value;
          setObservation({ ...observation, value });
          onNotesChange(value ? [observation.base.trimEnd(), observation.heading, value].filter(Boolean).join("\n\n") : observation.base);
        }} /></label>
        <p className="journey-muted">Your words are retained in the handoff draft with this release and source. Saving creates a draft that needs design review again. Notes do not verify implementation; existing builds keep their original context.</p>
        <Button type="button" variant="secondary" onClick={() => setObservation(null)}>Done writing</Button>
      </>}
    </section>}
  </section>;
}
