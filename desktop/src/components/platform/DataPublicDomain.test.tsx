import { beforeEach, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { DataPublicDomain, publicDataAddress } from "./DataPublicDomain";
import { dataApi, type DataDomainStatus } from "../../api/data";
import { productApi } from "../../api/product";
const access = vi.hoisted(() => ({ read: true, edit: true, deploy: true, secrets: true, loading: false, error: null as string | null, scope: "owned-account" }));
vi.mock("../../hooks/useProjectAccess", () => ({ useProjectAccess: () => ({ loading: access.loading, error: access.error, canRead: access.read,
  canEdit: !access.loading && !access.error && access.read && access.edit, permissions: { ...access } }) }));
vi.mock("../../hooks/useProjectDraft", () => ({ useProjectDraftScope: (id: string) => `${access.scope}:${id}` }));
beforeEach(() => { Object.assign(access, { read: true, edit: true, deploy: true, secrets: true, loading: false, error: null, scope: "owned-account" }); });
vi.mock("../../api/data", () => ({ dataApi: { domain: vi.fn(), configureDomain: vi.fn() } }));
vi.mock("../../api/product", async (original) => ({ ...await original<typeof import("../../api/product")>(), productApi: { targets: vi.fn() } }));
const initial: DataDomainStatus = { projectId: "owned", status: "not-configured", revision: 0, message: "Data remains private.", emailDelivery: "unconfigured" };
beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(dataApi.domain).mockResolvedValue(structuredClone(initial));
  vi.mocked(productApi.targets).mockResolvedValue({ targets: [{ id: "target", name: "My cloud", baseUrl: "https://coolify.example.com", serverUuid: "server" }] });
});
async function fill() {
  await screen.findByText("Data remains private.");
  fireEvent.change(screen.getByLabelText("Data deployment server"), { target: { value: "target" } });
  fireEvent.change(screen.getByLabelText("Data API domain"), { target: { value: "data.example.com" } });
  fireEvent.change(screen.getByLabelText("Published application address"), { target: { value: "https://app.example.com" } });
}
it("keeps mount read-only and clears public acknowledgement when the draft changes", async () => {
  render(<DataPublicDomain projectId="owned" disabled={false} ready onChange={vi.fn()} />);
  await fill();
  const publish = screen.getByRole("button", { name: "Publish data API" });
  expect((publish as HTMLButtonElement).disabled).toBe(true);
  expect(dataApi.configureDomain).not.toHaveBeenCalled();
  fireEvent.click(screen.getByLabelText("Make this project's client API available publicly at this domain."));
  expect((publish as HTMLButtonElement).disabled).toBe(false);
  fireEvent.change(screen.getByLabelText("Published application address"), { target: { value: "https://other.example.com" } });
  expect((publish as HTMLButtonElement).disabled).toBe(true);
});
it("sends only the acknowledged public request and keeps accepted work pending", async () => {
  const change = vi.fn();
  vi.mocked(dataApi.configureDomain).mockResolvedValue({ ...initial, revision: 1, status: "pending", message: "Preparing this saved attempt.", operation: { id: "job", action: "publish", status: "running", stage: "queued", message: "Queued", createdAt: "now", updatedAt: "now" } });
  render(<DataPublicDomain projectId="owned" disabled={false} ready onChange={change} />);
  await fill();
  fireEvent.click(screen.getByLabelText("Make this project's client API available publicly at this domain."));
  fireEvent.click(screen.getByRole("button", { name: "Publish data API" }));
  await waitFor(() => expect(dataApi.configureDomain).toHaveBeenCalledExactlyOnceWith("owned", { action: "publish", expectedRevision: 0, targetId: "target", domain: "data.example.com", applicationOrigin: "https://app.example.com", acknowledgePublic: true }));
  expect(await screen.findByText("Preparing this saved attempt.")).not.toBeNull();
  expect(screen.queryByLabelText("Public HTTPS API address")).toBeNull();
  expect(change).toHaveBeenCalledTimes(1);
});
it("prevents duplicate publication and keeps progress visible after the server accepts background work", async () => {
  let complete!: (value: DataDomainStatus) => void;
  vi.mocked(dataApi.configureDomain).mockImplementation(() => new Promise(resolve => { complete = resolve; }));
  render(<DataPublicDomain projectId="owned" disabled={false} ready onChange={vi.fn()} />);
  await fill();
  fireEvent.click(screen.getByLabelText("Make this project's client API available publicly at this domain."));
  const publish = screen.getByRole("button", { name: "Publish data API" }) as HTMLButtonElement;
  expect(publish.type).toBe("submit");
  fireEvent.click(publish);
  expect(publish.getAttribute("aria-busy")).toBe("true"); expect(publish.disabled).toBe(true);
  fireEvent.submit(publish.form!); fireEvent.click(publish);
  expect(dataApi.configureDomain).toHaveBeenCalledTimes(1);
  await act(async () => { complete({ ...initial, revision: 1, status: "pending", message: "Preparing this saved attempt.", operation: { id: "job", action: "publish", status: "running", stage: "queued", message: "Queued", createdAt: "now", updatedAt: "now" } }); });
  const update = await screen.findByRole("button", { name: "Update public connection" }) as HTMLButtonElement;
  expect(update.getAttribute("aria-busy")).toBe("true"); expect(update.disabled).toBe(true);
  const check = screen.getByRole("button", { name: "Check DNS and HTTPS" }) as HTMLButtonElement;
  expect(check.type).toBe("button"); expect(check.hasAttribute("aria-busy")).toBe(false); expect(check.disabled).toBe(true);
  expect(screen.queryByLabelText("Public HTTPS API address")).toBeNull();
});
it("reconciles an uncertain creation without offering another publish", async () => {
  vi.mocked(dataApi.domain).mockResolvedValue({ ...initial, revision: 2, status: "uncertain", targetId: "target", domain: "data.example.com", applicationOrigin: "https://app.example.com" });
  vi.mocked(dataApi.configureDomain).mockResolvedValue({ ...initial, revision: 3, status: "pending", message: "Waiting for DNS." });
  render(<DataPublicDomain projectId="owned" disabled={false} ready onChange={vi.fn()} />);
  const check = await screen.findByRole("button", { name: "Check DNS and HTTPS" });
  expect((screen.getByRole("button", { name: "Update public connection" }) as HTMLButtonElement).disabled).toBe(true);
  fireEvent.click(check);
  await waitFor(() => expect(dataApi.configureDomain).toHaveBeenCalledExactlyOnceWith("owned", { action: "refresh", expectedRevision: 2 }));
});
it("permits an explicitly confirmed public stop while native services are stopped", async () => {
  vi.mocked(dataApi.domain).mockResolvedValue({ ...initial, revision: 4, status: "verifiedHTTPS", publicUrl: "https://data.example.com", domain: "https://data.example.com", targetId: "target" });
  vi.mocked(dataApi.configureDomain).mockResolvedValue({ ...initial, revision: 5, status: "stopped" });
  render(<DataPublicDomain projectId="owned" disabled={false} ready={false} onChange={vi.fn()} />);
  const stop = await screen.findByRole("button", { name: "Stop public access" });
  expect((stop as HTMLButtonElement).disabled).toBe(true);
  expect((screen.getByRole("button", { name: "Update public connection" }) as HTMLButtonElement).disabled).toBe(true);
  fireEvent.click(screen.getByLabelText("Stop public API access for applications using this address."));
  fireEvent.click(stop);
  await waitFor(() => expect(dataApi.configureDomain).toHaveBeenCalledExactlyOnceWith("owned", { action: "stop", expectedRevision: 4 }));
});
it("ignores a late publish reply after switching projects", async () => {
  let complete!: (value: DataDomainStatus) => void;
  const change = vi.fn();
  vi.mocked(dataApi.configureDomain).mockImplementation(() => new Promise(resolve => { complete = resolve; }));
  const view = render(<DataPublicDomain projectId="owned" disabled={false} ready onChange={change} />);
  await fill();
  fireEvent.click(screen.getByLabelText("Make this project's client API available publicly at this domain."));
  fireEvent.click(screen.getByRole("button", { name: "Publish data API" }));
  vi.mocked(dataApi.domain).mockResolvedValue({ ...initial, projectId: "other" });
  view.rerender(<DataPublicDomain projectId="other" disabled={false} ready onChange={change} />);
  complete({ ...initial, status: "verifiedHTTPS", publicUrl: "https://old.example.com" });
  await waitFor(() => expect(dataApi.domain).toHaveBeenLastCalledWith("other"));
  expect(change).not.toHaveBeenCalled();
  expect(screen.queryByLabelText("Public HTTPS API address")).toBeNull();
});
it("rejects another project's readback instead of adopting its domain", async () => {
  vi.mocked(dataApi.domain).mockResolvedValue({ ...initial, projectId: "foreign", publicUrl: "https://foreign.example.com", status: "verifiedHTTPS" });
  render(<DataPublicDomain projectId="owned" disabled={false} ready onChange={vi.fn()} />);
  expect((await screen.findByRole("alert")).textContent).toContain("does not belong");
  expect(screen.queryByLabelText("Public HTTPS API address")).toBeNull();
});
it("keeps the saved address usable when server loading fails and offers a read-only recovery", async () => {
  vi.mocked(dataApi.domain).mockResolvedValue({ ...initial, status: "verifiedHTTPS", revision: 3, domain: "data.example.com", publicUrl: "https://data.example.com", targetId: "target" });
  vi.mocked(productApi.targets).mockRejectedValueOnce(new Error("Servers could not be loaded."));
  render(<DataPublicDomain projectId="owned" disabled={false} ready onChange={vi.fn()} />);
  expect((await screen.findByRole("alert")).textContent).toContain("Servers could not be loaded");
  expect((screen.getByLabelText(/Public HTTPS API address/) as HTMLInputElement).value).toBe("https://data.example.com");
  fireEvent.click(screen.getByRole("button", { name: "Reload saved connection" }));
  await waitFor(() => expect(screen.queryByRole("alert")).toBeNull());
  expect(screen.getByRole("option", { name: "My cloud" })).not.toBeNull();
  expect(dataApi.configureDomain).not.toHaveBeenCalled();
});
it("never presents an unverified or mismatched API address as HTTPS verified", () => {
  for (const value of [
    { status: "pending", publicUrl: "https://data.example.com" },
    { status: "verifiedHTTPS", publicUrl: "http://127.0.0.1:8730" },
    { status: "verifiedHTTPS", publicUrl: "https://user:password@data.example.com" },
    { status: "verifiedHTTPS", publicUrl: "https://data.example.com/admin" },
    { status: "verifiedHTTPS", publicUrl: "https://data.example.com", domain: "other.example.com" },
  ]) expect(publicDataAddress({ ...initial, ...value })).toBeNull();
  expect(publicDataAddress({ ...initial, status: "verifiedHTTPS", publicUrl: "https://data.example.com/", domain: "data.example.com" })).toBe("https://data.example.com");
});

