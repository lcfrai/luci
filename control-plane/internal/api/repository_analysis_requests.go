package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/assistance"
	"github.com/lcfrai/luci/control-plane/internal/httpx"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

type repositoryAnalysisRequest struct {
	ExpectedRevision                int     `json:"expectedRevision"`
	ExpectedBriefRevision           int     `json:"expectedBriefRevision"`
	SourceMode                      string  `json:"sourceMode"`
	Harness                         string  `json:"harness"`
	Model                           string  `json:"model"`
	AuthMode                        string  `json:"authMode"`
	AgentConnectionID               *string `json:"agentConnectionId"`
	ExpectedAgentConnectionRevision *int    `json:"expectedAgentConnectionRevision,omitempty"`
	AcknowledgeSourceRead           bool    `json:"acknowledgeSourceRead"`
	Message                         string  `json:"message"`
	ReplyToAnalysisID               string  `json:"replyToAnalysisId"`
	IdempotencyKey                  string  `json:"idempotencyKey"`
}

func validAnalysisRequestKey(key string) bool {
	if len(key) < 16 || len(key) > 128 {
		return false
	}
	for _, ch := range key {
		if !(ch >= 'a' && ch <= 'z') && !(ch >= 'A' && ch <= 'Z') && !(ch >= '0' && ch <= '9') && ch != '-' && ch != '_' {
			return false
		}
	}
	return true
}

// The key namespace includes the authenticated actor, owned project and the
// private worker root (by storage location). No user-supplied job path is used.
func repositoryRequestJobID(actor, project, key string) string {
	b, _ := json.Marshal([]string{actor, project, key})
	digest := sha256.Sum256(b)
	return "assist-request-" + hex.EncodeToString(digest[:24])
}

func repositoryRequestDigest(req repositoryAnalysisRequest) string {
	b, _ := json.Marshal(req)
	digest := sha256.Sum256(b)
	return hex.EncodeToString(digest[:])
}

func repositoryAnalysisActor(r *http.Request) string {
	actor, _, ok := management.ActorFromContext(r.Context())
	if !ok || actor.Disabled || actor.ID == "" {
		return ""
	}
	return actor.ID
}

func validRepositoryRequestJob(job assistance.Job, actor, project, key string) bool {
	return job.ID == repositoryRequestJobID(actor, project, key) && job.Kind == "repository" && job.ProjectID == project && job.RequestedBy == actor && job.IdempotencyKey == key && len(job.RequestDigest) == 64
}

// The browser's local login and a selected cloud workspace may be different
// accounts. Attribute ownership using the authenticated worker actor, never a
// client comparison against its local identity. This projection is not stored.
func assistanceJobProjection(r *http.Request, job assistance.Job) any {
	if job.Kind != "repository" {
		return job
	}
	actor := repositoryAnalysisActor(r)
	return struct {
		assistance.Job
		OwnRequest bool `json:"ownRequest"`
	}{Job: job, OwnRequest: actor != "" && actor == job.RequestedBy}
}

func writeAssistanceJob(w http.ResponseWriter, r *http.Request, status int, job assistance.Job) {
	httpx.WriteJSON(w, status, assistanceJobProjection(r, job))
}

var errRepositoryAdmissionClosed = errors.New("repository analysis admission closed")

func (a *API) returnClosedRepositoryAnalysis(w http.ResponseWriter, r *http.Request, id string) {
	job, err := a.assistanceStore().Read(id)
	if err == nil && assistance.Terminal(job.Status) {
		writeAssistanceJob(w, r, 200, job)
		return
	}
	httpx.WriteProblem(w, 503, "analysis_request_unavailable", "The request was not dispatched, but its final receipt could not be confirmed. Keep the original key and check its status.", nil)
}

func (a *API) failReservedRepositoryAnalysis(id string, reason ...string) {
	_, _ = a.assistanceStore().Update(id, func(job *assistance.Job) error {
		if job.Kind != "repository" || job.Status != "reserved" {
			return errors.New("analysis is no longer reserved")
		}
		job.Status = "failed"
		job.Message = "This request was interrupted before inference began. No analysis was started. Review your retained answer before explicitly starting a new request."
		if len(reason) > 0 {
			job.Message = reason[0]
		}
		return nil
	})
}

