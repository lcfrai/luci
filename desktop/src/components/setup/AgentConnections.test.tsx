import { expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createProductClient } from "../../api/product";
import { AgentConnections } from "./AgentConnections";

it("saves an explicitly selected local model without adding a paid fallback or exposing credentials", async () => {
  const fetcher = vi.fn<typeof fetch>().mockImplementation(
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

it("opens official Codex sign-in without a project and returns to its isolated terminal", async () => {
  const { useCockpit, resetCockpit } = await import("../../store/store");
  const { aHarness, aSession } = await import("../../test/fixtures");
  resetCockpit();
  useCockpit.setState({
    harnesses: [aHarness({ id: "codex" })],
    selection: {
      projectId: null,
      branchId: null,
      worktreeId: null,
      sessionId: null,
    },
  });
  const close = vi.fn();
  const fetcher = vi.fn<typeof fetch>().mockImplementation(
    async (url) =>
      new Response(
        JSON.stringify(
          String(url).endsWith("/codex/sign-in")
            ? aSession({
                id: "auth-session",
                worktreeId: "setup-codex",
                projectId: "",
                branchId: "",
                harness: "codex",
                authOnly: true,
                authMode: "chatgpt",
              })
            : {
                providers: [{ id: "openai", name: "Codex", configured: false }],
              },
        ),
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
  const post = fetcher.mock.calls.find((call) => call[1]?.method === "POST")!;
  expect(post[0]).toBe("http://localhost/v1/setup/agents/codex/sign-in");
  expect(JSON.parse(post[1]?.body as string)).toEqual({});
  expect(close).toHaveBeenCalledOnce();
  expect(useCockpit.getState().sessions["auth-session"]).toBeDefined();
  expect(useCockpit.getState().selection).toMatchObject({
    projectId: null,
    sessionId: "auth-session",
  });
  resetCockpit();
});

it("connects a stopped sign-in for future Codex sessions and keeps API billing unselected", async () => {
  const { useCockpit, resetCockpit } = await import("../../store/store");
  const { aSession } = await import("../../test/fixtures");
  resetCockpit();
  useCockpit.setState({
    sessions: {
      signed: aSession({
        id: "signed",
        harness: "codex",
        authOnly: true,
        status: "exited",
        createdAt: "2026-10-15T22:44:41Z",
      }),
      working: aSession({
        id: "working",
        harness: "codex",
        authOnly: false,
        status: "exited",
      }),
      pending: aSession({
        id: "pending",
        harness: "codex",
        authOnly: true,
        status: "running",
      }),
    },
  });
  const fetcher = vi.fn<typeof fetch>().mockImplementation(
    async (url, init) =>
      new Response(
        JSON.stringify(
          String(url).endsWith("/codex/account")
            ? {
                configured: init?.method === "POST",
                authMode: "chatgpt",
                message: "Account credential saved.",
              }
            : {
                providers: [{ id: "openai", name: "Codex", configured: true }],
              },
        ),
      ),
  );
  render(
    <AgentConnections
      client={createProductClient("http://localhost", fetcher)}
      cloud={false}
    />,
  );
  await userEvent.selectOptions(
    screen.getByLabelText("Coding agent"),
    "openai",
  );
  await screen.findByText("No Codex subscription sign-in saved.");
  const selector = screen.getByLabelText(
    "Ended sign-in session",
  ) as HTMLSelectElement;
  expect(Array.from(selector.options, (option) => option.value)).toEqual([
    "signed",
  ]);
  expect(selector.options[0].textContent).toContain("Started 2026-10-15T22:44:41Z");
  expect(screen.queryByLabelText("Completed sign-in session")).toBeNull();
  expect(screen.getByText(/An ended session does not confirm that authorization completed/)).not.toBeNull();
  expect(screen.getByText(/Save validates and retains the sign-in for this workspace/)).not.toBeNull();
  expect(fetcher.mock.calls.filter((call) => call[1]?.method === "POST")).toHaveLength(0);
  await userEvent.click(
    screen.getByRole("button", { name: "Save Codex sign-in" }),
  );
  await screen.findByText("Codex sign-in saved on this machine.");
  const post = fetcher.mock.calls.find((call) => call[1]?.method === "POST")!;
  expect(post[0]).toBe("http://localhost/v1/setup/agents/codex/account");
  expect(JSON.parse(post[1]?.body as string)).toEqual({ sessionId: "signed" });
  expect(
    (screen.getByLabelText("OpenAI API key") as HTMLInputElement).disabled,
  ).toBe(true);
  expect(
    (
      screen.getByRole("button", {
        name: "Save agent connection",
      }) as HTMLButtonElement
    ).disabled,
  ).toBe(true);
  expect(
    screen.getByText(/completed model request is still needed/),
  ).not.toBeNull();
  resetCockpit();
});

it("keeps a rejected sign-in unconnected and displays the actionable service error", async () => {
  const { useCockpit, resetCockpit } = await import("../../store/store");
  const { aSession } = await import("../../test/fixtures");
  resetCockpit();
  useCockpit.setState({
    sessions: {
      failed: aSession({
        id: "failed",
        harness: "codex",
        authOnly: true,
        status: "exited",
      }),
    },
  });
  const fetcher = vi
    .fn<typeof fetch>()
    .mockImplementation(async (url, init) => {
      if (init?.method === "POST")
        return new Response(
          JSON.stringify({
            error: "invalid_codex_account",
            message: "Complete Codex sign-in before saving this connection.",
          }),
          { status: 422 },
        );
      return new Response(
        JSON.stringify(
          String(url).endsWith("/codex/account")
            ? {
                configured: false,
                authMode: "chatgpt",
                message: "No account credential saved.",
              }
            : { providers: [] },
        ),
      );
    });
  render(
    <AgentConnections
      client={createProductClient("http://localhost", fetcher)}
      cloud={false}
    />,
  );
  await userEvent.selectOptions(
    screen.getByLabelText("Coding agent"),
    "openai",
  );
  await userEvent.click(
    screen.getByRole("button", { name: "Save Codex sign-in" }),
  );
  expect((await screen.findByRole("alert")).textContent).toContain(
    "Complete Codex sign-in",
  );
  expect(screen.queryByText("Codex sign-in saved on this machine.")).toBeNull();
  resetCockpit();
});

it("disconnects the subscription without deleting a separate API key or claiming to stop sessions", async () => {
  let connected = true;
  const fetcher = vi
    .fn<typeof fetch>()
    .mockImplementation(async (url, init) => {
      if (init?.method === "DELETE") connected = false;
      return new Response(
        JSON.stringify(
          String(url).endsWith("/codex/account")
            ? {
                configured: connected,
                authMode: "chatgpt",
                message: "Connection status.",
              }
            : {
                providers: [{ id: "openai", name: "Codex", configured: true }],
              },
        ),
      );
    });
  render(
    <AgentConnections
      client={createProductClient("http://localhost", fetcher)}
      cloud={true}
    />,
  );
  await userEvent.selectOptions(
    screen.getByLabelText("Coding agent"),
    "openai",
  );
  await screen.findByText("Codex sign-in saved in your cloud workspace.");
  await userEvent.click(
    screen.getByRole("button", { name: "Disconnect Codex sign-in" }),
  );
  await screen.findByText("No Codex subscription sign-in saved.");
  expect(
    screen.getByText(/Existing sessions keep their private sign-in/),
  ).not.toBeNull();
  expect(
    screen.getByText("Personal API key saved in the cloud workspace."),
  ).not.toBeNull();
  expect(
    fetcher.mock.calls
      .filter((call) => call[1]?.method === "DELETE")
      .map((call) => call[0]),
  ).toEqual(["http://localhost/v1/setup/agents/codex/account"]);
});

it("ends Checking after a status transport failure and refreshes only on request", async () => {
  let available = false;
  const fetcher = vi.fn<typeof fetch>().mockImplementation(async (url) => {
    if (String(url).endsWith("/codex/account")) {
      if (!available) throw new TypeError("fetch unavailable");
      return new Response(JSON.stringify({ configured: false, authMode: "chatgpt", message: "No saved connection." }));
    }
    return new Response(JSON.stringify({ providers: [] }));
  });
  render(<AgentConnections client={createProductClient("http://localhost", fetcher)} cloud={false} initialProvider="openai" />);
  await screen.findByText("Codex connection status is unavailable.");
  expect(screen.queryByText("Checking Codex sign-in…")).toBeNull();
  expect((await screen.findByRole("alert")).textContent).toContain("does not show whether device sign-in succeeded");
  expect(fetcher.mock.calls.filter(call => call[1]?.method === "POST")).toHaveLength(0);
  available = true;
  await userEvent.click(screen.getByRole("button", { name: "Refresh Codex connection" }));
  await screen.findByText("No Codex subscription sign-in saved.");
});

it("does not present a previous saved status as confirmed after an uncertain disconnect", async () => {
  const fetcher = vi.fn<typeof fetch>().mockImplementation(async (url, init) => {
    if (init?.method === "DELETE") throw new TypeError("connection lost");
    return new Response(JSON.stringify(String(url).endsWith("/codex/account")
      ? { configured: true, authMode: "chatgpt", message: "Saved." }
      : { providers: [] }));
  });
  render(<AgentConnections client={createProductClient("http://localhost", fetcher)} cloud={false} initialProvider="openai" />);
  await screen.findByText("Codex sign-in saved on this machine.");
  await userEvent.click(screen.getByRole("button", { name: "Disconnect Codex sign-in" }));
  await screen.findByText("Codex connection status is unavailable.");
  expect(screen.queryByText("Codex sign-in saved on this machine.")).toBeNull();
  // Preserve the last known billing selection until an explicit refresh settles it.
  expect((screen.getByLabelText("OpenAI API key") as HTMLInputElement).disabled).toBe(true);
  expect(fetcher.mock.calls.filter(call => call[1]?.method === "POST")).toHaveLength(0);
});
