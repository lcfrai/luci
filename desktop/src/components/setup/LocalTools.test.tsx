import { expect, it, vi } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createProductClient } from "../../api/product";
import { LocalTools } from "./LocalTools";

it("waits for the saved identity before allowing edits so initial loading cannot overwrite typing", async () => {
  let resolveIdentity!: (response: Response) => void;
  let resolveRuntime!: (response: Response) => void;
  const identity = new Promise<Response>((resolve) => {
    resolveIdentity = resolve;
  });
  const runtime = new Promise<Response>((resolve) => {
    resolveRuntime = resolve;
  });
  const fetcher = vi
    .fn<typeof fetch>()
    .mockImplementation(async (url, init) => {
      if (String(url).endsWith("/git/identity")) {
        return init?.method === "POST"
          ? new Response(init.body as string)
          : identity;
      }
      if (String(url).endsWith("/runtime/install")) return runtime;
      return new Response(JSON.stringify({ providers: [] }));
    });
  render(
    <LocalTools client={createProductClient("http://localhost", fetcher)} />,
  );
  await userEvent.click(
    screen.getByText("Git identity, agent connection and runtime"),
  );
  await screen.findByText("Loading Git identity and runtime…");
  const name = screen.getByRole("textbox", {
    name: "Commit author name",
  }) as HTMLInputElement;
  const email = screen.getByRole("textbox", {
    name: "Commit author email",
  }) as HTMLInputElement;
  const save = screen.getByRole("button", {
    name: "Save Git identity",
  }) as HTMLButtonElement;
  expect(name.disabled).toBe(true);
  expect(email.disabled).toBe(true);
  expect(save.disabled).toBe(true);
  await userEvent.type(name, "Would be lost");
  await userEvent.click(save);
  expect(name.value).toBe("");
  expect(fetcher.mock.calls.some((call) => call[1]?.method === "POST")).toBe(
    false,
  );
  await act(async () =>
    resolveIdentity(
      new Response(
        JSON.stringify({ name: "Saved name", email: "saved@example.invalid" }),
      ),
    ),
  );
  expect(name.disabled).toBe(true);
  await act(async () =>
    resolveRuntime(
      new Response(
        JSON.stringify({ id: "", status: "idle", message: "Not installed" }),
      ),
    ),
  );
  await waitFor(() => expect(name.disabled).toBe(false));
  expect(name.value).toBe("Saved name");
  expect(email.value).toBe("saved@example.invalid");
  expect(screen.queryByText("Loading Git identity and runtime…")).toBeNull();
  await userEvent.clear(name);
  await userEvent.type(name, "My chosen name");
  await userEvent.clear(email);
  await userEvent.type(email, "chosen@example.invalid");
  await userEvent.click(save);
  await screen.findByText(/Git identity saved for this machine/);
  const posts = fetcher.mock.calls.filter((call) => call[1]?.method === "POST");
  expect(posts).toHaveLength(1);
  expect(JSON.parse(posts[0][1]?.body as string)).toEqual({
    name: "My chosen name",
    email: "chosen@example.invalid",
  });
});

it("keeps identity controls disabled after a load failure and offers a working retry", async () => {
  let failIdentity = true;
  const fetcher = vi.fn<typeof fetch>().mockImplementation(async (url) => {
    if (String(url).endsWith("/git/identity")) {
      if (failIdentity) {
        failIdentity = false;
        return new Response(
          JSON.stringify({
            error: "unavailable",
            message: "Git identity unavailable",
          }),
          { status: 503 },
        );
      }
      return new Response(
        JSON.stringify({
          name: "Recovered",
          email: "recovered@example.invalid",
        }),
      );
    }
    return new Response(
      JSON.stringify(
        String(url).endsWith("/runtime/install")
          ? { id: "", status: "idle", message: "Not installed" }
          : { providers: [] },
      ),
    );
  });
  render(
    <LocalTools client={createProductClient("http://localhost", fetcher)} />,
  );
  await userEvent.click(
    screen.getByText("Git identity, agent connection and runtime"),
  );
  await screen.findByText(/Git identity unavailable/);
  const name = screen.getByRole("textbox", {
    name: "Commit author name",
  }) as HTMLInputElement;
  expect(name.disabled).toBe(true);
  await userEvent.click(
    screen.getByRole("button", {
      name: "Retry loading Git identity and runtime",
    }),
  );
  await screen.findByDisplayValue("Recovered");
  expect(name.disabled).toBe(false);
  expect(screen.queryByText(/Git identity unavailable/)).toBeNull();
});

