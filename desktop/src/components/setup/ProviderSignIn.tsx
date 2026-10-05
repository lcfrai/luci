import { Button } from "../fluid/components/button";
import { useEffect, useState } from "react";
import {
  errorText,
  type ProductClient,
  type ProviderLoginJob,
} from "../../api/product";

export function ProviderSignIn({
  client,
  provider,
}: {
  client: ProductClient;
  provider: "aws" | "azure";
}) {
  const [job, setJob] = useState<ProviderLoginJob | null>(null),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  useEffect(() => {
    setJob(null);
    setError("");
  }, [provider]);
  useEffect(() => {
    if (!job || !["running", "waiting"].includes(job.status)) return;
    let cancelled = false;
    const timer = setInterval(() => {
      void client
        .providerLogins()
        .then((v) => {
          const current = v.jobs.find((j) => j.id === job.id);
          if (current && !cancelled) setJob(current);
        })
        .catch((e) => {
          if (!cancelled) setError(errorText(e));
        });
    }, 2500);
    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, [client, job]);
  const officialUrl = (value?: string) => {
    if (!value) return null;
    try {
      const u = new URL(value);
      return u.protocol === "https:" &&
        [
          "microsoft.com",
          "microsoftonline.com",
          "aws.amazon.com",
          "docs.aws.amazon.com",
          "learn.microsoft.com",
        ].some((d) => u.hostname === d || u.hostname.endsWith(`.${d}`))
        ? u.toString()
        : null;
    } catch {
      return null;
    }
  };
  const active = job && ["running", "waiting"].includes(job.status);
  async function signIn() {
    setBusy(true);
    setError("");
    try {
      setJob(await client.providerLogin(provider));
    } catch (e) {
      setError(errorText(e));
    } finally {
      setBusy(false);
    }
  }
  const openOfficial = async (url: string) => {
    if ("__TAURI_INTERNALS__" in window) {
      const { openUrl } = await import("@tauri-apps/plugin-opener");
      await openUrl(url);
    } else {
      window.open(url, "_blank", "noopener,noreferrer");
    }
  };
  return (
    <section className="provider-signin">
      <h3>Connect your {provider === "aws" ? "AWS" : "Azure"} account</h3>
      <p className="journey-muted">
        Sign-in opens your provider’s own secure page. Credentials stay with its
        local CLI.
      </p>
      {error && (
        <p className="journey-error" role="alert">
          {error}
        </p>
      )}
      {job && <p role="status">{job.message}</p>}
      {job?.userCode && (
        <p>
          Your one-time sign-in code:{" "}
          <code className="provider-code">{job.userCode}</code>
        </p>
      )}
      <div className="setup-actions">
        <Button
          type="button"
          variant="secondary"
          disabled={busy || !!active}
          onClick={() => void signIn()}
        >
          {active
            ? "Waiting for provider sign-in…"
            : `Sign in to ${provider === "aws" ? "AWS" : "Azure"}`}
        </Button>
        {officialUrl(job?.verificationUrl) && (
          <Button
            type="button"
            variant="secondary"
            onClick={() =>
              void openOfficial(officialUrl(job?.verificationUrl)!)
            }
          >
            Open provider sign-in
          </Button>
        )}
        {officialUrl(job?.actionUrl) && (
          <Button
            type="button"
            variant="secondary"
            onClick={() => void openOfficial(officialUrl(job?.actionUrl)!)}
          >
            Install provider tools
          </Button>
        )}
      </div>
    </section>
  );
}
