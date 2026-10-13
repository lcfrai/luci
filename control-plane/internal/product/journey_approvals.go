package product

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/management"
)

const approvalStoreLimit = 16 << 10

var errApprovalStoreInvalid = errors.New("private review authority is invalid")

// Only the latest private selections confer authority. Revoking a selection on
// Save prevents replay of an older, genuinely reviewed project snapshot.
type journeyApprovalStore struct {
	Version           int                     `json:"version"`
	AccountScope      string                  `json:"accountScope"`
	ProjectID         string                  `json:"projectId"`
	ProjectScope      string                  `json:"projectScope"`
	Specification     *journeyApprovalReceipt `json:"specification,omitempty"`
	Design            *journeyApprovalReceipt `json:"design,omitempty"`
	LastSpecification *journeyApprovalReceipt `json:"lastSpecification,omitempty"`
	LastDesign        *journeyApprovalReceipt `json:"lastDesign,omitempty"`
}

type journeyApprovalReceipt struct {
	ID                      string `json:"id"`
	Kind                    string `json:"kind"`
	Revision                int    `json:"revision"`
	ContentDigest           string `json:"contentDigest"`
	BriefRevision           int    `json:"briefRevision"`
	BriefDigest             string `json:"briefDigest"`
	SpecificationRevision   int    `json:"specificationRevision,omitempty"`
	SpecificationDigest     string `json:"specificationDigest,omitempty"`
	SpecificationApproval   string `json:"specificationApproval,omitempty"`
	SketchRevision          int    `json:"sketchRevision,omitempty"`
	SketchDigest            string `json:"sketchDigest,omitempty"`
	ActorID                 string `json:"actorId"`
	DelegatedPolicyRevision int    `json:"delegatedPolicyRevision,omitempty"`
	ApprovedAt              string `json:"approvedAt"`
}

