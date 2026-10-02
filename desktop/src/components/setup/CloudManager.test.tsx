import { expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createProductClient, type CloudPlan } from "../../api/product";
import { CloudManager } from "./CloudManager";
const plan: CloudPlan = {
  id: "cp-1",
  provider: "aws",
  name: "luci",
  region: "ap-southeast-2",
  domain: "app.example.com",
  resources: [],
  estimate: {
    currency: "USD",
    monthly: null,
    verified: false,
    explanation: "No live quote",
  },
  steps: [],
  files: [],
  commands: [],
  warnings: [],
  createdAt: "2026-10-02T00:00:00Z",
};
const json = (v: unknown, status = 200) =>
  new Response(JSON.stringify(v), { status });
it("requires two independent acknowledgements before creating an unpriced cloud plan", async () => {
  const fetcher = vi
    .fn<typeof fetch>()
    .mockImplementation(async (url) =>
      String(url).endsWith("/create")
        ? json({
            id: "job-1",
            planId: "cp-1",
            name: "luci",
            provider: "aws",
            region: "ap-southeast-2",
            domain: "app.example.com",
            status: "creating",
            message: "Resources requested",
          })
        : json({ jobs: [], targets: [], connections: [] }),
    );
  render(
    <CloudManager
      client={createProductClient("http://localhost", fetcher)}
      plan={plan}
    />,
  );
  const create = screen.getByRole("button", {
    name: "Create cloud resources",
  }) as HTMLButtonElement;
  expect(create.disabled).toBe(true);
  await userEvent.click(screen.getByLabelText(/I accept charges/));
  expect(create.disabled).toBe(true);
  await userEvent.click(screen.getByLabelText(/monthly price is unverified/));
  expect(create.disabled).toBe(false);
  await userEvent.click(create);
  await screen.findByText("Resources requested");
  const calls = fetcher.mock.calls.filter((c) =>
    String(c[0]).endsWith("/create"),
  );
  expect(calls).toHaveLength(1);
  expect(JSON.parse(calls[0][1]?.body as string)).toEqual({
    planId: "cp-1",
    acknowledgeCosts: true,
    acknowledgeUnverifiedPrice: true,
  });
});
it("does not trust an inspected SSH host until a selected fingerprint is explicitly confirmed", async () => {
  const fetcher = vi
    .fn<typeof fetch>()
    .mockImplementation(async (url) =>
      String(url).endsWith("/inspect")
        ? json({
            id: "scan-1",
            host: "server.example.com",
            fingerprints: ["SHA256:trusted"],
            createdAt: "now",
          })
        : String(url).endsWith("/connect")
          ? json({
              id: "t-1",
              host: "server.example.com",
              localUrl: "http://127.0.0.1:55100",
              status: "connected",
              message: "Private connection ready",
            })
          : json({ jobs: [], targets: [], connections: [] }),
    );
  render(
    <CloudManager
      client={createProductClient("http://localhost", fetcher)}
      plan={null}
    />,
  );
  await userEvent.type(
    screen.getByLabelText("Server address"),
    "server.example.com",
  );
  await userEvent.click(
    screen.getByRole("button", { name: "Inspect server fingerprint" }),
  );
  const trust = (await screen.findByRole("button", {
    name: "Trust and connect",
  })) as HTMLButtonElement;
  expect(trust.disabled).toBe(true);
  await userEvent.selectOptions(
    screen.getByLabelText("Server fingerprint"),
    "SHA256:trusted",
  );
  expect(trust.disabled).toBe(true);
  await userEvent.click(screen.getByLabelText(/I compared this fingerprint/));
  await userEvent.click(trust);
  await screen.findByText("Private connection ready");
  const call = fetcher.mock.calls.find((c) =>
    String(c[0]).endsWith("/connect"),
  );
  expect(JSON.parse(call![1]?.body as string)).toEqual({
    scanId: "scan-1",
    acceptedFingerprint: "SHA256:trusted",
  });
});
it("retains the secret in the form after verification failure and accepts an explicit server selection", async () => {
  let attempts = 0;
  const fetcher = vi
    .fn<typeof fetch>()
    .mockImplementation(async (url, init) =>
      String(url).endsWith("/targets") && init?.method === "POST"
        ? ++attempts === 1
          ? json(
              {
                error: "choose_server",
                message: "Choose a server",
                servers: [{ uuid: "s-1", name: "App server" }],
              },
              409,
            )
          : json({
              id: "target-1",
              name: "My cloud",
              baseUrl: "http://127.0.0.1:55100",
              serverUuid: "s-1",
            })
        : json({ jobs: [], targets: [], connections: [] }),
    );
  render(
    <CloudManager
      client={createProductClient("http://localhost", fetcher)}
      plan={null}
    />,
  );
  await userEvent.type(
    screen.getByLabelText("Management address"),
    "http://127.0.0.1:55100",
  );
  await userEvent.type(screen.getByLabelText("Server API token"), "test-token");
  await userEvent.click(
    screen.getByRole("button", { name: "Verify and save connection" }),
  );
  expect((await screen.findByRole("alert")).textContent).toBe("Choose a server");
  expect(
    (screen.getByLabelText("Server API token") as HTMLInputElement).value,
  ).toBe("test-token");
  await userEvent.selectOptions(
    screen.getByLabelText("Application server"),
    "s-1",
  );
  await userEvent.click(
    screen.getByRole("button", { name: "Verify and save connection" }),
  );
  await waitFor(() =>
    expect(
      (screen.getByLabelText("Server API token") as HTMLInputElement).value,
    ).toBe(""),
  );
  expect(attempts).toBe(2);
});
