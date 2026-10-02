import { useEffect, useState } from "react";
import {
  errorText,
  productApi,
  type CloudReleaseRequest,
  type DeploymentTarget,
  type GitState,
} from "../../api/product";
export function CloudDeployForm({
  projectId,
  busy,
  onDeploy,
  onSetup,
}: {
  projectId: string;
  busy: string;
  onDeploy(input: CloudReleaseRequest): void;
  onSetup(): void;
}) {
  const [targets, setTargets] = useState<DeploymentTarget[]>([]),
    [git, setGit] = useState<GitState | null>(null),
    [error, setError] = useState("");
  const [targetId, setTargetId] = useState(""),
    [repository, setRepository] = useState(""),
    [domain, setDomain] = useState(""),
    [port, setPort] = useState(3000),
    [healthPath, setHealthPath] = useState("/"),
    [dockerfile, setDockerfile] = useState("Dockerfile");
  const load = async () => {
    setError("");
    try {
      const [t, g] = await Promise.all([
        productApi.targets(),
        productApi.git(projectId),
      ]);
      setTargets(t.targets);
      setGit(g);
    } catch (e) {
      setError(errorText(e));
    }
  };
  useEffect(() => {
    void load();
  }, [projectId]);
  const head = git?.log[0]?.hash ?? "";
  return (
    <form
      className="cloud-deploy-form"
      onSubmit={(e) => {
        e.preventDefault();
        if (git?.clean && head && targetId)
          onDeploy({
            targetId,
            repository,
            branch: git.branch,
            commit: head,
            domain,
            port,
            healthPath,
            dockerfile,
          });
      }}
    >
      <h2>Ship to your cloud</h2>
      <p className="journey-muted">
        Deploy a public HTTPS Git repository to a connected server. Luci checks
        the deployed commit and HTTPS health before showing a ready release.
      </p>
      {error && (
        <p className="journey-error" role="alert">
          {error}
        </p>
      )}
      {!targets.length && (
        <p>
          No deployment server connected yet.{" "}
          <button type="button" className="luci-button" onClick={onSetup}>
            Set up your cloud
          </button>
        </p>
      )}
      <label className="luci-field">
        Deployment server
        <select
          required
          value={targetId}
          onChange={(e) => setTargetId(e.target.value)}
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
          Public repository URL
          <input
            required
            type="url"
            value={repository}
            onChange={(e) => setRepository(e.target.value)}
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
          disabled={!!busy || !targetId || !git?.clean || !head}
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
