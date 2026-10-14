import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { createProductClient } from "../../api/product";
import { setAuthToken } from "../../api/authSession";
import { selectTeamScope } from "../../api/teamScope";
import { readAzureConnectionDescriptor, readAzureSetupProgress, type AzureConnectionDescriptor, type AzureSetupPlan } from "../../api/azureSetup";
import { AzureSetup } from "./AzureSetup";

const archive = new TextEncoder().encode("source-bound fixture installer");
// Vitest supplies Node's real WebCrypto; keep Node ambient types out of the app.
const { webcrypto } = await vi.importActual<{ webcrypto: Crypto }>("node:crypto");
const archiveDigest = await webcrypto.subtle.digest("SHA-256", archive);
const archiveSHA256 = Array.from(new Uint8Array(archiveDigest), byte => byte.toString(16).padStart(2, "0")).join("");
const plan: AzureSetupPlan = {
  version: 1, status: "needs_prerequisites", purpose: "develop-microvm", provider: "azure",
  setupHostRequirement: "Linux or WSL2 with Python 3, Go, Terraform, Azure CLI and an isolated Ubuntu build environment.",
  sourceCommit: "a".repeat(40), sourceRepository: "https://github.com/lcfrai/luci.git",
  bundleSHA256: archiveSHA256, bundleURL: "/v1/setup/cloud/azure/bundle",
  advancedAccountInputs: [], advancedSelectionTemplate: {},
  missingInputs: ["existing_durable_Azure_Blob_Terraform_backend", "exact_runtime_and_controller_packages", "key_vault_exact_version_references", "reviewed_Azure_quote_and_account_capacity"],
  commands: ["verify exact fixture installer", "python3 luci-azure-setup/azure-setup/installer.py plan --selection luci-azure-private/selection.json"],
  note: "Preparation does not connect or provision anything.",
};
const sshKey = new Uint8Array([0, 0, 0, 11, ...new TextEncoder().encode("ssh-ed25519"), 0, 0, 0, 32, ...Array.from({ length: 32 }, (_, index) => index)]);
const sshFingerprintBytes = new Uint8Array(await webcrypto.subtle.digest("SHA-256", sshKey));
const sshFingerprint = "SHA256:" + btoa(String.fromCharCode(...sshFingerprintBytes)).replace(/=+$/, "");
const descriptor: AzureConnectionDescriptor = {
  version: 1, state: "host_identity_observed_private_route_unverified", sourceCommit: plan.sourceCommit,
  scope: "luci-123456789abc", generation: "gen-123456789abc",
  controllerResourceID: `/subscriptions/${"a".repeat(8)}-${"a".repeat(4)}-${"a".repeat(4)}-${"a".repeat(4)}-${"a".repeat(12)}/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/luci-controller`,
  vmID: "a".repeat(8) + "-" + "a".repeat(4) + "-" + "a".repeat(4) + "-" + "a".repeat(4) + "-" + "a".repeat(12),
  bootID: "b".repeat(8) + "-" + "b".repeat(4) + "-" + "b".repeat(4) + "-" + "b".repeat(4) + "-" + "b".repeat(12),
  host: "10.0.1.10", port: 22, user: "luci", hostKey: "ssh-ed25519 " + btoa(String.fromCharCode(...sshKey)),
  fingerprint: sshFingerprint, productAPI: "http://127.0.0.1:8730", readbackDigest: "a".repeat(64), connected: false,
  selectionDigest: "b".repeat(64), bundleDigest: "c".repeat(64),
};
const progress = { version: 1, sourceCommit: plan.sourceCommit, scope: descriptor.scope, selectionDigest: descriptor.selectionDigest,
  stages: ["credentials", "images", "runtime-gallery", "controller-gallery", "infrastructure", "connection", "retirement"].map(id => ({ id, state: id === "connection" ? "host_identity_readback_pending" : "not_started", uncertain: id === "connection", receiptPath: "/private/receipt.json" })),
  connected: false, priceVerified: false, finalInvoiceVerified: false };
