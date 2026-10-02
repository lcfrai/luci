import { useEffect, useRef, useState, type FormEvent } from "react";
import { productApi, errorText, type Brief } from "../../api/product";
import { ProjectRequired, JourneyHeader } from "../journey/shared";

export function DiscoverLens() {
  return (
    <ProjectRequired>
      {(id) => <BriefEditor key={id} projectId={id} />}
    </ProjectRequired>
  );
}
export function BriefEditor({ projectId }: { projectId: string }) {
  const [brief, setBrief] = useState<Brief | null>(null);
  const [criteria, setCriteria] = useState("");
  const [error, setError] = useState("");
  const [saved, setSaved] = useState(false);
  const [busy, setBusy] = useState(false);
  const saving = useRef(false);
  const alive = useRef(true);
  const load = () =>
    productApi.brief(projectId).then((value) => {
      if (alive.current) {
        setBrief(value);
        setCriteria(value.criteria.join("\n"));
        setError("");
      }
    });
  useEffect(() => {
    alive.current = true;
    void load().catch((cause) => {
      if (alive.current) setError(errorText(cause));
    });
    return () => {
      alive.current = false;
    };
  }, [projectId]);
  const change = (patch: Partial<Brief>) => {
    if (brief) setBrief({ ...brief, ...patch });
    setSaved(false);
  };
  async function save(event: FormEvent) {
    event.preventDefault();
    if (!brief || saving.current) return;
    saving.current = true;
    setBusy(true);
    setError("");
    setSaved(false);
    try {
      const value = await productApi.saveBrief(projectId, {
        revision: brief.revision,
        purpose: brief.purpose,
        audience: brief.audience,
        constraints: brief.constraints,
        criteria: criteria
          .split("\n")
          .map((line) => line.trim())
          .filter(Boolean),
      });
      if (alive.current) {
        setBrief(value);
        setSaved(true);
      }
    } catch (cause) {
      if (alive.current) setError(errorText(cause));
    } finally {
      saving.current = false;
      if (alive.current) setBusy(false);
    }
  }
  return (
    <section className="journey-page">
      <JourneyHeader
        title="What are we making?"
        subtitle="A clear brief follows your project through design, development and every release."
      >
        <span className="journey-revision" role="status">
          {brief ? `Revision ${brief.revision}` : "Loading brief…"}
        </span>
      </JourneyHeader>
      {error && (
        <div className="journey-error" role="alert">
          {error}{" "}
          {
            <button
              className="luci-button"
              disabled={busy}
              onClick={() =>
                void load().catch((cause) => setError(errorText(cause)))
              }
            >
              Reload saved brief
            </button>
          }
        </div>
      )}
      {brief && (
        <form className="journey-form" onSubmit={save}>
          <label className="luci-field">
            The idea
            <textarea
              disabled={busy}
              rows={4}
              value={brief.purpose}
              onChange={(event) => change({ purpose: event.target.value })}
              placeholder="What should this app do, and why does it matter?"
            />
          </label>
          <label className="luci-field">
            Who it is for
            <textarea
              disabled={busy}
              rows={2}
              value={brief.audience}
              onChange={(event) => change({ audience: event.target.value })}
              placeholder="The people, their situation, and what they need."
            />
          </label>
          <label className="luci-field">
            Boundaries and constraints
            <textarea
              disabled={busy}
              rows={3}
              value={brief.constraints}
              onChange={(event) => change({ constraints: event.target.value })}
              placeholder="Platforms, budget, privacy, accessibility, or anything the app must respect."
            />
          </label>
          <label className="luci-field">
            What good looks like
            <textarea
              disabled={busy}
              rows={5}
              value={criteria}
              onChange={(event) => {
                setCriteria(event.target.value);
                setSaved(false);
              }}
              placeholder="One measurable outcome per line."
            />
            <small>Each line is one criterion to carry into the work.</small>
          </label>
          <div className="journey-actions">
            <button className="luci-button luci-primary" disabled={busy}>
              {busy ? "Saving brief…" : "Save brief"}
            </button>
            {saved && (
              <span role="status" className="journey-muted">
                Saved as revision {brief.revision}. Agent context updated.
              </span>
            )}
          </div>
        </form>
      )}
    </section>
  );
}
