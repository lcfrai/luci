import { expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AgentConnectionManager } from "./AgentConnectionManager";
import { createProductClient, type AgentConnectionCatalogue, type AgentConnectionMutation } from "../../api/product";

const initial = (): AgentConnectionCatalogue => ({ revision: 4, activeConnectionId: "gateway-one", connections: [{ id: "gateway-one", revision: 2, name: "My gateway", kind: "litellm", enabled: true, configured: true, keyPresent: true, endpoints: { responses: "https://gateway.example/v1", anthropicMessages: "https://gateway.example", chatCompletions: "https://gateway.example/v1" }, models: { codex: "my-coder", claudeCode: "my-claude", opencode: "my-local-model" } }] });
function mount(fail = false, saved = initial()) {
  let catalogue = saved;
  const fetcher = vi.fn<typeof fetch>().mockImplementation(async (_url, init) => {
    if (init?.method === "POST") {
      if (fail) return new Response(JSON.stringify({ code: "stale_revision", message: "Settings changed. Refresh saved connections and try again." }), { status: 409 });
      const input = JSON.parse(init.body as string) as AgentConnectionMutation;
      if (input.action === "select") catalogue = { ...catalogue, revision: catalogue.revision + 1, activeConnectionId: input.id };
      if (input.action === "save") {
        const previous = catalogue.connections.find(item => item.id === input.id);
        if (input.kind === "codex-subscription" && (input.apiKey === undefined ? previous?.keyPresent : input.apiKey !== "")) return new Response(JSON.stringify({ code: "invalid_connection", message: "Codex subscription connections cannot contain an API key." }), { status: 400 });
        const connection = { id: input.id || "gateway-two", revision: (previous?.revision ?? 0) + 1, name: input.name, kind: input.kind, enabled: input.enabled, endpoints: input.endpoints, models: input.models, configured: true, keyPresent: input.apiKey === undefined ? previous?.keyPresent ?? false : input.apiKey !== "" };
        catalogue = { ...catalogue, revision: catalogue.revision + 1, connections: [...catalogue.connections.filter(item => item.id !== connection.id), connection] };
      }
    }
    return new Response(JSON.stringify(catalogue));
  });
  render(<AgentConnectionManager client={createProductClient("http://localhost", fetcher)} />);
  return fetcher;
}
it("keeps saved keys private and preserves them on an ordinary connection edit", async () => {
  const fetcher = mount();
  await screen.findByText(/New sessions use My gateway\./);
  await userEvent.click(screen.getByText("Add or edit a connection"));
  await userEvent.selectOptions(screen.getByLabelText("Edit saved connection"), "gateway-one");
  expect((screen.getByLabelText("Connection API key") as HTMLInputElement).value).toBe("");
  await userEvent.clear(screen.getByLabelText("Connection name"));
  await userEvent.type(screen.getByLabelText("Connection name"), "My revised gateway");
  await userEvent.click(screen.getByRole("button", { name: "Save connection" }));
  await waitFor(() => expect(fetcher.mock.calls.some(call => call[1]?.method === "POST")).toBe(true));
  const post = fetcher.mock.calls.find(call => call[1]?.method === "POST")!;
  expect(post[0]).toBe("http://localhost/v1/setup/agent-connections");
  const input = JSON.parse(post[1]?.body as string);
  expect(input).toMatchObject({ action: "save", expectedRevision: 4, id: "gateway-one", name: "My revised gateway", endpoints: initial().connections[0].endpoints, models: initial().connections[0].models });
  expect(input).not.toHaveProperty("apiKey");
});
it("turns the gateway off explicitly without deleting the saved connection", async () => {
  const fetcher = mount();
  await screen.findByText(/New sessions use My gateway\./);
  await userEvent.click(screen.getByRole("switch", { name: "Use gateway" }));
  await screen.findByText("Gateway is off. New sessions use your own agent settings.");
  const posts = fetcher.mock.calls.filter(call => call[1]?.method === "POST");
  expect(posts).toHaveLength(1);
  expect(JSON.parse(posts[0][1]?.body as string)).toEqual({ action: "select", expectedRevision: 4, id: "" });
  expect(screen.getAllByRole("option", { name: "My gateway" }).length).toBeGreaterThan(0);
});
it("converts a keyed gateway to Codex subscription by explicitly clearing only that connection's key", async () => {
  const fetcher = mount();
  await screen.findByText(/New sessions use My gateway\./);
  await userEvent.click(screen.getByText("Add or edit a connection"));
  await userEvent.selectOptions(screen.getByLabelText("Edit saved connection"), "gateway-one");
  await userEvent.selectOptions(screen.getByLabelText("Connection type"), "codex-subscription");
  expect(screen.getByText("Saving this type removes this connection’s saved API key.")).not.toBeNull();
  expect(screen.queryByLabelText("Connection API key")).toBeNull();
  await userEvent.clear(screen.getByLabelText("Connection name"));
  await userEvent.type(screen.getByLabelText("Connection name"), "My Codex subscription");
  await userEvent.click(screen.getByRole("button", { name: "Save connection" }));
  await screen.findByText(/Connection saved\. Choose it as the active connection/);
  const posts = fetcher.mock.calls.filter(call => call[1]?.method === "POST");
  expect(posts).toHaveLength(1);
  expect(JSON.parse(posts[0][1]?.body as string)).toEqual({ action: "save", expectedRevision: 4, id: "gateway-one", name: "My Codex subscription", kind: "codex-subscription", enabled: true, endpoints: {}, models: { codex: "my-coder" }, apiKey: "" });
  expect(screen.getByText(/New sessions use My Codex subscription\./)).not.toBeNull();
  expect((screen.getByRole("switch", { name: "Use gateway" }) as HTMLInputElement).checked).toBe(false);
  expect(screen.queryByText("Saving this type removes this connection’s saved API key.")).toBeNull();
  expect(screen.queryByRole("alert")).toBeNull();
  await userEvent.selectOptions(screen.getByLabelText("Edit saved connection"), "");
  await userEvent.selectOptions(screen.getByLabelText("Edit saved connection"), "gateway-one");
  expect((screen.getByLabelText("Connection type") as HTMLSelectElement).value).toBe("codex-subscription");
  expect((screen.getByLabelText("Codex model · optional") as HTMLInputElement).value).toBe("my-coder");
});
it("clears a key only through an explicit choice", async () => {
  const fetcher = mount();
  await screen.findByText(/New sessions use My gateway\./);
  await userEvent.click(screen.getByText("Add or edit a connection"));
  await userEvent.selectOptions(screen.getByLabelText("Edit saved connection"), "gateway-one");
  await userEvent.click(screen.getByLabelText("Clear the saved key when saving"));
  await userEvent.click(screen.getByRole("button", { name: "Save connection" }));
  await waitFor(() => expect(fetcher.mock.calls.some(call => call[1]?.method === "POST")).toBe(true));
  expect(JSON.parse(fetcher.mock.calls.find(call => call[1]?.method === "POST")![1]?.body as string)).toHaveProperty("apiKey", "");
});
it("retains the connection draft after a refused save and clears the submitted secret", async () => {
  const fetcher = mount(true);
  await screen.findByText(/New sessions use My gateway\./);
  await userEvent.click(screen.getByText("Add or edit a connection"));
  await userEvent.selectOptions(screen.getByLabelText("Edit saved connection"), "gateway-one");
  await userEvent.type(screen.getByLabelText("Connection API key"), "private-replacement-key");
  await userEvent.click(screen.getByRole("button", { name: "Save connection" }));
  await screen.findByRole("alert");
  expect((screen.getByLabelText("Connection API key") as HTMLInputElement).value).toBe("");
  expect((screen.getByLabelText("Connection name") as HTMLInputElement).value).toBe("My gateway");
  expect(fetcher.mock.calls.filter(call => call[1]?.method === "POST")).toHaveLength(1);
});

