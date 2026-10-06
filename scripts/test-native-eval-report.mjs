import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";
import { annotation, nativeFailure, publishFailure } from "./native-eval-report.mjs";

const source = new URL("./native-installed-eval.mjs", import.meta.url).href;
const reporter = new URL("./native-eval-report.mjs", import.meta.url).href;
const secret = "private-fixture-value-do-not-publish";
const result = () => ({ mode: "seed", stage: "runtime-configuration", checks: ["private " + secret],
  completedCheckStages: ["authenticate", "save-definition"],
  actionStages: [{ call: "fill", kind: "label", label: "Private value", phase: "native-value", value: secret }],
  controlDiagnostics: { text: secret }, fixture: { password: secret } });

test("reports the actual named failed action and exact local assertion line without assertion values or receipt fields", () => {
  const error = Object.assign(new Error(secret), { code: "ERR_ASSERTION", actual: secret, expected: secret,
    stack: `AssertionError: ${secret}\n    at foreign (file:///foreign/native-installed-eval.mjs:55:7)\n    at run (${source}:623:5)` });
  const report = nativeFailure(result(), error);
  const output = [];
  publishFailure(report, { output: line => output.push(line), summaryPath: "" });
  assert.equal(report.category, "assertion-failed");
  assert.equal(report.line, 623);
  assert.match(output[0], /file=scripts\/native-installed-eval.mjs,line=623/);
  assert.match(output[0], /fill Private value; phase: native-value/);
  assert.match(output[0], /authenticate, save-definition/);
  assert(!JSON.stringify(report).includes(secret));
  assert(!output.join("").includes(secret));
});

test("allowlists WebDriver categories, controls and stages instead of redacting arbitrary remote text", () => {
  const error = Object.assign(new Error(`script returned ${secret}`), { webdriverError: "session not created" });
  const report = nativeFailure({ ...result(), stage: secret, completedCheckStages: [secret], actionStages: [{ label: secret }] }, error);
  assert.equal(report.category, "webdriver: session not created");
  assert.equal(report.stage, "unknown");
  assert.equal(report.lastAction, null);
  assert.deepEqual(report.completedStages, []);
  error.webdriverError = secret;
  assert.equal(nativeFailure(result(), error).category, "unexpected-failure");
});

test("distinguishes owned wait/click deadlines and network failures without forwarding nested errors", () => {
  for (const category of ["wait-timeout", "click-timeout", "preparation-timeout"]) {
    assert.equal(nativeFailure(result(), { nativeFailure: category, message: secret }).category, category);
  }
  assert.equal(nativeFailure(result(), { cause: { code: "ECONNREFUSED", message: secret } }).category, "connection-refused");
  assert.equal(nativeFailure(result(), { name: "TimeoutError", message: secret }).category, "request-timeout");
  assert.equal(nativeFailure(result(), { stack: `Error\n at foreign (file:///foreign/native-installed-eval.mjs:12:8)` }).line, undefined);
});

test("escapes workflow-command properties and newlines before emitting a single annotation", () => {
  const line = annotation("Native,100%: \n::error::injected", "failure\r\n::error::injected%", "scripts/a,b:%.mjs", 17);
  assert.equal(line.split("\n").length, 1);
  assert.match(line, /title=Native%2C100%25%3A %0A%3A%3Aerror%3A%3Ainjected/);
  assert.match(line, /::failure%0D%0A::error::injected%25$/);
});

test("setup diagnostics expose only numeric status, bounded version and known exception class", () => {
  const output = [];
  publishFailure({ mode: "installer", stage: "download-edge-driver", category: "unexpected-failure", runtimeVersion: "151.0.7922.173", httpStatus: 404, exceptionType: "HttpResponseException", line: 152, message: secret }, { output: value => output.push(value), summaryPath: "" });
  assert.match(output[0], /scripts\/test-installed-windows.ps1,line=152/);
  assert.match(output[0], /151.0.7922.173/);
  assert.match(output[0], /HTTP status: 404/);
  assert.match(output[0], /HttpResponseException/);
  const suppressed = [];
  publishFailure({ mode: secret, stage: secret, category: secret, runtimeVersion: secret, httpStatus: secret, exceptionType: secret, completedStages: Array(100).fill(secret) }, { output: value => suppressed.push(value), summaryPath: "" });
  assert(![...output, ...suppressed].join("").includes(secret));
  assert(suppressed[0].length < 500);
});

test("real reporter output and summary preserve a failing process exit even when summary writing fails", () => {
  const root = mkdtempSync(join(tmpdir(), "luci-native-report-"));
  try {
    for (const summary of [join(root, "summary.md"), join(root, "absent", "summary.md")]) {
      const child = spawnSync(process.execPath, ["--input-type=module", "-e", `import {publishFailure} from ${JSON.stringify(reporter)}; process.exitCode=17; publishFailure({mode:'seed',stage:'save-sketch',category:'click-timeout',completedChecks:8,lastAction:{call:'click',label:'Save design',phase:'lookup'}});`], { encoding: "utf8", env: { ...process.env, GITHUB_STEP_SUMMARY: summary } });
      assert.equal(child.status, 17);
      assert.match(child.stdout, /^::error /);
      assert.match(child.stdout, /Save design; phase: lookup/);
      assert(!child.stdout.includes(secret));
    }
    assert.match(readFileSync(join(root, "summary.md"), "utf8"), /save-sketch/);
  } finally { rmSync(root, { recursive: true, force: true }); }
});

test("the actual installer CLI publishes safe metadata without reading private receipts", () => {
  const child = spawnSync(process.execPath, [fileURLToPath(new URL("./native-eval-report.mjs", import.meta.url)), "--installer", "fresh-install", "native-journey-failed", "2", "94", "unknown", "0", "RuntimeException"], { encoding: "utf8", env: { ...process.env, GITHUB_STEP_SUMMARY: "" } });
  assert.equal(child.status, 0);
  assert.match(child.stdout, /stage: fresh-install; reason: native-journey-failed/);
  assert.match(child.stdout, /Completed checks: 2/);
});
