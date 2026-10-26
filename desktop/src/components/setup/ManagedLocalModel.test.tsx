import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";
import { createProductClient, type AgentConnectionCatalogue } from "../../api/product";
import type { LocalModelSnapshot } from "../../api/localModel";
import { ManagedLocalModel } from "./ManagedLocalModel";
import { selectTeamScope } from "../../api/teamScope";
import { setAuthToken } from "../../api/authSession";

vi.mock("../auth/AuthProvider", () => ({ useAuth: () => ({ profile: { user: { id: "qa" } } }) }));

afterEach(() => { vi.useRealTimers(); setAuthToken(null); selectTeamScope(""); });

const snapshot = (state: LocalModelSnapshot["status"]["state"] = "stopped"): LocalModelSnapshot => ({
  available: true, canManage: true, revision: 3,
  config: { enabled: true, startWithLuci: false, executablePath: "C:\\Tools\\Ollama\\ollama.exe", modelStore: "E:\\Models", port: 11435, model: "local-coder-16k:latest", profile: { contextTokens: 16384, kvCacheType: "q8_0", flashAttention: true, parallelism: 1, jobMemoryMiB: 7168, processMemoryMiB: 6656, cpuLimitPercent: 20 } },
  status: { state, generation: state === "stopped" ? "" : "18446744073709551614", observedAt: new Date().toISOString(), endpoint: "http://localhost:11435/v1", model: "local-coder-16k:latest" },
});

function mount(initial = snapshot()) {
  let current = initial;
  let catalogue: AgentConnectionCatalogue = { revision: 7, activeConnectionId: "byo", connections: [{ id: "byo", name: "Local · local-coder-16k:latest", kind: "compatible-api", revision: 1, enabled: true, configured: true, keyPresent: true, endpoints: { chatCompletions: "https://provider.example/v1" }, models: { opencode: "other-model" } }] };
  const fetcher = vi.fn<typeof fetch>().mockImplementation(async (url, init) => {
    const path = new URL(String(url)).pathname;
    const body = init?.body ? JSON.parse(String(init.body)) : undefined;
    if (path === "/v1/setup/local-model/start") current = { ...current, status: { ...current.status, state: "starting", generation: "18446744073709551614" } };
    else if (path === "/v1/setup/local-model/stop") current = { ...current, status: { ...current.status, state: "stopped" } };
    else if (path === "/v1/setup/local-model" && body) current = { ...current, revision: current.revision + 1, config: { ...current.config!, ...body } };
    if (path === "/v1/setup/agent-connections") {
      if (body?.action === "save") catalogue = { ...catalogue, revision: catalogue.revision + 1, connections: [...catalogue.connections, { ...body, id: "managed-one", revision: 1, configured: true, keyPresent: false }] };
      if (body?.action === "select") catalogue = { ...catalogue, revision: catalogue.revision + 1, activeConnectionId: body.id };
      return new Response(JSON.stringify(catalogue));
    }
    return new Response(JSON.stringify(current));
  });
  const client = createProductClient("http://localhost", fetcher);
  const view = render(<ManagedLocalModel client={client} cloud={false} />);
  return { fetcher, client, view, setSnapshot: (value: LocalModelSnapshot) => { current = value; }, posts: () => fetcher.mock.calls.filter(([, init]) => init?.method === "POST") };
}

it("viewing and saving configuration never starts, downloads or selects a model", async () => {
  const fixture = mount();
  await screen.findByText("Stopped");
  expect(fixture.posts()).toHaveLength(0);
  await userEvent.click(screen.getByText("Configure local model"));
  expect((screen.getByLabelText("Start with Luci when capacity is available") as HTMLInputElement).checked).toBe(false);
  await userEvent.click(screen.getByLabelText("Start with Luci when capacity is available"));
  await userEvent.click(screen.getByRole("button", { name: "Save model configuration" }));
  await screen.findByText("Configuration saved. The model has not been started or downloaded.");
  expect(fixture.posts()).toHaveLength(1);
  expect(fixture.posts()[0][0]).toBe("http://localhost/v1/setup/local-model");
  expect(JSON.parse(String(fixture.posts()[0][1]?.body))).toMatchObject({ expectedRevision: 3, startWithLuci: true, model: "local-coder-16k:latest" });
});

it("starts once with the saved revision without claiming the model is loaded", async () => {
  const fixture = mount();
  await screen.findByText("Stopped");
  await userEvent.dblClick(screen.getByRole("button", { name: "Start model" }));
  await screen.findByText("Starting");
  expect(fixture.posts()).toHaveLength(1);
  expect(JSON.parse(String(fixture.posts()[0][1]?.body))).toEqual({ expectedRevision: 3 });
  expect(screen.queryByText("Model loaded")).toBeNull();
  expect(screen.getByRole("button", { name: "Use for OpenCode" }).matches(":disabled")).toBe(true);
});

