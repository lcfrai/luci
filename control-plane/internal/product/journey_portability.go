package product

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
)

// Context identity follows public project content through clone and restore.
// It is a lineage label, never an authorization or private-review capability.
// Legacy repositories derive the same label from their original saved ID on
// every machine, without a GET writing or randomly changing their files.
func journeyContextIdentity(sourceID string) string {
	return "ctx-" + reviewDigest("luci-project-context-v1:" + sourceID)[:24]
}

func validJourneyContextIdentity(id string) bool {
	return strings.HasPrefix(id, "ctx-") && len(id) == 28 && strings.Trim(id[4:], "0123456789abcdef") == ""
}

func normalizeJourneyIdentity(state *journeyState, projectID string) error {
	sourceID := state.Brief.ProjectID
	if sourceID == "" || !prototypeReleaseID.MatchString(sourceID) || state.Design.ProjectID != sourceID {
		return errors.New("saved project context has inconsistent project identities")
	}
	if err := validateStoredJourneyScene(state.Design.Scene); err != nil {
		return err
	}
	if state.ContextID == "" {
		if state.Version == 2 {
			return errors.New("saved project context is missing its portable identity")
		}
		state.ContextID = journeyContextIdentity(sourceID)
	}
	if !validJourneyContextIdentity(state.ContextID) || (state.HistoryHead != "" && !hexDigest(state.HistoryHead)) {
		return errors.New("saved project context has an invalid portable identity or history reference")
	}
	if state.Plan != nil && state.Plan.ProjectID != sourceID || state.Specification != nil && state.Specification.ProjectID != sourceID || state.DesignPlan != nil && state.DesignPlan.ProjectID != sourceID {
		return errors.New("saved project context has inconsistent project identities")
	}
	for _, row := range state.BriefRevisions {
		if row.ProjectID != sourceID {
			return errors.New("saved brief history has inconsistent project identities")
		}
	}
	for _, row := range state.DesignRevisions {
		if err := validateStoredJourneyScene(row.Scene); err != nil {
			return err
		}
		if row.ProjectID != sourceID {
			return errors.New("saved sketch history has inconsistent project identities")
		}
	}
	for _, row := range state.SpecificationRevisions {
		if row.ProjectID != sourceID {
			return errors.New("saved definition history has inconsistent project identities")
		}
	}
	for _, row := range state.DesignPlanRevisions {
		if row.ProjectID != sourceID {
			return errors.New("saved design history has inconsistent project identities")
		}
	}
	if sourceID != projectID {
		state.importedContext = true
		rebindJourneyIdentity(state, projectID)
	}
	return nil
}

// Public content may be imported; saved review claims and prototype identities
// cannot follow it into another local project. Private authority is also scoped
// independently by readApprovedJourney to the current account and path.
func rebindJourneyIdentity(state *journeyState, id string) {
	state.approvalAuthority = nil
	state.Brief.ProjectID = id
	state.Design.ProjectID = id
	if state.Plan != nil {
		state.Plan.ProjectID = id
		state.Plan.SpecificationDigest = ""
	}
	rebindSpecification := func(spec *ProjectSpecification) { spec.ProjectID = id; spec.Status = "draft"; spec.Digest = "" }
	rebindDesign := func(design *ProjectDesignPlan) {
		design.ProjectID = id
		design.Status = "draft"
		design.Digest = ""
		design.PrototypeReleaseID = ""
		design.PrototypeCommit = ""
	}
	if state.Specification != nil {
		rebindSpecification(state.Specification)
	}
	if state.DesignPlan != nil {
		rebindDesign(state.DesignPlan)
	}
	for i := range state.BriefRevisions {
		state.BriefRevisions[i].ProjectID = id
	}
	for i := range state.DesignRevisions {
		state.DesignRevisions[i].ProjectID = id
	}
	for i := range state.SpecificationRevisions {
		rebindSpecification(&state.SpecificationRevisions[i])
	}
	for i := range state.DesignPlanRevisions {
		rebindDesign(&state.DesignPlanRevisions[i])
	}
}

// Earlier canonical files pretty-printed raw scenes; incidental whitespace
// must not make a previously valid drawing exceed the request-size bound.
func validateStoredJourneyScene(scene json.RawMessage) error {
	if len(scene) == 0 {
		return nil
	}
	if len(scene) > journeyStateLimit {
		return errors.New("saved drawing scene exceeds the context size limit")
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, scene); err != nil {
		return errors.New("saved drawing scene is invalid JSON")
	}
	return validateScene(compact.Bytes())
}
