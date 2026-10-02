import { expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createProductClient } from "../../api/product";
import { AgentConnections } from "./AgentConnections";

it("saves an explicitly selected local model without adding a paid fallback or exposing credentials", async () => {
  const fetcher = vi
    .fn<typeof fetch>()
    .mockImplementation(
      async (_url, init) =>
        new Response(
          JSON.stringify({
            providers: [
              {
                id: "local",
                name: "OpenCode",
                configured: init?.method === "POST",
                model: init?.method === "POST" ? "local-coder" : "",
              },
            ],
          }),
        ),
    );
  render(
    <AgentConnections
      client={createProductClient("http://localhost", fetcher)}
      cloud={false}
    />,
  );
  await userEvent.selectOptions(screen.getByLabelText("Coding agent"), "local");
  expect(
    (
      screen.getByRole("button", {
        name: "Save agent connection",
      }) as HTMLButtonElement
    ).disabled,
  ).toBe(true);
  await userEvent.type(
    screen.getByLabelText("Installed model ID"),
    "local-coder",
  );
  await userEvent.click(
    screen.getByRole("button", { name: "Save agent connection" }),
  );
  await screen.findByText("Endpoint saved for local-coder.");
  const post = fetcher.mock.calls.find((call) => call[1]?.method === "POST")!;
  expect(post[0]).toBe("http://localhost/v1/setup/agents/local");
  expect(JSON.parse(post[1]?.body as string)).toEqual({
    baseUrl: "http://localhost:11434/v1",
    model: "local-coder",
  });
  expect(screen.getByText(/provider has not been validated/)).not.toBeNull();
});

it("opens the official Codex sign-in as an auth-only session and returns to its terminal", async () => {
  const { api } = await import("../../api");
  const { useCockpit, resetCockpit } = await import("../../store/store");
  const { aHarness, aSession } = await import("../../test/fixtures");
  resetCockpit();
  useCockpit.setState({
    harnesses: [aHarness({ id: "codex" })],
    selection: {
      projectId: "p1",
      branchId: "b1",
      worktreeId: "w1",
      sessionId: null,
    },
  });
  const create = vi
    .spyOn(api, "createSession")
    .mockResolvedValue(
      aSession({ id: "auth-session", worktreeId: "w1", harness: "codex" }),
    );
  const close = vi.fn();
  const fetcher = vi
    .fn<typeof fetch>()
    .mockResolvedValue(
      new Response(
        JSON.stringify({
          providers: [{ id: "openai", name: "Codex", configured: false }],
        }),
      ),
    );
  render(
    <AgentConnections
      client={createProductClient("http://localhost", fetcher)}
      cloud={false}
      onSessionStarted={close}
    />,
  );
  await userEvent.selectOptions(
    screen.getByLabelText("Coding agent"),
    "openai",
  );
  await userEvent.click(
    screen.getByRole("button", { name: "Open Codex account sign-in" }),
  );
  expect(create).toHaveBeenCalledWith({
    worktreeId: "w1",
    harness: "codex",
    authOnly: true,
  });
  expect(close).toHaveBeenCalledOnce();
  expect(useCockpit.getState().sessions["auth-session"]).toBeDefined();
  create.mockRestore();
  resetCockpit();
});