it("stops the exact generation without losing uint64 precision", async () => {
  const fixture = mount(snapshot("loaded"));
  await screen.findByText("Model loaded");
  await userEvent.click(screen.getByRole("button", { name: "Stop model" }));
  await screen.findByText("Stopped");
  expect(JSON.parse(String(fixture.posts()[0][1]?.body))).toEqual({ expectedRevision: 3, expectedGeneration: "18446744073709551614" });
});

it("does not replace or select a service whose retirement is unknown", async () => {
  const fixture = mount(snapshot("unknown"));
  await screen.findByText("Status unconfirmed");
  expect(screen.getByRole("button", { name: "Start model" }).matches(":disabled")).toBe(true);
  expect(screen.getByRole("button", { name: "Stop model" }).matches(":disabled")).toBe(true);
  expect(screen.getByRole("button", { name: "Use for OpenCode" }).matches(":disabled")).toBe(true);
  expect(fixture.posts()).toHaveLength(0);
});

it("preserves an unsaved draft when refreshed configuration has changed", async () => {
  const fixture = mount();
  await screen.findByText("Stopped");
  await userEvent.click(screen.getByText("Configure local model"));
  await userEvent.clear(screen.getByLabelText("Installed model"));
  await userEvent.type(screen.getByLabelText("Installed model"), "my-unsaved-model");
  fixture.setSnapshot({ ...snapshot(), revision: 4 });
  await userEvent.click(screen.getByRole("button", { name: "Refresh model status" }));
  await screen.findByText(/Your draft is preserved/);
  expect((screen.getByLabelText("Installed model") as HTMLInputElement).value).toBe("my-unsaved-model");
  expect(screen.getByRole("button", { name: "Save model configuration" }).matches(":disabled")).toBe(true);
  expect(screen.getByRole("button", { name: "Start model" }).matches(":disabled")).toBe(true);
  expect(fixture.posts()).toHaveLength(0);
});

it("keeps host paths and management actions absent for ordinary members", async () => {
  const value = snapshot("available");
  value.canManage = false;
  delete value.config!.executablePath; delete value.config!.modelStore;
  mount(value);
  await screen.findByText("Server available");
  expect(screen.queryByRole("button", { name: "Start model" })).toBeNull();
  expect(screen.queryByLabelText("Ollama application")).toBeNull();
  expect(screen.getByRole("button", { name: "Use for OpenCode" }).matches(":disabled")).toBe(false);
});

it("keeps edits made after an explicit reload was requested", async () => {
  const fixture = mount();
  await screen.findByText("Stopped");
  await userEvent.click(screen.getByText("Configure local model"));
  let finish!: (value: Response) => void;
  fixture.fetcher.mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  await userEvent.click(screen.getByRole("button", { name: "Reload saved configuration" }));
  await userEvent.clear(screen.getByLabelText("Installed model"));
  await userEvent.type(screen.getByLabelText("Installed model"), "newer-unsaved-model");
  await act(async () => { finish(new Response(JSON.stringify({ ...snapshot(), revision: 4 }))); });
  expect((screen.getByLabelText("Installed model") as HTMLInputElement).value).toBe("newer-unsaved-model");
  expect(screen.getByText(/Your draft is preserved/)).not.toBeNull();
  expect(fixture.posts()).toHaveLength(0);
});

it("marks a failed observation unavailable and resumes serial status reads after recovery", async () => {
  vi.useFakeTimers();
  let fixture!: ReturnType<typeof mount>;
  await act(async () => { fixture = mount(snapshot("loaded")); });
  expect(screen.getByText("Model loaded")).not.toBeNull();
  fixture.fetcher.mockRejectedValueOnce(new Error("temporary transport failure"));
  await act(async () => { await vi.advanceTimersByTimeAsync(2000); });
  expect(screen.getByText("Status unavailable")).not.toBeNull();
  expect(screen.queryByText("Model loaded")).toBeNull();
  expect(screen.getByRole("button", { name: "Use for OpenCode" }).matches(":disabled")).toBe(true);
  fixture.setSnapshot(snapshot("loaded"));
  await act(async () => { await vi.advanceTimersByTimeAsync(2000); });
  expect(screen.getByText("Model loaded")).not.toBeNull();
  expect(screen.queryByText("Status unavailable")).toBeNull();
  expect(fixture.fetcher.mock.calls).toHaveLength(3);
  expect(fixture.posts()).toHaveLength(0);
  fixture.view.unmount();
});

it("creates a managed connection and selects it without overwriting a same-name BYO entry", async () => {
  const fixture = mount(snapshot("available"));
  await screen.findByText("Server available");
  await userEvent.click(screen.getByRole("button", { name: "Use for OpenCode" }));
  await screen.findByText(/Local model selected for new OpenCode sessions/);
  const bodies = fixture.posts().map(([, init]) => JSON.parse(String(init?.body)));
  expect(bodies).toHaveLength(2);
  expect(bodies[0]).toMatchObject({ action: "save", expectedRevision: 7, managedLocalModel: { revision: 3 }, openCodeService: "local-ollama", openCodeThinking: "off", endpoints: { chatCompletions: "http://localhost:11435/v1" }, models: { opencode: "local-coder-16k:latest" } });
  expect(bodies[0]).not.toHaveProperty("id");
  expect(bodies[1]).toEqual({ action: "select", expectedRevision: 8, id: "managed-one" });
});

