package product

import (
	"context"
	"time"
)

// ProjectOutcomes describes the bounded retained history, not lifetime
// throughput or current production health. It contains no personal dialogue.
type ProjectOutcomes struct {
	Measurements                    *ProjectOutcomeMeasurements `json:"measurements,omitempty"`
	Status                          string                      `json:"status"`
	Scope                           string                      `json:"scope"`
	ObservedAt                      string                      `json:"observedAt"`
	RetainedRuns                    *int                        `json:"retainedRuns"`
	RecordedAcceptances             *int                        `json:"recordedAcceptances"`
	UnconfirmedAcceptedObservations *int                        `json:"unconfirmedAcceptedObservations"`
	ReviewRequiredRuns              *int                        `json:"reviewRequiredRuns"`
	UnresolvedCheckRepairTasks      *int                        `json:"unresolvedCheckRepairTasks"`
	OmittedRuns                     *int                        `json:"omittedRuns"`
	LatestAccepted                  *ProjectOutcomeAcceptance   `json:"latestAccepted"`
}

type ProjectOutcomeAcceptance struct {
	RunID      string `json:"runId"`
	CheckID    string `json:"checkId"`
	ReleaseID  string `json:"releaseId"`
	AcceptedAt string `json:"acceptedAt"`
	Commit     string `json:"commit"`
	Target     string `json:"target"`
}

func unavailableProjectOutcomes() ProjectOutcomes {
	return ProjectOutcomes{Status: "unavailable", Scope: "retained-run-history", ObservedAt: time.Now().UTC().Format(time.RFC3339Nano)}
}

// List has already validated private records. Rejoin acceptance here rather
// than treating a state label or a release alone as accepted work. Observation
// validity remains a separate recorded fact; neither value certifies live health.
func summarizeProjectOutcomes(ctx context.Context, projectID string, runs []ProjectRun) ProjectOutcomes {
	view := unavailableProjectOutcomes()
	if ctx.Err() != nil || len(runs) > projectRunHistoryLimit {
		return view
	}
	accepted, unconfirmed, reviews, repairs, omitted := 0, 0, 0, 0, 0
	seen := map[string]bool{}
	var latest time.Time
	for _, run := range runs {
		if ctx.Err() != nil || run.ProjectID != projectID || run.Input.Implementation.ProjectID != projectID || run.ID == "" || seen[run.ID] {
			return unavailableProjectOutcomes()
		}
		seen[run.ID] = true
		if projectRunAttentionKind(run) != "" {
			reviews++
		}
		if run.Swarm != nil && run.State != "complete" && run.State != "cancelled" {
			unresolved := map[string]bool{}
			for _, wave := range run.Swarm.CheckRepairs {
				for _, member := range wave.Tasks {
					if member.TaskID != "" && !checkRepairMemberIntegrated(run, member) {
						unresolved[member.TaskID] = true
					}
				}
			}
			repairs += len(unresolved)
		}
		if run.State != "complete" {
			if run.Acceptance != nil {
				return unavailableProjectOutcomes()
			}
			continue
		}
		if run.Acceptance == nil {
			return unavailableProjectOutcomes()
		}
		at, err := time.Parse(time.RFC3339Nano, run.Acceptance.AcceptedAt)
		if err != nil {
			return unavailableProjectOutcomes()
		}
		var joined *ProjectOutcomeAcceptance
		for _, check := range run.Checks {
			if check.CheckID != run.Acceptance.CheckID || validProjectRunCheck(run.Input, check) != nil {
				continue
			}
			for _, release := range run.Releases {
				if release.ReleaseID == run.Acceptance.ReleaseID && release.CheckID == check.CheckID && validProjectRunRelease(run.Input, check, release) == nil {
					if joined != nil {
						return unavailableProjectOutcomes()
					}
					joined = &ProjectOutcomeAcceptance{RunID: run.ID, CheckID: check.CheckID, ReleaseID: release.ReleaseID, AcceptedAt: run.Acceptance.AcceptedAt, Commit: release.Commit, Target: release.Target}
				}
			}
		}
		if joined == nil {
			return unavailableProjectOutcomes()
		}
		accepted++
		if run.Observation == nil || !run.Observation.Valid {
			unconfirmed++
		}
		if view.LatestAccepted == nil || at.After(latest) || at.Equal(latest) && run.ID < view.LatestAccepted.RunID {
			latest, view.LatestAccepted = at, joined
		}
	}
	retained := len(runs)
	view.Measurements = summarizeRunMeasurements(runs)
	view.Status = "recorded"
	view.RetainedRuns, view.RecordedAcceptances, view.UnconfirmedAcceptedObservations = &retained, &accepted, &unconfirmed
	view.ReviewRequiredRuns, view.UnresolvedCheckRepairTasks, view.OmittedRuns = &reviews, &repairs, &omitted
	return view
}
