import { useEffect, useState } from "react";
import { contextHistoryApi, type ContextChangeEvent, type ContextHistoryResponse } from "../../api/contextHistory";
import { errorText } from "../../api/product";
import { useProjectDraftScope } from "../../hooks/useProjectDraft";
import { Button } from "../fluid/components/button";
import "./context-history.css";

const documentNames: Record<ContextChangeEvent["kind"], string> = {
  brief: "brief", specification: "definition", sketch: "sketch", design: "design handoff", plan: "implementation plan",
};
function activity(event: ContextChangeEvent) {
  const document = documentNames[event.kind];
  if (event.status === "unconfirmed") {
    return event.action === "approve" ? `requested approval of the ${document}` : `requested an update to the ${document}`;
  }
  if (event.action === "approve") return `approved the ${document}`;
  if (event.action === "apply-analysis") return `applied repository analysis to the ${document}`;
  return `saved the ${document}`;
}

export function ContextHistory({ projectId }: { projectId: string }) {
  const scope = useProjectDraftScope(projectId);
  if (!scope) return <p className="journey-muted">Context activity is available when your account and workspace are connected.</p>;
  return <ScopedContextHistory key={scope} projectId={projectId} />;
}

function ScopedContextHistory({ projectId }: { projectId: string }) {
  const [history, setHistory] = useState<ContextHistoryResponse | null>(null);
  const [error, setError] = useState("");
  const [refresh, setRefresh] = useState(0);
  const [loading, setLoading] = useState(true);
  const [visible, setVisible] = useState(12);
  useEffect(() => {
    let current = true;
    setLoading(true); setError(""); setHistory(null); setVisible(12);
    void contextHistoryApi.list(projectId).then(value => {
      if (current) setHistory(value);
    }).catch(cause => {
      if (current) setError(errorText(cause));
    }).finally(() => {
      if (current) setLoading(false);
    });
    return () => { current = false; };
  }, [projectId, refresh]);

  return <section className="context-history" aria-label="Context activity">
    <div className="context-history-heading">
      <h3>Context activity</h3>
      <Button type="button" variant="secondary" size="compact" disabled={loading} onClick={() => setRefresh(value => value + 1)}>Refresh activity</Button>
    </div>
    <p className="journey-muted">Who saved and reviewed each revision in this workspace. Earlier activity does not grant approval of the current document.</p>
    {loading && <p role="status">Loading context activity…</p>}
    {error && <p className="journey-error" role="alert">{error}</p>}
    {history && <>
      {history.pruned && <p className="journey-muted">Older activity is no longer retained. This list shows the most recent recorded changes.</p>}
      {history.events.length === 0 ? <p className="journey-muted">No authenticated activity has been recorded for this project. Imported or earlier revisions have unknown authorship.</p> : <>
        <ol className="context-history-events">{history.events.slice(0, visible).map(event => <li key={event.id}>
          <p><strong>{event.actor.displayName}</strong> {activity(event)}</p>
          <p className="context-history-meta">Revision {event.revision}<span aria-hidden="true"> · </span><time dateTime={event.at}>{new Date(event.at).toLocaleString()}</time></p>
          {event.status === "unconfirmed" && <p className="context-history-unconfirmed">Outcome not confirmed. Reload the document to check whether this change was saved before retrying.</p>}
        </li>)}</ol>
        {history.events.length > visible && <Button type="button" variant="secondary" size="compact" onClick={() => setVisible(value => value + 12)}>Show older activity</Button>}
      </>}
    </>}
  </section>;
}