it("rechecks a changed owner before writing or selecting a connection", async () => {
  const fixture = mount(snapshot("available"));
  await screen.findByText("Server available");
  fixture.setSnapshot({ ...snapshot("available"), revision: 4 });
  await userEvent.click(screen.getByRole("button", { name: "Use for OpenCode" }));
  await screen.findByText(/The local model changed/);
  expect(fixture.posts()).toHaveLength(0);
});

it("does not continue a multi-step selection after leaving the screen", async () => {
  const fixture = mount(snapshot("available"));
  await screen.findByText("Server available");
  let finish!: (value: Response) => void;
  fixture.fetcher.mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  await userEvent.click(screen.getByRole("button", { name: "Use for OpenCode" }));
  fixture.view.unmount();
  await act(async () => { finish(new Response(JSON.stringify(snapshot("available")))); });
  expect(fixture.posts()).toHaveLength(0);
});

it.each(["team", "account"] as const)("does not continue selection when the %s changes during a catalogue read", async kind => {
  const fixture = mount(snapshot("available"));
  await screen.findByText("Server available");
  let finish!: (value: Response) => void;
  fixture.fetcher.mockImplementationOnce(async () => new Response(JSON.stringify(snapshot("available"))));
  fixture.fetcher.mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  await userEvent.click(screen.getByRole("button", { name: "Use for OpenCode" }));
  await waitFor(() => expect(finish).toBeDefined());
  await act(async () => {
    if (kind === "team") selectTeamScope("team-abcdefghijklmnopqrstuv");
    else setAuthToken("unit-test-new-account");
    finish(new Response(JSON.stringify({ revision: 7, connections: [], activeConnectionId: "" })));
  });
  expect(fixture.posts()).toHaveLength(0);
  expect(screen.queryByText(/Local model selected for new OpenCode sessions/)).toBeNull();
  fixture.view.unmount();
});

it("does not load the host-model panel in a cloud workspace", async () => {
  const fetcher = vi.fn<typeof fetch>();
  render(<ManagedLocalModel client={createProductClient("https://workspace.example", fetcher)} cloud />);
  await waitFor(() => expect(screen.queryByRole("region", { name: "Local model" })).toBeNull());
  expect(fetcher).not.toHaveBeenCalled();
});

it("does not expose host model controls through a selected team workspace", async () => {
  selectTeamScope("team-abcdefghijklmnopqrstuv");
  const fetcher = vi.fn<typeof fetch>();
  const view = render(<ManagedLocalModel client={createProductClient("http://localhost", fetcher)} cloud={false} />);
  expect(screen.queryByRole("region", { name: "Local model" })).toBeNull();
  expect(fetcher).not.toHaveBeenCalled();
  view.unmount();
});

it("does not present an old observation as a selectable service", async () => {
  const value = snapshot("loaded"); value.status.observedAt = new Date(Date.now() - 6000).toISOString();
  mount(value);
  await screen.findByText("Model loaded");
  expect(screen.getByRole("button", { name: "Use for OpenCode" }).matches(":disabled")).toBe(true);
});

it.each([
  ["gpu_binary_untrusted", "Luci could not verify the installed NVIDIA tool’s signature. Model startup is paused."],
  ["gpu_metadata_unavailable", "Luci could not verify the installed NVIDIA tool’s version information. Model startup is paused."],
  ["gpu_process_unavailable", "Luci could not start or observe its GPU capacity check. No model was started."],
  ["gpu_process_exited", "The GPU capacity check exited before it produced a usable observation. No model was started."],
  ["gpu_pipe_unavailable", "Luci could not read the GPU capacity check’s output. No model was started."],
  ["gpu_sample_invalid", "The GPU capacity check returned invalid or changing device information. No model was started."],
  ["gpu_sample_missing", "Luci did not confirm GPU capacity within five seconds. No model was started."],
  ["gpu_sample_stale", "The GPU capacity check did not produce a current observation. No model was started."],
  ["gpu_capacity_low", "The GPU needs at least 6 GiB of free memory to start this model. Close other GPU work before trying again."],
] as const)("explains %s without selecting or starting a model", async (code, message) => {
  const value = snapshot("blocked");
  value.status.failure = { code, ...(code === "gpu_process_exited" ? { exitCode: 17 } : {}) };
  const fixture = mount(value);
  await screen.findByText(code === "gpu_process_exited" ? `${message} Exit code: 17.` : message);
  expect(screen.getByRole("button", { name: "Use for OpenCode" }).matches(":disabled")).toBe(true);
  expect(fixture.posts()).toHaveLength(0);
  expect(screen.queryByText(/private vendor output/)).toBeNull();
});
