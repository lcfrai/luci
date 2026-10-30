package management

import "testing"

func TestTeamGoalOutcomeRoutesMustBelongToExactReturnedGoalRevision(t *testing.T) {
	prefix := `{"goals":[{"id":"goal-one","revision":3,"projects":[{"projectId":"p-one"}]}],"applications":[],"outcomeRouting":{"routes":[`
	for _, row := range []string{
		`{"goalId":"goal-one","goalRevision":2,"projectId":"p-one"}`,
		`{"goalId":"goal-one","goalRevision":3,"projectId":"p-hidden"}`,
		`{"goalId":"goal-other","goalRevision":3,"projectId":"p-one"}`,
	} {
		if _, err := portfolioGoalResponseProjects([]byte(prefix + row + `]}}`)); err == nil {
			t.Fatal("orphan/stale goal route accepted", row)
		}
	}
	ids, err := portfolioGoalResponseProjects([]byte(prefix + `{"goalId":"goal-one","goalRevision":3,"projectId":"p-one"}]}}`))
	if err != nil || len(ids) != 1 || ids[0] != "p-one" {
		t.Fatal("valid route lost response-time authorization", ids, err)
	}
}
