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
export interface AzureConnectionDescriptor {
  version: 1;
  state: "host_identity_observed_private_route_unverified";
  sourceCommit: string;
  scope: string;
  generation: string;
  controllerResourceID: string;
  dataDiskID: string;
  vmID: string;
  bootID: string;
  host: string;
  port: 22;
  user: "luci";
  hostKey: string;
  fingerprint: string;
  productAPI: "http://127.0.0.1:8730";
  readbackDigest: string;
  connected: false;
  selectionDigest: string;
  bundleDigest: string;
  observedAtUTC: string;
}
export interface AzureSetupProgress {
  version: 1;
  sourceCommit: string;
  scope: string;
  selectionDigest: string;
  stages: { id: string; state: string; uncertain: boolean; receiptPath: string }[];
  connected: false;
  priceVerified: false;
  finalInvoiceVerified: false;
}
const sha256 = /^[a-f0-9]{64}$/;
const uuid = /^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/i;
const stageIds = ["credentials", "images", "runtime-gallery", "controller-gallery", "infrastructure", "connection", "retirement"];
function record(value: unknown): value is Record<string, unknown> { return !!value && typeof value === "object" && !Array.isArray(value); }
function rfc1918(value: unknown): value is string {
  if (typeof value !== "string") return false;
  const octets = value.split(".");
  if (octets.length !== 4 || octets.some(part => !/^(0|[1-9][0-9]{0,2})$/.test(part) || Number(part) > 255)) return false;
  const [a, b] = octets.map(Number);
  return a === 10 || a === 172 && b >= 16 && b <= 31 || a === 192 && b === 168;
}
async function parseBoundedFile(file: File): Promise<unknown> {
  if (!file || file.size < 2 || file.size > 16 * 1024) throw new Error("This Azure receipt is too large or empty. Choose the exact installer JSON receipt.");
  const value = JSON.parse(await file.text()) as unknown;
  return value;
}
export async function readAzureConnectionDescriptor(file: File, plan: AzureSetupPlan): Promise<AzureConnectionDescriptor> {
  const value = await parseBoundedFile(file);
  if (!record(value)) throw new Error("The Azure connection receipt is malformed.");
  const keys = ["version", "state", "sourceCommit", "scope", "generation", "controllerResourceID", "dataDiskID", "vmID", "bootID", "host", "port", "user", "hostKey", "fingerprint", "productAPI", "readbackDigest", "connected", "selectionDigest", "bundleDigest", "observedAtUTC"];
  if (Object.keys(value).some(key => !keys.includes(key)) || value.version !== 1 || value.state !== "host_identity_observed_private_route_unverified" ||
    value.sourceCommit !== plan.sourceCommit || typeof value.scope !== "string" || !/^[a-zA-Z0-9_.:-]{1,160}$/.test(value.scope) ||
    typeof value.generation !== "string" || !/^[a-zA-Z0-9_.:-]{1,160}$/.test(value.generation) ||
    typeof value.controllerResourceID !== "string" || !/^\/subscriptions\/[0-9a-f-]{36}\/resourcegroups\/[a-z0-9_-]+\/providers\/microsoft\.compute\/virtualmachines\/[a-z0-9_-]+$/.test(value.controllerResourceID) ||
    typeof value.dataDiskID !== "string" || !/^\/subscriptions\/[0-9a-f-]{36}\/resourcegroups\/[a-z0-9_-]+\/providers\/microsoft\.compute\/disks\/[a-z0-9_-]+$/.test(value.dataDiskID) ||
    value.controllerResourceID.split("/providers/")[0] !== value.dataDiskID.split("/providers/")[0] ||
    !uuid.test(String(value.vmID)) || !uuid.test(String(value.bootID)) || !rfc1918(value.host) || value.port !== 22 || value.user !== "luci" ||
    value.productAPI !== "http://127.0.0.1:8730" || value.connected !== false ||
    typeof value.readbackDigest !== "string" || !sha256.test(value.readbackDigest) ||
    typeof value.selectionDigest !== "string" || !sha256.test(value.selectionDigest) ||
    typeof value.bundleDigest !== "string" || !sha256.test(value.bundleDigest) ||
    typeof value.observedAtUTC !== "string" || !/^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d{1,6})?Z$/.test(value.observedAtUTC) || !Number.isFinite(Date.parse(value.observedAtUTC))) {
    throw new Error("The Azure connection receipt does not match this plan or the supported private host contract.");
  }
  const parts = typeof value.hostKey === "string" ? value.hostKey.split(" ") : [];
  if (parts.length !== 2 || parts[0] !== "ssh-ed25519" || !/^[A-Za-z0-9+/]+={0,2}$/.test(parts[1])) throw new Error("The Azure controller host key is invalid.");
  let raw: Uint8Array;
  try { raw = Uint8Array.from(atob(parts[1]), char => char.charCodeAt(0)); }
  catch { throw new Error("The Azure controller host key is invalid."); }
  const prefix = new Uint8Array([0, 0, 0, 11, ...new TextEncoder().encode("ssh-ed25519"), 0, 0, 0, 32]);
  if (raw.length !== 51 || prefix.some((byte, index) => raw[index] !== byte) || !globalThis.crypto?.subtle) throw new Error("The Azure controller host key cannot be verified.");
  const digest = new Uint8Array(await globalThis.crypto.subtle.digest("SHA-256", Uint8Array.from(raw).buffer));
  const fingerprint = "SHA256:" + btoa(String.fromCharCode(...digest)).replace(/=+$/, "");
  if (value.fingerprint !== fingerprint) throw new Error("The Azure controller fingerprint differs from its host key.");
  return value as unknown as AzureConnectionDescriptor;
}
export async function readAzureSetupProgress(file: File, plan: AzureSetupPlan, descriptor?: AzureConnectionDescriptor | null): Promise<AzureSetupProgress> {
  const value = await parseBoundedFile(file);
  if (!record(value) || value.version !== 1 || value.sourceCommit !== plan.sourceCommit ||
    typeof value.scope !== "string" || !/^[a-zA-Z0-9_.:-]{1,160}$/.test(value.scope) ||
    typeof value.selectionDigest !== "string" || !sha256.test(value.selectionDigest) ||
    descriptor && (value.scope !== descriptor.scope || value.selectionDigest !== descriptor.selectionDigest) ||
    value.connected !== false || value.priceVerified !== false || value.finalInvoiceVerified !== false ||
    !Array.isArray(value.stages) || value.stages.length !== stageIds.length ||
    value.stages.some((row: unknown, index: number) => !record(row) || row.id !== stageIds[index] ||
      typeof row.state !== "string" || !/^[a-z][a-z0-9_]{0,79}$/.test(row.state) ||
      typeof row.uncertain !== "boolean" || typeof row.receiptPath !== "string" || row.receiptPath.length > 1024)) {
    throw new Error("The Azure progress receipt is stale or invalid for this plan.");
  }
  return value as unknown as AzureSetupProgress;
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
