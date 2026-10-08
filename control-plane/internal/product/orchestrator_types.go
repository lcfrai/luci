package product

import "errors"

var (
	ErrOrchestratorConflict = errors.New("The conversation or project context changed. Reload before continuing; your message is still a draft.")
	ErrOrchestratorInvalid  = errors.New("The private project conversation is unavailable or invalid.")
	ErrOrchestratorBusy     = errors.New("An earlier Luci turn is still running or unconfirmed. Observe or cancel it before starting another.")
	ErrOrchestratorCapacity = errors.New("This private conversation reached its storage limit. Download the history and use Start a continued discussion in Conversation history and storage. Unanswered material questions and pending amendment receipts are retained.")
	ErrOrchestratorQuestion = errors.New("Answer Luci's unresolved material question before starting new implementation work.")
)

type OrchestratorRunTask struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}
type OrchestratorRunOperation struct {
	ID        string `json:"id"`
	ChildID   string `json:"childId"`
	Status    string `json:"status"`
	UpdatedAt string `json:"updatedAt"`
}

type OrchestratorRunSummary struct {
	// Older immutable conversation snapshots may predate this projection.
	ContextFreshness      *ProjectRunContextFreshness `json:"contextFreshness,omitempty"`
	OperationCount        int                         `json:"operationCount"`
	OperationsOmitted     bool                        `json:"operationsOmitted"`
	ScopeOmitted          bool                        `json:"scopeOmitted"`
	TaskCount             int                         `json:"taskCount"`
	RequirementCount      int                         `json:"requirementCount"`
	Activity              string                      `json:"activity"`
	ActivityReason        string                      `json:"activityReason"`
	DispatchCancelled     bool                        `json:"dispatchCancelled"`
	Operations            []OrchestratorRunOperation  `json:"operations"`
	Tasks                 []OrchestratorRunTask       `json:"tasks"`
	Requirements          []RequirementSnapshot       `json:"requirements"`
	SpecificationRevision int                         `json:"specificationRevision"`
	DesignRevision        int                         `json:"designRevision"`
	Outcome               string                      `json:"outcome"`
	ID                    string                      `json:"id"`
	State                 string                      `json:"state"`
	Valid                 bool                        `json:"valid"`
	Reason                string                      `json:"reason"`
}
type OrchestratorContext struct {
	// Optional to preserve earlier immutable snapshot digests.
	TaskHandoffs *OrchestratorTaskHandoffs `json:"taskHandoffs,omitempty"`
	// Recorded review obligations are distinct from unfinished execution.
	NeedsAttention  []ProjectRunAttention    `json:"needsAttention,omitempty"`
	ProjectID       string                   `json:"projectId"`
	Digest          string                   `json:"digest"`
	Brief           Brief                    `json:"brief"`
	Specification   ProjectSpecification     `json:"specification"`
	DesignPlan      ProjectDesignPlan        `json:"designPlan"`
	Plan            ProjectPlan              `json:"plan"`
	SketchRevision  int                      `json:"sketchRevision"`
	SourceCommit    string                   `json:"sourceCommit"`
	SourceDirty     bool                     `json:"sourceDirty"`
	SourceKnown     bool                     `json:"sourceKnown"`
	CurrentRun      *OrchestratorRunSummary  `json:"currentRun,omitempty"`
	ActiveRuns      []OrchestratorRunSummary `json:"activeRuns"`
	ChecksSummary   string                   `json:"checksSummary"`
	ReleasesSummary string                   `json:"releasesSummary"`
}
type OrchestratorProposal struct {
	Target        string                `json:"target"`
	Summary       string                `json:"summary"`
	Specification *ProjectSpecification `json:"specification"`
	DesignPlan    *ProjectDesignPlan    `json:"designPlan"`
	Plan          *ProjectPlan          `json:"plan"`
}
type OrchestratorAnswer struct {
	Summary    string                `json:"summary"`
	Question   string                `json:"question"`
	Options    []string              `json:"options"`
	Rationale  string                `json:"rationale"`
	Blocking   bool                  `json:"blocking"`
	Impact     []string              `json:"impact"`
	Proposal   *OrchestratorProposal `json:"proposal"`
	NextAction string                `json:"nextAction"`
}
type OrchestratorTurnRequest struct {
	ExpectedRevision      int    `json:"expectedRevision"`
	ExpectedContextDigest string `json:"expectedContextDigest"`
	IdempotencyKey        string `json:"idempotencyKey"`
	Message               string `json:"message"`
	ReplyToTurnID         string `json:"replyToTurnId,omitempty"`
	RouteDigest           string `json:"-"`
}
type OrchestratorTurn struct {
	ID                 string              `json:"id"`
	ProjectID          string              `json:"projectId"`
	Sequence           int                 `json:"sequence"`
	Actor              ContextActor        `json:"actor"`
	Message            string              `json:"message"`
	JobID              string              `json:"jobId"`
	ReplyToTurnID      string              `json:"replyToTurnId,omitempty"`
	ContextDigest      string              `json:"contextDigest"`
	Context            OrchestratorContext `json:"context"`
	Status             string              `json:"status"`
	CreatedAt          string              `json:"createdAt"`
	UpdatedAt          string              `json:"updatedAt"`
	Answer             *OrchestratorAnswer `json:"answer,omitempty"`
	AppliedRevision    int                 `json:"appliedRevision,omitempty"`
	QuestionResolvedBy string              `json:"questionResolvedBy,omitempty"`
}
type OrchestratorQuestion struct {
	TurnID    string   `json:"turnId"`
	Question  string   `json:"question"`
	Rationale string   `json:"rationale"`
	Options   []string `json:"options"`
}
type OrchestratorAction struct {
	RunID  string `json:"runId,omitempty"`
	ID     string `json:"id"`
	Label  string `json:"label"`
	Reason string `json:"reason"`
	Lens   string `json:"lens"`
}
type OrchestratorView struct {
	StorageDigest     string                 `json:"storageDigest"`
	ArchivePresent    bool                   `json:"archivePresent"`
	ProjectID         string                 `json:"projectId"`
	Revision          int                    `json:"revision"`
	Context           OrchestratorContext    `json:"context"`
	Turns             []OrchestratorTurn     `json:"turns"`
	NextAction        OrchestratorAction     `json:"nextAction"`
	BlockingQuestions []OrchestratorQuestion `json:"blockingQuestions"`
	HasEarlier        bool                   `json:"hasEarlier"`
	BeforeSequence    int                    `json:"beforeSequence"`
}
