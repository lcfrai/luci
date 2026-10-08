package product

import (
	"encoding/json"
	"sort"
)

const orchestratorHandoffLimit = 32 << 10
const orchestratorHandoffReadLimit = 16

// This is attributed context, never an execution gate or verification result.
// Details refer to the exact retained attempt, including superseded attempts.
type OrchestratorTaskHandoffs struct {
	AttemptCount    int                       `json:"attemptCount"`
	OmittedCount    int                       `json:"omittedCount"`
	UnreportedCount int                       `json:"unreportedCount"`
	Entries         []OrchestratorTaskHandoff `json:"entries"`
}

type OrchestratorTaskQuestion struct {
	QuestionID             string `json:"questionId"`
	Question               string `json:"question"`
	Impact                 string `json:"impact"`
	WorkerReportedBlocking bool   `json:"workerReportedBlocking"`
}

// Human answers retain the original immutable question so terse replies stay
// meaningful. This derived projection does not change the full receipt schema.
type OrchestratorTaskAnswer struct {
	TaskHandoffAnswer
	Question string `json:"question"`
	Impact   string `json:"impact"`
}

type OrchestratorTaskHandoff struct {
	RunID                    string                     `json:"runId"`
	TaskID                   string                     `json:"taskId"`
	OperationID              string                     `json:"operationId"`
	ChildID                  string                     `json:"childId"`
	Status                   string                     `json:"status"`
	Digest                   string                     `json:"digest,omitempty"`
	AnswersDigest            string                     `json:"answersDigest,omitempty"`
	DetailsStatus            string                     `json:"detailsStatus"`
	ContextDigest            string                     `json:"contextDigest,omitempty"`
	SourceCommit             string                     `json:"sourceCommit,omitempty"`
	SourceDirty              bool                       `json:"sourceDirty"`
	QuestionsKnown           bool                       `json:"questionsKnown"`
	PendingQuestionCount     int                        `json:"pendingQuestionCount"`
	WorkerQuestions          []OrchestratorTaskQuestion `json:"workerQuestions"`
	QuestionsOmitted         int                        `json:"questionsOmitted"`
	WorkerSummary            string                     `json:"workerSummary,omitempty"`
	SummaryOmitted           bool                       `json:"summaryOmitted"`
	WorkerDecisionCount      int                        `json:"workerDecisionCount"`
	WorkerDecisions          []TaskHandoffDecision      `json:"workerDecisions"`
	DecisionsOmitted         int                        `json:"decisionsOmitted"`
	WorkerLimitationCount    int                        `json:"workerLimitationCount"`
	WorkerLimitations        []string                   `json:"workerLimitations"`
	LimitationsOmitted       int                        `json:"limitationsOmitted"`
	WorkerReportedCheckCount int                        `json:"workerReportedCheckCount"`
	AnswerCount              int                        `json:"answerCount"`
	Answers                  []OrchestratorTaskAnswer   `json:"answers"`
	AnswersOmitted           int                        `json:"answersOmitted"`
	RecordedChecksKnown      bool                       `json:"recordedChecksKnown"`
	RecordedCheckCount       int                        `json:"recordedCheckCount"`
	RecordedCheckIDs         []string                   `json:"recordedCheckIds"`
	RecordedChecksOmitted    int                        `json:"recordedChecksOmitted"`
	Integration              *ProjectSwarmIntegration   `json:"integration,omitempty"`
}

