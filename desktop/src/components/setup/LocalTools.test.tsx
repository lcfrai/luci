import { expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createProductClient } from "../../api/product";
import { LocalTools } from "./LocalTools";

it("saves a personal agent key only on explicit submission and clears it after success", async () => {
  const fetcher = vi
    .fn<typeof fetch>()
    .mockImplementation(
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
                        configured: init?.method === "POST",
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
