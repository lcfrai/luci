import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DeploymentWorkspace, ProjectDeploy } from "../lens/DeployLens";
import { productApi, type ProjectConfiguration } from "../../api/product";
import { resetCockpit } from "../../store/store";

vi.mock("./DeploymentOverview", () => ({ DeploymentOverview: () => null }));
vi.mock("../platform/PlatformWorkspace", () => ({ PlatformWorkspace: () => <div>Application controls</div> }));
const emptyConfiguration: ProjectConfiguration = { revision: 0, variables: [], updatedAt: "" };
beforeEach(() => {
  resetCockpit();
  vi.spyOn(productApi, "configuration").mockResolvedValue(emptyConfiguration);
  vi.spyOn(productApi, "releases").mockResolvedValue({ releases: [] });
  vi.spyOn(productApi, "gitDelivery").mockResolvedValue({ branch: "main", head: "" });
  vi.spyOn(productApi, "git").mockResolvedValue({ branch: "main", clean: true, changes: [], diff: "", log: [] });
  vi.spyOn(productApi, "targets").mockResolvedValue({ targets: [] });
});
afterEach(() => vi.restoreAllMocks());

it("keeps one private configuration draft across both workspaces and every deployment destination", async () => {
  render(<DeploymentWorkspace projectId="p-1" />);
  await userEvent.click(screen.getByText("Runtime configuration", { selector: "summary" }));
  const name = await screen.findByRole("textbox", { name: "Variable name" });
  const secret = screen.getByLabelText(/^Private value/) as HTMLInputElement;
  const user = userEvent.setup();
  await user.type(name, "APP_SECRET");
  await user.type(secret, "only-in-this-draft");
  await user.click(screen.getByText("Runtime configuration", { selector: "summary" }));
  const tab = screen.getByRole("tab", { name: "Application platform" });
  tab.focus();
  await user.keyboard("{ArrowRight}");
  expect(productApi.releases).not.toHaveBeenCalled();
  expect(screen.getByText("Application controls")).not.toBeNull();
  await user.keyboard("{Enter}");
  const publishing = screen.getByRole("tab", { name: "Previews and publishing" });
  expect(screen.getByRole("tabpanel", { name: "Previews and publishing" }).id).toBe(publishing.getAttribute("aria-controls"));
  for (const label of ["GitHub Pages", "Your cloud", "On this machine"]) {
    await user.click(screen.getByRole("tab", { name: label }));
    expect(screen.getByLabelText(/^Private value/)).toBe(secret);
    expect(secret.value).toBe("only-in-this-draft");
    expect((name as HTMLInputElement).value).toBe("APP_SECRET");
  }
  await user.click(tab);
  await user.click(screen.getByText("Runtime configuration", { selector: "summary" }));
  expect(screen.getByLabelText(/^Private value/)).toBe(secret);
  expect(secret.value).toBe("only-in-this-draft");
  expect(productApi.configuration).toHaveBeenCalledExactlyOnceWith("p-1");
  expect(screen.getAllByRole("region", { name: "Runtime configuration" })).toHaveLength(1);
});

it("also retains private drafts on the direct publishing entry when Pages is selected", async () => {
  render(<ProjectDeploy projectId="p-1" />);
  await userEvent.type(await screen.findByRole("textbox", { name: "Variable name" }), "APP_SECRET");
  const secret = screen.getByLabelText(/^Private value/) as HTMLInputElement;
  await userEvent.type(secret, "direct-entry-draft");
  await userEvent.click(screen.getByRole("tab", { name: "GitHub Pages" }));
  expect(screen.getByLabelText(/^Private value/)).toBe(secret);
  await userEvent.click(screen.getByRole("tab", { name: "On this machine" }));
  expect(secret.value).toBe("direct-entry-draft");
  expect(productApi.configuration).toHaveBeenCalledOnce();
});

it("keeps deployment tabs locked until a configuration mutation is acknowledged", async () => {
  let finish!: (value: ProjectConfiguration) => void;
  const save = vi.spyOn(productApi, "saveConfiguration").mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  render(<DeploymentWorkspace projectId="p-1" />);
  await userEvent.click(screen.getByText("Runtime configuration", { selector: "summary" }));
  await userEvent.click(screen.getByRole("tab", { name: "Previews and publishing" }));
  await userEvent.type(await screen.findByRole("textbox", { name: "Variable name" }), "APP_SECRET");
  await userEvent.type(screen.getByLabelText(/^Private value/), "private-draft");
  await userEvent.click(screen.getByRole("button", { name: "Save runtime variable" }));
  const platform = screen.getByRole("tab", { name: "Application platform" });
  expect(platform.getAttribute("aria-disabled")).toBe("true");
  await userEvent.click(platform);
  expect(screen.getByRole("tab", { name: "Previews and publishing" }).getAttribute("aria-selected")).toBe("true");
  await userEvent.click(screen.getByRole("tab", { name: "GitHub Pages" }));
  expect(screen.getByRole("tab", { name: "On this machine" }).getAttribute("aria-selected")).toBe("true");
  expect(productApi.gitDelivery).not.toHaveBeenCalled();
  expect(save).toHaveBeenCalledExactlyOnceWith("p-1", 0, [{ name: "APP_SECRET", value: "private-draft" }]);
  await act(async () => finish({ ...emptyConfiguration, revision: 1, variables: [{ name: "APP_SECRET" }] }));
  await waitFor(() => expect(platform.getAttribute("aria-disabled")).toBe("false"));
  await userEvent.click(platform);
  expect(screen.getByText("Application controls")).not.toBeNull();
});
