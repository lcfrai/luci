package registry

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTaskContainerFactPersistsClonesAndCannotMoveToAnotherOriginal(t *testing.T) {
	row := sampleSession()
	row.Command, row.Args = "/usr/local/bin/node", []string{"/usr/local/lib/luci/codex-task-result.cjs", strings.Repeat("c", 64), "/usr/local/bin/codex", "exec"}
	row.TaskTerminalImage = "sha256:" + strings.Repeat("b", 64)
	row.TaskTerminalCreatedAt, row.TaskTerminalStartedAt = row.CreatedAt, row.CreatedAt.Add(time.Second)
	row.TaskTerminalResult = &TaskTerminalResult{Method: TaskTerminalResultMethod, SessionCreatedAt: row.CreatedAt, Ref: row.SandboxRef, Image: row.TaskTerminalImage, ContractDigest: row.Args[1], CreatedAt: row.TaskTerminalCreatedAt, StartedAt: row.TaskTerminalStartedAt, FinishedAt: row.TaskTerminalStartedAt.Add(time.Hour), NormalZero: true}
	if !row.HasTaskContainerResult() {
		t.Fatal("exact original fact was rejected")
	}
	path := filepath.Join(t.TempDir(), "sessions.json")
	store, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Create(row); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := reopened.Get(row.ID)
	if err != nil || !loaded.HasTaskContainerResult() {
		t.Fatal("original terminal fact lost on reopen", err)
	}
	clone := loaded.Clone()
	clone.TaskTerminalResult.NormalZero = false
	if !loaded.TaskTerminalResult.NormalZero {
		t.Fatal("clone shared mutable terminal fact")
	}
	for _, field := range []string{"ref", "image", "created", "contract", "start", "oom"} {
		t.Run(field, func(t *testing.T) {
			next := loaded.Clone()
			switch field {
			case "ref":
				next.SandboxRef += "-replacement"
			case "image":
				next.TaskTerminalImage += "-replacement"
			case "created":
				next.CreatedAt = next.CreatedAt.Add(time.Second)
			case "contract":
				next.Args[1] = strings.Repeat("d", 64)
			case "start":
				next.TaskTerminalStartedAt = next.TaskTerminalStartedAt.Add(time.Second)
			case "oom":
				next.TaskTerminalResult.OOMKilled = true
			}
			if next.HasTaskContainerResult() {
				t.Fatal("terminal fact adopted another original or invented OOM success", field)
			}
		})
	}
}

func TestLegacySessionEncodingHasNoNewTaskContainerFact(t *testing.T) {
	data, err := json.Marshal(sampleSession())
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"taskTerminalImage", "taskTerminalCreatedAt", "taskTerminalStartedAt", "taskTerminalResult"} {
		if bytes.Contains(data, []byte(`"`+key+`"`)) {
			t.Fatal("legacy session encoding changed", key)
		}
	}
}
