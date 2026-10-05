import { afterEach, beforeEach, expect, it, vi } from "vitest";
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ProjectConfiguration } from "./ProjectConfiguration";
import { ProjectDeploy } from "../lens/DeployLens";
import {
  createProductClient,
  productApi,
  ProductApiError,
  type ProjectConfiguration as Configuration,
} from "../../api/product";
import { setAuthToken } from "../../api/authSession";
import { resetCockpit, useCockpit } from "../../store/store";

const stored: Configuration = {
  revision: 2,
  variables: [{ name: "DATABASE_URL" }],
  updatedAt: "2026-10-02T00:00:00Z",
};
beforeEach(() => {
  resetCockpit();
  vi.spyOn(productApi, "configuration").mockResolvedValue(stored);
});
afterEach(() => {
  setAuthToken(null);
  vi.restoreAllMocks();
});

it("configures the selected deployment project without reading values back and blocks deployment while saving", async () => {
  vi.spyOn(productApi, "releases").mockResolvedValue({ releases: [] });
  // Even unexpected extra response properties cannot become visible secret text.
  vi.mocked(productApi.configuration).mockResolvedValue({
    ...stored,
    variables: [{ name: "DATABASE_URL", value: "never-read-this-back" }],
  } as unknown as Configuration);
  let finish!: (metadata: Configuration) => void;
  const pending = new Promise<Configuration>((resolve) => {
    finish = resolve;
  });
  const save = vi
    .spyOn(productApi, "saveConfiguration")
    .mockReturnValue(pending);
  render(<ProjectDeploy projectId="my-project" />);
  await userEvent.click(
    await screen.findByRole("button", { name: "Update DATABASE_URL" }),
  );
  const input = screen.getByLabelText(/^Private value/) as HTMLInputElement;
  expect(document.activeElement).toBe(input);
  expect(input.type).toBe("password");
  expect(input.value).toBe("");
  expect(document.body.textContent).not.toContain("never-read-this-back");
  await userEvent.type(input, "postgres://owner:private-test-secret@db/app");
  expect(save).not.toHaveBeenCalled();
  await userEvent.click(
    screen.getByRole("button", { name: "Save runtime variable" }),
  );
  expect(save).toHaveBeenCalledWith("my-project", 2, [
    {
      name: "DATABASE_URL",
      value: "postgres://owner:private-test-secret@db/app",
    },
  ]);
  expect(input.value).toBe("");
  expect(
    (
      screen.getByRole("button", {
        name: "Build local preview",
      }) as HTMLButtonElement
    ).disabled,
  ).toBe(true);
  await act(async () => {
    finish({ ...stored, revision: 3 });
    await pending;
  });
  await screen.findByText(/Runtime configuration saved/);
  expect(
    (
      screen.getByRole("button", {
        name: "Build local preview",
      }) as HTMLButtonElement
    ).disabled,
  ).toBe(false);
  expect(document.body.textContent).not.toContain("private-test-secret");
  expect(screen.getByText(/excluded from project backups/)).not.toBeNull();
  expect(screen.getByText(/does not provision a database/)).not.toBeNull();
});

it("requires an explicit empty value and removes only the selected name at the current revision", async () => {
  const save = vi
    .spyOn(productApi, "saveConfiguration")
    .mockResolvedValueOnce({ ...stored, revision: 3 })
    .mockResolvedValueOnce({ revision: 4, variables: [], updatedAt: "" });
  render(<ProjectConfiguration projectId="p-1" />);
  await userEvent.click(
    await screen.findByRole("button", { name: "Update DATABASE_URL" }),
  );
  expect(
    (
      screen.getByRole("button", {
        name: "Save runtime variable",
      }) as HTMLButtonElement
    ).disabled,
  ).toBe(true);
  await userEvent.click(screen.getByLabelText("Set an empty value"));
  await userEvent.click(
    screen.getByRole("button", { name: "Save runtime variable" }),
  );
  await screen.findByText(/Runtime configuration saved/);
  expect(save).toHaveBeenNthCalledWith(1, "p-1", 2, [
    { name: "DATABASE_URL", value: "" },
  ]);
  await userEvent.click(
    screen.getByRole("button", { name: "Remove DATABASE_URL" }),
  );
  await screen.findByText(/Variable removed/);
  expect(save).toHaveBeenNthCalledWith(2, "p-1", 3, [
    { name: "DATABASE_URL", remove: true },
  ]);
  expect(
    screen.queryByRole("button", { name: "Remove DATABASE_URL" }),
  ).toBeNull();
});