beforeEach(() => {
  vi.stubGlobal("crypto", webcrypto);
  setAuthToken("private-fixture-owner");
  Object.defineProperty(URL, "createObjectURL", { configurable: true, value: vi.fn(() => "blob:verified-fixture") });
  Object.defineProperty(URL, "revokeObjectURL", { configurable: true, value: vi.fn() });
  vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => {});
});
afterEach(() => { cleanup(); setAuthToken(null); vi.unstubAllGlobals(); });
function fixture(fingerprints = [sshFingerprint]) {
  const fetcher = vi.fn<typeof fetch>(async input => {
    const route = new URL(String(input)).pathname;
    if (route.endsWith("/plan")) return new Response(JSON.stringify(plan));
    if (route.endsWith("/bundle")) return new Response(archive, { headers: { "Content-Type": "application/x-tar" } });
    if (route.endsWith("/ssh/connections")) return new Response(JSON.stringify({ connections: [] }));
    if (route.endsWith("/ssh/inspect")) return new Response(JSON.stringify({ id: "ssh-1", host: descriptor.host, fingerprints, createdAt: new Date().toISOString() }));
    throw new Error(`Unexpected setup request ${route}`);
  });
  const client = createProductClient("http://localhost:19783", fetcher);
  const user = userEvent.setup();
  const copy = vi.spyOn(navigator.clipboard, "writeText").mockResolvedValue();
  return { fetcher, client, user, copy };
}
async function prepare(f: ReturnType<typeof fixture>) {
  await f.user.click(screen.getByRole("button", { name: "Prepare Azure instructions" }));
  await screen.findByRole("heading", { name: "Instructions prepared · not connected" });
}

it("requests an offline plan with exact nonsecret fields and only copies after authenticated verified download", async () => {
  const f = fixture(); render(<AzureSetup client={f.client} disabled={false} budget={45} />);
  expect(f.fetcher).not.toHaveBeenCalled();
  await f.user.clear(screen.getByLabelText("Deployment name")); await f.user.type(screen.getByLabelText("Deployment name"), "my-luci");
  await prepare(f);
  expect(JSON.parse(String(f.fetcher.mock.calls[0][1]?.body))).toEqual({ name: "my-luci", region: "australiaeast", monthlyBudgetUsd: 45, maxConcurrentVMs: 1 });
  expect(f.fetcher.mock.calls[0][1]?.headers).toHaveProperty("Authorization", "Bearer private-fixture-owner");
  expect(screen.getByRole("button", { name: "Copy setup commands" })).toHaveProperty("disabled", true);
  expect(screen.queryByLabelText("Verified Azure setup commands")).toBeNull();
  await f.user.click(screen.getByText("Prerequisites and installer identity"));
  expect(screen.getByRole("heading", { name: "Private access" })).toBeTruthy();
  expect(screen.getByRole("heading", { name: "Your Azure account and costs" })).toBeTruthy();
  await f.user.click(screen.getByRole("button", { name: "Download verified Azure installer" }));
  await screen.findByText("Verified installer download started. Your workspace is still unconnected.");
  expect(f.fetcher.mock.calls[1][1]?.method).toBe("GET");
  expect(f.fetcher.mock.calls[1][1]?.headers).toHaveProperty("Authorization", "Bearer private-fixture-owner");
  expect(URL.createObjectURL).toHaveBeenCalledOnce();
  expect(HTMLAnchorElement.prototype.click).toHaveBeenCalledOnce();
  await f.user.click(screen.getByRole("button", { name: "Copy setup commands" }));
  await waitFor(() => expect(f.copy).toHaveBeenCalledWith(plan.commands.join("\n")));
  expect(f.fetcher.mock.calls.map(([url]) => new URL(String(url)).pathname)).toEqual(["/v1/setup/cloud/azure/plan", "/v1/setup/cloud/azure/bundle"]);
});