it("applies one LiteLLM address to the explicitly selected agents and exact model aliases", async () => {
  const fetcher = mount();
  await screen.findByText(/New sessions use My gateway\./);
  await userEvent.click(screen.getByText("Add or edit a connection"));
  await userEvent.type(screen.getByLabelText("Connection name"), "My new gateway");
  await userEvent.type(screen.getByLabelText("Gateway server address"), "https://models.example/team/v1/");
  for (const [agent, field, alias] of [["Codex", "Codex model", "supplied-coder"], ["Claude Code", "Claude Code model", "supplied-claude"], ["OpenCode", "OpenCode model", "supplied-local"]]) {
    await userEvent.click(screen.getByLabelText(`Use ${agent}`));
    await userEvent.type(screen.getByLabelText(field), alias);
  }
  await userEvent.type(screen.getByLabelText("Connection API key"), "private-gateway-key");
  await userEvent.click(screen.getByRole("button", { name: "Apply gateway address" }));
  expect(fetcher.mock.calls.some(call => call[1]?.method === "POST")).toBe(false);
  expect(screen.getByText(/Protocol and model access have not been verified/)).not.toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Save connection" }));
  await waitFor(() => expect(fetcher.mock.calls.some(call => call[1]?.method === "POST")).toBe(true));
  const input = JSON.parse(fetcher.mock.calls.find(call => call[1]?.method === "POST")![1]?.body as string);
  expect(input).toMatchObject({ kind: "litellm", endpoints: { responses: "https://models.example/team/v1", anthropicMessages: "https://models.example/team", chatCompletions: "https://models.example/team/v1" }, models: { codex: "supplied-coder", claudeCode: "supplied-claude", opencode: "supplied-local" } });
});

