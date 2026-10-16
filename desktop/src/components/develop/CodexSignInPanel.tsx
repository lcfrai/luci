import { useEffect, useRef, useState } from "react";
import { api } from "../../api";
import { productApi, type ProductClient } from "../../api/product";
import { codexConnectionError, type CodexConnectionAction } from "../../api/codexConnectionErrors";
import { useAttachState, useCodexDeviceChallenge } from "../../api/attachRegistry";
import { CODEX_DEVICE_URL, DEVICE_CODE_MAX_AGE_MS, isCodexDeviceSignIn } from "../../api/codexDeviceSignIn";
import { OWN_AGENT_PROVIDER_CHANGED } from "../../hooks/useAgentConnections";
import { useCockpit } from "../../store/store";
import type { Session } from "../../store/types";
import { Button } from "../fluid/components/button";

export function CodexSignInPanel({ session, canControl, client = productApi }: {
  session: Session;
  canControl: boolean;
  client?: ProductClient;
}) {
  const challenge = useCodexDeviceChallenge(session.id);
  const attach = useAttachState(session.id);
  const [expired, setExpired] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [saved, setSaved] = useState(false);
  const working = useRef(false);
  const alive = useRef(true);
  useEffect(() => { alive.current = true; return () => { alive.current = false; }; }, []);
  useEffect(() => {
    setExpired(false);
    if (!challenge) return;
    const remaining = challenge.observedAt + DEVICE_CODE_MAX_AGE_MS - Date.now();
    if (remaining <= 0) { setExpired(true); return; }
    const timer = setTimeout(() => setExpired(true), remaining);
    return () => clearTimeout(timer);
  }, [challenge]);
  const ended = session.status === "exited" || attach.phase === "ended";
  const usable = !!challenge && !ended && !expired && canControl;

  async function action(run: () => Promise<void>, operation: CodexConnectionAction = "other") {
    if (working.current || !canControl) return;
    working.current = true;
    setBusy(true); setError(""); setNotice("");
    try { await run(); }
    catch (cause) { if (alive.current) setError(codexConnectionError(cause, operation)); }
    finally { working.current = false; if (alive.current) setBusy(false); }
  }
  async function stopCurrent() {
    if (session.status !== "exited") {
      // The normal non-purge stop observes backend retirement before adoption
      // or allocation of another device sign-in; text never grants that proof.
      const stopped = await api.killSession(session.id);
      if (!alive.current) throw new Error("The sign-in view closed. Return to it before continuing.");
      if (stopped.id !== session.id || stopped.status !== "exited") throw new Error("Sign-in has not stopped safely. Retry stop before continuing.");
      useCockpit.getState().upsertSession(stopped);
    }
  }
  async function freshSignIn() {
    const other = Object.values(useCockpit.getState().sessions).find(row =>
      row.id !== session.id && isCodexDeviceSignIn(row) && row.status !== "exited");
    if (other) throw new Error("Another Codex sign-in is active. Open that session or stop it first.");
    await stopCurrent();
    if (!alive.current) return;
    const fresh = await client.startCodexSignIn();
    if (!alive.current) return;
    if (!isCodexDeviceSignIn(fresh) || fresh.id === session.id) throw new Error("A fresh Codex device sign-in was not returned. Refresh your sessions before trying again.");
    const state = useCockpit.getState();
    state.upsertSession(fresh);
    state.selectProject(null); state.setLens("develop"); state.focusSession(fresh.id);
  }
  async function saveSignIn() {
    await stopCurrent();
    if (!alive.current) return;
    const account = await client.connectCodexAccount(session.id);
    if (!account.configured) throw new Error("Saving this sign-in was not confirmed. Keep the session and refresh Codex connection before trying Save again.");
    window.dispatchEvent(new Event(OWN_AGENT_PROVIDER_CHANGED));
    if (alive.current) { setSaved(true); setNotice("Codex sign-in saved for this Luci workspace. You can return to your project."); }
  }
  return <section className="codex-signin-panel" aria-label="Connect Codex with ChatGPT">
    <h3>Connect Codex with ChatGPT</h3>
    {saved ? <p role="status">{notice}</p> : <>
      <p>{ended
        ? "This sign-in session ended. If you completed OpenAI sign-in, save it below. If it was interrupted, start a new sign-in."
        : expired
          ? "This code may have expired. Start a new sign-in to get another code."
          : "Copy the code here, then open OpenAI’s sign-in page. Ctrl+C inside the terminal cancels sign-in."}</p>
      {usable && <div className="codex-signin-code">
        <label>One-time Codex code<input aria-label="One-time Codex code" readOnly value={challenge.code} onFocus={event => event.currentTarget.select()} /></label>
        <Button type="button" variant="secondary" disabled={busy} onClick={() => void action(async () => {
          if (!navigator.clipboard?.writeText) throw new Error("Copy is unavailable here. Select the code above and copy it outside the terminal.");
          await navigator.clipboard.writeText(challenge.code);
          if (alive.current) setNotice("Code copied. Paste it on OpenAI’s sign-in page.");
        })}>Copy code</Button>
        <Button type="button" variant="secondary" disabled={busy} onClick={() => void action(async () => {
          if ("__TAURI_INTERNALS__" in window) {
            const { openUrl } = await import("@tauri-apps/plugin-opener");
            await openUrl(CODEX_DEVICE_URL);
          } else {
            const opened = window.open(CODEX_DEVICE_URL, "_blank", "noopener,noreferrer");
            // Browsers may return null with noopener even after opening safely.
            void opened;
          }
        })}>Open OpenAI sign-in</Button>
      </div>}
      {!ended && !expired && !challenge && <p className="journey-muted">Waiting for Codex’s official device code. The original terminal remains below.</p>}
      <div className="setup-actions">
        <Button type="button" disabled={busy || !canControl} onClick={() => void action(saveSignIn, "save")}>
          {busy ? "Working…" : "Save Codex sign-in"}
        </Button>
        {(ended || expired) && <Button type="button" variant="secondary" disabled={busy || !canControl} onClick={() => void action(freshSignIn, "start")}>Start new device sign-in</Button>}
      </div>
      {!ended && <p className="journey-muted">After completing OpenAI’s page, Save stops this terminal and asks Luci to validate and retain its sign-in. Opening the page or copying a code does not sign you in.</p>}
      {notice && <p role="status">{notice}</p>}
    </>}
    {error && <p role="alert" className="journey-error">{error}</p>}
    {!canControl && <p className="journey-muted">Account administration and credential access are required to control this sign-in.</p>}
  </section>;
}
