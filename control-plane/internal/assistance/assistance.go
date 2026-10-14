// Package assistance holds bounded, durable project-assistant jobs. It has
// no model credentials and never applies a proposal to a project.
package assistance

import (
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
)

const MaxResultBytes = 256 << 10

// Cross-phase jobs retain one immutable context snapshot as well as the result.
const MaxJobBytes = 2*MaxResultBytes + (64 << 10)

type SourceFile struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
	Bytes  int    `json:"bytes"`
}
type Source struct {
	Mode          string       `json:"mode"`
	Commit        string       `json:"commit,omitempty"`
	Digest        string       `json:"digest"`
	FileCount     int          `json:"fileCount"`
	IncludedBytes int          `json:"includedBytes"`
	SkippedFiles  int          `json:"skippedFiles"`
	Partial       bool         `json:"partial"`
	Limitation    string       `json:"limitation"`
	Files         []SourceFile `json:"files"`
}
type Finding struct {
	Kind    string   `json:"kind"`
	Summary string   `json:"summary"`
	Paths   []string `json:"paths"`
}

type Result struct {
	Question  string          `json:"question"`
	Options   []string        `json:"options"`
	Rationale string          `json:"rationale"`
	Proposal  json.RawMessage `json:"proposal"`
	Findings  []Finding       `json:"findings,omitempty"`
}

type Job struct {
	ID                      string          `json:"id"`
	ProjectID               string          `json:"projectId"`
	ExpectedRevision        int             `json:"expectedRevision"`
	ExpectedBriefRevision   int             `json:"expectedBriefRevision"`
	SessionID               string          `json:"sessionId"`
	Status                  string          `json:"status"`
	Message                 string          `json:"message"`
	FailureCode             string          `json:"failureCode,omitempty"`
	CreatedAt               string          `json:"createdAt"`
	UpdatedAt               string          `json:"updatedAt"`
	Result                  *Result         `json:"result,omitempty"`
	Kind                    string          `json:"kind,omitempty"`
	Harness                 string          `json:"harness,omitempty"`
	Model                   string          `json:"model,omitempty"`
	AuthMode                string          `json:"authMode,omitempty"`
	AgentConnectionID       string          `json:"agentConnectionId,omitempty"`
	AgentConnectionRevision int             `json:"agentConnectionRevision,omitempty"`
	AgentConnectionKind     string          `json:"agentConnectionKind,omitempty"`
	AgentConnectionProtocol string          `json:"agentConnectionProtocol,omitempty"`
	Source                  *Source         `json:"source,omitempty"`
	AppliedRevision         int             `json:"appliedRevision,omitempty"`
	RequestedBy             string          `json:"requestedBy,omitempty"`
	IdempotencyKey          string          `json:"idempotencyKey,omitempty"`
	RequestDigest           string          `json:"requestDigest,omitempty"`
	ReplyToAnalysisID       string          `json:"replyToAnalysisId,omitempty"`
	ReplySourceChanged      bool            `json:"replySourceChanged,omitempty"`
	UserMessage             string          `json:"userMessage,omitempty"`
	OrchestratorTurnID      string          `json:"orchestratorTurnId,omitempty"`
	OrchestratorContext     json.RawMessage `json:"orchestratorContext,omitempty"`
	OrchestratorResult      json.RawMessage `json:"orchestratorResult,omitempty"`
}

// Store serializes changes and persists before returning. The caller supplies
// its existing private, no-link file boundary, including Windows ACL handling.
type Store struct {
	mu    sync.Mutex
	read  func(string) ([]byte, error)
	write func(string, []byte) error
}

func NewStore(read func(string) ([]byte, error), write func(string, []byte) error) *Store {
	return &Store{read: read, write: write}
}

func (s *Store) Read(id string) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readJob(id)
}
func (s *Store) readJob(id string) (Job, error) {
	var job Job
	b, err := s.read(id)
	if err != nil {
		return job, err
	}
	if len(b) > MaxJobBytes || json.Unmarshal(b, &job) != nil || job.ID != id {
		return Job{}, errors.New("invalid assistant job")
	}
	return job, nil
}
func (s *Store) Create(job Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	job.UpdatedAt = job.CreatedAt
	b, err := json.Marshal(job)
	if err != nil {
		return err
	}
	if len(b) > MaxJobBytes {
		return errors.New("assistant job exceeds its limit")
	}
	return s.write(job.ID, b)
}
func (s *Store) Update(id string, mutate func(*Job) error) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, err := s.readJob(id)
	if err != nil {
		return job, err
	}
	if err = mutate(&job); err != nil {
		return Job{}, err
	}
	job.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	b, err := json.Marshal(job)
	if err == nil && len(b) > MaxJobBytes {
		err = errors.New("assistant job exceeds its limit")
	}
	if err == nil {
		err = s.write(id, b)
	}
	return job, err
}
func Terminal(status string) bool {
	return status == "succeeded" || status == "failed" || status == "cancelled"
}