it("preserves custom saved paths when editing without explicitly applying the address preset", async () => {
  const saved = initial();
  saved.connections[0].endpoints = { responses: "https://gateway.example/special/codex", anthropicMessages: "https://gateway.example/anthropic-route", chatCompletions: "https://gateway.example/custom-chat/v1" };
  const fetcher = mount(false, saved);
  await screen.findByText(/New sessions use My gateway\./);
  await userEvent.click(screen.getByText("Add or edit a connection"));
  await userEvent.selectOptions(screen.getByLabelText("Edit saved connection"), "gateway-one");
  expect((screen.getByLabelText("Gateway server address") as HTMLInputElement).value).toBe("");
  await userEvent.type(screen.getByLabelText("Gateway server address"), "https://new-address.example");
  await userEvent.type(screen.getByLabelText("Connection name"), " renamed");
  await userEvent.click(screen.getByRole("button", { name: "Save connection" }));
  await waitFor(() => expect(fetcher.mock.calls.some(call => call[1]?.method === "POST")).toBe(true));
  const input = JSON.parse(fetcher.mock.calls.find(call => call[1]?.method === "POST")![1]?.body as string);
  expect(input.endpoints).toEqual(saved.connections[0].endpoints);
  expect(input.models).toEqual(saved.connections[0].models);
  expect(input).not.toHaveProperty("apiKey");
});

it("refuses an invalid gateway address without replacing the saved endpoints or sending a request", async () => {
  const fetcher = mount();
  await screen.findByText(/New sessions use My gateway\./);
  await userEvent.click(screen.getByText("Add or edit a connection"));
  await userEvent.selectOptions(screen.getByLabelText("Edit saved connection"), "gateway-one");
  await userEvent.clear(screen.getByLabelText("Gateway server address"));
  await userEvent.type(screen.getByLabelText("Gateway server address"), "https://credential@gateway.example");
  await userEvent.click(screen.getByRole("button", { name: "Apply gateway address" }));
  expect((await screen.findByRole("alert")).textContent).toContain("without credentials");
  expect(fetcher.mock.calls.some(call => call[1]?.method === "POST")).toBe(false);
  await userEvent.click(screen.getByRole("button", { name: "Save connection" }));
  await waitFor(() => expect(fetcher.mock.calls.some(call => call[1]?.method === "POST")).toBe(true));
  const input = JSON.parse(fetcher.mock.calls.find(call => call[1]?.method === "POST")![1]?.body as string);
  expect(input.endpoints).toEqual(initial().connections[0].endpoints);
});
