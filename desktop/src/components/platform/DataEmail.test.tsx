import { beforeEach, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { DataEmail } from "./DataEmail";
import { dataEmailApi, type DataEmailStatus } from "../../api/dataEmail";
vi.mock("../../api/dataEmail", () => ({ dataEmailApi: { status: vi.fn(), action: vi.fn() } }));
const empty: DataEmailStatus = {
  projectId: "owned", status: "unconfigured", message: "Email is not configured.", revision: 0, appliedRevision: 0,
  configuration: { host: "", port: 465, username: "", fromAddress: "", senderName: "Luci", applicationOrigin: "", hasPassword: false },
  applicationOrigin: "", originSource: "local", connection: { status: "not-checked", revision: 0 }, test: { status: "not-sent", revision: 0 },
};
const saved: DataEmailStatus = {
  ...empty, revision: 1, status: "saved", message: "Settings saved privately.",
  configuration: { host: "smtp.example.test", port: 465, username: "smtp-user", fromAddress: "app@example.test", senderName: "Example", applicationOrigin: "http://localhost:3000", hasPassword: true },
};
const applied: DataEmailStatus = { ...saved, status: "configured", appliedRevision: 1, message: "Applied to healthy Auth.", connection: { status: "passed", revision: 1 } };
beforeEach(() => { vi.resetAllMocks(); vi.mocked(dataEmailApi.status).mockResolvedValue(structuredClone(empty)); });
async function fill() {
  await screen.findByText("Email is not configured.");
  for (const [label, value] of [["SMTP server", "smtp.example.test"], ["SMTP username", "smtp-user"], ["SMTP password", "private-mail-password"], ["Sender email address", "app@example.test"], ["Sender name", "Example"], ["Application redirect address", "http://localhost:3000"]])
    fireEvent.change(screen.getByLabelText(label), { target: { value } });
}
it("mount only reads and saving is separate from checking, applying and sending", async () => {
  vi.mocked(dataEmailApi.action).mockResolvedValue(saved);
  render(<DataEmail projectId="owned" ready />);
  await fill();
  expect(dataEmailApi.action).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole("button", { name: "Save email settings" }));
  await waitFor(() => expect(dataEmailApi.action).toHaveBeenCalledExactlyOnceWith("owned", { action: "save", expectedRevision: 0, configuration: { host: "smtp.example.test", port: 465, username: "smtp-user", password: "private-mail-password", fromAddress: "app@example.test", senderName: "Example", applicationOrigin: "http://localhost:3000" } }));
  expect((screen.getByLabelText("SMTP password") as HTMLInputElement).value).toBe("");
  expect(await screen.findByText("Settings saved privately.")).not.toBeNull();
});
it("shows the pending save and prevents duplicate submits while clearing the entered password", async () => {
  let complete!: (value: DataEmailStatus) => void;
  vi.mocked(dataEmailApi.action).mockImplementation(() => new Promise(resolve => { complete = resolve; }));
  render(<DataEmail projectId="owned" ready />);
  await fill();
  const save = screen.getByRole("button", { name: "Save email settings" }) as HTMLButtonElement;
  const check = screen.getByRole("button", { name: "Check SMTP connection" }) as HTMLButtonElement;
  expect(save.type).toBe("submit"); expect(check.type).toBe("button");
  expect((screen.getByRole("button", { name: "Send test email" }) as HTMLButtonElement).type).toBe("submit");
  fireEvent.click(save);
  expect(save.getAttribute("aria-busy")).toBe("true"); expect(save.disabled).toBe(true);
  expect(check.hasAttribute("aria-busy")).toBe(false);
  expect((screen.getByLabelText("SMTP password") as HTMLInputElement).value).toBe("");
  fireEvent.submit(save.form!); fireEvent.click(check);
  expect(dataEmailApi.action).toHaveBeenCalledTimes(1);
  await act(async () => { complete(saved); });
  await screen.findByText("Settings saved privately.");
  expect(save.hasAttribute("aria-busy")).toBe(false); expect(save.disabled).toBe(false);
});
it("keeps the specific action busy while a saved background operation is still running", async () => {
  vi.mocked(dataEmailApi.status).mockResolvedValue({ ...saved, operation: { id: "apply-job", action: "apply", status: "running", stage: "verifying", message: "Checking Auth health.", createdAt: "now", updatedAt: "now" } });
  render(<DataEmail projectId="owned" ready />);
  await screen.findByText("Checking Auth health.");
  const apply = screen.getByRole("button", { name: "Apply to Auth" }) as HTMLButtonElement;
  expect(apply.getAttribute("aria-busy")).toBe("true"); expect(apply.disabled).toBe(true);
  expect(screen.getByRole("button", { name: "Save email settings" }).hasAttribute("aria-busy")).toBe(false);
  expect(dataEmailApi.action).not.toHaveBeenCalled();
});
it("retains saved credentials without returning or resubmitting a fake password", async () => {
  vi.mocked(dataEmailApi.status).mockResolvedValue(saved); vi.mocked(dataEmailApi.action).mockResolvedValue({ ...saved, revision: 2 });
  render(<DataEmail projectId="owned" ready />);
  await screen.findByText("Settings saved privately.");
  expect((screen.getByLabelText("SMTP password") as HTMLInputElement).value).toBe("");
  fireEvent.change(screen.getByLabelText("Sender name"), { target: { value: "Updated name" } });
  fireEvent.click(screen.getByRole("button", { name: "Save email settings" }));
  await waitFor(() => expect(dataEmailApi.action).toHaveBeenCalled());
  const input = vi.mocked(dataEmailApi.action).mock.calls[0][1];
  expect(input.configuration).not.toHaveProperty("password"); expect(input.configuration).not.toHaveProperty("hasPassword");
});
it("draft changes block checks, applies and sends until saved", async () => {
  vi.mocked(dataEmailApi.status).mockResolvedValue(applied); render(<DataEmail projectId="owned" ready />);
  await screen.findByText("Applied to healthy Auth.");
  fireEvent.change(screen.getByLabelText("SMTP server"), { target: { value: "smtp.changed.test" } });
  expect((screen.getByRole("button", { name: "Check SMTP connection" }) as HTMLButtonElement).disabled).toBe(true);
  expect((screen.getByRole("button", { name: "Apply to Auth" }) as HTMLButtonElement).disabled).toBe(true);
  expect((screen.getByLabelText("SMTP password") as HTMLInputElement).required).toBe(true);
});
it("checks saved settings without mail and gates application on stack readiness", async () => {
  vi.mocked(dataEmailApi.status).mockResolvedValue(saved); vi.mocked(dataEmailApi.action).mockResolvedValue({ ...saved, connection: { status: "passed", revision: 1 }, message: "Connected without email." });
  render(<DataEmail projectId="owned" ready={false} />); await screen.findByText("Settings saved privately.");
  expect((screen.getByRole("button", { name: "Apply to Auth" }) as HTMLButtonElement).disabled).toBe(true);
  fireEvent.click(screen.getByRole("button", { name: "Check SMTP connection" }));
  await waitFor(() => expect(dataEmailApi.action).toHaveBeenCalledExactlyOnceWith("owned", { action: "check", expectedRevision: 1 }));
  expect(await screen.findByText(/Connection verified: TLS and SMTP authentication succeeded/)).not.toBeNull();
});
it("test requires applied settings, an explicit recipient and acknowledgement and reports acceptance", async () => {
  vi.mocked(dataEmailApi.status).mockResolvedValue(applied); vi.mocked(dataEmailApi.action).mockResolvedValue({ ...applied, test: { status: "accepted", revision: 1, acceptedAt: "now" } });
  render(<DataEmail projectId="owned" ready />); await screen.findByText("Applied to healthy Auth.");
  fireEvent.change(screen.getByLabelText("Test recipient"), { target: { value: "recipient@example.test" } });
  const send = screen.getByRole("button", { name: "Send test email" }); expect((send as HTMLButtonElement).disabled).toBe(true);
  fireEvent.click(screen.getByLabelText("Send one test email to this recipient.")); fireEvent.click(send);
  await waitFor(() => expect(dataEmailApi.action).toHaveBeenCalledExactlyOnceWith("owned", { action: "test", expectedRevision: 1, recipient: "recipient@example.test", acknowledgeSend: true }));
  expect(await screen.findByText(/SMTP accepted the test email/)).not.toBeNull(); expect(screen.getByText(/delivery has not been confirmed/)).not.toBeNull();
  expect((screen.getByLabelText("Send one test email to this recipient.") as HTMLInputElement).checked).toBe(false);
});
it("public data connection owns the redirect origin and Admin permissions block mutations", async () => {
  vi.mocked(dataEmailApi.status).mockResolvedValue({ ...saved, originSource: "public-domain", applicationOrigin: "https://app.example.test" });
  render(<DataEmail projectId="owned" ready canManage={false} />); await screen.findByText("Settings saved privately.");
  const origin = screen.getByLabelText("Application redirect address") as HTMLInputElement;
  expect(origin.value).toBe("https://app.example.test"); expect(origin.disabled).toBe(true);
  expect((screen.getByRole("button", { name: "Save email settings" }) as HTMLButtonElement).disabled).toBe(true);
  expect(dataEmailApi.action).not.toHaveBeenCalled();
});
it("failed applies show truthful recovery errors without overwriting the saved state", async () => {
  vi.mocked(dataEmailApi.status).mockResolvedValue(saved); vi.mocked(dataEmailApi.action).mockRejectedValue(new Error("Previous Auth settings restored; draft retained."));
  render(<DataEmail projectId="owned" ready />); await screen.findByText("Settings saved privately.");
  fireEvent.click(screen.getByRole("button", { name: "Apply to Auth" }));
  expect(await screen.findByRole("alert")).toHaveProperty("textContent", "Previous Auth settings restored; draft retained.");
  expect((screen.getByLabelText("SMTP server") as HTMLInputElement).value).toBe("smtp.example.test");
});
it("ignores a late operation after switching projects and rejects foreign readback", async () => {
  let complete!: (value: DataEmailStatus) => void;
  vi.mocked(dataEmailApi.status).mockResolvedValue(saved); vi.mocked(dataEmailApi.action).mockImplementation(() => new Promise(resolve => { complete = resolve; }));
  const changed = vi.fn(); const view = render(<DataEmail projectId="owned" ready onChange={changed} />);
  await screen.findByText("Settings saved privately."); fireEvent.click(screen.getByRole("button", { name: "Apply to Auth" }));
  changed.mockClear(); vi.mocked(dataEmailApi.status).mockResolvedValue({ ...empty, projectId: "other" });
  view.rerender(<DataEmail projectId="other" ready onChange={changed} />);
  await screen.findByText("Email is not configured.");
  await act(async () => { complete(applied); });
  expect(screen.queryByText("Applied to healthy Auth.")).toBeNull(); expect(changed).toHaveBeenCalledTimes(1);
  vi.mocked(dataEmailApi.status).mockResolvedValue({ ...empty, projectId: "foreign" });
  fireEvent.click(screen.getByRole("button", { name: "Refresh email status" }));
  expect(await screen.findByRole("alert")).toHaveProperty("textContent", expect.stringContaining("could not verify this project's email"));
});
