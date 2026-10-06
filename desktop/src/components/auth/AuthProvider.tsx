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
import { rememberInvitedTeam } from "../../api/teamWorkspacePreference";
import { detachAllTerminals } from "../../api/attachRegistry";
import { resetCockpit } from "../../store/store";
import { AdminPanel } from "./AdminPanel";
import {
  portableAccessAvailable,
  portableRequest,
  PortableAccessError,
} from "./portableAccess";
import {
  AuthEntry,
  type ControlStatus,
  type LocalAccess,
  type EntryMode,
  type EntryDraft,
} from "./AuthEntry";
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
interface AuthContextValue {
  profile: AuthProfile;
  refresh(): Promise<void>;
  openAccount(): void;
  openAdmin(): void;
  logout(): Promise<void>;
}
const AuthContext = createContext<AuthContextValue | null>(null);
export const useAuth = () => useContext(AuthContext);
const entryErrorText = (cause: unknown) =>
  typeof cause === "string" ? cause : errorText(cause);

export interface AuthResult {
  token: string;
  user: ControlUser;
  teamId?: string;
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const fragmentToken = useRef<string | null>(null);
  const portableToken = useRef<string | null>(null);
  const generation = useRef(0);
  const [profile, setProfile] = useState<AuthProfile | null>(null);
  const [status, setStatus] = useState<ControlStatus | null>(null);
  const [local, setLocal] = useState<LocalAccess>({ available: false });
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [localError, setLocalError] = useState("");
  const [connectionError, setConnectionError] = useState(false);
  const [mode, setMode] = useState<EntryMode>("sign-in");
  const [bootstrapToken, setBootstrapToken] = useState("");
  const [panel, setPanel] = useState<"account" | "admin" | null>(null);
  const refresh = useCallback(async () => {
    const token = authToken();
    const current = await productApi.request<AuthProfile>("/v1/auth/me");
    if (token && token === authToken()) setProfile(current);
  }, []);
  const clear = useCallback(() => {
    setAuthToken(null);
    detachAllTerminals();
    resetCockpit();
    setProfile(null);
    setPanel(null);
    setMode("sign-in");
  }, []);
  const load = useCallback(async () => {
    const attempt = ++generation.current;
    if (fragmentToken.current === null) {
      const hash = new URLSearchParams(window.location.hash.slice(1));
      const candidate = hash.get("bootstrap") || "";
      const owner = hash.get("owner") || "";
      portableToken.current =
        portableAccessAvailable() && /^[A-Za-z0-9_-]{43}$/.test(owner)
          ? owner
          : "";
      fragmentToken.current = /^[A-Za-z0-9_-]{43}$/.test(candidate)
        ? candidate
        : "";
      if (hash.has("bootstrap") || hash.has("owner")) {
        hash.delete("bootstrap");
        hash.delete("owner");
        window.history.replaceState(
          null,
          "",
          `${window.location.pathname}${window.location.search}${hash.size ? `#${hash}` : ""}`,
        );
      }
    }
    setLoading(true);
    setError("");
    setLocalError("");
    setConnectionError(false);
    try {
      const current = await productApi.request<ControlStatus>(
        "/v1/control-plane/status",
      );
      if (generation.current !== attempt) return;
      setStatus(current);
      if (current.bootstrapRequired) {
        // An obsolete session must not stop first-run ownership verification.
        clear();
        if (fragmentToken.current) setBootstrapToken(fragmentToken.current);
      } else {
        fragmentToken.current = "";
        setBootstrapToken("");
      }
      if ("__TAURI_INTERNALS__" in window) {
        try {
          const { invoke } = await import("@tauri-apps/api/core");
          const access = await invoke<LocalAccess>("local_access_info");
          if (generation.current !== attempt) return;
          setLocal({ ...access, transport: "native" });
          if (current.bootstrapRequired) {
            const token = await invoke<string | null>("bootstrap_token");
            if (generation.current !== attempt) return;
            if (token) setBootstrapToken(token);
          }
        } catch (cause) {
          if (generation.current !== attempt) return;
          setLocal({ available: false });
          setLocalError(entryErrorText(cause));
        }
      } else if (portableToken.current) {
        try {
          const access = await portableRequest<{ owner: ControlUser | null }>(
            portableToken.current,
            "/v1/auth/local-owner",
          );
          if (generation.current !== attempt) return;
          setLocal({
            available: true,
            owner: access.owner,
            transport: "portable",
          });
        } catch (cause) {
          if (generation.current !== attempt) return;
          setLocal({ available: false });
          setLocalError(
            `${entryErrorText(cause)} Reopen Start-Luci.ps1 or start-luci.sh for a fresh local access link.`,
          );
        }
      }
      if (!current.bootstrapRequired && authToken()) {
        try {
          await refresh();
        } catch (cause) {
          if (generation.current !== attempt) return;
          if (!authToken())
            setError("Your sign-in expired. Sign in again to continue.");
          else {
            setError(errorText(cause));
            setConnectionError(true);
          }
        }
      }
    } catch (cause) {
      if (generation.current !== attempt) return;
      setStatus(null);
      setError(errorText(cause));
      setConnectionError(true);
    } finally {
      if (generation.current === attempt) setLoading(false);
    }
  }, [refresh, clear]);
  useEffect(() => {
    void load();
    return () => {
      generation.current++;
    };
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
      setError("");
      await load();
    }
  };
  const changeMode = (value: EntryMode) => {
    setMode(value);
    setError("");
    setConnectionError(false);
  };
  const submit = async (draft: EntryDraft) => {
    if (busy || !status) return;
    setBusy(true);
    setError("");
    setConnectionError(false);
    try {
      let result: AuthResult;
      const identity = {
        username: draft.username,
        displayName: draft.displayName,
        password: draft.password,
      };
      if (status.bootstrapRequired) {
        result = await productApi.request<AuthResult>("/v1/auth/bootstrap", {
          bootstrapToken,
          ...identity,
        });
      } else if (mode === "recover") {
        if (!local.available || !local.owner)
          throw new Error(
            "Recover this account through its workspace administrator.",
          );
        if (local.transport === "portable") {
          result = await portableRequest<AuthResult>(
            portableToken.current || "",
            "/v1/auth/local-owner/recover",
            { newPassword: draft.password },
          );
        } else {
          const { invoke } = await import("@tauri-apps/api/core");
          result = await invoke<AuthResult>("recover_local_owner", {
            newPassword: draft.password,
          });
        }
      } else if (mode === "create" && local.available) {
        if (local.transport === "portable") {
          result = await portableRequest<AuthResult>(
            portableToken.current || "",
            "/v1/auth/local-account",
            identity,
          );
        } else {
          const { invoke } = await import("@tauri-apps/api/core");
          result = await invoke<AuthResult>("create_local_account", identity);
        }
      } else if (mode === "create") {
        result = await productApi.request<AuthResult>(
          "/v1/auth/invitations/accept",
          { inviteCode: draft.inviteCode, ...identity },
        );
      } else {
        result = await productApi.request<AuthResult>("/v1/auth/login", {
          username: draft.username,
          password: draft.password,
        });
      }
      setAuthToken(result.token);
      if (mode === "create" && result.teamId) rememberInvitedTeam(productApi.baseUrl, result.user.id, result.teamId);
      setBootstrapToken("");
      fragmentToken.current = "";
      setStatus({ ...status, configured: true, bootstrapRequired: false });
      setMode("sign-in");
      await refresh();
    } catch (cause) {
      setError(entryErrorText(cause));
      if (cause instanceof PortableAccessError && cause.status === 403) {
        setLocalError(
          "Reopen Start-Luci.ps1 or start-luci.sh for a fresh local access link. Your account details have been kept in this form.",
        );
      }
      if (
        local.available &&
        (mode === "create" || mode === "recover") &&
        /local service.*(unavailable|did not|connection)|request did not reach/i.test(
          entryErrorText(cause),
        )
      ) {
        setLocalError(
          "Check the local service, then retry. Your account details are still in this form.",
        );
      }
      const failure = cause as { status?: number; code?: string };
      setConnectionError(failure.status === 0 || (failure.status ?? 0) >= 500);
      if (failure.code === "bootstrap_denied") {
        // Another browser may have completed first setup while this form was open.
        try {
          const current = await productApi.request<ControlStatus>(
            "/v1/control-plane/status",
          );
          setStatus(current);
          if (!current.bootstrapRequired) {
            setBootstrapToken("");
            setMode("sign-in");
            setError(
              "This workspace already has an account. Sign in, create a separate account, or recover your sign-in below.",
            );
          }
        } catch {
          setConnectionError(true);
        }
      }
    } finally {
      setBusy(false);
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
    <AuthEntry
      status={status}
      local={local}
      loading={loading}
      busy={busy}
      error={error}
      connectionError={connectionError}
      localError={localError}
      portableLocal={portableAccessAvailable()}
      mode={mode}
      bootstrapToken={bootstrapToken}
      onBootstrapToken={setBootstrapToken}
      onMode={changeMode}
      onRetry={() => void load()}
      onSubmit={submit}
    />
  );
}
