package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/proxy"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func terminalContractFixture() (product.ProjectRun, registry.Session, *registry.ReviewedContext, []string) {
	b := &registry.ImplementationContext{ProjectID: "p-a", WorktreeID: "w-a", TaskIDs: []string{"task-a"}, SourceCommit: strings.Repeat("a", 40), SourceDigest: strings.Repeat("b", 64)}
	row := registry.Session{ID: "s-12345678", ProjectID: "p-a", WorktreeID: "w-a", Harness: "codex", ProjectRunID: "run-a", ProjectRunOperationID: "op-a", ImplementationContext: b, Status: registry.StatusExited, TaskTerminalExitZero: true, TaskTerminalObserverStarted: true}
	descriptor := &registry.ReviewedContext{Version: 1, Digest: strings.Repeat("c", 64)}
	row.ReviewedContext = descriptor
	run := product.ProjectRun{ID: "run-a", ProjectID: "p-a", Swarm: &product.ProjectSwarm{Tasks: []product.ProjectSwarmTask{{ID: "task-a", Role: "coding"}}}, Operations: []product.ProjectRunOperation{{ID: "op-a", Kind: "session", ChildID: row.ID, TaskID: "task-a", WorktreeID: "w-a", Attempt: 1, RequestDigest: strings.Repeat("d", 64), Implementation: b.Clone()}}}
	return run, row, descriptor, []string{"exec", "--color", "never", "--dangerously-bypass-approvals-and-sandbox", "--", "Original <saved> task"}
}

func TestTaskTerminalContractBindsOriginalAttemptContextSourceAndReviewer(t *testing.T) {
	run, row, descriptor, args := terminalContractFixture()
	original, err := terminalContract(run, row, descriptor, "codex", args, "luci/sandbox:dev")
	if err != nil {
		t.Fatal(err)
	}
	if original.Role != "coding" || original.Attempt != 1 || original.SourceCommit != row.ImplementationContext.SourceCommit || original.ContextDigest != descriptor.Digest || original.CommandDigest != terminalCommandDigest("/usr/local/bin/codex", args) {
		t.Fatal("contract lost original bindings", original)
	}
	for _, kind := range []string{"session", "operation", "implementation", "reviewer", "attempt"} {
		t.Run(kind, func(t *testing.T) {
			nextRun, nextRow, nextDescriptor, nextArgs := terminalContractFixture()
			switch kind {
			case "session":
				nextRow.ID = "s-99999999"
			case "operation":
				nextRow.ProjectRunOperationID = "op-other"
			case "implementation":
				nextRow.ImplementationContext.SourceDigest = strings.Repeat("e", 64)
			case "reviewer":
				nextRun.Swarm.Tasks[0].Role = "reviewer"
			case "attempt":
				nextRun.Operations[0].Attempt = 0
			}
			if _, err := terminalContract(nextRun, nextRow, nextDescriptor, "codex", nextArgs, "luci/sandbox:dev"); err == nil {
				t.Fatal("unbound identity was accepted")
			}
		})
	}
	run.Swarm.Tasks[0].Role = "reviewer"
	run.Swarm.Tasks[0].ReviewOnly = true
	run.Swarm.Tasks[0].ReviewOfTaskID = "task-coding"
	review, err := terminalContract(run, row, descriptor, "codex", args, "luci/sandbox:dev")
	if err != nil || review.Role != "reviewer" || review.ReviewOfTaskID != "task-coding" {
		t.Fatal(review, err)
	}
	if terminalCommandDigest("a", []string{"bc"}) == terminalCommandDigest("ab", []string{"c"}) {
		t.Fatal("ambiguous argv framing")
	}
}

