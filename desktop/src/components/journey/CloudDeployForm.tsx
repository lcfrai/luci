import { useEffect, useId, useMemo, useRef, useState } from "react";
import {
  errorText,
  productApi,
  type CloudReleaseRequest,
  type DeploymentTarget,
  type GitState,
} from "../../api/product";
import { Button } from "../fluid/components/button";
import { useProjectDraft, useProjectDraftScope } from "../../hooks/useProjectDraft";
import { useUnsavedProjectDraft } from "../../hooks/useUnsavedProjectDraft";

type CloudDeploymentDraft = {
  targetId: string; repository: string; domain: string; port: number; healthPath: string; dockerfile: string;
  source: "public" | "private"; dataClientFramework: "none" | "vite" | "nextjs";
};
const emptyDraft: CloudDeploymentDraft = { targetId: "", repository: "", domain: "", port: 3000, healthPath: "/", dockerfile: "Dockerfile", source: "private", dataClientFramework: "none" };
const draftKeys = Object.keys(emptyDraft).sort().join(",");
function validCloudDraft(value: unknown): value is CloudDeploymentDraft {
  if (!value || typeof value !== "object" || Array.isArray(value) || Object.keys(value).sort().join(",") !== draftKeys) return false;
  const row = value as CloudDeploymentDraft;
  // Keep credentials, key references, URL userinfo/query tokens and consent out of browser recovery.
  return Object.entries(emptyDraft).every(([key, example]) => typeof example === "number" ? Number.isInteger(row.port) && row.port > 0 && row.port <= 65535
    : typeof row[key as keyof CloudDeploymentDraft] === "string" && (row[key as keyof CloudDeploymentDraft] as string).length <= 4096 && !/[\r\n\0]/.test(row[key as keyof CloudDeploymentDraft] as string)) &&
    !/[@?#]/.test(row.repository + row.domain + row.healthPath) && ["public", "private"].includes(row.source) && ["none", "vite", "nextjs"].includes(row.dataClientFramework);
}
type CloudDeployFormProps = {
  projectId: string;
  busy: string;
  disabled?: boolean;
  onDeploy(input: CloudReleaseRequest): void;
  onSetup(): void;
  canManageBacking?: boolean;
  canPrepareAccess?: boolean;
  onUnretainedChange?(unretained: boolean): void;
};
export function CloudDeployForm(props: CloudDeployFormProps) {
  const scope = useProjectDraftScope(props.projectId);
  return <ProjectCloudDeployForm key={JSON.stringify([scope, props.projectId])} {...props} scope={scope} />;
}
function ProjectCloudDeployForm({
  projectId,
  busy,
  disabled = false,
  onDeploy,
  onSetup,
  canManageBacking = true,
  canPrepareAccess = true,
  onUnretainedChange,
  scope,
}: CloudDeployFormProps & { scope: string | null }) {
  const alive = useRef(true),
    preparing = useRef(false);
  const checking = useRef(false);
  const [loading, setLoading] = useState(true);
  const generation = useRef(0), fieldId = useId();
  const [source, setSource] = useState<"public" | "private">("private"),
    [privateKeyUuid, setPrivateKeyUuid] = useState("");
  const [keyBusy, setKeyBusy] = useState(false),
    [keyMessage, setKeyMessage] = useState("");
  const [targets, setTargets] = useState<DeploymentTarget[]>([]),
    [git, setGit] = useState<GitState | null>(null),
    [error, setError] = useState("");
  const [targetId, setTargetId] = useState(""),
    [repository, setRepository] = useState(""),
    [domain, setDomain] = useState(""),
    [port, setPort] = useState(3000),
    [healthPath, setHealthPath] = useState("/"),
    [dockerfile, setDockerfile] = useState("Dockerfile");
  const [dataClientFramework, setDataClientFramework] = useState<"none" | "vite" | "nextjs">("none");
  const [changed, setChanged] = useState(false);
  const baseline = useRef<CloudDeploymentDraft>(emptyDraft);
  const value = useMemo(() => ({ targetId, repository, domain, port, healthPath, dockerfile, source, dataClientFramework }), [targetId, repository, domain, port, healthPath, dockerfile, source, dataClientFramework]);
  const draft = useProjectDraft({ scope: scope ? JSON.stringify([scope, "cloud-deployment-form"]) : null, kind: "preview-setup", base: projectId, value, dirty: changed, valid: validCloudDraft });
  function applySettings(next: CloudDeploymentDraft) {
    setTargetId(next.targetId); setRepository(next.repository); setDomain(next.domain); setPort(next.port);
    setHealthPath(next.healthPath); setDockerfile(next.dockerfile); setSource(next.source); setDataClientFramework(next.dataClientFramework);
    setPrivateKeyUuid(""); setKeyMessage(""); setError("");
  }
  function discardDraft() { draft.clear(); applySettings(baseline.current); setChanged(false); }
  useUnsavedProjectDraft(projectId, changed || !!draft.pending, "Cloud deployment settings", "deploy", { retained: draft.retained || !!draft.pending, discard: discardDraft });
  const unretained = changed && !draft.retained;
  useEffect(() => { onUnretainedChange?.(unretained); return () => onUnretainedChange?.(false); }, [unretained, onUnretainedChange]);
  const fieldsLocked = !!busy || keyBusy || loading || !!draft.pending;
  const load = async (initial = false) => {
    if (checking.current || preparing.current) return;
    checking.current = true;
    const ticket = generation.current;
    setLoading(true);
    setError("");
    try {
      const [t, g, delivery] = await Promise.all([
        productApi.targets(),
        productApi.git(projectId),
        productApi.gitDelivery(projectId),
      ]);
      if (!alive.current || generation.current !== ticket) return;
      if (initial) {
        baseline.current = { ...emptyDraft, repository: delivery.origin || "", source: delivery.publication?.visibility === "public" ? "public" : "private" };
        setRepository(delivery.origin || "");
        setSource(
          delivery.publication?.visibility === "public" ? "public" : "private",
        );
      }
      setTargets(t.targets);
      setGit(g);
    } catch (e) {
      if (alive.current && generation.current === ticket) setError(errorText(e));
    } finally {
      checking.current = false;
      if (alive.current && generation.current === ticket) setLoading(false);
    }
  };
  useEffect(() => {
    alive.current = true;
    generation.current++;
    void load(true);
    return () => {
      alive.current = false;
      generation.current++;
    };
  }, [projectId]);
  async function prepareAccess() {
    if (!canPrepareAccess || preparing.current || loading || busy || draft.pending || !targetId || !repository.trim()) return;
    preparing.current = true;
    const ticket = generation.current;
    setKeyBusy(true);
    setError("");
    setKeyMessage("");
    try {
      const result = await productApi.prepareDeployKey(
        projectId,
        targetId,
        repository.trim(),
      );
      if (alive.current && generation.current === ticket) {
        setPrivateKeyUuid(
          result.status === "ready" ? result.privateKeyUuid || "" : "",
        );
        setKeyMessage(result.message);
      }
    } catch (cause) {
      if (alive.current && generation.current === ticket) setError(errorText(cause));
    } finally {
      preparing.current = false;
      if (alive.current && generation.current === ticket) setKeyBusy(false);
    }
  }
  const head = git?.log[0]?.hash ?? "";
  const selectedTarget = targets.find(target => target.id === targetId);
  return (
    <form
      className="cloud-deploy-form"
      onInvalidCapture={event => { const details = (event.target as HTMLElement).closest<HTMLDetailsElement>("details"); if (details) details.open = true; }}
      onSubmit={(e) => {
        e.preventDefault();
        if (
          !disabled && !busy && !keyBusy && !loading && !draft.pending && git?.clean &&
          head &&
          selectedTarget &&
          (source === "public" || privateKeyUuid.trim())
        )
          onDeploy({
            targetId,
            source,
            ...(source === "private"
              ? { privateKeyUuid: privateKeyUuid.trim() }
              : {}),
            repository,
            branch: git.branch,
            commit: head,
            domain,
            port,
            healthPath,
            dockerfile,
            ...(dataClientFramework !== "none" ? { dataClientFramework } : {}),
          });
      }}
    >
      <h2>Ship to your cloud</h2>
      <p className="journey-muted">
        Deploy your reviewed repository to a connected server. Luci checks the
        deployed commit and HTTPS health before showing a ready release.
      </p>
      {draft.pending && <section className="setup-plan" aria-label="Cloud deployment draft recovery">
        <h3>Recover cloud deployment settings</h3>
        <p>Settings from this account, workspace and project are available in this browser. Restore them to review the current server and source again. Access keys and deployment approval are not restored.</p>
        <Button type="button" variant="secondary" disabled={!!busy || keyBusy || loading || disabled} onClick={() => { const restored = draft.take(); if (restored) { applySettings(restored.value); setChanged(true); } }}>Restore cloud deployment draft</Button>
        <Button type="button" variant="tertiary" disabled={!!busy || keyBusy} onClick={discardDraft}>Discard cloud deployment draft</Button>
      </section>}
      {draft.error && <p role="alert" className="journey-warning">Cloud deployment settings could not be retained safely. Keep this form open or discard the draft before leaving. Credentials must stay out of saved URLs.</p>}
      {changed && !draft.pending && <div className="journey-actions">{draft.retained && <p role="status">Cloud deployment settings are kept in this browser. Access keys must be selected or prepared again after recovery.</p>}<Button type="button" variant="tertiary" disabled={!!busy || keyBusy} onClick={discardDraft}>Discard cloud deployment draft</Button></div>}
      {error && (
        <p className="journey-error" role="alert">
          {error}
        </p>
      )}
      {loading && <p role="status">Checking your connected deployment servers and saved source…</p>}
      {!loading && !targets.length && (
        <p>
          No deployment server connected yet.{" "}
          {canManageBacking ? (
            <Button type="button" variant="secondary" onClick={onSetup}>
              Set up your cloud
            </Button>
          ) : (
            "Ask your administrator to connect a deployment server."
          )}
        </p>
      )}
      <label className="luci-field">
        Deployment server
        <select
          required
          disabled={fieldsLocked}
          value={targetId}
          onChange={(e) => {
            setChanged(true);
            setTargetId(e.target.value);
            setPrivateKeyUuid("");
            setKeyMessage("");
          }}
        >
          <option value="">Choose your server</option>
          {targets.map((t) => (
            <option key={t.id} value={t.id}>
              {t.name}
            </option>
          ))}
        </select>
      </label>
      <div className="setup-field-grid">
        <label className="luci-field">
          Repository URL · HTTPS
          <input
            required
            type="url"
            disabled={fieldsLocked}
            value={repository}
            onChange={(e) => {
              setChanged(true);
              setRepository(e.target.value);
              setPrivateKeyUuid("");
              setKeyMessage("");
            }}
            placeholder="https://github.com/you/project"
          />
        </label>
        <label className="luci-field">
          Application domain
          <input
            required
            value={domain}
            disabled={fieldsLocked}
            onChange={(e) => { setChanged(true); setDomain(e.target.value); }}
            placeholder="app.example.com"
          />
        </label>
      </div>
      <details className="deployment-details">
        <summary>Build and health options</summary>
        <div className="setup-field-grid">
        <label className="luci-field">
          Cloud Dockerfile
          <input
            required
            value={dockerfile}
            disabled={fieldsLocked}
            onChange={(e) => { setChanged(true); setDockerfile(e.target.value); }}
          />
        </label>
        <label className="luci-field">
          Cloud app port
          <input
            required
            type="number"
            min="1"
            max="65535"
            value={port}
            disabled={fieldsLocked}
            onChange={(e) => { setChanged(true); setPort(Number(e.target.value)); }}
          />
        </label>
        <label className="luci-field">
          Health check path
          <input
            required
            value={healthPath}
            disabled={fieldsLocked}
            onChange={(e) => { setChanged(true); setHealthPath(e.target.value); }}
            placeholder="/health"
          />
        </label>
        </div>
        <label className="luci-field">
          <span id={`${fieldId}-data-label`}>Browser data variables</span>
          <select aria-labelledby={`${fieldId}-data-label`} aria-describedby={`${fieldId}-data-help`} value={dataClientFramework} disabled={fieldsLocked} onChange={event => { setChanged(true); setDataClientFramework(event.target.value as "none" | "vite" | "nextjs"); }}>
            <option value="none">Runtime only</option>
            <option value="vite">Vite · VITE_SUPABASE_URL and VITE_SUPABASE_ANON_KEY</option>
            <option value="nextjs">Next.js · NEXT_PUBLIC_SUPABASE_URL and NEXT_PUBLIC_SUPABASE_ANON_KEY</option>
          </select>
          <small id={`${fieldId}-data-help`}>A connected public data API supplies its HTTPS address and anonymous key. Vite and Next.js embed these public values in the browser build. Your Dockerfile must declare and use both matching ARG names before building the frontend. Redeploy after changing the data connection. Administrator keys stay private.</small>
        </label>
      </details>
      <label className="luci-field">
        Repository access
        <select
          value={source}
          disabled={fieldsLocked}
          onChange={(event) => { setChanged(true); setSource(event.target.value as typeof source); setPrivateKeyUuid(""); setKeyMessage(""); }}
        >
          <option value="private">
            Private repository · read-only deploy key
          </option>
          <option value="public">Public repository</option>
        </select>
      </label>
      {source === "private" && (
        <section className="setup-plan">
          <p className="journey-muted">
            Use your connected GitHub account to create a read-only repository
            key and store its private half on this deployment server. This
            permits source checkout; it does not push code.
          </p>
          <Button type="button"
            variant="secondary"
            loading={keyBusy}
            disabled={!canPrepareAccess || fieldsLocked || !targetId || !repository.trim()}
            onClick={() => void prepareAccess()}
          >
            {keyBusy
              ? "Preparing read-only access…"
              : "Prepare read-only GitHub access"}
          </Button>
          {!canPrepareAccess && <p role="status">Ask a team owner for secrets access to prepare private repository credentials.</p>}
          {keyMessage && <p role="status">{keyMessage}</p>}
          <label className="luci-field">
            Deployment server key ID
            <input
              value={privateKeyUuid}
              disabled={fieldsLocked}
              onChange={(event) => { setChanged(true); setPrivateKeyUuid(event.target.value); }}
            />
            <small>
              Filled after preparing access. You may instead enter an existing
              key ID configured on this server for another Git provider.
            </small>
          </label>
        </section>
      )}
      {git && (
        <p className={git.clean && head ? "journey-muted" : "journey-warning"}>
          {git.clean && head
            ? `Release from ${git.branch} · ${head.slice(0, 12)}. Push this commit to your repository before deploying.`
            : "Commit your current changes in Develop → Git before deploying."}
        </p>
      )}
      <div className="setup-actions">
        <Button type="submit"
          variant="primary"
          loading={!!busy}
          disabled={
            disabled || !!draft.pending || !!busy ||
            keyBusy ||
            loading ||
            !selectedTarget ||
            !git?.clean ||
            !head ||
            (source === "private" && !privateKeyUuid.trim())
          }
        >
          {busy || "Deploy to cloud"}
        </Button>
        <Button type="button"
          variant="ghost"
          loading={loading}
          disabled={!!busy || loading || keyBusy}
          onClick={() => void load()}
        >
          Refresh project and servers
        </Button>
      </div>
    </form>
  );
}
