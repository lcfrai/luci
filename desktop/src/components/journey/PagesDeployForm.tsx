import { useEffect, useState } from "react";
import {
  errorText,
  productApi,
  type GitDelivery,
  type PagesReleaseRequest,
} from "../../api/product";

export function PagesDeployForm({
  projectId,
  busy,
  onDeploy,
}: {
  projectId: string;
  busy: string;
  onDeploy(input: PagesReleaseRequest): void;
}) {
  const [delivery, setDelivery] = useState<GitDelivery | null>(null);
  const [acknowledgePublic, setAcknowledgePublic] = useState(false);
  const [error, setError] = useState("");
  useEffect(() => {
    let active = true;
    setDelivery(null);
    setAcknowledgePublic(false);
    setError("");
    productApi
      .gitDelivery(projectId)
      .then((value) => {
        if (active) setDelivery(value);
      })
      .catch((cause) => {
        if (active) setError(errorText(cause));
      });
    return () => {
      active = false;
    };
  }, [projectId]);
  const pushed =
    !!delivery?.head &&
    delivery.publication?.verified === true &&
    delivery.publication.commit === delivery.head;
  return (
    <form
      className="cloud-deploy-form"
      onSubmit={(event) => {
        event.preventDefault();
        if (delivery && pushed && acknowledgePublic && !busy)
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
      {!delivery && !error && (
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
          disabled={!!busy}
          onChange={(event) => setAcknowledgePublic(event.target.checked)}
        />
        I understand that this site and its repository are public.
      </label>
      <button
        className="luci-button luci-primary"
        disabled={!!busy || !pushed || !acknowledgePublic}
      >
        {busy || "Deploy to GitHub Pages"}
      </button>
    </form>
  );
}
