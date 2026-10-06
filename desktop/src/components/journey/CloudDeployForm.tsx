import { useEffect, useId, useRef, useState } from "react";
import {
  errorText,
  productApi,
  type CloudReleaseRequest,
  type DeploymentTarget,
  type GitState,
} from "../../api/product";
import { Button } from "../fluid/components/button";
type CloudDeployFormProps = {
  projectId: string;
  busy: string;
  disabled?: boolean;
  onDeploy(input: CloudReleaseRequest): void;
  onSetup(): void;
  canManageBacking?: boolean;
  canPrepareAccess?: boolean;
};
export function CloudDeployForm(props: CloudDeployFormProps) {
  return <ProjectCloudDeployForm key={props.projectId} {...props} />;
}
function ProjectCloudDeployForm({
  projectId,
  busy,
  disabled = false,
  onDeploy,
  onSetup,
  canManageBacking = true,
  canPrepareAccess = true,
}: CloudDeployFormProps) {
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
    if (!canPrepareAccess || preparing.current || loading || busy || !targetId || !repository.trim()) return;
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
          !disabled && !busy && !keyBusy && !loading && git?.clean &&
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
          disabled={!!busy || keyBusy || loading}
          value={targetId}
          onChange={(e) => {
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
            disabled={!!busy || keyBusy || loading}
            value={repository}
            onChange={(e) => {
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
            disabled={!!busy || keyBusy || loading}
            onChange={(e) => setDomain(e.target.value)}
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
            disabled={!!busy || keyBusy || loading}
            onChange={(e) => setDockerfile(e.target.value)}
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
            disabled={!!busy || keyBusy || loading}
            onChange={(e) => setPort(Number(e.target.value))}
          />
        </label>
        <label className="luci-field">
          Health check path
          <input
            required
            value={healthPath}
            disabled={!!busy || keyBusy || loading}
            onChange={(e) => setHealthPath(e.target.value)}
            placeholder="/health"
          />
        </label>
        </div>
        <label className="luci-field">
          <span id={`${fieldId}-data-label`}>Browser data variables</span>
          <select aria-labelledby={`${fieldId}-data-label`} aria-describedby={`${fieldId}-data-help`} value={dataClientFramework} disabled={!!busy || keyBusy || loading} onChange={event => setDataClientFramework(event.target.value as "none" | "vite" | "nextjs")}>
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
          disabled={!!busy || keyBusy || loading}
          onChange={(event) => setSource(event.target.value as typeof source)}
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
            disabled={!canPrepareAccess || !!busy || keyBusy || loading || !targetId || !repository.trim()}
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
              disabled={!!busy || keyBusy}
              onChange={(event) => setPrivateKeyUuid(event.target.value)}
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
            disabled || !!busy ||
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
