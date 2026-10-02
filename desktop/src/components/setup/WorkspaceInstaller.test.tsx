import { beforeEach, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { WorkspaceInstaller } from "./WorkspaceInstaller";
import { createProductClient } from "../../api/product";
import { changeWorkspace } from "./workspace";
import { resetCockpit, useCockpit } from "../../store/store";
import * as attachments from "../../api/attachRegistry";
beforeEach(() => {
  vi.restoreAllMocks();
  resetCockpit();
});
it("requires installation consent and preserves the active workspace when verification fails", async () => {
  const fetcher = vi
    .fn<typeof fetch>()
    .mockImplementation(async (url, init) =>
      String(url).endsWith("/activate")
        ? new Response(
            JSON.stringify({
              error: "not_ready",
              message: "Remote Luci is not ready",
            }),
            { status: 409 },
          )
        : new Response(
            JSON.stringify(
              init?.method === "POST"
                ? {
                    id: "install-1",
                    connectionId: "t-1",
                    status: "ready",
                    message: "Luci installed",
                  }
                : { jobs: [] },
            ),
          ),
    );
  const client = createProductClient("http://localhost", fetcher),
    reload = vi.fn();
  useCockpit.getState().selectProject("local-project");
  render(
    <WorkspaceInstaller
      client={client}
      connections={[
        {
          id: "t-1",
          host: "server.example.com",
          localUrl: "http://127.0.0.1:50100",
          status: "connected",
          message: "Connected",
        },
      ]}
      switchWorkspace={(fn) => changeWorkspace(fn, reload)}
    />,
  );
  const install = screen.getByRole("button", {
    name: "Install Luci workspace",
  }) as HTMLButtonElement;
  expect(install.disabled).toBe(true);
  await userEvent.click(screen.getByLabelText(/Install Luci and its runtime/));
  await userEvent.click(install);
  await screen.findByText("ready · Luci installed");
  const request = fetcher.mock.calls.find((c) =>
    String(c[0]).endsWith("/install-luci"),
  );
  expect(JSON.parse(request![1]?.body as string)).toEqual({
    acknowledgeInstall: true,
  });
  await userEvent.click(
    screen.getByRole("button", { name: "Use this cloud workspace" }),
  );
  await screen.findByText("Remote Luci is not ready");
  expect(reload).not.toHaveBeenCalled();
  expect(useCockpit.getState().selection.projectId).toBe("local-project");
});
it("detaches terminals and clears persisted machine-specific UI before restarting consumers", async () => {
  const detach = vi
      .spyOn(attachments, "detachAllTerminals")
      .mockImplementation(() => {}),
    reload = vi.fn();
  useCockpit.getState().selectProject("old-project");
  useCockpit.setState({
    pinned: ["old-session"],
    tiles: { "old-session": { x: 10, y: 10, w: 200, h: 100 } as never },
    projects: [{ id: "old-project" } as never],
    sessions: { "old-session": { id: "old-session" } as never },
  });
  const previousDefaults = useCockpit.getState().defaults;
  await changeWorkspace(
    async () => ({ workspace: { mode: "cloud", ready: true } }),
    reload,
  );
  expect(detach).toHaveBeenCalledOnce();
  expect(reload).toHaveBeenCalledOnce();
  expect(useCockpit.getState().selection.projectId).toBeNull();
  expect(useCockpit.getState().pinned).toEqual([]);
  expect(useCockpit.getState().sessions).toEqual({});
  expect(useCockpit.getState().defaults).toEqual(previousDefaults);
  const persisted = JSON.parse(localStorage.getItem("luci.cockpit")!);
  expect(persisted.state.selection.projectId).toBeNull();
  expect(persisted.state.pinned).toEqual([]);
});
