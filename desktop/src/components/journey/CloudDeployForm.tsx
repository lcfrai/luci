import { useEffect, useId, useRef, useState } from "react";
import {
  errorText,
  productApi,
  type CloudReleaseRequest,
  type DeploymentTarget,
  type GitState,
} from "../../api/product";
type CloudDeployFormProps = {
  projectId: string;
  busy: string;
  onDeploy(input: CloudReleaseRequest): void;
  onSetup(): void;
  canManageBacking?: boolean;
};
export function CloudDeployForm(props: CloudDeployFormProps) {
  return <ProjectCloudDeployForm key={props.projectId} {...props} />;
}
function ProjectCloudDeployForm({
  projectId,
  busy,
  onDeploy,
  onSetup,
  canManageBacking = true,
}: CloudDeployFormProps) {
  const alive = useRef(true),
    preparing = useRef(false);
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
    const ticket = generation.current;
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
    if (preparing.current) return;
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
  return (
    <form
      className="cloud-deploy-form"
      onSubmit={(e) => {
        e.preventDefault();
        if (
          !busy && !keyBusy && git?.clean &&
          head &&
          targetId &&
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
      {!targets.length && (
        <p>
          No deployment server connected yet.{" "}
          {canManageBacking ? (
            <button type="button" className="luci-button" onClick={onSetup}>
              Set up your cloud
            </button>
          ) : (
            "Ask your administrator to connect a deployment server."
          )}
        </p>
      )}
      <label className="luci-field">
        Deployment server
        <select
          required
          disabled={!!busy || keyBusy}
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
            disabled={!!busy || keyBusy}
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
            onChange={(e) => setDomain(e.target.value)}
            placeholder="app.example.com"
          />
        </label>
        <label className="luci-field">
          Cloud Dockerfile
          <input
            required
            value={dockerfile}
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
            onChange={(e) => setPort(Number(e.target.value))}
          />
        </label>
        <label className="luci-field">
          Health check path
          <input
            required
            value={healthPath}
            onChange={(e) => setHealthPath(e.target.value)}
            placeholder="/health"
          />
        </label>
      </div>
      <label className="luci-field">
        Repository access
        <select
          value={source}
          disabled={!!busy || keyBusy}
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
          <button
            type="button"
            className="luci-button"
            disabled={!!busy || keyBusy || !targetId || !repository.trim()}
            onClick={() => void prepareAccess()}
          >
            {keyBusy
              ? "Preparing read-only access…"
              : "Prepare read-only GitHub access"}
          </button>
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
      <label className="luci-field">
        <span id={`${fieldId}-data-label`}>Browser data variables</span>
        <select aria-labelledby={`${fieldId}-data-label`} aria-describedby={`${fieldId}-data-help`} value={dataClientFramework} disabled={!!busy || keyBusy} onChange={event => setDataClientFramework(event.target.value as "none" | "vite" | "nextjs")}>
          <option value="none">Runtime only</option>
          <option value="vite">Vite · VITE_SUPABASE_URL and VITE_SUPABASE_ANON_KEY</option>
          <option value="nextjs">Next.js · NEXT_PUBLIC_SUPABASE_URL and NEXT_PUBLIC_SUPABASE_ANON_KEY</option>
        </select>
        <small id={`${fieldId}-data-help`}>A connected public data API supplies its HTTPS address and anonymous key. Vite and Next.js embed these public values in the browser build. Your Dockerfile must declare and use both matching ARG names before building the frontend. Redeploy after changing the data connection. Administrator keys stay private.</small>
      </label>
      {git && (
        <p className={git.clean && head ? "journey-muted" : "journey-warning"}>
          {git.clean && head
            ? `Release from ${git.branch} · ${head.slice(0, 12)}. Push this commit to your repository before deploying.`
            : "Commit your current changes in Develop → Git before deploying."}
        </p>
      )}
      <div className="setup-actions">
        <button
          className="luci-button luci-primary"
          disabled={
            !!busy ||
            keyBusy ||
            !targetId ||
            !git?.clean ||
            !head ||
            (source === "private" && !privateKeyUuid.trim())
          }
        >
          {busy || "Deploy to cloud"}
        </button>
        <button
          type="button"
          className="luci-button"
          disabled={!!busy}
          onClick={() => void load()}
        >
          Refresh project and servers
        </button>
      </div>
    </form>
  );
}
