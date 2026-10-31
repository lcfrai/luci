package product

import "time"

type ProjectOutcomeMeasurements struct {
	Version                              int    `json:"version"`
	Scope                                string `json:"scope"`
	MeasuredTaskIntervals                int    `json:"measuredTaskIntervals"`
	UnknownTaskIntervals                 int    `json:"unknownTaskIntervals"`
	RecordedNoWorkerAttempts             int    `json:"recordedNoWorkerAttempts"`
	TaskRuntimeMilliseconds              *int64 `json:"taskRuntimeMilliseconds"`
	RunToAcceptanceMilliseconds          *int64 `json:"runToAcceptanceMilliseconds"`
	AcceptedIntervalsMeasured            int    `json:"acceptedIntervalsMeasured"`
	AcceptedIntervalsUnknown             int    `json:"acceptedIntervalsUnknown"`
	RecordedHumanActions                 *int   `json:"recordedHumanActions"`
	OmittedHumanActions                  int    `json:"omittedHumanActions"`
	RunsWithEarlierUninstrumentedActions int    `json:"runsWithEarlierUninstrumentedActions"`
	HumanActionScope                     string `json:"humanActionScope"`
	InputTokens                          *int64 `json:"inputTokens"`
	OutputTokens                         *int64 `json:"outputTokens"`
}

// Running-state labels and UpdatedAt are not clocks. Task duration uses only
// the exact original stopped-container fact retained by controller observation.
// Sums are overlapping task durations, not elapsed project time or CPU effort.
func summarizeRunMeasurements(runs []ProjectRun) *ProjectOutcomeMeasurements {
	view := &ProjectOutcomeMeasurements{Version: 1, Scope: "retained-run-history", HumanActionScope: "recorded-run-controls"}
	runtime, acceptedRuntime := int64(0), int64(0)
	human, measuredRuns := 0, 0
	for _, run := range runs {
		if !validRunMeasurements(run) {
			return nil
		}
		if run.Measurements == nil {
			view.RunsWithEarlierUninstrumentedActions++
		} else {
			measuredRuns++
			human += len(run.Measurements.HumanActions)
			view.OmittedHumanActions += run.Measurements.OmittedHumanActions
			if run.Measurements.SinceRevision > 1 {
				view.RunsWithEarlierUninstrumentedActions++
			}
		}
		for _, operation := range run.Operations {
			if operation.Kind != "session" {
				continue
			}
			if operation.RuntimeTiming != nil && operation.RuntimeTiming.ElapsedMilliseconds != nil {
				view.MeasuredTaskIntervals++
				runtime += *operation.RuntimeTiming.ElapsedMilliseconds
			} else if operation.StartFailure.Valid() {
				view.RecordedNoWorkerAttempts++
			} else {
				view.UnknownTaskIntervals++
			}
		}
		if run.Acceptance != nil {
			start, a := time.Parse(time.RFC3339Nano, run.CreatedAt)
			end, b := time.Parse(time.RFC3339Nano, run.Acceptance.AcceptedAt)
			if a == nil && b == nil && !end.Before(start) && end.Sub(start) <= 365*24*time.Hour {
				view.AcceptedIntervalsMeasured++
				acceptedRuntime += end.Sub(start).Milliseconds()
			} else {
				view.AcceptedIntervalsUnknown++
			}
		}
	}
	if view.MeasuredTaskIntervals > 0 {
		view.TaskRuntimeMilliseconds = &runtime
	}
	if view.AcceptedIntervalsMeasured > 0 {
		view.RunToAcceptanceMilliseconds = &acceptedRuntime
	}
	if measuredRuns > 0 {
		view.RecordedHumanActions = &human
	}
	// Upstream harnesses in this cut expose no trusted usage-count or provider
	// charge receipt. Null is deliberately distinct from zero or an estimate.
	return view
}
