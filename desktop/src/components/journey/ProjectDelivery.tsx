import { useEffect, useRef, useState } from "react";
import {
  productApi,
  errorText,
  type GitDelivery,
  type GitState,
  type GitHubAccount,
  type ProviderLoginJob,
  type GitPublication,
} from "../../api/product";
import { useCockpit } from "../../store/store";
import { useAuth } from "../auth/AuthProvider";

function githubLink(value?: string) {
  try {
    const url = new URL(value || "");
    return url.protocol === "https:" &&
      ["github.com", "cli.github.com"].includes(url.hostname)
      ? url.href
      : undefined;
  } catch {
    return undefined;
  }
}
export function ProjectDelivery({
  projectId,
  git,
}: {
  projectId: string;
  git: GitState;
}) {
  const auth = useAuth();
  const allowPublic = !auth || auth.profile.policy.allowPublicPublishing;
  const [delivery, setDelivery] = useState<GitDelivery | null>(null);
  const [account, setAccount] = useState<GitHubAccount | null>(null),
    [login, setLogin] = useState<ProviderLoginJob | null>(null);
  const [remote, setRemote] = useState(""),
    [owner, setOwner] = useState(""),
    [name, setName] = useState("");
  const [visibility, setVisibility] = useState<"private" | "public">("private"),
    [acknowledged, setAcknowledged] = useState(false);
  const [busy, setBusy] = useState(""),
    [error, setError] = useState(""),
    [notice, setNotice] = useState("");
  const alive = useRef(true),
    working = useRef(false);
  const projectName = useCockpit(
    (state) =>
      state.projects.find((project) => project.id === projectId)?.name || "",
  );
  const head = git.log[0]?.hash || "";
  async function refresh() {
    const value = await productApi.gitDelivery(projectId);
    if (alive.current) {
      setDelivery(value);
      setRemote(value.origin || "");
    }
  }
  useEffect(() => {
    alive.current = true;
    setName(projectName);
    void refresh().catch((cause) => {
      if (alive.current) setError(errorText(cause));
    });
    return () => {
      alive.current = false;
    };
  }, [projectId, head]);
  useEffect(() => {
    if (!login || !["running", "waiting"].includes(login.status)) return;
    let cancelled = false;
    const timer = setInterval(() => {
      void productApi
        .githubLogins()
        .then((value) => {
          const next = value.jobs.find((job) => job.id === login.id);
          if (!cancelled && next) setLogin(next);
        })
        .catch((cause) => {
          if (!cancelled) setError(errorText(cause));
        });
    }, 2000);
    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, [login?.id, login?.status]);
  async function run(label: string, action: () => Promise<void>) {
    if (working.current) return;
    working.current = true;
    setBusy(label);
    setError("");
    setNotice("");
    try {
      await action();
    } catch (cause) {
      if (alive.current) setError(errorText(cause));
    } finally {
      working.current = false;
      if (alive.current) setBusy("");
    }
  }
  async function openOfficial(value?: string) {
    const url = githubLink(value);
    if (!url) return;
    if ("__TAURI_INTERNALS__" in window) {
      const { openUrl } = await import("@tauri-apps/plugin-opener");
      await openUrl(url);
    } else window.open(url, "_blank", "noopener,noreferrer");
  }
  async function published(result: GitPublication) {
    await refresh();
    if (alive.current)
      setNotice(
        result.verified
          ? `Remote commit verified: ${result.commit?.slice(0, 8)}.`
          : `${result.status}: repository connected. Push your reviewed commit next.`,
      );
  }
  return (
    <section className="project-plan">
      <h2>Publish your reviewed work</h2>
      <p className="journey-muted">
        Connect a repository, then push the exact commit shown here. Creating a
        repository does not push your project.
      </p>
      {error && (
        <p className="journey-error" role="alert">
          {error}
        </p>
      )}
      {notice && (
        <p className="journey-success" role="status">
          {notice}
        </p>
      )}
      <div className="journey-actions">
        <button
          className="luci-button"
          disabled={!!busy}
          onClick={() =>
            void run("Checking GitHub", async () => {
              const result = await productApi.githubCheck();
              if (alive.current) setAccount(result);
            })
          }
        >
          Check GitHub account
        </button>
        <button
          className="luci-button"
          disabled={
            !!busy || (!!login && ["running", "waiting"].includes(login.status))
          }
          onClick={() =>
            void run("Opening GitHub sign-in", async () => {
              const result = await productApi.githubLogin();
              if (alive.current) setLogin(result);
            })
          }
        >
          Sign in to GitHub
        </button>
      </div>
      {account && (
        <p role="status">
          {account.message}
          {account.signedIn && account.login ? ` · ${account.login}` : ""}{" "}
          {githubLink(account.actionUrl) && (
            <button
              className="luci-button"
              onClick={() =>
                void run("Opening official install page", () =>
                  openOfficial(account.actionUrl),
                )
              }
            >
              Install GitHub CLI
            </button>
          )}
        </p>
      )}
      {login && (
        <p role="status">
          {login.status} · {login.message}{" "}
          {login.userCode && <strong>{login.userCode}</strong>}{" "}
          {githubLink(login.verificationUrl) && (
            <button
              className="luci-button"
              onClick={() =>
                void run("Opening GitHub sign-in", () =>
                  openOfficial(login.verificationUrl),
                )
              }
            >
              Continue GitHub sign-in
            </button>
          )}
        </p>
      )}
      {!delivery?.origin && (
        <details className="delivery-create">
          <summary>Create a GitHub repository</summary>
          <form
            className="journey-form"
            onSubmit={(event) => {
              event.preventDefault();
              void run("Creating repository", async () =>
                published(
                  await productApi.githubCreate(projectId, {
                    name: name.trim(),
                    ...(owner.trim() ? { owner: owner.trim() } : {}),
                    visibility: allowPublic ? visibility : "private",
                    acknowledgePublic: allowPublic && acknowledged,
                  }),
                ),
              );
            }}
          >
            <div className="setup-field-grid">
              <label className="luci-field">
                Repository name
                <input
                  required
                  value={name}
                  onChange={(event) => setName(event.target.value)}
                />
              </label>
              <label className="luci-field">
                GitHub owner · optional
                <input
                  value={owner}
                  onChange={(event) => setOwner(event.target.value)}
                  placeholder="Your signed-in account"
                />
              </label>
            </div>
            <label className="luci-field">
              Repository visibility
              <select
                value={visibility}
                onChange={(event) => {
                  setVisibility(event.target.value as typeof visibility);
                  setAcknowledged(false);
                }}
              >
                <option value="private">Private</option>
                {allowPublic && <option value="public">Public</option>}
              </select>
            </label>
            {allowPublic && visibility === "public" && (
              <label className="setup-check">
                <input
                  type="checkbox"
                  checked={acknowledged}
                  onChange={(event) => setAcknowledged(event.target.checked)}
                />
                Create a public repository. Code I push there will be visible to
                everyone.
              </label>
            )}
            <button
              className="luci-button"
              disabled={
                !!busy ||
                !name.trim() ||
                (visibility === "public" && !acknowledged)
              }
            >
              Create repository
            </button>
          </form>
        </details>
      )}
      <form
        className="check-command-form"
        onSubmit={(event) => {
          event.preventDefault();
          void run("Connecting repository", async () =>
            published(await productApi.gitRemote(projectId, remote.trim())),
          );
        }}
      >
        <label className="luci-field">
          Repository origin · HTTPS
          <input
            required
            type="url"
            value={remote}
            onChange={(event) => setRemote(event.target.value)}
            placeholder="https://github.com/you/project.git"
          />
        </label>
        <button className="luci-button" disabled={!!busy || !remote.trim()}>
          Connect existing repository
        </button>
      </form>
      <p>{delivery?.origin || "No remote repository connected."}</p>
      <button
        className="luci-button"
        disabled={!!busy}
        onClick={() => void run("Refreshing connection", refresh)}
      >
        Refresh repository connection
      </button>
      <p className="journey-muted">
        {head
          ? `Reviewed commit ${head.slice(0, 12)} on ${git.branch}.`
          : "Create your first commit before pushing."}{" "}
        {!git.clean && "Commit your current changes before pushing."}
      </p>
      <button
        className="luci-button luci-primary"
        disabled={!!busy || !delivery?.origin || !git.clean || !head}
        onClick={() =>
          void run("Pushing and verifying remote commit", async () =>
            published(await productApi.gitPush(projectId, head, git.branch)),
          )
        }
      >
        {busy || "Push reviewed commit"}
      </button>
      {delivery?.publication && (
        <p className="journey-muted">
          Last publication: {delivery.publication.status}
          {delivery.publication.verified
            ? " · remote commit verified"
            : " · verification pending"}
          {delivery.publication.commit
            ? ` · ${delivery.publication.commit.slice(0, 8)}`
            : ""}
          .
        </p>
      )}
    </section>
  );
}
