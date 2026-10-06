// Public CI diagnostics are allowlisted metadata, never raw exceptions, page
// content, field values, subprocess output or private fixture data.
import { appendFileSync } from "node:fs";
import { fileURLToPath, pathToFileURL } from "node:url";

const stages = new Set([
  "admission", "install-tauri-driver", "find-webview-runtime", "download-edge-driver",
  "verify-edge-driver", "download-gsudo", "verify-gsudo", "verify-medium-integrity", "start-driver",
  "install-previous", "install-candidate", "fresh-install", "previous-version-seed", "version-upgrade",
  "clean-restart", "preserve-user-data", "reinstall-candidate", "same-version-reinstall", "uninstall",
  "verify-uninstall", "cleanup", "driver-ready", "start-webview", "native-bridge", "sidecar-ownership",
  "local-access", "authenticate", "partial-first-run", "workspace-settings", "git-identity",
  "create-project", "save-definition", "approve-definition", "draw-sketch", "save-sketch",
  "implementation-plan", "read-context", "git-commit", "deployment-tabs", "runtime-configuration",
  "verify-project", "verify-definition", "verify-sketch", "verify-plan", "verify-git",
  "verify-configuration", "account-isolation", "owner-recovery", "recovered-sign-in", "native-shutdown",
]);
const controls = new Set([
  "Continue to projects", "Close settings", "Bootstrap token", "Display name", "Username", "Password",
  "Confirm password", "Create account", "Create Super Admin account", "Sign in", "Sign out",
  ".rail-account button:first-child", "Forgot sign-in?", "New password", "Recover owner account",
  "Workspace setup", "Settings", "On this computer", '[data-kind="project"]', "Define", "Specification",
  "The idea", "Requirement 1", "Design", "Sketch", ".excalidraw canvas", "Develop", "Task 1", "Git",
  "Deploy", "Private value", "Projects folder", "Save settings", "Your Git commit identity",
  "Git identity, agent connection and runtime", "Commit author name", "Commit author email",
  "Save Git identity", "New project", '.project-entry[role="dialog"][aria-modal="true"]', "Project name",
  "Create project", "Who it is for", "Boundaries and constraints", "Add requirement", "Acceptance scenario 1",
  "Save definition", "What good looks like", "Save brief", "Approve definition", ".excalidraw__canvas.interactive",
  "Save design", "New task", "Add task", "Save implementation plan", "Files", "LUCI.md", "Commit message",
  "Commit changes", "Previews and publishing", "Application platform", "Runtime configuration",
  "Variable name", "Save runtime variable",
]);
const phases = new Set(["lookup", "center-scroll", "revalidate", "native-click", "native-clear", "native-value", "completed"]);
const categories = new Set([
  "wait-timeout", "click-timeout", "preparation-timeout", "assertion-failed", "request-timeout", "request-aborted",
  "missing-file", "access-denied", "connection-refused", "transport-failed", "invalid-json", "unexpected-failure",
  "tauri-driver-install-failed", "webview-runtime-missing", "edge-driver-mismatch", "gsudo-checksum-mismatch",
  "directory-access-failed", "medium-integrity-failed", "installer-timeout", "installer-exit-failed",
  "installed-resource-missing", "native-journey-failed", "preserved-data-missing", "preserved-data-changed",
  "private-configuration-count", "uninstaller-count", "uninstall-incomplete", "invalid-install-path", "admission-refused",
]);
const webdriverErrors = new Set([
  "stale element reference", "element click intercepted", "element not interactable", "invalid session id",
  "session not created", "no such window", "no such element", "javascript error", "script timeout", "timeout",
  "unknown error", "invalid argument", "unsupported operation", "move target out of bounds",
]);
const exceptionTypes = new Set(["HttpResponseException", "WebException", "Win32Exception", "IOException", "FileNotFoundException", "DirectoryNotFoundException", "UnauthorizedAccessException", "RuntimeException", "CommandNotFoundException", "InvalidOperationException"]);
const count = value => Number.isSafeInteger(value) && value >= 0 ? Math.min(value, 10000) : 0;
const escapeData = value => String(value).replaceAll("%", "%25").replaceAll("\r", "%0D").replaceAll("\n", "%0A");
const escapeProperty = value => escapeData(value).replaceAll(":", "%3A").replaceAll(",", "%2C");
export const annotation = (title, message, file, line) => `::error file=${escapeProperty(file)}${line ? `,line=${count(line)}` : ""},title=${escapeProperty(title)}::${escapeData(message)}`;
function sourceLine(stack) {
  const source = new URL("./native-installed-eval.mjs", import.meta.url);
  for (const frame of String(stack || "").split("\n").slice(1, 30)) {
    const match = /\((.+):(\d+):\d+\)$/.exec(frame.trim()) || /^at (.+):(\d+):\d+$/.exec(frame.trim());
    if (match && [source.href, fileURLToPath(source)].includes(match[1])) return count(Number(match[2]));
  }
}

