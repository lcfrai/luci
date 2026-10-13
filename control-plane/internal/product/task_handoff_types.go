package product

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode/utf8"
)

const TaskHandoffReportLimit = 64 << 10
const TaskDependencyPacketLimit = 8 << 20

// Every field here is a worker claim. No outcome authorizes execution,
// integration, requirement acceptance, or a join into recorded check evidence.
type TaskHandoffReport struct {
	Version        int                        `json:"version"`
	Summary        string                     `json:"summary"`
	ChangedFiles   []string                   `json:"changedFiles"`
	Decisions      []TaskHandoffDecision      `json:"decisions"`
	ReportedChecks []TaskHandoffReportedCheck `json:"reportedChecks"`
	Questions      []TaskHandoffQuestion      `json:"questions"`
	Limitations    []string                   `json:"limitations"`
}
type TaskHandoffDecision struct {
	Decision  string `json:"decision"`
	Rationale string `json:"rationale"`
}
type TaskHandoffReportedCheck struct {
	Command string `json:"command"`
	Outcome string `json:"outcome"`
	Detail  string `json:"detail"`
}
type TaskHandoffQuestion struct {
	Question string `json:"question"`
	Impact   string `json:"impact"`
	Blocking bool   `json:"blocking"`
}
type TaskHandoffAnswer struct {
	QuestionID  string `json:"questionId"`
	Answer      string `json:"answer"`
	AnsweredAt  string `json:"answeredAt"`
	Attribution string `json:"attribution"`
}

// Small reference only: full reports never expand every run-list response.
type TaskHandoffReference struct {
	Digest        string `json:"digest"`
	Status        string `json:"status"`
	Summary       string `json:"summary"`
	Questions     int    `json:"questions"`
	Unanswered    int    `json:"unanswered"`
	AnswersDigest string `json:"answersDigest,omitempty"`
	CapturedAt    string `json:"capturedAt"`
}
type TaskDependencyReference struct {
	Digest string `json:"digest"`
	Count  int    `json:"count"`
}

// Identities are supplied by the private run, never decoded from a report.
type TaskHandoffCaptureRequest struct {
	ProjectSwarmRecoveryRequest
	TaskID, WorktreeID, RequestDigest, ImplementationDigest string
}
type TaskHandoffCapture struct {
	// Kept through source observation and receipt publication; never serialized.
	Release       func()
	Status        string
	ReasonCode    string
	ContextDigest string
	Report        *TaskHandoffReport
}

type TaskHandoffReceipt struct {
	Version              int                `json:"version"`
	ProjectID            string             `json:"projectId"`
	RunID                string             `json:"runId"`
	TaskID               string             `json:"taskId"`
	OperationID          string             `json:"operationId"`
	ChildID              string             `json:"childId"`
	WorktreeID           string             `json:"worktreeId"`
	RequestDigest        string             `json:"requestDigest"`
	CloudRequestDigest   string             `json:"cloudRequestDigest,omitempty"`
	CloudCaptureDigest   string             `json:"cloudCaptureDigest,omitempty"`
	CloudReportDigest    string             `json:"cloudReportDigest,omitempty"`
	CloudExitFrameDigest string             `json:"cloudExitFrameDigest,omitempty"`
	ImplementationDigest string             `json:"implementationDigest"`
	ContextDigest        string             `json:"contextDigest,omitempty"`
	SourceCommit         string             `json:"sourceCommit"`
	SourceDigest         string             `json:"sourceDigest"`
	SourceDirty          bool               `json:"sourceDirty"`
	Status               string             `json:"status"`
	ReasonCode           string             `json:"reasonCode,omitempty"`
	CapturedAt           string             `json:"capturedAt"`
	Reported             *TaskHandoffReport `json:"reported"`
}
type TaskHandoffView struct {
	ProjectID      string                    `json:"projectId"`
	RunID          string                    `json:"runId"`
	TaskID         string                    `json:"taskId"`
	OperationID    string                    `json:"operationId"`
	Status         string                    `json:"status"`
	Digest         string                    `json:"digest,omitempty"`
	Receipt        *TaskHandoffReceipt       `json:"receipt"`
	Answers        []TaskHandoffAnswer       `json:"answers"`
	QuestionIDs    []string                  `json:"questionIds"`
	RecordedChecks []ProjectRunCheckEvidence `json:"recordedChecks"`
	Integration    *ProjectSwarmIntegration  `json:"integration"`
}
type TaskDependencyPacket struct {
	Version         int               `json:"version"`
	ProjectID       string            `json:"projectId"`
	RunID           string            `json:"runId"`
	TaskID          string            `json:"taskId"`
	OperationID     string            `json:"operationId"`
	Dependencies    []TaskHandoffView `json:"dependencies"`
	PreviousAttempt *TaskHandoffView  `json:"previousAttempt,omitempty"`
}