// The caller has already authorized and captured these canonical runs. Read
// immutable receipt objects against that same snapshot, not a second run read.
// At most sixteen bounded receipt/answer pairs are read; there is no backend
// observation, history scan or mutation. Missing reports leave counts unknown.
func (s *ProjectRunStore) orchestratorHandoffs(runs []ProjectRun) *OrchestratorTaskHandoffs {
	type candidate struct {
		run ProjectRun
		op  ProjectRunOperation
	}
	candidates := []candidate{}
	result := &OrchestratorTaskHandoffs{Entries: []OrchestratorTaskHandoff{}}
	for _, run := range runs {
		for i := len(run.Operations) - 1; i >= 0; i-- {
			op := run.Operations[i]
			if op.TaskID == "" {
				continue
			}
			result.AttemptCount++
			if op.Handoff == nil {
				result.UnreportedCount++
			}
			candidates = append(candidates, candidate{run, op})
		}
	}
	if result.AttemptCount == 0 {
		return nil
	}
	priority := func(op ProjectRunOperation) int {
		if op.Handoff == nil {
			return 3
		}
		if op.Handoff.Status == "captured" && op.Handoff.Unanswered > 0 {
			return 0
		}
		if op.Handoff.Status == "captured" {
			return 1
		}
		return 2
	}
	sort.SliceStable(candidates, func(i, j int) bool { return priority(candidates[i].op) < priority(candidates[j].op) })
	if len(candidates) > orchestratorHandoffReadLimit {
		candidates = candidates[:orchestratorHandoffReadLimit]
	}
	views := make([]TaskHandoffView, len(candidates))
	for i, c := range candidates {
		e := OrchestratorTaskHandoff{RunID: c.run.ID, TaskID: c.op.TaskID, OperationID: c.op.ID, ChildID: c.op.ChildID, Status: "not-recorded", DetailsStatus: "not-recorded", WorkerQuestions: []OrchestratorTaskQuestion{}, WorkerDecisions: []TaskHandoffDecision{}, WorkerLimitations: []string{}, Answers: []OrchestratorTaskAnswer{}, RecordedCheckIDs: []string{}}
		if c.op.Handoff != nil {
			e.Status, e.Digest, e.AnswersDigest = c.op.Handoff.Status, c.op.Handoff.Digest, c.op.Handoff.AnswersDigest
			e.DetailsStatus = "unavailable"
			v, err := s.taskHandoffView(c.run, c.op)
			if err == nil {
				views[i] = v
				e.DetailsStatus, e.Integration = "recorded", v.Integration
				e.ContextDigest, e.SourceCommit, e.SourceDirty = v.Receipt.ContextDigest, v.Receipt.SourceCommit, v.Receipt.SourceDirty
				e.RecordedChecksKnown, e.RecordedCheckCount = true, len(v.RecordedChecks)
				e.RecordedChecksOmitted = e.RecordedCheckCount
				if r := v.Receipt.Reported; r != nil {
					e.QuestionsKnown = true
					e.PendingQuestionCount = len(v.QuestionIDs) - len(v.Answers)
					e.QuestionsOmitted = e.PendingQuestionCount
					e.AnswerCount, e.AnswersOmitted = len(v.Answers), len(v.Answers)
					e.SummaryOmitted = r.Summary != ""
					e.WorkerDecisionCount, e.DecisionsOmitted = len(r.Decisions), len(r.Decisions)
					e.WorkerLimitationCount, e.LimitationsOmitted = len(r.Limitations), len(r.Limitations)
					e.WorkerReportedCheckCount = len(r.ReportedChecks)
				}
			}
		}
		result.Entries = append(result.Entries, e)
	}
	result.OmittedCount = result.AttemptCount - len(result.Entries)
	for len(result.Entries) > 0 {
		encoded, _ := json.MarshalIndent(result, "", "  ")
		if len(encoded) <= orchestratorHandoffLimit {
			break
		}
		result.Entries = result.Entries[:len(result.Entries)-1]
		result.OmittedCount++
	}
	views = views[:len(result.Entries)]
	// Base rows reserve identity/availability for every selected attempt first.
	// Whole fields are retained or explicitly omitted; no silent text clipping.
	add := func(i int, change func(*OrchestratorTaskHandoff)) bool {
		previous := result.Entries[i]
		change(&result.Entries[i])
		encoded, _ := json.MarshalIndent(result, "", "  ")
		if len(encoded) > orchestratorHandoffLimit {
			result.Entries[i] = previous
			return false
		}
		return true
	}
	for i, v := range views {
		if v.Receipt == nil || v.Receipt.Reported == nil {
			continue
		}
		answered := map[string]bool{}
		for _, answer := range v.Answers {
			answered[answer.QuestionID] = true
		}
		for j, q := range v.Receipt.Reported.Questions {
			if answered[v.QuestionIDs[j]] {
				continue
			}
			if !add(i, func(e *OrchestratorTaskHandoff) {
				e.WorkerQuestions = append(e.WorkerQuestions, OrchestratorTaskQuestion{v.QuestionIDs[j], q.Question, q.Impact, q.Blocking})
				e.QuestionsOmitted--
			}) {
				break
			}
		}
	}
	for i, v := range views {
		if v.Receipt == nil {
			continue
		}
		for _, check := range v.RecordedChecks {
			if !add(i, func(e *OrchestratorTaskHandoff) {
				e.RecordedCheckIDs = append(e.RecordedCheckIDs, check.CheckID)
				e.RecordedChecksOmitted--
			}) {
				break
			}
		}
		if r := v.Receipt.Reported; r != nil {
			add(i, func(e *OrchestratorTaskHandoff) { e.WorkerSummary = r.Summary; e.SummaryOmitted = false })
			questions := make(map[string]TaskHandoffQuestion, len(v.QuestionIDs))
			for j, question := range r.Questions {
				questions[v.QuestionIDs[j]] = question
			}
			for _, a := range v.Answers {
				question, ok := questions[a.QuestionID]
				if !ok {
					continue
				} // Never emit an answer stripped of its meaning.
				answer := OrchestratorTaskAnswer{TaskHandoffAnswer: a, Question: question.Question, Impact: question.Impact}
				if !add(i, func(e *OrchestratorTaskHandoff) { e.Answers = append(e.Answers, answer); e.AnswersOmitted-- }) {
					break
				}
			}
			for _, d := range r.Decisions {
				if !add(i, func(e *OrchestratorTaskHandoff) {
					e.WorkerDecisions = append(e.WorkerDecisions, d)
					e.DecisionsOmitted--
				}) {
					break
				}
			}
			for _, l := range r.Limitations {
				if !add(i, func(e *OrchestratorTaskHandoff) {
					e.WorkerLimitations = append(e.WorkerLimitations, l)
					e.LimitationsOmitted--
				}) {
					break
				}
			}
		}
	}
	return result
}
