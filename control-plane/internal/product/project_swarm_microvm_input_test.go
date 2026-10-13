package product

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend/e2bbe"
)

func TestMicroVMInputCapturesExactReviewedTaskAndGitBaseWithoutPrivateSource(t *testing.T) {
	s, mux, p, run, base, _ := swarmHTTPFixture(t)
	run = swarmReserveHTTP(t, mux, base, run, "task-a", "cloud-input-attempt-123456")
	op := run.Operations[0]
	input, err := s.BuildProjectSwarmMicroVMInput(context.Background(), p.ID, run.ID, op.ID, "owner-a", 600, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if input.Request.Identity.TaskID != "task-a" || input.Request.Identity.OperationID != op.ChildID ||
		input.Request.SourceDigest != implementationDigest(swarmStartingCommit(op)) ||
		input.Request.ContextDigest != implementationDigest(op.Implementation) || len(input.Archive) == 0 {
		t.Fatal("cloud input lost its selected task, base or context binding", input.Request)
	}
	binding := e2bbe.TransferBinding{Identity: input.Request.Identity, SourceDigest: input.Request.SourceDigest, ContextDigest: input.Request.ContextDigest, Direction: "input"}
	verified, err := e2bbe.VerifyTransfer(bytes.NewReader(input.Archive), binding, input.Request.BundleDigest)
	if err != nil || len(verified.Files) != len(input.Manifest.Files) {
		t.Fatal("input bytes did not match complete transfer manifest", err)
	}
	contextCount, source := 0, 0
	for _, file := range verified.Files {
		if strings.HasPrefix(file.Path, "context/") {
			contextCount++
		} else if strings.HasPrefix(file.Path, "source/") {
			source++
			if strings.HasPrefix(file.Path, "source/.luci/") || file.Path == "source/LUCI.md" {
				t.Fatal("generated context entered writable source", file.Path)
			}
		}
	}
	if source == 0 || contextCount == 0 {
		t.Fatal("input omitted source or approved context", source, contextCount)
	}
	if _, err := s.BuildProjectSwarmMicroVMInput(context.Background(), p.ID, run.ID, op.ID, "owner-a", 0, 1000); err == nil {
		t.Fatal("cloud request with no task lifetime passed input admission")
	}
	worker := mustSwarmPathFixture(t, s, p.ID, run.ID, op.ID)
	if err := os.WriteFile(filepath.Join(worker, "unreviewed.txt"), []byte("not reviewed"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.BuildProjectSwarmMicroVMInput(context.Background(), p.ID, run.ID, op.ID, "owner-a", 600, 1000); err == nil {
		t.Fatal("dirty task checkout admitted an old reviewed source bundle")
	}
}