it("bounds and validates nonsecret controller/progress receipts before any SSH effect", async () => {
  expect(await readAzureConnectionDescriptor(new File([JSON.stringify(descriptor)], "azure-connection.json"), plan)).toMatchObject({ host: descriptor.host, fingerprint: sshFingerprint });
  expect(await readAzureSetupProgress(new File([JSON.stringify(progress)], "status.json"), plan, descriptor)).toMatchObject({ connected: false, priceVerified: false });
  await expect(readAzureConnectionDescriptor(new File(["x".repeat(16385)], "too-large.json"), plan)).rejects.toThrow(/too large/);
  await expect(readAzureConnectionDescriptor(new File([JSON.stringify({ ...descriptor, host: "8.8.8.8" })], "public.json"), plan)).rejects.toThrow(/private host contract/);
  await expect(readAzureConnectionDescriptor(new File([JSON.stringify({ ...descriptor, fingerprint: "SHA256:wrong" })], "wrong-key.json"), plan)).rejects.toThrow(/differs/);
  await expect(readAzureSetupProgress(new File([JSON.stringify({ ...progress, selectionDigest: "d".repeat(64) })], "stale.json"), plan, descriptor)).rejects.toThrow(/stale/);
});

it("imports a private controller receipt into protected SSH inspection without importing credentials or silently connecting", async () => {
  const f = fixture(); render(<AzureSetup client={f.client} disabled={false} />); await prepare(f);
  await f.user.upload(screen.getByLabelText(/^Connection file/), new File([JSON.stringify(descriptor)], "azure-connection.json", { type: "application/json" }));
  expect(await screen.findByText(/Connection file loaded/)).toBeTruthy();
  expect(screen.getByRole("heading", { name: "3. Connect workspace" })).toBeTruthy();
  expect(screen.getByText(/controller VM/).closest("details")?.open).toBe(false);
  expect(screen.getByLabelText("Server address")).toHaveProperty("value", descriptor.host);
  expect(screen.getByLabelText("Server address")).toHaveProperty("disabled", true);
  expect(screen.getByLabelText("Private key path")).toHaveProperty("value", "");
  expect(f.fetcher.mock.calls.map(([url]) => new URL(String(url)).pathname)).toEqual(["/v1/setup/cloud/azure/plan", "/v1/setup/ssh/connections"]);
  await f.user.click(screen.getByRole("button", { name: "Inspect server fingerprint" }));
  expect(await screen.findByRole("checkbox", { name: /host key shown by my authenticated Azure setup command/ })).toBeTruthy();
  expect(screen.getByRole("button", { name: "Trust and connect" })).toHaveProperty("disabled", true);
  expect(f.fetcher.mock.calls.map(([url]) => new URL(String(url)).pathname)).not.toContain("/v1/setup/ssh/connect");
});

it("shows uncertain installer stages and refuses a different observed SSH host key", async () => {
  const f = fixture(["SHA256:" + "z".repeat(43)]); render(<AzureSetup client={f.client} disabled={false} />); await prepare(f);
  await f.user.upload(screen.getByLabelText(/^Setup progress file/), new File([JSON.stringify(progress)], "status.json", { type: "application/json" }));
  expect(await screen.findByText(/Outcome uncertain · reconcile before continuing/)).toBeTruthy();
  await f.user.click(screen.getByText("Commands to save these files"));
  expect(screen.getByText(/installer.py status --bundle luci-azure-setup --selection luci-azure-private\/selection.json > luci-azure-private\/status.json/)).toBeTruthy();
  expect(screen.getAllByText(/Coding environment image/).length).toBe(2);
  expect(screen.getAllByText(/Cloud resources/).length).toBe(2);
  await f.user.upload(screen.getByLabelText(/^Connection file/), new File([JSON.stringify(descriptor)], "azure-connection.json", { type: "application/json" }));
  await screen.findByText(/Connection file loaded/);
  await f.user.click(screen.getByRole("button", { name: "Inspect server fingerprint" }));
  expect((await screen.findByRole("alert")).textContent).toMatch(/server key differs/);
  expect(screen.getByRole("button", { name: "Trust and connect" })).toHaveProperty("disabled", true);
  expect(f.fetcher.mock.calls.map(([url]) => new URL(String(url)).pathname)).not.toContain("/v1/setup/ssh/connect");
});

it("refuses changed installer bytes and never downloads or enables copy", async () => {
  const f = fixture(); render(<AzureSetup client={f.client} disabled={false} />);
  await prepare(f); f.fetcher.mockResolvedValueOnce(new Response("different installer"));
  await f.user.click(screen.getByRole("button", { name: "Download verified Azure installer" }));
  expect((await screen.findByRole("alert")).textContent).toMatch(/installer changed.*Prepare fresh instructions/);
  expect(URL.createObjectURL).not.toHaveBeenCalled(); expect(f.copy).not.toHaveBeenCalled();
  expect(screen.getByRole("button", { name: "Copy setup commands" })).toHaveProperty("disabled", true);
});

