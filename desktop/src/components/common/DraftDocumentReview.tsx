import { useState, type ReactNode } from "react";

const fieldNames: Record<string, string> = {
  audience: "Who it is for", constraints: "Boundaries and constraints", outOfScope: "Out of scope",
  requirements: "Outcomes", startingPoint: "Starting point", acceptance: "Acceptance scenario",
  journeys: "User journeys", actor: "Person or role", steps: "Steps and recovery", requirementIds: "Related requirements",
  roles: "Access roles", canDo: "Can do", cannotDo: "Cannot do or access", entities: "Data entities",
  fields: "Fields and relationships", privacy: "Privacy and retention", constraint: "Boundaries and requirements",
  glossary: "Glossary terms", meaning: "Meaning in this project", reason: "Why", alternatives: "Alternatives considered",
  questions: "Open questions", screens: "Screens", states: "States and recovery", widths: "Responsive widths",
  accessibility: "Accessibility and interaction", tokens: "Design tokens", assets: "Assets and provenance",
  path: "Project file path", licence: "Licence and source", implementationNotes: "Implementation notes",
  prototypeReleaseId: "Prototype release", prototypeCommit: "Prototype source commit", tasks: "Tasks",
  id: "ID", projectId: "Project", version: "Document format", revision: "Saved revision",
  briefRevision: "Brief revision", specificationRevision: "Definition revision", specificationDigest: "Definition fingerprint",
  sketchRevision: "Sketch revision", digest: "Content fingerprint", updatedAt: "Last updated", "$order": "Order",
  commands: "Check commands", executable: "Executable", args: "Arguments", buildId: "Selected build",
};
const metadata = new Set(["projectId", "version", "revision", "briefRevision", "specificationRevision", "specificationDigest", "sketchRevision", "status", "digest", "updatedAt"]);
const documentOrder = ["startingPoint", "purpose", "audience", "constraints", "outOfScope", "requirements", "journeys", "roles", "entities", "integrations", "glossary", "decisions", "questions", "screens", "tokens", "assets", "implementationNotes", "prototypeReleaseId", "prototypeCommit", "tasks", "title"];
const recordOrder = ["title", "name", "term", "question", "route", "purpose", "priority", "acceptance", "status", "actor", "steps", "requirementIds", "states", "widths", "accessibility"];
type RecordValue = Record<string, unknown>;
const isRecord = (value: unknown): value is RecordValue => !!value && typeof value === "object" && !Array.isArray(value);

export function draftFieldName(part: string, path: string[] = []): string {
  if (part === "args" && path.length === 1) return "Arguments being edited · JSON text";
  if (part === "executable" && path.length === 1) return "Executable being added";
  if (part === "purpose") return path.length === 1 ? "The idea" : "Purpose";
  if (part === "title" && path.length === 1) return "Task being added";
  return fieldNames[part] ?? part.replace(/([a-z])([A-Z])/g, "$1 $2").replace(/^./, character => character.toUpperCase());
}

function recordName(value: unknown): { key: string; text: string } | undefined {
  if (!isRecord(value)) return undefined;
  for (const key of ["title", "name", "term", "question", "path", "label"]) {
    if (typeof value[key] === "string" && value[key]) return { key, text: value[key] };
  }
}

function atPath(document: unknown, path: string[]): unknown {
  return path.reduce<unknown>((value, part) => part.startsWith("id:") && Array.isArray(value)
    ? value.find(item => isRecord(item) && item.id === part.slice(3))
    : isRecord(value) ? value[part] : undefined, document);
}

export function draftFieldLabel(path: string[], documents: unknown[]): string {
  return path.map((part, index) => {
    if (part.startsWith("id:")) {
      const records = documents.map(document => atPath(document, path.slice(0, index + 1)));
      return records.map(recordName).find(Boolean)?.text ?? part.slice(3);
    }
    const existingLabels: Record<string, string> = { constraints: "Constraints", audience: "Audience", outOfScope: "Outside this version" };
    return existingLabels[part] ?? draftFieldName(part, path.slice(0, index + 1));
  }).join(" · ");
}

/** Native disclosure keeps long documents and technical JSON out of the initial render. */
export function DraftReviewDisclosure({ title, children, className = "" }: {
  title: string; children(): ReactNode; className?: string;
}) {
  const [open, setOpen] = useState(false);
  return <details className={`draft-review-disclosure ${className}`} onToggle={event => setOpen(event.currentTarget.open)}>
    <summary>{title}</summary>{open && children()}
  </details>;
}

