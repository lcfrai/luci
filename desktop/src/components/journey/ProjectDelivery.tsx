import { useEffect, useLayoutEffect, useRef, useState } from "react";
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
import { ApiError } from "../../api/client";
import { useAuth } from "../auth/AuthProvider";
import { useProjectAccess } from "../../hooks/useProjectAccess";
import { useProjectDraftScope } from "../../hooks/useProjectDraft";
import { useUnsavedProjectDraft } from "../../hooks/useUnsavedProjectDraft";

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
interface Props {
  projectId: string;
  git: GitState;
}
export function ProjectDelivery(props: Props) {
  const scope = useProjectDraftScope(props.projectId);
  return <DeliveryFields key={JSON.stringify([productApi.baseUrl, scope, props.projectId])} {...props} />;
}

function DeliveryFields({ projectId, git }: Props) {
  const auth = useAuth();
  const allowPublic = !auth || auth.profile.policy.allowPublicPublishing;
  const access = useProjectAccess(projectId), workspace = useProjectAccess("");
  const canPublish = access.canEdit && access.permissions.deploy;
  const canUseGitHub = access.canRead && workspace.canEdit && workspace.permissions.secrets;
  const projectName = useCockpit(
    (state) => state.projects.find((project) => project.id === projectId)?.name || "",
  );
  const [delivery, setDelivery] = useState<GitDelivery | null>(null);
  const [account, setAccount] = useState<GitHubAccount | null>(null),
    [login, setLogin] = useState<ProviderLoginJob | null>(null);
  const [remote, setRemote] = useState(""),
    [owner, setOwner] = useState(""),
    [name, setName] = useState(projectName);
  const [visibility, setVisibility] = useState<"private" | "public">("private"),
    [acknowledged, setAcknowledged] = useState(false);
  const [busy, setBusy] = useState(""),
    [error, setError] = useState(""),
    [notice, setNotice] = useState("");
  const [loading, setLoading] = useState(true), [reviewRequired, setReviewRequired] = useState(false);
  const alive = useRef(true), readSequence = useRef(0), loginSequence = useRef(0);
  const remoteDraft = useRef({ value: "", revision: 0 }), storedOrigin = useRef(""), nameEdited = useRef(false);
  type ActionKind = "publication" | "github";
  type Operation = { kind: ActionKind; generation: number; confirmed: boolean };
  const working = useRef<Operation | null>(null);
  const permission = useRef({ read: access.canRead, publication: canPublish, github: canUseGitHub, publicationGeneration: 0, githubGeneration: 0 });
  if (permission.current.read && !access.canRead) ++readSequence.current;
  if (permission.current.publication && !canPublish) ++permission.current.publicationGeneration;
  if (permission.current.github && !canUseGitHub) ++permission.current.githubGeneration;
  Object.assign(permission.current, { read: access.canRead, publication: canPublish, github: canUseGitHub });
  const hasDraft = remote !== (delivery?.origin || "") || (!delivery?.origin && (name !== projectName || !!owner || visibility !== "private" || acknowledged));
  // Only navigation metadata enters the shared guard. Even an invalid URL may
  // contain credentials, so repository fields remain in component memory.
  useUnsavedProjectDraft(projectId, access.canRead && hasDraft, "Repository settings", "merge");
  const head = git.log[0]?.hash || "";
  function updateRemote(value: string) {
    remoteDraft.current.value = value;
    setRemote(value);
  }
  async function refresh(clearReviewRequired = false) {
    if (!permission.current.read) return;
    const sequence = ++readSequence.current, generation = permission.current.publicationGeneration;
    const revision = remoteDraft.current.revision;
    const pristine = remoteDraft.current.value === storedOrigin.current;
    const current = () => alive.current && permission.current.read && sequence === readSequence.current;
    setLoading(true);
    try {
      const value = await productApi.gitDelivery(projectId);
      if (!current()) return;
      setDelivery(value);
      storedOrigin.current = value.origin || "";
      if (pristine && revision === remoteDraft.current.revision) updateRemote(storedOrigin.current);
      setError("");
      if (clearReviewRequired && generation === permission.current.publicationGeneration) setReviewRequired(false);
    } catch (cause) {
      if (current()) {
        setError(cause instanceof ApiError && ["network_unavailable", "timeout"].includes(cause.code)
          ? "The latest repository status could not be read. Refresh the repository connection to try again."
          : errorText(cause));
        setReviewRequired(true);
      }
    } finally {
      if (current()) setLoading(false);
    }
  }
  useLayoutEffect(() => {
    alive.current = true;
    return () => {
      alive.current = false;
      ++readSequence.current;
      ++permission.current.publicationGeneration;
      ++permission.current.githubGeneration;
    };
  }, []);
  useEffect(() => { if (!nameEdited.current) setName(projectName); }, [projectName]);
  useEffect(() => {
    if (access.canRead) void refresh();
    return () => { ++readSequence.current; };
  }, [projectId, head, access.canRead]);
  function currentOperation(operation: Operation) {
    const generation = operation.kind === "publication" ? permission.current.publicationGeneration : permission.current.githubGeneration;
    return alive.current && working.current === operation && permission.current.read && permission.current[operation.kind] && operation.generation === generation;
  }
  useEffect(() => {
    const operation = working.current;
    if (operation && !currentOperation(operation)) {
      // Retire the response's authority without unlocking the actual request.
      // Regranting access cannot admit another write while this one still runs.
      if (operation.kind === "publication") {
        setReviewRequired(true);
        if (!operation.confirmed) setError("Project access changed while publishing. Refresh the repository connection and inspect its state before trying again. Your draft is kept.");
      }
    }
    if (!canUseGitHub) { setAccount(null); setLogin(null); }
    if (!access.canRead) { setDelivery(null); setLoading(false); setError(""); setNotice(""); }
  }, [access.canRead, canPublish, canUseGitHub]);
  useEffect(() => {
    if (!canUseGitHub || !login || !["running", "waiting"].includes(login.status)) return;
    let cancelled = false;
    const timer = setInterval(() => {
      const sequence = ++loginSequence.current, generation = permission.current.githubGeneration;
      const current = () => !cancelled && alive.current && permission.current.github && generation === permission.current.githubGeneration && sequence === loginSequence.current;
      void productApi
        .githubLogins()
        .then((value) => {
          const next = value.jobs.find((job) => job.id === login.id);
          if (current() && next) setLogin(next);
        })
        .catch((cause) => {
          if (current()) setError(errorText(cause));
        });
    }, 2000);
    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, [login?.id, login?.status, canUseGitHub]);
  const publicationBlocked = !canPublish || !!busy || loading || !delivery || reviewRequired;
  async function run(label: string, kind: ActionKind, action: (current: () => boolean) => Promise<void>) {
    if (working.current || !permission.current.read || !permission.current[kind] || (kind === "publication" && publicationBlocked)) return;
    const operation: Operation = { kind, generation: kind === "publication" ? permission.current.publicationGeneration : permission.current.githubGeneration, confirmed: false };
    working.current = operation;
    if (kind === "publication") ++readSequence.current;
    setBusy(label);
    setError("");
    setNotice("");
    try {
      await action(() => currentOperation(operation));
    } catch (cause) {
      if (currentOperation(operation)) {
        setError(errorText(cause));
        if (kind === "publication") setReviewRequired(true);
      }
    } finally {
      if (working.current === operation) {
        working.current = null;
        if (alive.current) setBusy("");
      }
    }
  }
  async function openOfficial(value: string | undefined, current: () => boolean) {
    const url = githubLink(value);
    if (!url || !current()) return;
    if ("__TAURI_INTERNALS__" in window) {
      const { openUrl } = await import("@tauri-apps/plugin-opener");
      if (current()) await openUrl(url);
    } else window.open(url, "_blank", "noopener,noreferrer");
  }
  async function published(result: GitPublication, current: () => boolean, submittedRemoteRevision?: number) {
    if (!current()) return;
    // A confirmed mutation is authoritative even if its follow-up read fails.
    // Retire any read started before confirmation and preserve newer typing.
    ++readSequence.current;
    if (working.current) working.current.confirmed = true;
    const acceptRemote = submittedRemoteRevision === undefined
      ? remoteDraft.current.value === storedOrigin.current
      : submittedRemoteRevision === remoteDraft.current.revision;
    storedOrigin.current = result.repository;
    setDelivery({ origin: result.repository, branch: result.branch || git.branch, head, publication: result });
    if (acceptRemote) updateRemote(result.repository);
    setReviewRequired(true);
    setNotice(
        result.verified
          ? `Remote commit verified: ${result.commit?.slice(0, 8)}.`
          : `${result.status}: repository connected. Push your reviewed commit next.`,
      );
    await refresh(true);
  }
  if (!access.canRead) return <section className="project-plan"><h2>Publish your reviewed work</h2><p role="status">Project access is unavailable.</p></section>;
  return (
    <section className="project-plan">
      <h2>Publish your reviewed work</h2>
      <p className="journey-muted">
        Connect a repository, then push the exact commit shown here. Creating a
        repository does not push your project.
      </p>
      {!canPublish && <p className="journey-muted" role="status">{access.loading ? "Checking project permissions…" : "Editing and deployment access are required to connect or publish a repository."}</p>}
      {!canUseGitHub && <p className="journey-muted" role="status">Workspace editing and secret access are required to manage GitHub sign-in.</p>}
      {loading && <p role="status">Reading the repository connection…</p>}
      {reviewRequired && <p className="journey-warning" role="status">Refresh the repository connection before publishing again. Your draft is kept.</p>}
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
          disabled={!!busy || !canUseGitHub}
          onClick={() =>
            void run("Checking GitHub", "github", async (current) => {
              const result = await productApi.githubCheck();
              if (current()) setAccount(result);
            })
          }
        >
          Check GitHub account
        </button>
        <button
          className="luci-button"
          disabled={
            !!busy || !canUseGitHub || (!!login && ["running", "waiting"].includes(login.status))
          }
          onClick={() =>
            void run("Opening GitHub sign-in", "github", async (current) => {
              const result = await productApi.githubLogin();
              if (current()) setLogin(result);
            })
          }
        >
          Sign in to GitHub
        </button>
      </div>
      {canUseGitHub && account && (
        <p role="status">
          {account.message}
          {account.signedIn && account.login ? ` · ${account.login}` : ""}{" "}
          {githubLink(account.actionUrl) && (
            <button
              className="luci-button"
              disabled={!!busy}
              onClick={() =>
                void run("Opening official install page", "github", (current) =>
                  openOfficial(account.actionUrl, current),
                )
              }
            >
              Install GitHub CLI
            </button>
          )}
        </p>
      )}
      {canUseGitHub && login && (
        <p role="status">
          {login.status} · {login.message}{" "}
          {login.userCode && <strong>{login.userCode}</strong>}{" "}
          {githubLink(login.verificationUrl) && (
            <button
              className="luci-button"
              disabled={!!busy}
              onClick={() =>
                void run("Opening GitHub sign-in", "github", (current) =>
                  openOfficial(login.verificationUrl, current),
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
              if (!name.trim() || (allowPublic && visibility === "public" && !acknowledged)) return;
              void run("Creating repository", "publication", async (current) =>
                published(
                  await productApi.githubCreate(projectId, {
                    name: name.trim(),
                    ...(owner.trim() ? { owner: owner.trim() } : {}),
                    visibility: allowPublic ? visibility : "private",
                    acknowledgePublic: allowPublic && acknowledged,
                  }),
                  current,
                ),
              );
            }}
          >
            <div className="setup-field-grid">
              <label className="luci-field">
                Repository name
                <input
                  required
                  disabled={!canPublish}
                  value={name}
                  onChange={(event) => { nameEdited.current = true; setName(event.target.value); setAcknowledged(false); }}
                />
              </label>
              <label className="luci-field">
                GitHub owner · optional
                <input
                  disabled={!canPublish}
                  value={owner}
                  onChange={(event) => { setOwner(event.target.value); setAcknowledged(false); }}
                  placeholder="Your signed-in account"
                />
              </label>
            </div>
            <label className="luci-field">
              Repository visibility
              <select
                disabled={!canPublish}
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
                  disabled={!canPublish}
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
                publicationBlocked ||
                !name.trim() ||
                (allowPublic && visibility === "public" && !acknowledged)
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
          if (!remote.trim()) return;
          const revision = remoteDraft.current.revision;
          void run("Connecting repository", "publication", async (current) =>
            published(await productApi.gitRemote(projectId, remote.trim()), current, revision),
          );
        }}
      >
        <label className="luci-field">
          Repository origin · HTTPS
          <input
            required
            disabled={!canPublish}
            type="url"
            value={remote}
            onChange={(event) => { ++remoteDraft.current.revision; updateRemote(event.target.value); }}
            placeholder="https://github.com/you/project.git"
          />
        </label>
        <button className="luci-button" disabled={publicationBlocked || !remote.trim()}>
          Connect existing repository
        </button>
      </form>
      <p>{delivery?.origin || "No remote repository connected."}</p>
      <button
        className="luci-button"
        disabled={!!busy || loading}
        onClick={() => void refresh(true)}
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
        disabled={publicationBlocked || !delivery?.origin || !git.clean || !head}
        onClick={() => {
          if (!delivery?.origin || !git.clean || !head) return;
          void run("Pushing and verifying remote commit", "publication", async (current) =>
            published(await productApi.gitPush(projectId, head, git.branch), current),
          );
        }}
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