it("does not overwrite a concurrent edit and requires fresh metadata and a re-entered value before retrying", async () => {
  vi.mocked(productApi.configuration)
    .mockResolvedValueOnce(stored)
    .mockResolvedValue({ ...stored, revision: 7 });
  const save = vi
    .spyOn(productApi, "saveConfiguration")
    .mockRejectedValueOnce(
      new ProductApiError(
        "conflict",
        "Unsafe server detail: submitted-secret",
        409,
        {},
      ),
    )
    .mockResolvedValue({ ...stored, revision: 8 });
  render(<ProjectConfiguration projectId="p-1" />);
  await userEvent.click(
    await screen.findByRole("button", { name: "Update DATABASE_URL" }),
  );
  await userEvent.type(
    screen.getByLabelText(/^Private value/),
    "submitted-secret",
  );
  await userEvent.click(
    screen.getByRole("button", { name: "Save runtime variable" }),
  );
  await screen.findByText(/Configuration changed elsewhere/);
  expect(document.body.textContent).not.toContain("submitted-secret");
  expect(
    (
      screen.getByRole("button", {
        name: "Remove DATABASE_URL",
      }) as HTMLButtonElement
    ).disabled,
  ).toBe(true);
  expect(
    (screen.getByLabelText(/^Private value/) as HTMLInputElement).value,
  ).toBe("");
  expect(save).toHaveBeenCalledTimes(1);
  await userEvent.click(
    screen.getByRole("button", { name: "Refresh stored names" }),
  );
  await waitFor(() =>
    expect(
      (screen.getByLabelText(/^Private value/) as HTMLInputElement).disabled,
    ).toBe(false),
  );
  expect(
    (
      screen.getByRole("button", {
        name: "Save runtime variable",
      }) as HTMLButtonElement
    ).disabled,
  ).toBe(true);
  await userEvent.type(
    screen.getByLabelText(/^Private value/),
    "replacement-secret",
  );
  await userEvent.click(
    screen.getByRole("button", { name: "Save runtime variable" }),
  );
  await screen.findByText(/Runtime configuration saved/);
  expect(save).toHaveBeenNthCalledWith(2, "p-1", 7, [
    { name: "DATABASE_URL", value: "replacement-secret" },
  ]);
});

it("does not render provider error bodies or leave a submitted private value in the field", async () => {
  vi.spyOn(productApi, "saveConfiguration").mockRejectedValue(
    new Error("Failure includes private-secret-from-provider"),
  );
  render(<ProjectConfiguration projectId="p-1" />);
  await userEvent.click(
    await screen.findByRole("button", { name: "Update DATABASE_URL" }),
  );
  await userEvent.type(
    screen.getByLabelText(/^Private value/),
    "private-secret-from-provider",
  );
  await userEvent.click(
    screen.getByRole("button", { name: "Save runtime variable" }),
  );
  await screen.findByText(/Could not save runtime configuration/);
  expect(document.body.textContent).not.toContain(
    "private-secret-from-provider",
  );
  expect(
    (screen.getByLabelText(/^Private value/) as HTMLInputElement).value,
  ).toBe("");
});

it("discards private fields and ignores a pending save response after switching projects", async () => {
  vi.mocked(productApi.configuration).mockImplementation(async (id) =>
    id === "first" ? stored : { revision: 0, variables: [], updatedAt: "" },
  );
  let finish!: (metadata: Configuration) => void;
  const pending = new Promise<Configuration>((resolve) => {
    finish = resolve;
  });
  vi.spyOn(productApi, "saveConfiguration").mockReturnValue(pending);
  const view = render(<ProjectConfiguration projectId="first" />);
  await userEvent.click(
    await screen.findByRole("button", { name: "Update DATABASE_URL" }),
  );
  await userEvent.type(
    screen.getByLabelText(/^Private value/),
    "first-project-secret",
  );
  await userEvent.click(
    screen.getByRole("button", { name: "Save runtime variable" }),
  );
  view.rerender(<ProjectConfiguration projectId="second" />);
  await screen.findByText("No runtime variables saved.");
  await act(async () => {
    finish({ ...stored, revision: 3 });
    await pending;
  });
  expect(
    screen.queryByRole("button", { name: "Update DATABASE_URL" }),
  ).toBeNull();
  expect(
    (screen.getByLabelText("Variable name") as HTMLInputElement).value,
  ).toBe("");
  expect(
    (screen.getByLabelText(/^Private value/) as HTMLInputElement).value,
  ).toBe("");
  expect(screen.queryByText(/Runtime configuration saved/)).toBeNull();
});

