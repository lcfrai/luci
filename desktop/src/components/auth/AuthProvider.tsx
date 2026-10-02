import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { productApi, errorText } from "../../api/product";
import { AUTH_EXPIRED, authToken, setAuthToken } from "../../api/authSession";
import { detachAllTerminals } from "../../api/attachRegistry";
import { resetCockpit } from "../../store/store";
import { AdminPanel } from "./AdminPanel";
import "./auth.css";

export interface ControlUser {
  id: string;
  username: string;
  displayName: string;
  role: "member" | "admin" | "super-admin";
  disabled: boolean;
}
export interface ControlPolicy {
  sessionMemoryMiB?: number;
  sessionCPUMillis?: number;
  checkMemoryMiB?: number;
  checkCPUMillis?: number;
  allowHostCommands: boolean;
  allowCloudProvisioning: boolean;
  allowPublicPublishing: boolean;
  maxSessionsPerUser: number;
  maxConcurrentChecksPerUser: number;
  monthlyBudgetUsd: number;
  allowedHarnesses: string[];
  backingMode: "docker" | "aws" | "azure";
  instanceName: string;
}
export interface AuthProfile {
  user: ControlUser;
  permissions: {
    manageUsers: boolean;
    manageBacking: boolean;
    managePolicy: boolean;
    inspectAudit: boolean;
    runHostCommands: boolean;
  };
  policy: ControlPolicy;
}
interface ControlStatus {
  configured: boolean;
  instanceName: string;
  backing: string;
  bootstrapRequired: boolean;
  bootstrapTokenPath?: string;
}
interface AuthContextValue {
  profile: AuthProfile;
  refresh(): Promise<void>;
  openAccount(): void;
  openAdmin(): void;
  logout(): Promise<void>;
}
const AuthContext = createContext<AuthContextValue | null>(null);
export const useAuth = () => useContext(AuthContext);

