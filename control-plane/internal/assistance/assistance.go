// Package assistance holds bounded, durable definition-assistant jobs. It has
// no model credentials and never applies a proposal to a project.
package assistance

import (
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"sync"
	"time"
)

const MaxResultBytes = 256 << 10

type Result struct {
	Question  string          `json:"question"`
	Options   []string        `json:"options"`
	Rationale string          `json:"rationale"`
	Proposal  json.RawMessage `json:"proposal"`
}

type Job struct {
	ID                    string  `json:"id"`
	ProjectID             string  `json:"projectId"`
	ExpectedRevision      int     `json:"expectedRevision"`
	ExpectedBriefRevision int     `json:"expectedBriefRevision"`
	SessionID             string  `json:"sessionId"`
	Status                string  `json:"status"`
	Message               string  `json:"message"`
	CreatedAt             string  `json:"createdAt"`
	UpdatedAt             string  `json:"updatedAt"`
	Result                *Result `json:"result,omitempty"`
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
	if len(b) > MaxResultBytes+8192 || json.Unmarshal(b, &job) != nil || job.ID != id {
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
func schemaFor(t reflect.Type) map[string]any {
	switch t.Kind() {
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
			properties[name] = schemaFor(f.Type)
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
const Launcher = `import json, os, subprocess, sys
with open('/luci-input/prompt.txt', 'rb') as source:
    prompt = source.read(262145)
if len(prompt) > 262144:
    sys.exit(2)
env = dict(os.environ)
for key in ('OPENAI_API_KEY', 'CODEX_API_KEY'):
    env.pop(key, None)
command = ['codex', '--no-daemon', '-c', 'forced_login_method="chatgpt"', '-c', 'model_provider="openai"', '-c', 'cli_auth_credentials_store="file"', '-c', 'features.shell_tool=false', '-c', 'features.shell_snapshot=false', '-c', 'features.apps=false', '-c', 'features.remote_plugin=false', '-c', 'features.multi_agent=false', '-c', 'features.hooks=false', '-c', 'features.goals=false', '-c', 'web_search="disabled"', '--ask-for-approval', 'never', 'exec', '--skip-git-repo-check', '--sandbox', 'read-only', '--output-schema', '/luci-input/schema.json', '--output-last-message', '/luci-output/result.json', '-']
completed = subprocess.run(command, input=prompt, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, env=env)
with open('/luci-output/exit.json.tmp', 'w') as target:
    json.dump({'exitCode': completed.returncode}, target)
os.replace('/luci-output/exit.json.tmp', '/luci-output/exit.json')
sys.exit(completed.returncode)
`
