import { productApi, type ProductClient } from "./product";
import { teamScopeGeneration } from "./teamScope";

export interface AzureSetupRequest {
  name: string;
  region: string;
  monthlyBudgetUsd: number;
  maxConcurrentVMs: number;
}
export interface AzureSetupPlan {
  version: 1;
  status: "needs_prerequisites";
  purpose: "develop-microvm";
  provider: "azure";
  setupHostRequirement: string;
  sourceCommit: string;
  sourceRepository: string;
  bundleSHA256: string;
  bundleURL: "/v1/setup/cloud/azure/bundle";
  advancedAccountInputs: string[];
  advancedSelectionTemplate: unknown;
  missingInputs: string[];
  commands: string[];
  note: string;
}
const bundlePath = "/v1/setup/cloud/azure/bundle";
const strings = (value: unknown): value is string[] => Array.isArray(value) && value.length <= 64 && value.every(item => typeof item === "string" && item.length <= 20000);
function checkedPlan(value: AzureSetupPlan): AzureSetupPlan {
  if (!value || value.version !== 1 || value.status !== "needs_prerequisites" || value.provider !== "azure" || value.purpose !== "develop-microvm" ||
    value.bundleURL !== bundlePath || !/^[a-f0-9]{64}$/.test(value.bundleSHA256) || !/^[a-f0-9]{40}$/.test(value.sourceCommit) ||
    typeof value.setupHostRequirement !== "string" || !strings(value.missingInputs) || !strings(value.commands) || !value.commands.length) {
    throw new Error("These Azure instructions could not be verified. Update Luci or prepare a fresh plan.");
  }
  return value;
}

export function createAzureSetupClient(client: Pick<ProductClient, "request" | "requestBlob"> = productApi) {
  return {
    plan: async (input: AzureSetupRequest, signal?: AbortSignal) => checkedPlan(await client.request<AzureSetupPlan>("/v1/setup/cloud/azure/plan", input, "POST", 60000, signal)),
    download: async (plan: AzureSetupPlan, signal?: AbortSignal) => {
      checkedPlan(plan);
      const scope = teamScopeGeneration();
      const blob = await client.requestBlob(bundlePath, { method: "GET" }, 60000, signal);
      if (!blob.size || blob.size > 4 * 1024 * 1024) throw new Error("The Azure installer download is incomplete. Prepare fresh instructions and try again.");
      if (!globalThis.crypto?.subtle) throw new Error("This browser cannot verify the installer. Open Luci on its supported local address and retry.");
      const digest = await crypto.subtle.digest("SHA-256", await blob.arrayBuffer());
      const actual = Array.from(new Uint8Array(digest), byte => byte.toString(16).padStart(2, "0")).join("");
      signal?.throwIfAborted();
      if (scope !== teamScopeGeneration()) throw new Error("Your workspace changed during this download. Prepare instructions for the current workspace.");
      if (actual !== plan.bundleSHA256) throw new Error("The installer changed since these instructions were prepared. Prepare fresh instructions before downloading or copying commands.");
      return blob;
    },
  };
}

const prerequisiteNames: Record<string, [string, string]> = {
  existing_durable_Azure_Blob_Terraform_backend: ["Your Azure account and costs", "An Azure subscription, resource group and durable state storage you control."],
  reviewed_Azure_quote_and_account_capacity: ["Your Azure account and costs", "A current Azure quote, account capacity and permission to create the reviewed resources. Your budget is an admission limit, not a billing cap."],
  exact_runtime_and_controller_packages: ["Runtime preparation", "The exact Luci runtime and controller packages; follow the installer's package preparation guide."],
  immutable_gallery_image_versions: ["Runtime preparation", "Versioned Ubuntu images suitable for the isolated worker runtime."],
  controller_operator_template: ["Runtime preparation", "The controller's private runtime template, prepared with the installer guide."],
  private_network_and_ssh_settings: ["Private access", "An unused private network, supported VM size/zone and your SSH public key."],
  key_vault_exact_version_references: ["Private access", "Private, versioned Key Vault references. Keep secrets out of this form and copied commands."],
};
export function azurePrerequisites(missing: string[]) {
  const groups = new Map<string, string[]>();
  for (const input of missing) {
    const [group, label] = prerequisiteNames[input] ?? ["Additional installer requirements", input.replace(/_/g, " ")];
    groups.set(group, [...(groups.get(group) ?? []), label]);
  }
  return [...groups].map(([title, items]) => ({ title, items }));
}
