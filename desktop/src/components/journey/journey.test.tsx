import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { productApi } from "../../api/product";
import { BriefEditor } from "../lens/DiscoverLens";
import { ProjectFiles } from "../lens/FilesLens";
import { ProjectGit } from "../lens/MergeLens";
import { ApiError } from "../../api/client";

beforeEach(() => vi.restoreAllMocks());
it("keeps the edited brief on a revision conflict instead of claiming it was saved", async () => {
  vi.spyOn(productApi, "brief").mockResolvedValue({
    projectId: "p-1",
    revision: 2,
    purpose: "Original",
    audience: "People",
    constraints: "",
    criteria: ["Works offline"],
    updatedAt: "",
  });
  const save = vi
    .spyOn(productApi, "saveBrief")
    .mockRejectedValue(
      new ApiError(
        "conflict",
        "This brief changed. Reload before saving.",
        409,
      ),
    );
  render(<BriefEditor projectId="p-1" />);
  const idea = await screen.findByRole("textbox", { name: "The idea" });
  await userEvent.clear(idea);
  await userEvent.type(idea, "A better idea");
  await userEvent.click(screen.getByRole("button", { name: "Save brief" }));
  await screen.findByRole("alert");
  expect((idea as HTMLTextAreaElement).value).toBe("A better idea");
  expect(screen.queryByText(/Agent context updated/)).toBeNull();
  expect(save).toHaveBeenCalledWith("p-1", {
    revision: 2,
    purpose: "A better idea",
    audience: "People",
    constraints: "",
    criteria: ["Works offline"],
  });
});

it("reads actual file content and keeps the current folder when the server refuses a path", async () => {
  const files = vi
    .spyOn(productApi, "files")
    .mockResolvedValueOnce({
      path: "",
      entries: [
        { name: "README.md", path: "README.md", kind: "file", size: 11 },
      ],
    })
    .mockResolvedValueOnce({
      path: "README.md",
      content: "Hello Luci",
      truncated: false,
    })
    .mockRejectedValueOnce(
      new ApiError("bad_request", "This path is unavailable", 400),
    );
  render(<ProjectFiles projectId="p-1" />);
  await userEvent.click(
    await screen.findByRole("button", { name: /README.md/ }),
  );
  expect(await screen.findByText("Hello Luci")).not.toBeNull();
  expect(files).toHaveBeenNthCalledWith(2, "p-1", "README.md");
  await userEvent.click(screen.getByRole("button", { name: "Refresh" }));
  await screen.findByRole("alert");
  expect(screen.getByText("Hello Luci")).not.toBeNull();
});

it("refreshes Git only after a successful commit and never claims a failed commit succeeded", async () => {
  vi.spyOn(productApi, "git").mockResolvedValue({
    branch: "main",
    clean: false,
    changes: [{ path: "app.ts", status: "M" }],
    log: [],
    diff: "+ real change",
  });
  const commit = vi
    .spyOn(productApi, "commit")
    .mockRejectedValue(new Error("Configure your Git identity first."));
  render(<ProjectGit projectId="p-1" />);
  await userEvent.type(
    await screen.findByRole("textbox", { name: "Commit message" }),
    "Build the app",
  );
  await userEvent.click(screen.getByRole("button", { name: "Commit changes" }));
  await waitFor(() => expect(commit).toHaveBeenCalledOnce());
  expect((await screen.findByRole("alert")).textContent).toContain(
    "Git identity",
  );
  expect(screen.queryByText("Changes committed locally.")).toBeNull();
  expect(
    (
      screen.getByRole("textbox", {
        name: "Commit message",
      }) as HTMLTextAreaElement
    ).value,
  ).toBe("Build the app");
});