func TestTaskTerminalCollectorRequiresControllerExitAndExactReceiptCorrelation(t *testing.T) {
	run, row, descriptor, args := terminalContractFixture()
	a := &API{deps: Deps{ConversationsDir: t.TempDir(), ProjectsDir: t.TempDir()}}
	contract, err := terminalContract(run, row, descriptor, "/usr/local/bin/codex", args, "luci/sandbox:dev")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(contract)
	raw = append(raw, '\n')
	dir, err := a.taskTerminalPath(row.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err = reviewedWriteFile(filepath.Join(dir, "contract.json"), raw); err != nil {
		t.Fatal(err)
	}
	row.Command = "/usr/local/bin/node"
	row.Args = append([]string{taskTerminalWrapper, reviewedHash(raw), "/usr/local/bin/codex"}, args...)
	output := t.TempDir()
	root, err := os.OpenRoot(output)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	report := []byte(`{"version":1,"summary":"Reported result","changedFiles":[],"decisions":[],"reportedChecks":[],"questions":[],"limitations":[]}`)
	receipt := []byte(`{"version":1,"contractDigest":"` + reviewedHash(raw) + `","reportDigest":"` + reviewedHash(report) + `"}`)
	if err = reviewedWriteFile(filepath.Join(output, "terminal-receipt.json"), receipt); err != nil {
		t.Fatal(err)
	}
	if err = a.verifyTerminalReport(run, row, root, output, report); err != nil {
		t.Fatal(err)
	}
	row.TaskTerminalExitZero = false
	if err = a.verifyTerminalReport(run, row, root, output, report); err == nil {
		t.Fatal("same-UID receipt invented original zero exit")
	}
	row.TaskTerminalExitZero = true
	if err = a.verifyTerminalReport(run, row, root, output, append(report, ' ')); err == nil {
		t.Fatal("different report bytes accepted")
	}
	run.Operations[0].Attempt++
	if err = a.verifyTerminalReport(run, row, root, output, report); err == nil {
		t.Fatal("receipt was adopted by another attempt")
	}
	legacy := registry.Session{}
	if err = a.verifyTerminalReport(run, legacy, root, output, nil); err != nil {
		t.Fatal("legacy report gained a terminal receipt requirement", err)
	}
}

func TestTaskTerminalTypedExitFactRejectsUnknownNonzeroAndStaleObservers(t *testing.T) {
	for _, kind := range []string{"zero", "nonzero", "unknown", "stale-command", "stale-context", "stale-creation", "stale-operation"} {
		t.Run(kind, func(t *testing.T) {
			f := attachNewFixture(t, nil, func(row *registry.Session) {
				terminalFixtureSession(row)
				row.Harness = "codex"
				row.Command = "/usr/local/bin/node"
				row.Args = []string{taskTerminalWrapper, strings.Repeat("a", 64), "/usr/local/bin/codex", "exec", "--", "prompt"}
				row.ProjectRunID = "run-a"
				row.ProjectRunOperationID = "op-a"
				row.TaskTerminalObserverStarted = true
				row.ReviewedContext = &registry.ReviewedContext{Version: 1, Digest: strings.Repeat("b", 64)}
			})
			observed, err := f.store.Get(f.id)
			if err != nil {
				t.Fatal(err)
			}
			payload := "exit status 0"
			switch kind {
			case "nonzero":
				payload = "exit status 1"
			case "unknown":
				payload = "unknown"
			case "stale-command":
				observed.Args[1] = strings.Repeat("c", 64)
			case "stale-context":
				observed.ReviewedContext.Digest = strings.Repeat("d", 64)
			case "stale-creation":
				observed.CreatedAt = observed.CreatedAt.Add(-time.Second)
			case "stale-operation":
				observed.ProjectRunOperationID = "op-old"
			}
			f.api.markTaskTerminalExitedWithEOF(context.Background(), observed, proxy.DecodeProcessExit([]byte(payload)))
			after, err := f.store.Get(f.id)
			if err != nil {
				t.Fatal(err)
			}
			if after.TaskTerminalExitZero != (kind == "zero") || f.be.destroyCount() != 0 {
				t.Fatal("stale or untyped lifecycle was promoted", kind, after.TaskTerminalExitZero)
			}
		})
	}
}

func TestTaskTerminalLegacyEncodingAndSchemaPinStayExplicit(t *testing.T) {
	raw, err := json.Marshal(registry.Session{})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("taskTerminalExitZero")) {
		t.Fatal("legacy session encoding changed")
	}
	schema, err := os.ReadFile(filepath.Join("..", "..", "..", "images", "sandbox", "task-handoff.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	if reviewedHash(schema) != taskTerminalSchemaDigest {
		t.Fatal("installed schema and controller source disagree")
	}
	var parsed map[string]any
	if json.Unmarshal(schema, &parsed) != nil || parsed["additionalProperties"] != false {
		t.Fatal("schema is not closed")
	}
}

func TestTaskTerminalExitFactIsClearedByRealRestartAllocation(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	row := sessionsStarting("s-0000000b")
	row.Harness = "shell"
	row.Model = ""
	row.Command = "/bin/bash"
	row.Args = []string{"-l", "-i"}
	row.TaskTerminalExitZero = true
	row.TaskTerminalObserverStarted = true
	sessionsSeed(t, env.st, row)
	sessionsDecodeOne(t, env.do(t, http.MethodPost, "/v1/sessions/s-0000000b/restart", ""), http.StatusOK)
	sessionsAwaitWatch(t, done)
	after, err := env.st.Get(row.ID)
	if err != nil || (after.TaskTerminalExitZero || after.TaskTerminalObserverStarted) {
		t.Fatal("new allocation retained old terminal fact", err)
	}
}

func TestTaskTerminalGenericObservationUpgradesOnlyAfterOriginalEOF(t *testing.T) {
	for _, kind := range []string{"original-eof", "exit-frame-only", "changed-contract"} {
		t.Run(kind, func(t *testing.T) {
			f := attachNewFixture(t, nil, func(row *registry.Session) {
				terminalFixtureSession(row)
				row.Command = "/usr/local/bin/node"
				row.Args = []string{taskTerminalWrapper, strings.Repeat("a", 64), "/usr/local/bin/codex", "exec", "--", "prompt"}
				row.ReviewedContext = &registry.ReviewedContext{Version: 1, Digest: strings.Repeat("b", 64)}
				row.ProjectRunID = "run-a"
				row.ProjectRunOperationID = "op-a"
				row.TaskTerminalObserverStarted = true
			})
			original, err := f.store.Get(f.id)
			if err != nil {
				t.Fatal(err)
			}
			_, err = f.store.Update(f.id, func(row *registry.Session) error {
				row.Status = registry.StatusExited
				row.ExitReason = "process exited"
				row.AttachToken = ""
				if kind == "changed-contract" {
					row.Args[1] = strings.Repeat("c", 64)
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			exit := proxy.DecodeProcessExit([]byte("exit status 0"))
			if kind == "exit-frame-only" {
				f.api.markAttachedSessionExitedWithExit(original, exit)
			} else {
				f.api.markTaskTerminalExitedWithEOF(context.Background(), original, exit)
			}
			current, err := f.store.Get(f.id)
			if err != nil || current.TaskTerminalExitZero != (kind == "original-eof") {
				t.Fatal("generic observation upgraded the wrong evidence", kind, err)
			}
		})
	}
}

func TestOtherHarnessContractsKeepExactOriginalRoleAndExecutable(t *testing.T) {
	for _, h := range []struct {
		harness, command string
		args             []string
	}{{"claude-code", "claude", []string{"--print", "--permission-mode", "bypassPermissions", "saved task"}}, {"opencode", "opencode", []string{"run", "--pure", "--auto", "--", "saved task"}}} {
		run, row, descriptor, _ := terminalContractFixture()
		row.Harness = h.harness
		run.Swarm.Tasks[0].Role = "reviewer"
		run.Swarm.Tasks[0].ReviewOnly = true
		run.Swarm.Tasks[0].ReviewOfTaskID = "task-coding"
		c, err := terminalContract(run, row, descriptor, h.command, h.args, "sha256:"+strings.Repeat("a", 64))
		if err != nil || c.Version != 2 || c.Harness != h.harness || c.Role != "reviewer" || !c.ReviewOnly || c.CommandDigest != terminalCommandDigest("/usr/local/bin/"+h.command, h.args) {
			t.Fatal(c, err)
		}
		if _, err := terminalContract(run, row, descriptor, "codex", h.args, c.Image); err == nil {
			t.Fatal("harness executable mismatch accepted")
		}
		row.ImplementationContext.SourceDigest = strings.Repeat("f", 64)
		if _, err := terminalContract(run, row, descriptor, h.command, h.args, c.Image); err == nil {
			t.Fatal("original source mismatch accepted")
		}
	}
	run, row, descriptor, args := terminalContractFixture()
	c, err := terminalContract(run, row, descriptor, "codex", args, "image")
	raw, _ := json.Marshal(c)
	if err != nil || c.Version != 1 || bytes.Contains(raw, []byte("harness")) {
		t.Fatal("legacy Codex contract changed", string(raw), err)
	}
}
