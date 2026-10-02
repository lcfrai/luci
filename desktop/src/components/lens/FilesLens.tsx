import { useEffect, useRef, useState } from "react";
import { productApi, errorText, type FileResult } from "../../api/product";
import { ProjectRequired, JourneyHeader } from "../journey/shared";

export function FilesLens() {
  return (
    <ProjectRequired>
      {(id) => <ProjectFiles key={id} projectId={id} />}
    </ProjectRequired>
  );
}
export function ProjectFiles({ projectId }: { projectId: string }) {
  const [path, setPath] = useState("");
  const [result, setResult] = useState<FileResult | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const request = useRef(0);
  async function open(next: string) {
    const current = ++request.current;
    setLoading(true);
    setError("");
    try {
      const value = await productApi.files(projectId, next);
      if (current === request.current) {
        setResult(value);
        setPath(next);
      }
    } catch (cause) {
      if (current === request.current) setError(errorText(cause));
    } finally {
      if (current === request.current) setLoading(false);
    }
  }
  useEffect(() => {
    void open("");
    return () => {
      request.current += 1;
    };
  }, [projectId]);
  return (
    <section className="journey-page">
      <JourneyHeader
        title="Project files"
        subtitle={
          path ||
          "Browse the files in your project. Sensitive and out-of-project paths stay closed."
        }
      >
        <button
          className="luci-button"
          onClick={() => void open("")}
          disabled={loading}
        >
          Project root
        </button>
        {path && (
          <button
            className="luci-button"
            onClick={() => void open(path.split("/").slice(0, -1).join("/"))}
            disabled={loading}
          >
            Up one folder
          </button>
        )}
        <button
          className="luci-button"
          onClick={() => void open(path)}
          disabled={loading}
        >
          Refresh
        </button>
      </JourneyHeader>
      {error && (
        <p className="journey-error" role="alert">
          {error}
        </p>
      )}
      {loading && (
        <p className="journey-muted" role="status">
          Reading files…
        </p>
      )}
      {result && "entries" in result && (
        <ul className="journey-directory">
          {result.entries.length === 0 && (
            <li className="journey-muted">This folder is empty.</li>
          )}
          {result.entries.map((entry) => (
            <li key={entry.path}>
              <button onClick={() => void open(entry.path)} disabled={loading}>
                <span aria-hidden="true">
                  {entry.kind === "directory" ? "▤" : "·"}
                </span>
                <span>{entry.name}</span>
                <small>
                  {entry.kind === "directory"
                    ? "Folder"
                    : `${entry.size.toLocaleString()} bytes`}
                </small>
              </button>
            </li>
          ))}
        </ul>
      )}
      {result && "content" in result && (
        <>
          {result.truncated && (
            <p className="journey-warning">
              This file is too large to display in full.
            </p>
          )}
          <pre className="journey-code">{result.content}</pre>
        </>
      )}
    </section>
  );
}