it("preserves the same public connection draft through permission refresh and blocks read-only publication", async () => {
  const change = vi.fn(), view = render(<DataPublicDomain projectId="owned" disabled={false} ready onChange={change} />); await fill();
  const domain = screen.getByLabelText("Data API domain") as HTMLInputElement;
  access.loading = true; access.secrets = false; view.rerender(<DataPublicDomain projectId="owned" disabled={false} ready onChange={change} />);
  expect(screen.getByLabelText("Data API domain")).toBe(domain); expect(domain.value).toBe("data.example.com");
  access.loading = false; access.secrets = true; access.edit = false; view.rerender(<DataPublicDomain projectId="owned" disabled={false} ready onChange={change} />);
  expect((screen.getByRole("button", { name: "Publish data API" }) as HTMLButtonElement).disabled).toBe(true);
  fireEvent.submit(domain.closest("form")!); expect(dataApi.configureDomain).not.toHaveBeenCalled();
});
it("rejects a delayed public connection result after editing is revoked and restored", async () => {
  let finish!: (value: DataDomainStatus) => void;
  vi.mocked(dataApi.configureDomain).mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  const change = vi.fn(), view = render(<DataPublicDomain projectId="owned" disabled={false} ready onChange={change} />); await fill();
  fireEvent.click(screen.getByLabelText("Make this project's client API available publicly at this domain.")); fireEvent.click(screen.getByRole("button", { name: "Publish data API" }));
  access.edit = false; view.rerender(<DataPublicDomain projectId="owned" disabled={false} ready onChange={change} />);
  access.edit = true; view.rerender(<DataPublicDomain projectId="owned" disabled={false} ready onChange={change} />);
  await act(async () => finish({ ...initial, revision: 1, status: "pending", message: "Late published receipt" }));
  expect(change).not.toHaveBeenCalled(); expect(screen.queryByText("Late published receipt")).toBeNull();
  expect((screen.getByRole("button", { name: "Publish data API" }) as HTMLButtonElement).disabled).toBe(false);
});
it("does not request private domain configuration or deployment targets without secrets access", () => {
  access.secrets = false; render(<DataPublicDomain projectId="owned" disabled={false} ready onChange={vi.fn()} />);
  expect(dataApi.domain).not.toHaveBeenCalled(); expect(productApi.targets).not.toHaveBeenCalled();
});