function orderedKeys(value: RecordValue, order: string[]): string[] {
  return [...order.filter(key => Object.prototype.hasOwnProperty.call(value, key)), ...Object.keys(value).filter(key => !order.includes(key))];
}

function friendlyScalar(value: unknown, field: string): string {
  if (value === null || value === "") return "Not set";
  if (typeof value === "boolean") return value ? "Yes" : "No";
  if (field === "startingPoint") return ({ new: "A new idea", existing: "An existing application" } as Record<string, string>)[String(value)] ?? String(value);
  if (field === "priority") return ({ must: "Must have", should: "Should have", could: "Could have" } as Record<string, string>)[String(value)] ?? String(value);
  if (field === "status") return ({ todo: "Not started", doing: "In progress", done: "Done", draft: "Draft", approved: "Approved", outdated: "Outdated" } as Record<string, string>)[String(value)] ?? String(value);
  if (field === "widths" && typeof value === "number") return `${value} px`;
  return String(value);
}

/** Read-only presentation of structured context; no interpretation or merging of drawings. */
export function DraftDocumentReview({ value, path = [], documents = [], absent = "Not present in this version" }: {
  value: unknown; path?: string[]; documents?: unknown[]; absent?: string;
}) {
  const sources = [value, ...documents];
  function idLabel(id: string, collectionPath: string[]): ReactNode {
    const record = sources.map(document => atPath(document, collectionPath)).filter(Array.isArray)
      .flat().find(item => isRecord(item) && item.id === id);
    const name = recordName(record)?.text;
    return <>{name && <span>{name} </span>}<span className="draft-record-id">{id}</span></>;
  }
  function fields(record: RecordValue, currentPath: string[], omit: string[] = []): ReactNode {
    const keys = orderedKeys(record, currentPath.length ? recordOrder : documentOrder).filter(key => !omit.includes(key));
    if (!keys.length) return <p className="draft-value-empty">No details</p>;
    return <dl className="draft-review-fields">{keys.map(key => <div className="draft-review-field" key={key}>
      <dt>{draftFieldName(key, [...currentPath, key])}</dt><dd>{read(record[key], [...currentPath, key])}</dd>
    </div>)}</dl>;
  }
  function card(record: RecordValue, currentPath: string[], index: number): ReactNode {
    const name = recordName(record);
    const id = typeof record.id === "string" ? record.id : undefined;
    return <div className="draft-review-record">
      <div className="draft-record-heading"><strong>{name?.text ?? `Item ${index + 1}`}</strong>{id !== undefined && <span className="draft-record-id">{id || "ID not set"}</span>}</div>
      {fields(record, currentPath, [...(name ? [name.key] : []), ...(id !== undefined ? ["id"] : [])])}
    </div>;
  }
  function read(current: unknown, currentPath: string[]): ReactNode {
    const field = currentPath[currentPath.length - 1] ?? "";
    if (current === undefined) return <p className="draft-value-empty">{absent}</p>;
    if (Array.isArray(current)) {
      if (!current.length) return <p className="draft-value-empty">None</p>;
      if (field === "$order") return <ol className="draft-review-list" aria-label="Record order">{current.map((id, index) => <li key={index}>{typeof id === "string" ? idLabel(id, currentPath.slice(0, -1)) : read(id, currentPath)}</li>)}</ol>;
      return <ol className={current.every(isRecord) ? "draft-review-records" : "draft-review-list"} aria-label={draftFieldName(field, currentPath)}>{current.map((item, index) => <li key={index}>
        {isRecord(item) ? card(item, [...currentPath, typeof item.id === "string" ? `id:${item.id}` : String(index)], index)
          : field === "requirementIds" && typeof item === "string" ? idLabel(item, ["requirements"]) : read(item, currentPath)}
      </li>)}</ol>;
    }
    if (isRecord(current)) {
      if (currentPath.length === 0) {
        const recorded = Object.fromEntries(Object.entries(current).filter(([key]) => metadata.has(key)));
        return <>{fields(current, currentPath, [...metadata])}{!!Object.keys(recorded).length &&
          <DraftReviewDisclosure title="Recorded revision details" className="draft-technical">{() => <>
            <p className="journey-muted">These identify the recorded context. Restoring a draft does not save or approve your edits.</p>
            {fields(recorded, ["metadata"])}
          </>}</DraftReviewDisclosure>}</>;
      }
      return typeof current.id === "string" ? card(current, currentPath, 0) : fields(current, currentPath);
    }
    return <p className={current === null || current === "" ? "draft-value-empty" : "draft-value"}>{friendlyScalar(current, field)}</p>;
  }
  return <div className="draft-document-review">{read(value, path)}</div>;
}
