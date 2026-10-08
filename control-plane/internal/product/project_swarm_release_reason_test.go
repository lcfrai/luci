package product

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func TestSwarmFailedQueueRetainsDiagnosisWhileCleanupRetries(t *testing.T) {
	s, mux, run, base, admitted, starts, children := schedulerFixture(t)
	*admitted = true
	enqueueSchedulerFixture(t, mux, run, base, []string{"task-a"})
	if err := s.RunSwarmSchedulerOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	var diagnostic registry.SessionStartFailure
	for id, child := range children {
		diagnostic = registry.NewSessionStartFailure("admission", "memory_pressure")
		child.StartFailure = diagnostic
		child.Status = registry.StatusExited
		children[id] = child
	}
	held := true
	s.swarmScheduler.runtime.Release = func(context.Context, string) error {
		if held {
			return errors.New("controlled lifecycle still unresolved")
		}
		return nil
	}
	if err := s.RunSwarmSchedulerOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	queue, err := s.ProjectSwarmQueueSnapshot(run.ProjectID)
	if err != nil || queue.Counts.Failed != 1 || queue.Counts.Completed != 0 || queue.Items[0].ReasonCode != "memory_pressure" || queue.Items[0].MemoryMiB != 1024 || !strings.Contains(queue.Items[0].Reason, "remains held") {
		t.Fatal("lost failed/held distinction", queue, err)
	}
	held = false
	if err := s.RunSwarmSchedulerOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	queue, err = s.ProjectSwarmQueueSnapshot(run.ProjectID)
	if err != nil || queue.Counts.Failed != 1 || queue.Items[0].MemoryMiB != 0 || queue.Items[0].CPUMillis != 0 || queue.Items[0].Reason != diagnostic.Message || queue.Items[0].ReasonCode != "memory_pressure" || *starts != 1 {
		t.Fatal("successful cleanup kept stale hold or retried child", queue, err)
	}
}