export function nativeFailure(result, error) {
  let category = categories.has(error?.nativeFailure) ? error.nativeFailure : "unexpected-failure";
  if (webdriverErrors.has(error?.webdriverError)) category = `webdriver: ${error.webdriverError}`;
  else if (error?.code === "ERR_ASSERTION") category = "assertion-failed";
  else if (error?.name === "TimeoutError") category = "request-timeout";
  else if (error?.name === "AbortError") category = "request-aborted";
  else if (error instanceof SyntaxError) category = "invalid-json";
  else {
    const code = error?.code || error?.cause?.code;
    category = ({ ENOENT: "missing-file", EACCES: "access-denied", EPERM: "access-denied", ECONNREFUSED: "connection-refused", ECONNRESET: "transport-failed" })[code] || category;
  }
  const action = result.actionStages?.at(-1);
  return {
    mode: ["seed", "verify"].includes(result.mode) ? result.mode : "unknown",
    stage: stages.has(result.stage) ? result.stage : "unknown",
    category,
    lastAction: action && controls.has(action.label) ? {
      call: ["click", "fill"].includes(action.call) ? action.call : "unknown",
      kind: ["button", "label", "css", "file"].includes(action.kind) ? action.kind : "unknown",
      label: action.label, phase: phases.has(action.phase) ? action.phase : "unknown",
    } : null,
    completedChecks: count(result.checks?.length),
    completedStages: (result.completedCheckStages || []).filter(stage => stages.has(stage)).slice(-12),
    // Only a source line number is retained from the private stack.
    line: sourceLine(error?.stack),
  };
}

export function publishFailure(report, { output = console.log, summaryPath = process.env.GITHUB_STEP_SUMMARY } = {}) {
  const installer = report.mode === "installer";
  const stage = stages.has(report.stage) ? report.stage : "unknown";
  const category = categories.has(report.category) || [...webdriverErrors].some(code => report.category === `webdriver: ${code}`) ? report.category : "unexpected-failure";
  const action = report.lastAction;
  const lines = [
    `Mode: ${installer ? "installer" : ["seed", "verify"].includes(report.mode) ? report.mode : "unknown"}; stage: ${stage}; reason: ${category}.`,
    ...(action && controls.has(action.label) ? [`Last named action: ${["click", "fill"].includes(action.call) ? action.call : "unknown"} ${action.label}; phase: ${phases.has(action.phase) ? action.phase : "unknown"}.`] : []),
    ...(Array.isArray(report.completedStages) && report.completedStages.length ? [`Recent completed check stages: ${report.completedStages.filter(value => stages.has(value)).slice(-12).join(", ")}.`] : []),
    ...(typeof report.runtimeVersion === "string" && /^\d{1,5}(?:\.\d{1,5}){3}$/.test(report.runtimeVersion) ? [`WebView runtime and requested matching Edge driver: ${report.runtimeVersion}.`] : []),
    ...(Number.isInteger(report.httpStatus) && report.httpStatus >= 100 && report.httpStatus <= 599 ? [`HTTP status: ${report.httpStatus}.`] : []),
    ...(exceptionTypes.has(report.exceptionType) ? [`Exception type: ${report.exceptionType}.`] : []),
    `Completed checks: ${count(report.completedChecks)}. Failure remains fatal; full diagnostics remain in the local native-installed-eval receipts.`,
  ];
  const message = lines.join("\n");
  output(annotation("Installed Windows native acceptance", message, installer ? "scripts/test-installed-windows.ps1" : "scripts/native-installed-eval.mjs", report.line));
  if (summaryPath) {
    // Metadata is allowlisted above; escape markup as a second boundary.
    const escaped = message.replaceAll("&", "&amp;").replaceAll("<", "&lt;").replaceAll(">", "&gt;");
    try { appendFileSync(summaryPath, `### Installed Windows native acceptance failed\n\n<pre>${escaped}</pre>\n\n`); }
    catch { output("::warning::Could not append the safe native acceptance summary; the failure annotation above is retained."); }
  }
  return message;
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const [kind, stage, category, completedChecks, line, runtimeVersion, httpStatus, exceptionType] = process.argv.slice(2);
  if (kind !== "--installer") throw new Error("Only safe installer metadata is accepted by this reporter.");
  publishFailure({ mode: "installer", stage, category, completedChecks: Number(completedChecks), line: Number(line), runtimeVersion, httpStatus: Number(httpStatus), exceptionType });
}
