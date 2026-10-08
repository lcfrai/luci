package product

import (
	"context"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func TestSwarmSchedulerSeparatesFailedStartFromFailedWorkerAndNormalExit(t *testing.T) {
	for _, tc := range []struct {
		name, exit, state, code, attention string
		diagnostic                         bool
	}{
		{"start-denied", "not parsed", "failed", "dispatch_paused", "blocked", true},
		{"worker-failed", "process terminated abnormally", "failed", "worker_failed", "task-review", false},
		{"normal-exit", "process exited", "completed", "review_required", "task-review", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, mux, run, base, admitted, starts, children := schedulerFixture(t)
			*admitted = true
			enqueueSchedulerFixture(t, mux, run, base, []string{"task-a"})
			if err := s.RunSwarmSchedulerOnce(context.Background()); err != nil {
				t.Fatal(err)
			}
			for id, child := range children {
				child.Status = registry.StatusExited
				child.ExitReason = tc.exit
				if tc.diagnostic {
					child.StartFailure = registry.NewSessionStartFailure("admission", tc.code)
				}
				children[id] = child
			}
			if err := s.RunSwarmSchedulerOnce(context.Background()); err != nil {
				t.Fatal(err)
			}
			queue, err := s.ProjectSwarmQueueSnapshot(run.ProjectID)
			if err != nil || len(queue.Items) != 1 {
				t.Fatal(err, queue)
			}
			item := queue.Items[0]
			if item.State != tc.state || item.ReasonCode != tc.code {
				t.Fatal("outcome conflated", item)
			}
			if tc.state == "failed" && (queue.Counts.Failed != 1 || queue.Counts.Completed != 0) {
				t.Fatal("failure counted as completed", queue.Counts)
			}
			saved, err := s.GetProjectSwarmRun(run.ProjectID, run.ID)
			if err != nil {
				t.Fatal(err)
			}
			if kind := projectRunAttentionKind(saved); kind != tc.attention {
				t.Fatal("wrong attention", kind)
			}
			if tc.diagnostic && (!saved.Operations[0].StartFailure.Valid() || saved.Operations[0].Handoff != nil) {
				t.Fatal("missing diagnosis or invented report")
			}
			for range 2 {
				if err := s.RunSwarmSchedulerOnce(context.Background()); err != nil {
					t.Fatal(err)
				}
			}
			if *starts != 1 {
				t.Fatal("terminal attempt replayed", *starts)
			}
		})
	}
}
