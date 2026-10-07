import { productApi, type ProductClient } from "./product";

export type DependencyId = "git" | "docker" | "github" | "aws" | "azure";
export type DependencyAction = "none" | "check" | "install_winget" | "elevation" | "restart_app" | "restart_computer" | "start_docker" | "manual_install";
export interface DependencyJob {
  id: string;
  dependencyId: DependencyId;
  status: "queued" | "installing" | "ready" | "action_required" | "failed" | "interrupted";
  message: string;
  actionRequired: DependencyAction;
  createdAt: string;
  updatedAt: string;
  exitCode?: string;
}
export interface Dependency {
  id: DependencyId;
  name: string;
  packageId: string;
  supported: boolean;
  reason: string;
  check: { status: "unchecked" | "ready" | "missing" | "action_required"; version?: string; message: string; checkedAt?: string };
  actionRequired: DependencyAction;
  job: DependencyJob | null;
}
export interface DependencyStatus {
  scope: "control-plane-host";
  platform: string;
  canInstall: boolean;
  installer: { id: "winget"; status: "unchecked" | "ready" | "missing" | "untrusted" | "unsupported"; message: string; checkedAt?: string };
  dependencies: Dependency[];
}
export function createDependencyClient(client: Pick<ProductClient, "request"> = productApi) {
  const base = "/v1/setup/dependencies";
  return {
    get: () => client.request<DependencyStatus>(base),
    check: () => client.request<DependencyStatus>(`${base}/check`, {}),
    install: (id: DependencyId, acceptAgreements: boolean) => client.request<DependencyJob>(`${base}/${id}/install`, { acceptAgreements }),
  };
}
export type DependencyClient = ReturnType<typeof createDependencyClient>;
export const dependencyApi = createDependencyClient();