it("invalidates verified commands when deployment choices change", async () => {
  const f = fixture(); render(<AzureSetup client={f.client} disabled={false} />);
  await prepare(f); await f.user.click(screen.getByRole("button", { name: "Download verified Azure installer" }));
  await screen.findByLabelText("Verified Azure setup commands");
  await f.user.clear(screen.getByLabelText("Azure region"));
  await f.user.type(screen.getByLabelText("Azure region"), "australiasoutheast");
  expect(screen.queryByLabelText("Verified Azure setup commands")).toBeNull();
  expect(screen.queryByRole("button", { name: "Copy setup commands" })).toBeNull();
  expect(f.fetcher).toHaveBeenCalledTimes(2);
});

it("discards a delayed plan when permission is revoked", async () => {
  const f = fixture(); let resolve!: (response: Response) => void;
  f.fetcher.mockImplementationOnce(() => new Promise<Response>(done => { resolve = done; }));
  const view = render(<AzureSetup client={f.client} disabled={false} />);
  await f.user.click(screen.getByRole("button", { name: "Prepare Azure instructions" }));
  view.rerender(<AzureSetup client={f.client} disabled />);
  await act(async () => { resolve(new Response(JSON.stringify(plan))); });
  await waitFor(() => expect(screen.queryByText("Preparing instructions…")).toBeNull());
  expect(screen.queryByRole("heading", { name: "Instructions prepared · not connected" })).toBeNull();
  expect(screen.getByRole("button", { name: "Prepare Azure instructions" }).matches(":disabled")).toBe(true);
  await f.user.click(screen.getByRole("button", { name: "Prepare Azure instructions" }));
  expect(f.fetcher).toHaveBeenCalledTimes(1);
  expect(URL.createObjectURL).not.toHaveBeenCalled();
});

it("keeps package/API failures honest and refuses an unexpected bundle URL", async () => {
  const f = fixture(); render(<AzureSetup client={f.client} disabled={false} />);
  f.fetcher.mockResolvedValueOnce(new Response(JSON.stringify({ error: "azure_setup_unavailable", message: "This package has no verified Azure installer." }), { status: 503 }));
  await f.user.click(screen.getByRole("button", { name: "Prepare Azure instructions" }));
  expect((await screen.findByRole("alert")).textContent).toMatch(/no verified Azure installer/);
  f.fetcher.mockResolvedValueOnce(new Response(JSON.stringify({ ...plan, bundleURL: "https://example.invalid/installer?token=private" })));
  await f.user.click(screen.getByRole("button", { name: "Prepare Azure instructions" }));
  expect((await screen.findByRole("alert")).textContent).toMatch(/could not be verified/);
  expect(screen.queryByRole("button", { name: "Download verified Azure installer" })).toBeNull();
  expect(f.fetcher).toHaveBeenCalledTimes(2);
});

it("never downloads a response that arrives after leaving setup", async () => {
  const f = fixture(); const view = render(<AzureSetup client={f.client} disabled={false} />);
  await prepare(f); let resolve!: (response: Response) => void;
  f.fetcher.mockImplementationOnce(() => new Promise<Response>(done => { resolve = done; }));
  await f.user.click(screen.getByRole("button", { name: "Download verified Azure installer" }));
  view.unmount(); resolve(new Response(archive));
  await new Promise(done => setTimeout(done, 0));
  expect(URL.createObjectURL).not.toHaveBeenCalled();
});

it("drops the plan when the authenticated workspace changes", async () => {
  const f = fixture(); render(<AzureSetup client={f.client} disabled={false} />);
  await prepare(f); act(() => selectTeamScope(""));
  await waitFor(() => expect(screen.queryByRole("heading", { name: "Instructions prepared · not connected" })).toBeNull());
  expect(screen.queryByRole("button", { name: "Copy setup commands" })).toBeNull();
  expect(f.fetcher).toHaveBeenCalledTimes(1);
});