it("validates environment names and UTF-8 value size before submitting", async () => {
  const save = vi.spyOn(productApi, "saveConfiguration");
  render(<ProjectConfiguration projectId="p-1" />);
  await screen.findByRole("button", { name: "Update DATABASE_URL" });
  await userEvent.type(screen.getByLabelText("Variable name"), "INVALID-NAME");
  await userEvent.type(screen.getByLabelText(/^Private value/), "a-value");
  await userEvent.click(
    screen.getByRole("button", { name: "Save runtime variable" }),
  );
  await screen.findByText(/Use an environment variable name/);
  fireEvent.change(screen.getByLabelText("Variable name"), {
    target: { value: "VALID_NAME" },
  });
  fireEvent.change(screen.getByLabelText(/^Private value/), {
    target: { value: "é".repeat(8193) },
  });
  await userEvent.click(
    screen.getByRole("button", { name: "Save runtime variable" }),
  );
  await screen.findByText(/at most 16 KiB/);
  expect(save).not.toHaveBeenCalled();
});

it("authenticates the encoded project route and sends secrets only in the explicit update body", async () => {
  setAuthToken("configuration-auth-token");
  const fetcher = vi
    .fn<typeof fetch>()
    .mockImplementation(async () => new Response(JSON.stringify(stored)));
  const api = createProductClient("http://localhost:8730", fetcher);
  await api.configuration("project/with spaces");
  await api.saveConfiguration("project/with spaces", 2, [
    { name: "API_KEY", value: "request-only-secret" },
  ]);
  expect(fetcher.mock.calls[0]).toMatchObject([
    "http://localhost:8730/v1/product/projects/project%2Fwith%20spaces/configuration",
    {
      method: "GET",
      headers: { Authorization: "Bearer configuration-auth-token" },
    },
  ]);
  expect(fetcher.mock.calls[0][1]?.body).toBeUndefined();
  const [url, request] = fetcher.mock.calls[1];
  expect(url).not.toContain("request-only-secret");
  expect(request).toMatchObject({
    method: "POST",
    headers: {
      Authorization: "Bearer configuration-auth-token",
      "Content-Type": "application/json",
    },
  });
  expect(JSON.parse(request?.body as string)).toEqual({
    expectedRevision: 2,
    updates: [{ name: "API_KEY", value: "request-only-secret" }],
  });
});

it("protects an unsaved private variable across navigation until explicitly discarded", async () => {
  useCockpit.getState().selectProject("p-1");
  useCockpit.getState().setLens("deploy");
  render(<ProjectConfiguration projectId="p-1" />);
  await screen.findByRole("button", { name: "Update DATABASE_URL" });
  await userEvent.type(screen.getByLabelText("Variable name"), "PRIVATE_TOKEN");
  await userEvent.type(screen.getByLabelText(/^Private value/), "draft-only-secret");
  useCockpit.getState().setLens("develop");
  expect(useCockpit.getState().lens).toBe("deploy");
  expect(useCockpit.getState().pendingNavigation).toEqual({ kind: "lens", lens: "develop" });
  expect(JSON.stringify(useCockpit.getState().unsavedProjectDrafts)).not.toContain("draft-only-secret");
  await userEvent.click(screen.getByRole("button", { name: "Discard variable draft" }));
  expect((screen.getByLabelText(/^Private value/) as HTMLInputElement).value).toBe("");
  expect((screen.getByLabelText("Variable name") as HTMLInputElement).value).toBe("");
  useCockpit.getState().setLens("develop");
  expect(useCockpit.getState().lens).toBe("develop");
});

it("keeps an unsubmitted private draft when metadata refresh fails, then uses the newly read revision", async () => {
  vi.mocked(productApi.configuration).mockResolvedValueOnce(stored).mockRejectedValueOnce(new Error("Private provider detail")).mockResolvedValue({ ...stored, revision: 7 });
  const save = vi.spyOn(productApi, "saveConfiguration").mockResolvedValue({ ...stored, revision: 8 });
  render(<ProjectConfiguration projectId="p-1" />);
  await screen.findByRole("button", { name: "Update DATABASE_URL" });
  await userEvent.type(screen.getByLabelText("Variable name"), "NEW_VARIABLE");
  await userEvent.type(screen.getByLabelText(/^Private value/), "unsubmitted-private-value");
  await userEvent.click(screen.getByRole("button", { name: "Refresh stored names" }));
  await screen.findByRole("alert");
  expect((screen.getByLabelText(/^Private value/) as HTMLInputElement).value).toBe("unsubmitted-private-value");
  expect(document.body.textContent).not.toContain("Private provider detail");
  await userEvent.click(screen.getByRole("button", { name: "Refresh stored names" }));
  await waitFor(() => expect((screen.getByRole("button", { name: "Save runtime variable" }) as HTMLButtonElement).disabled).toBe(false));
  await userEvent.click(screen.getByRole("button", { name: "Save runtime variable" }));
  expect(save).toHaveBeenCalledExactlyOnceWith("p-1", 7, [{ name: "NEW_VARIABLE", value: "unsubmitted-private-value" }]);
  expect((screen.getByLabelText(/^Private value/) as HTMLInputElement).value).toBe("");
});
