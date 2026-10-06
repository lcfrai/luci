import { useEffect, useRef, useState } from "react";
import {
  errorText,
  productApi,
  type GitDelivery,
  type PagesReleaseRequest,
} from "../../api/product";
import { Button } from "../fluid/components/button";

type Props = { projectId: string; busy: string; disabled?: boolean; onDeploy(input: PagesReleaseRequest): void };
export function PagesDeployForm(props: Props) { return <ProjectPagesDeployForm key={props.projectId} {...props} />; }
function ProjectPagesDeployForm({
  projectId,
  busy,
  disabled = false,
  onDeploy,
}: Props) {
  const [delivery, setDelivery] = useState<GitDelivery | null>(null);
  const [acknowledgePublic, setAcknowledgePublic] = useState(false);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const active = useRef(true), checking = useRef(false);
  async function load() {
    if (checking.current) return;
    checking.current = true;
    setLoading(true); setDelivery(null); setAcknowledgePublic(false); setError("");
    try {
      const value = await productApi.gitDelivery(projectId);
      if (active.current) setDelivery(value);
    } catch (cause) { if (active.current) setError(errorText(cause)); }
    finally { checking.current = false; if (active.current) setLoading(false); }
  }
  useEffect(() => {
    active.current = true;
    void load();
    return () => {
      active.current = false;
    };
  }, [projectId]);
  const pushed =
    !!delivery?.head &&
    delivery.publication?.visibility === "public" &&
    delivery.publication?.verified === true &&
    delivery.publication.commit === delivery.head;
  return (
    <form
      className="cloud-deploy-form"
      onSubmit={(event) => {
        event.preventDefault();
        if (!disabled && delivery && pushed && acknowledgePublic && !busy && !loading)
          onDeploy({
            commit: delivery.head,
            branch: delivery.branch,
            acknowledgePublic,
          });
      }}
    >
      <h2>Publish a static site</h2>
      <p>
        GitHub Pages hosts public HTML, CSS and JavaScript at a github.io
        address, with no hosting purchase. Commit docs/index.html,
        docs/.nojekyll and your Dockerfile, then push through Develop → Git.
      </p>
      <p>
        Server code, databases and private runtime variables are not deployed to
        Pages. Use relative asset paths for your repository’s URL.
      </p>
      <p>After setup, pushes to this default branch also republish /docs.</p>
      {error && (
        <p role="alert" className="journey-error">
          {error}
        </p>
      )}
      {loading && (
        <p role="status">Checking your pushed revision…</p>
      )}
      {delivery && (
        <p>
          <strong>{delivery.origin || "No GitHub repository connected"}</strong>
          <br />
          {delivery.branch} ·{" "}
          {delivery.head ? delivery.head.slice(0, 12) : "No commit"}
        </p>
      )}
      {delivery && !pushed && (
        <p role="status">
          Push the reviewed commit to a public GitHub repository before
          deploying.
        </p>
      )}
      <label className="setup-check">
        <input
          type="checkbox"
          checked={acknowledgePublic}
          disabled={disabled || !!busy || loading}
          onChange={(event) => setAcknowledgePublic(event.target.checked)}
        />
        I understand that this site and its repository are public.
      </label>
      <div className="journey-actions"><Button type="submit"
        variant="primary"
        loading={!!busy}
        disabled={disabled || !!busy || loading || !pushed || !acknowledgePublic}
      >
        {busy || "Deploy to GitHub Pages"}
      </Button>
      <Button type="button" variant="ghost" loading={loading} disabled={!!busy || loading} onClick={() => void load()}>Refresh pushed revision</Button></div>
    </form>
  );
}