// A status read never allocates, repairs, resolves a provider, collects source,
// or starts inference. Missing does not establish that a delayed POST is absent.
func (a *API) handleRepositoryAnalysisRequest(w http.ResponseWriter, r *http.Request) {
	actor, project, key := repositoryAnalysisActor(r), r.PathValue("id"), r.PathValue("requestId")
	if actor == "" {
		httpx.WriteProblem(w, 403, "analysis_actor_required", "Sign in again before recovering your repository analysis request.", nil)
		return
	}
	if a.deps.Product == nil || !validAnalysisRequestKey(key) {
		httpx.WriteProblem(w, 404, "analysis_request_not_found", "No recorded request was found in this account and project. A delayed request may still arrive; retry only the exact original request.", nil)
		return
	}
	if _, err := a.deps.Product.ResolveProject(project); err != nil {
		httpx.WriteProblem(w, 404, "project_not_found", "No project in your workspace has that id.", nil)
		return
	}
	job, err := a.assistanceStore().Read(repositoryRequestJobID(actor, project, key))
	if errors.Is(err, os.ErrNotExist) {
		httpx.WriteProblem(w, 404, "analysis_request_not_found", "No recorded request was found in this account and project. A delayed request may still arrive; retry only the exact original request.", nil)
		return
	}
	if err != nil || !validRepositoryRequestJob(job, actor, project, key) {
		httpx.WriteProblem(w, 503, "analysis_request_unavailable", "The original request record cannot be verified. Keep its identity and restore private state or contact the host operator; do not start a replacement request.", nil)
		return
	}
	writeAssistanceJob(w, r, 200, job)
}

// Jobs are durable request receipts, including after conversation archival.
// Do not prune them or recycle keys to admit more work: that could replay paid
// requests. A bounded listing and fail-closed capacity precede new admission.
func (a *API) repositoryAnalysisCapacity() error {
	dir := filepath.Join(a.deps.ConversationsDir, "assistance", "jobs")
	if _, err := os.Lstat(dir); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err := codexRegularPath(a.deps.ConversationsDir, dir); err != nil {
		return err
	}
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer f.Close()
	entries, err := f.ReadDir(4096)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	if len(entries) >= 4096 {
		return errors.New("Repository analysis storage has reached its 4096-receipt limit. Continue editing the definition manually; existing requests remain recoverable. Do not delete their receipts to retry paid work.")
	}
	return nil
}

func (a *API) repositoryReply(jobID, actor, project string) (assistance.Job, error) {
	job, err := a.assistanceStore().Read(jobID)
	if err != nil || job.Kind != "repository" || job.ProjectID != project || job.RequestedBy == "" || job.RequestedBy != actor || job.Status != "succeeded" || job.Source == nil || job.Result == nil || strings.TrimSpace(job.Result.Question) == "" {
		return assistance.Job{}, errors.New("This question cannot be continued by the current account. Choose your own completed analysis, or start a fresh analysis of the current source.")
	}
	// Reject damaged private results rather than treating their claims as input.
	encoded, marshalErr := json.Marshal(job.Result)
	_, decodeErr := assistance.DecodeResult(encoded)
	if marshalErr != nil || decodeErr != nil || a.validateAssistProposal(job, job.Result) != nil {
		return assistance.Job{}, errors.New("The saved analysis cannot be verified. Start a fresh analysis of the current source.")
	}
	return job, nil
}

// Retain earlier human decisions when several question-only responses precede
// a proposal. Every ancestor comes from the same private actor/project history;
// client text cannot inject an earlier result or claim another person's answer.
func (a *API) repositoryFollowUp(previous assistance.Job, actor, project string, source assistance.Source) (*product.RepositoryAnalysisFollowUp, error) {
	view := func(job assistance.Job) product.RepositoryAnalysisFollowUp {
		return product.RepositoryAnalysisFollowUp{AnalysisID: job.ID, Source: *job.Source, Result: *job.Result, UserMessage: job.UserMessage, SourceChanged: job.Source.Digest != source.Digest || job.Source.Mode != source.Mode}
	}
	follow := view(previous)
	seen := map[string]bool{previous.ID: true}
	for id := previous.ReplyToAnalysisID; id != ""; {
		if seen[id] || len(seen) >= 16 {
			return nil, errors.New("This clarification chain has reached its safe review limit or cannot be verified. Save the agreed decisions in the definition and start a fresh analysis; the saved history remains available.")
		}
		seen[id] = true
		job, err := a.repositoryReply(id, actor, project)
		if err != nil {
			return nil, err
		}
		follow.Earlier = append(follow.Earlier, view(job))
		id = job.ReplyToAnalysisID
	}
	// Chronological earlier exchanges, followed by the direct parent and the
	// new top-level userMessage, make the answer's question unambiguous.
	for i, j := 0, len(follow.Earlier)-1; i < j; i, j = i+1, j-1 {
		follow.Earlier[i], follow.Earlier[j] = follow.Earlier[j], follow.Earlier[i]
	}
	return &follow, nil
}
