import { useEffect, useRef, useState } from "react";
import { orchestratorApi, type OrchestratorView } from "../../api/orchestrator";
import { errorText } from "../../api/product";
import { Button } from "../fluid/components/button";

export function ConversationStorage({ projectId, view, canEdit, busy, onBusy, onView }: {
  projectId: string; view: OrchestratorView; canEdit: boolean; busy: boolean;
  onBusy(value: boolean): void; onView(value: OrchestratorView, rotated: boolean): void;
}) {
  const [downloadDigest, setDownloadDigest] = useState(""), [acknowledged, setAcknowledged] = useState(false);
  const [notice, setNotice] = useState(""), [error, setError] = useState("");
  const alive = useRef(true), pending = useRef(false);
  useEffect(() => { alive.current = true; return () => { alive.current = false; }; }, []);
  const backedUp = !!downloadDigest && downloadDigest === view.storageDigest;
  async function download() {
    if (busy || pending.current) return;
    pending.current = true; onBusy(true); setError(""); setNotice(""); setDownloadDigest(""); setAcknowledged(false);
    try {
      const result = await orchestratorApi.export(projectId);
      if (!alive.current) return;
      const current = await orchestratorApi.get(projectId);
      if (!alive.current) return;
      const url = URL.createObjectURL(new Blob([JSON.stringify(result, null, 2)], { type: "application/json" }));
      const link = document.createElement("a"); link.href = url; link.download = `luci-${projectId}-private-conversation.json`;
      document.body.appendChild(link); link.click(); link.remove(); setTimeout(() => URL.revokeObjectURL(url), 1000);
      onView(current, false);
      if (result.storageDigest !== current.storageDigest) { setNotice("The conversation changed during download. Download it again before freeing space."); return; }
      setDownloadDigest(result.storageDigest);
      setNotice("Download started. Confirm the file was saved before freeing conversation space.");
    } catch (cause) { if (alive.current) setError(errorText(cause)); }
    finally { pending.current = false; if (alive.current) onBusy(false); }
  }
  async function rotate() {
    if (!canEdit || busy || pending.current || !backedUp || !acknowledged) return;
    pending.current = true; onBusy(true); setError(""); setNotice("");
    try {
      const current = await orchestratorApi.rotate(projectId, { expectedRevision: view.revision, expectedStorageDigest: downloadDigest, acknowledgeExportedHistory: true });
      if (!alive.current) return;
      onView(current, true); setDownloadDigest(""); setAcknowledged(false);
      setNotice("Recent turns and unresolved decisions are retained. Older conversation is available in the downloaded file and the current private archive.");
    } catch (cause) { if (alive.current) { setError(`${errorText(cause)} Download the current history before trying again.`); setDownloadDigest(""); setAcknowledged(false); } }
    finally { pending.current = false; if (alive.current) onBusy(false); }
  }
  return <details className="orchestrator-storage"><summary>Conversation history and storage</summary>
    <p>Download the retained conversation{view.archivePresent ? " and its previous archive" : ""}. Keep this file private: it includes messages, people's names and saved project context.</p>
    <Button variant="secondary" size="compact" disabled={busy} onClick={() => void download()}>Download private conversation</Button>
    <p>To free space, Luci keeps the latest eight turns and unresolved decisions, and moves older turns into one private archive. The next cleanup replaces that archive; download includes it first. Current project documents and approvals stay unchanged.</p>
    {notice && <p role="status">{notice}</p>}{error && <p role="alert" className="orchestrator-error">{error}</p>}
    <label className="orchestrator-ack"><input type="checkbox" checked={acknowledged} disabled={!canEdit || busy || !backedUp} onChange={event => setAcknowledged(event.target.checked)} />I saved this private history file and want to free conversation space.</label>
    <Button variant="secondary" size="compact" disabled={!canEdit || busy || !backedUp || !acknowledged} onClick={() => void rotate()}>Free conversation space</Button>
  </details>;
}