it("saves a personal agent key only on explicit submission and clears it after success", async () => {
  const fetcher = vi.fn<typeof fetch>().mockImplementation(
    async (url, init) =>
      new Response(
        JSON.stringify(
          String(url).endsWith("/git/identity")
            ? { name: "Eval", email: "eval@example.invalid" }
            : String(url).endsWith("/runtime/install")
              ? { id: "", status: "idle", message: "Not installed" }
              : {
                  providers: [
                    {
                      id: "anthropic",
                      name: "Claude Code",
                      configured:
                        init?.method === "POST" &&
                        !!JSON.parse(init.body as string).apiKey,
                    },
                  ],
                },
        ),
      ),
  );
  render(
    <LocalTools client={createProductClient("http://localhost", fetcher)} />,
  );
  await userEvent.click(
    screen.getByText("Git identity, agent connection and runtime"),
  );
  await screen.findByDisplayValue("Eval");
  const input = screen.getByLabelText(/Anthropic API key/) as HTMLInputElement;
  await userEvent.type(input, "sk-ant-test-eval-only");
  expect(
    fetcher.mock.calls.filter((c) => c[1]?.method === "POST"),
  ).toHaveLength(0);
  await userEvent.click(
    screen.getByRole("button", { name: "Save agent connection" }),
  );
  await screen.findByText("Personal API key saved on this machine.");
  expect(input.value).toBe("");
  const posts = fetcher.mock.calls.filter((c) => c[1]?.method === "POST");
  expect(posts).toHaveLength(1);
  expect(JSON.parse(posts[0][1]?.body as string)).toEqual({
    apiKey: "sk-ant-test-eval-only",
  });
  await userEvent.click(
    screen.getByRole("button", { name: "Remove saved key" }),
  );
  await waitFor(() =>
    expect(
      screen.queryByText("Personal API key saved on this machine."),
    ).toBeNull(),
  );
  expect(
    JSON.parse(
      fetcher.mock.calls[fetcher.mock.calls.length - 1][1]?.body as string,
    ),
  ).toEqual({ apiKey: "" });
});


it('keeps manual runtime preparation blocked until the actual readiness state permits it', async () => {
  const fetcher = vi.fn<typeof fetch>().mockImplementation(async (url, init) => new Response(JSON.stringify(
    String(url).endsWith('/git/identity') ? { name: 'Builder', email: 'builder@example.invalid' }
      : String(url).endsWith('/runtime/install') ? init?.method === 'POST' ? { status: 'installing', message: 'Real job accepted.' } : { status: 'idle' }
        : { providers: [] }
  )));
  const client = createProductClient('http://localhost', fetcher);
  const view = render(<LocalTools client={client} runtimeBlockedReason="Confirm Docker storage in Settings." />);
  await userEvent.click(screen.getByText('Git identity, agent connection and runtime'));
  await screen.findByDisplayValue('Builder');
  const install = await screen.findByRole('button', { name: 'Install agent runtime' });
  expect((install as HTMLButtonElement).disabled).toBe(true);
  await userEvent.click(install);
  expect(fetcher.mock.calls.filter(([url, init]) => String(url).endsWith('/runtime/install') && init?.method === 'POST')).toHaveLength(0);
  view.rerender(<LocalTools client={client} runtimeBlockedReason="" />);
  await userEvent.click(install);
  await screen.findByText('installing · Real job accepted.');
  expect(fetcher.mock.calls.filter(([url, init]) => String(url).endsWith('/runtime/install') && init?.method === 'POST')).toHaveLength(1);
});