export function AuthProvider({ children }: { children: ReactNode }) {
  const fragmentToken = useRef<string | null>(null);
  const [profile, setProfile] = useState<AuthProfile | null>(null);
  const [status, setStatus] = useState<ControlStatus | null>(null);
  const [loading, setLoading] = useState(true),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const [username, setUsername] = useState(""),
    [password, setPassword] = useState(""),
    [displayName, setDisplayName] = useState(""),
    [bootstrapToken, setBootstrapToken] = useState("");
  const [panel, setPanel] = useState<"account" | "admin" | null>(null);
  const refresh = useCallback(async () => {
    const token = authToken();
    const current = await productApi.request<AuthProfile>("/v1/auth/me");
    // A late profile response cannot sign a logged-out or changed account back in.
    if (token && token === authToken()) setProfile(current);
  }, []);
  const clear = useCallback(() => {
    setAuthToken(null);
    detachAllTerminals();
    resetCockpit();
    setProfile(null);
    setPanel(null);
    setPassword("");
  }, []);
  const load = useCallback(async () => {
    if (fragmentToken.current === null) {
      const hash = new URLSearchParams(window.location.hash.slice(1));
      const candidate = hash.get("bootstrap") || "";
      fragmentToken.current = /^[A-Za-z0-9_-]{43}$/.test(candidate)
        ? candidate
        : "";
      if (hash.has("bootstrap")) {
        hash.delete("bootstrap");
        window.history.replaceState(
          null,
          "",
          `${window.location.pathname}${window.location.search}${hash.size ? `#${hash}` : ""}`,
        );
      }
    }
    setLoading(true);
    setError("");
    try {
      const current = await productApi.request<ControlStatus>(
        "/v1/control-plane/status",
      );
      setStatus(current);
      if (current.bootstrapRequired && fragmentToken.current)
        setBootstrapToken(fragmentToken.current);
      if (!current.bootstrapRequired) {
        fragmentToken.current = "";
        setBootstrapToken("");
      }
      if (authToken()) await refresh();
      if (current.bootstrapRequired && "__TAURI_INTERNALS__" in window) {
        const { invoke } = await import("@tauri-apps/api/core");
        const token = await invoke<string | null>("bootstrap_token");
        if (token) setBootstrapToken(token);
      }
    } catch (cause) {
      setError(errorText(cause));
    } finally {
      setLoading(false);
    }
  }, [refresh]);
  useEffect(() => {
    void load();
  }, [load]);
  useEffect(() => {
    const expired = () => {
      clear();
      setError("Your sign-in expired. Sign in again to continue.");
    };
    window.addEventListener(AUTH_EXPIRED, expired);
    return () => window.removeEventListener(AUTH_EXPIRED, expired);
  }, [clear]);
  const logout = async () => {
    try {
      await productApi.request("/v1/auth/logout", {});
    } finally {
      clear();
    }
  };
  if (profile)
    return (
      <AuthContext.Provider
        value={{
          profile,
          refresh,
          openAccount: () => setPanel("account"),
          openAdmin: () => setPanel("admin"),
          logout,
        }}
      >
        {children}
        {panel && <AdminPanel mode={panel} onClose={() => setPanel(null)} />}
      </AuthContext.Provider>
    );
  return (
    <main className="auth-screen">
      <section className="auth-card">
        <p className="journey-eyebrow">
          LUCI · {status?.instanceName || "YOUR WORKSPACE"}
        </p>
        <h1>
          {status?.bootstrapRequired
            ? "Make this workspace yours."
            : "Welcome back."}
        </h1>
        <p className="journey-muted">
          {status?.bootstrapRequired
            ? "Create the first Super Admin. This account manages people, execution backing and workspace policy."
            : "Sign in to your workspace. Your projects and execution runtime are scoped to your account."}
        </p>
        {loading && <p role="status">Connecting to the control plane…</p>}
        {error && (
          <p className="journey-error" role="alert">
            {error}
          </p>
        )}
        {!loading && status && (
          <form
            className="journey-form"
            onSubmit={(event) => {
              event.preventDefault();
              if (busy) return;
              setBusy(true);
              setError("");
              void productApi
                .request<{ token: string; user: ControlUser }>(
                  status.bootstrapRequired
                    ? "/v1/auth/bootstrap"
                    : "/v1/auth/login",
                  status.bootstrapRequired
                    ? { bootstrapToken, username, displayName, password }
                    : { username, password },
                )
                .then(async (result) => {
                  setAuthToken(result.token);
                  setPassword("");
                  setBootstrapToken("");
                  fragmentToken.current = "";
                  await refresh();
                })
                .catch((cause) => setError(errorText(cause)))
                .finally(() => setBusy(false));
            }}
          >
            {status.bootstrapRequired && (
              <>
                <label className="luci-field">
                  Bootstrap token
                  <input
                    type="password"
                    autoComplete="off"
                    required
                    value={bootstrapToken}
                    onChange={(event) => setBootstrapToken(event.target.value)}
                  />
                  <small>
                    {bootstrapToken
                      ? "Loaded for the managed local service, or entered by you."
                      : `Use the one-time token on the control-plane host${status.bootstrapTokenPath ? ` at ${status.bootstrapTokenPath}` : ""}.`}
                  </small>
                </label>
                <label className="luci-field">
                  Display name
                  <input
                    required
                    value={displayName}
                    onChange={(event) => setDisplayName(event.target.value)}
                    autoComplete="name"
                  />
                </label>
              </>
            )}
            <label className="luci-field">
              Username
              <input
                required
                value={username}
                onChange={(event) => setUsername(event.target.value)}
                autoComplete="username"
              />
            </label>
            <label className="luci-field">
              Password
              <input
                required
                type="password"
                autoComplete={
                  status.bootstrapRequired ? "new-password" : "current-password"
                }
                value={password}
                onChange={(event) => setPassword(event.target.value)}
              />
            </label>
            <button className="luci-button luci-primary" disabled={busy}>
              {busy
                ? "Connecting…"
                : status.bootstrapRequired
                  ? "Create Super Admin account"
                  : "Sign in"}
            </button>
          </form>
        )}
        {!loading && (
          <button
            className="luci-button"
            disabled={busy}
            onClick={() => void load()}
          >
            Retry connection
          </button>
        )}
      </section>
    </main>
  );
}