func reviewDigest(value any) string {
	data, _ := json.Marshal(value)
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func (s *Server) approvalScopes(dir, id string) (journeyApprovalStore, error) {
	// Server.New and every configuration update enforce stateOutsideProjects.
	// Callers hold s.mu while approving/saving; never recursively acquire it
	// through projectRoot here. Validate the concrete project and private roots.
	if !prototypeReleaseID.MatchString(id) || stateOutsideProjects(dir, s.stateDir) != nil {
		return journeyApprovalStore{}, errors.New("reviews require private account state")
	}
	state, err := guardedProjectPath(s.stateDir, ".", false)
	if err != nil {
		return journeyApprovalStore{}, err
	}
	project, err := guardedProjectPath(dir, ".", false)
	if err != nil {
		return journeyApprovalStore{}, err
	}
	// Real normalizes Windows long names/case as well as POSIX canonical paths.
	state, err = hostpath.Real(state)
	if err != nil {
		return journeyApprovalStore{}, err
	}
	project, err = hostpath.Real(project)
	if err != nil {
		return journeyApprovalStore{}, err
	}
	return journeyApprovalStore{Version: 1, AccountScope: reviewDigest(state), ProjectID: id, ProjectScope: reviewDigest(project)}, nil
}

func (s *Server) approvalPath(id string, create bool) (string, error) {
	if !prototypeReleaseID.MatchString(id) {
		return "", errors.New("reviews require private account state")
	}
	root, err := guardedProjectPath(s.stateDir, "review-approvals", true)
	if err != nil {
		return "", err
	}
	if create {
		if err = os.Mkdir(root, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return "", err
		}
	}
	root, err = guardedProjectPath(s.stateDir, "review-approvals", false)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() {
		return "", errors.New("private review directory is unavailable")
	}
	if create {
		if runtime.GOOS == "windows" {
			err = restrictPrivateFile(root)
		} else {
			err = os.Chmod(root, 0700)
		}
		if err != nil {
			return "", err
		}
	}
	return guardedProjectPath(s.stateDir, "review-approvals/"+id+".json", create)
}

func validApprovalReceipt(receipt *journeyApprovalReceipt, kind string) bool {
	if receipt == nil {
		return true
	}
	if !strings.HasPrefix(receipt.ID, "approval-") || len(receipt.ID) != 33 || !hexDigest(receipt.ContentDigest) || !hexDigest(receipt.BriefDigest) || receipt.Kind != kind || receipt.Revision <= 0 || receipt.BriefRevision < 0 || receipt.DelegatedPolicyRevision < 0 || !validConnectionToken(receipt.ActorID, 128, false) {
		return false
	}
	if _, err := time.Parse(time.RFC3339Nano, receipt.ApprovedAt); err != nil {
		return false
	}
	if kind == "specification" {
		return receipt.SpecificationRevision == 0 && receipt.SpecificationDigest == "" && receipt.SpecificationApproval == "" && receipt.SketchRevision == 0 && receipt.SketchDigest == ""
	}
	return receipt.SketchRevision >= 0 && hexDigest(receipt.SketchDigest) && (receipt.SpecificationRevision == 0 && receipt.SpecificationDigest == "" && receipt.SpecificationApproval == "" || receipt.SpecificationRevision > 0 && hexDigest(receipt.SpecificationDigest) && strings.HasPrefix(receipt.SpecificationApproval, "approval-") && len(receipt.SpecificationApproval) == 33)
}

func (s *Server) readApprovalStore(dir, id string) (journeyApprovalStore, error) {
	expected, err := s.approvalScopes(dir, id)
	if err != nil {
		return expected, err
	}
	path, err := s.approvalPath(id, false)
	if err != nil {
		return expected, err
	}
	if err := hostpath.OneName(path); err != nil {
		return expected, errors.New("private review authority must have one private file name")
	}
	file, err := os.Open(path)
	if err != nil {
		return expected, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > approvalStoreLimit {
		return expected, errApprovalStoreInvalid
	}
	data, err := io.ReadAll(io.LimitReader(file, approvalStoreLimit+1))
	if err != nil {
		return expected, err
	}
	if len(data) > approvalStoreLimit {
		return expected, errApprovalStoreInvalid
	}
	var saved journeyApprovalStore
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&saved) != nil || decoder.Decode(new(any)) != io.EOF || saved.Version != expected.Version || saved.AccountScope != expected.AccountScope || saved.ProjectID != id || saved.ProjectScope != expected.ProjectScope || !validApprovalReceipt(saved.Specification, "specification") || !validApprovalReceipt(saved.Design, "design") || !validApprovalReceipt(saved.LastSpecification, "specification") || !validApprovalReceipt(saved.LastDesign, "design") {
		return expected, errApprovalStoreInvalid
	}
	return saved, nil
}

func (s *Server) saveApprovalStore(store journeyApprovalStore) error {
	path, err := s.approvalPath(store.ProjectID, true)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(path); err == nil {
		if err := hostpath.OneName(path); err != nil {
			return errors.New("private review authority must have one private file name")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return atomicJSON(path, store)
}

// Raw project JSON cannot populate approvalAuthority. A corrupt, missing,
// copied or linked private store leaves content readable but requires review.
func (s *Server) readApprovedJourney(dir, id string) (journeyState, error) {
	state, err := readJourney(dir, id)
	if err != nil {
		return state, err
	}
	if authority, err := s.readApprovalStore(dir, id); err == nil {
		state.approvalAuthority = &authority
	}
	return state, nil
}

func specificationApprovalMatches(state journeyState) bool {
	if state.approvalAuthority == nil || state.approvalAuthority.Specification == nil || state.Specification == nil {
		return false
	}
	spec, receipt := state.Specification, state.approvalAuthority.Specification
	return spec.Status == "approved" && spec.ProjectID == state.approvalAuthority.ProjectID && spec.Revision == receipt.Revision && spec.Digest == receipt.ContentDigest && spec.Digest == specificationContentDigest(*spec) && spec.BriefRevision == state.Brief.Revision && receipt.BriefRevision == state.Brief.Revision && receipt.BriefDigest == reviewDigest(state.Brief)
}

func designApprovalMatches(state journeyState) bool {
	if state.approvalAuthority == nil || state.approvalAuthority.Design == nil || state.DesignPlan == nil {
		return false
	}
	plan, receipt := state.DesignPlan, state.approvalAuthority.Design
	if plan.Status != "approved" || plan.ProjectID != state.approvalAuthority.ProjectID || plan.Revision != receipt.Revision || plan.Digest != receipt.ContentDigest || plan.Digest != designPlanContentDigest(*plan) || plan.BriefRevision != state.Brief.Revision || receipt.BriefRevision != state.Brief.Revision || receipt.BriefDigest != reviewDigest(state.Brief) || plan.SketchRevision != state.Design.Revision || receipt.SketchRevision != state.Design.Revision || receipt.SketchDigest != reviewDigest(state.Design) {
		return false
	}
	if state.Specification == nil {
		return plan.SpecificationRevision == 0 && receipt.SpecificationRevision == 0 && receipt.SpecificationApproval == ""
	}
	return specificationApprovalMatches(state) && plan.SpecificationRevision == state.Specification.Revision && receipt.SpecificationRevision == state.Specification.Revision && receipt.SpecificationDigest == state.Specification.Digest && receipt.SpecificationApproval == state.approvalAuthority.Specification.ID
}

func (s *Server) revokeJourneyApproval(dir, id string, specification bool) error {
	store, err := s.readApprovalStore(dir, id)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil && !errors.Is(err, errApprovalStoreInvalid) {
		return err
	}
	if specification {
		if store.Specification != nil {
			store.LastSpecification = store.Specification
		}
		store.Specification = nil
	}
	if store.Design != nil {
		store.LastDesign = store.Design
	}
	store.Design = nil
	return s.saveApprovalStore(store)
}

func authenticatedReviewActor(w http.ResponseWriter, r *http.Request) (string, bool) {
	actor, _, ok := management.ActorFromContext(r.Context())
	if !ok || actor.Disabled || !validConnectionToken(actor.ID, 128, false) {
		problem(w, 401, "sign_in_required", "Sign in to explicitly approve this review.")
		return "", false
	}
	return actor.ID, true
}

// Called only from authenticated approval POSTs. A receipt is committed before
// the exact new approved snapshot is exported; a partial write cannot match an
// older snapshot and retries require another explicit review.
func (s *Server) recordJourneyApproval(ctx context.Context, dir, id, kind, actor string, state *journeyState) error {
	store, err := s.readApprovalStore(dir, id)
	if err != nil && !errors.Is(err, os.ErrNotExist) && !errors.Is(err, errApprovalStoreInvalid) {
		return err
	}
	receipt := &journeyApprovalReceipt{ID: newID("approval-"), Kind: kind, BriefRevision: state.Brief.Revision, BriefDigest: reviewDigest(state.Brief), ActorID: actor, ApprovedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	if gate, ok := ctx.Value(coordinatorDelegatedReviewKey{}).(coordinatorDelegatedReviewGate); ok {
		receipt.DelegatedPolicyRevision = gate.PolicyRevision
	}
	switch kind {
	case "specification":
		receipt.Revision, receipt.ContentDigest = state.Specification.Revision, state.Specification.Digest
		store.Specification, store.Design = receipt, nil
	case "design":
		receipt.Revision, receipt.ContentDigest = state.DesignPlan.Revision, state.DesignPlan.Digest
		receipt.SketchRevision, receipt.SketchDigest = state.Design.Revision, reviewDigest(state.Design)
		if state.Specification != nil {
			if !specificationApprovalMatches(*state) || store.Specification == nil || store.Specification.ID != state.approvalAuthority.Specification.ID {
				return errors.New("approve the current specification before its design")
			}
			receipt.SpecificationRevision, receipt.SpecificationDigest, receipt.SpecificationApproval = state.Specification.Revision, state.Specification.Digest, state.approvalAuthority.Specification.ID
		}
		store.Design = receipt
	default:
		return errors.New("unsupported review kind")
	}
	if err = s.saveApprovalStore(store); err != nil {
		return err
	}
	state.approvalAuthority = &store
	return nil
}

func requireJourneyReviews(state journeyState) error {
	if state.Specification == nil || currentSpecification(state).Status != "approved" {
		return errors.New("Explicitly approve the current product definition before deploying. Project files alone cannot grant review approval.")
	}
	if state.DesignPlan == nil || currentDesignPlan(state).Status != "approved" {
		return errors.New("Explicitly approve the current design handoff before deploying. Project files alone cannot grant review approval.")
	}
	return nil
}