func (s *Server) SetTaskHandoffCollector(authorize func(context.Context, ProjectSwarmRecoveryRequest) error, collect func(context.Context, TaskHandoffCaptureRequest) (TaskHandoffCapture, error)) {
	s.projectRunDispatchMu.Lock()
	defer s.projectRunDispatchMu.Unlock()
	s.taskHandoffAuthorize, s.taskHandoffCollector = authorize, collect
}

func handoffText(value string, max int) bool {
	return len(value) <= max && utf8.ValidString(value) && !strings.ContainsRune(value, '\x00')
}

func DecodeTaskHandoffReport(data []byte) (*TaskHandoffReport, error) {
	if len(data) == 0 || len(data) > TaskHandoffReportLimit || !utf8.Valid(data) || uniqueHandoffJSON(data) != nil {
		return nil, errors.New("The task report is invalid or exceeds its 64 KiB limit.")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	var v TaskHandoffReport
	if d.Decode(&v) != nil || d.Decode(new(any)) != io.EOF || v.Version != 1 || !handoffText(v.Summary, 4000) || len(v.ChangedFiles) > 128 || len(v.Decisions) > 8 || len(v.ReportedChecks) > 16 || len(v.Questions) > 8 || len(v.Limitations) > 16 {
		return nil, errors.New("The task report does not match the bounded version 1 schema.")
	}
	if v.ChangedFiles == nil || v.Decisions == nil || v.ReportedChecks == nil || v.Questions == nil || v.Limitations == nil {
		return nil, errors.New("Use explicit arrays for changedFiles, decisions, reportedChecks, questions and limitations; missing or null fields do not establish an empty result.")
	}
	for _, path := range v.ChangedFiles {
		if !validProjectTaskWritePath(path) {
			return nil, errors.New("The task report contains an invalid project-relative path.")
		}
	}
	for _, x := range v.Decisions {
		if !handoffText(x.Decision, 2000) || !handoffText(x.Rationale, 2000) {
			return nil, errors.New("A reported decision exceeds its text limit.")
		}
	}
	for _, x := range v.ReportedChecks {
		if !handoffText(x.Command, 2000) || !handoffText(x.Detail, 2000) || x.Outcome != "passed" && x.Outcome != "failed" && x.Outcome != "not-run" && x.Outcome != "unknown" {
			return nil, errors.New("A reported check is invalid.")
		}
	}
	for _, x := range v.Questions {
		if strings.TrimSpace(x.Question) == "" || !handoffText(x.Question, 2000) || !handoffText(x.Impact, 2000) {
			return nil, errors.New("A reported question is invalid.")
		}
	}
	for _, x := range v.Limitations {
		if !handoffText(x, 2000) {
			return nil, errors.New("A reported limitation exceeds its text limit.")
		}
	}
	return &v, nil
}

// encoding/json otherwise accepts duplicate fields with last-value wins.
func uniqueHandoffJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	var value func(int) error
	value = func(depth int) error {
		if depth > 16 {
			return errors.New("nested report")
		}
		t, err := d.Token()
		if err != nil {
			return err
		}
		delim, ok := t.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				key, err := d.Token()
				if err != nil {
					return err
				}
				name, ok := key.(string)
				if !ok || seen[name] {
					return errors.New("duplicate report field")
				}
				seen[name] = true
				if err = value(depth + 1); err != nil {
					return err
				}
			}
		case '[':
			for d.More() {
				if err = value(depth + 1); err != nil {
					return err
				}
			}
		default:
			return errors.New("invalid report")
		}
		_, err = d.Token()
		return err
	}
	if err := value(0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return errors.New("trailing report")
	}
	return nil
}
