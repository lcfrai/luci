// Contract §6.3. Six rows, none disabled, none numbered, none marked complete —
// docs/AGENTIC-UX.md §4 rule 12 and §9 item 1.
import type { JSX } from "react";
import { useCockpit } from "../../store/store";
import type { Lens } from "../../store/types";
import "./subrail.css";

export const LENSES: ReadonlyArray<{ id: Lens; label: string; glyph: string }> = [
  { id: "discover", label: "Discover", glyph: "◇" },
  { id: "design", label: "Design", glyph: "◈" },
  { id: "develop", label: "Develop", glyph: "◆" },
  { id: "files", label: "Files", glyph: "▤" },
  { id: "merge", label: "Merge", glyph: "⑂" },
  { id: "deploy", label: "Deploy", glyph: "▲" },
];

export function SubRail(): JSX.Element {
  const lens = useCockpit((s) => s.lens);
  const setLens = useCockpit((s) => s.setLens);
  const branchId = useCockpit((s) => s.selection.branchId);
  const projectId = useCockpit((s) => s.selection.projectId);
  const selectProject = useCockpit((s) => s.selectProject);
  const branchName = useCockpit((s) => {
    if (projectId === null || branchId === null) return "";
    return (s.branchesByProject[projectId] ?? []).find((b) => b.id === branchId)?.name ?? "";
  });
  const projectName = useCockpit(
    (s) => s.projects.find((p) => p.id === projectId)?.name ?? "",
  );

  return (
    <aside className="sub" aria-label="Phases">
      <div className="sub-head">
        <button type="button" className="sub-back" onClick={() => selectProject(projectId)}>
          ← {projectName}
        </button>
        <span className="sub-branch">{branchName}</span>
      </div>
      <nav className="sub-list">
        {LENSES.map((entry) => (
          <button
            key={entry.id}
            type="button"
            className="sub-row"
            aria-current={lens === entry.id ? "page" : undefined}
            onClick={() => setLens(entry.id)}
          >
            <span className="sub-glyph" aria-hidden="true">
              {entry.glyph}
            </span>
            {entry.label}
          </button>
        ))}
      </nav>
    </aside>
  );
}