func DecodeResult(data []byte) (Result, error) {
	var result Result
	if len(data) > MaxResultBytes {
		return result, errors.New("assistant result exceeds its limit")
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return result, err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return Result{}, errors.New("trailing assistant result")
	}
	if len(result.Question) > 4000 || len(result.Rationale) > 8000 || len(result.Options) > 6 || len(result.Proposal) == 0 {
		return Result{}, errors.New("invalid assistant result")
	}
	for _, option := range result.Options {
		if strings.TrimSpace(option) == "" || len(option) > 1000 {
			return Result{}, errors.New("invalid assistant option")
		}
	}
	if len(result.Findings) > 40 {
		return Result{}, errors.New("too many repository findings")
	}
	for _, finding := range result.Findings {
		if (finding.Kind != "observed" && finding.Kind != "inference" && finding.Kind != "gap") || strings.TrimSpace(finding.Summary) == "" || len(finding.Summary) > 4000 || len(finding.Paths) > 12 {
			return Result{}, errors.New("invalid repository finding")
		}
	}
	if strings.TrimSpace(result.Question) == "" && string(result.Proposal) == "null" {
		return Result{}, errors.New("assistant supplied neither a question nor a proposal")
	}
	if result.Options == nil {
		result.Options = []string{}
	}
	return result, nil
}

// Schema derives field spelling from the shared specification type. Every
// property is required and extra fields are rejected, as Codex strict output
// schemas require. Proposal provenance is still validated by the API.
func Schema(specification any) ([]byte, error) {
	proposal := schemaFor(reflect.TypeOf(specification))
	return json.Marshal(map[string]any{"type": "object", "additionalProperties": false, "required": []string{"question", "options", "rationale", "proposal"}, "properties": map[string]any{
		"question": map[string]any{"type": "string"}, "options": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "rationale": map[string]any{"type": "string"}, "proposal": map[string]any{"anyOf": []any{proposal, map[string]any{"type": "null"}}},
	}})
}

func RepositorySchema(specification any) ([]byte, error) {
	b, err := Schema(specification)
	if err != nil {
		return nil, err
	}
	var schema map[string]any
	if err = json.Unmarshal(b, &schema); err != nil {
		return nil, err
	}
	schema["required"] = []string{"question", "options", "rationale", "proposal", "findings"}
	schema["properties"].(map[string]any)["findings"] = schemaFor(reflect.TypeOf([]Finding{}))
	return json.Marshal(schema)
}

// StrictSchema derives a complete structured-output contract, including nullable
// pointer fields. It describes proposals without granting authority to apply them.
func StrictSchema(value any) ([]byte, error) {
	if value == nil {
		return nil, errors.New("assistant schema requires a concrete type")
	}
	return json.Marshal(schemaFor(reflect.TypeOf(value)))
}

func schemaFor(t reflect.Type) map[string]any {
	switch t.Kind() {
	case reflect.Pointer:
		return map[string]any{"anyOf": []any{schemaFor(t.Elem()), map[string]any{"type": "null"}}}
	case reflect.Struct:
		properties := map[string]any{}
		required := []string{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			name := strings.Split(f.Tag.Get("json"), ",")[0]
			if name == "-" {
				continue
			}
			if name == "" {
				name = f.Name
			}
			fieldSchema := schemaFor(f.Type)
			if values := f.Tag.Get("enum"); f.Type.Kind() == reflect.String && values != "" {
				fieldSchema["enum"] = strings.Split(values, ",")
			}
			if f.Type.Kind() == reflect.String && f.Tag.Get("output") == "empty" {
				fieldSchema["enum"] = []string{""}
			}
			if minimum, err := strconv.Atoi(f.Tag.Get("modelMinLength")); f.Type.Kind() == reflect.String && err == nil && minimum > 0 && minimum <= 1000 {
				fieldSchema["minLength"] = minimum
			}
			properties[name] = fieldSchema
			required = append(required, name)
		}
		return map[string]any{"type": "object", "additionalProperties": false, "properties": properties, "required": required}
	case reflect.Slice:
		return map[string]any{"type": "array", "items": schemaFor(t.Elem())}
	case reflect.Int, reflect.Int64:
		return map[string]any{"type": "integer"}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	default:
		return map[string]any{"type": "string"}
	}
}

// Launcher is trusted source, mounted read-only. Prompt data reaches stdin,
// never shell interpolation, process arguments, container labels or logs.
// Only a bounded final response and a numeric exit status cross back to Luci.
const Launcher = RepositoryLauncher
