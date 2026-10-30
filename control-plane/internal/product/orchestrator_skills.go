package product

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"unicode/utf8"
)

const orchestratorSkillCatalogueLimit = 24
const orchestratorSkillContextBytes = 48 << 10

// A bounded, untrusted catalogue excerpt, never an installed tool or permission.
// Full immutable instructions remain in the existing project skill registry.
type OrchestratorSkillSummary struct {
	ID                 string   `json:"id"`
	Revision           int      `json:"revision"`
	Digest             string   `json:"digest"`
	Title              string   `json:"title"`
	Provenance         string   `json:"provenance"`
	Roles              []string `json:"roles"`
	InstructionExcerpt string   `json:"instructionExcerpt"`
}

type OrchestratorSkillsContext struct {
	ProjectID       string                     `json:"projectId"`
	Revision        int                        `json:"revision"`
	RegistryDigest  string                     `json:"registryDigest"`
	Versions        []OrchestratorSkillSummary `json:"versions"`
	OmittedVersions int                        `json:"omittedVersions"`
	Assignments     []ProjectSkillAssignment   `json:"assignments"`
}

type OrchestratorSkillSuggestion struct {
	TaskID        string `json:"taskId"`
	Role          string `json:"role" enum:"coding,reviewer"`
	SkillID       string `json:"skillId"`
	SkillRevision int    `json:"skillRevision"`
	SkillDigest   string `json:"skillDigest"`
	Reason        string `json:"reason"`
}

func skillExcerpt(value string, maximum int) string {
	if len(value) <= maximum {
		return value
	}
	value = value[:maximum]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value
}

func orchestratorSkillsContext(state journeyState) (*OrchestratorSkillsContext, error) {
	if state.Skills == nil {
		return nil, nil
	}
	saved := currentSkills(state)
	if err := validateSkillRegistry(saved); err != nil {
		return nil, err
	}
	out := &OrchestratorSkillsContext{ProjectID: saved.ProjectID, Revision: saved.Revision, RegistryDigest: reviewDigest(saved), Versions: []OrchestratorSkillSummary{}, Assignments: append([]ProjectSkillAssignment{}, saved.Assignments...)}
	versions := append([]ProjectSkillVersion{}, saved.Versions...)
	sort.Slice(versions, func(i, j int) bool {
		if versions[i].ID != versions[j].ID {
			return versions[i].ID < versions[j].ID
		}
		return versions[i].Revision > versions[j].Revision
	})
	for _, v := range versions {
		if len(out.Versions) >= orchestratorSkillCatalogueLimit {
			break
		}
		out.Versions = append(out.Versions, OrchestratorSkillSummary{ID: v.ID, Revision: v.Revision, Digest: v.Digest, Title: v.Title, Provenance: skillExcerpt(strings.TrimSpace(v.Provenance), 256), Roles: append([]string{}, v.Roles...), InstructionExcerpt: skillExcerpt(strings.TrimSpace(v.Instructions), 512)})
	}
	out.OmittedVersions = len(versions) - len(out.Versions)
	// JSON escaping also consumes the catalogue budget. Preserve assignments and
	// report every omitted version instead of truncating a reference or silently
	// expanding context. The canonical registry digest still binds all versions.
	for {
		raw, err := json.Marshal(out)
		if err != nil {
			return nil, err
		}
		if len(raw) <= orchestratorSkillContextBytes {
			break
		}
		if len(out.Versions) == 0 {
			return nil, ErrOrchestratorInvalid
		}
		out.Versions = out.Versions[:len(out.Versions)-1]
		out.OmittedVersions++
	}
	if err := validateOrchestratorSkillsContext(out, state.Brief.ProjectID); err != nil {
		return nil, err
	}
	return out, nil
}

func validateOrchestratorSkillsContext(value *OrchestratorSkillsContext, projectID string) error {
	if value == nil {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil || len(raw) > orchestratorSkillContextBytes || value.ProjectID != projectID || value.Revision < 0 || !hexDigest(value.RegistryDigest) || value.Versions == nil || len(value.Versions) > orchestratorSkillCatalogueLimit || value.OmittedVersions < 0 || len(value.Versions)+value.OmittedVersions > 128 || value.Assignments == nil || len(value.Assignments) > 64 {
		return ErrOrchestratorInvalid
	}
	seen := map[string]bool{}
	for _, v := range value.Versions {
		key := skillKey(v.ID, v.Revision)
		if !designIDPattern.MatchString(v.ID) || v.Revision < 1 || !hexDigest(v.Digest) || seen[key] || strings.TrimSpace(v.Title) == "" || len(v.Title) > 200 || len(v.Provenance) > 256 || strings.TrimSpace(v.Provenance) == "" || len(v.InstructionExcerpt) > 512 || strings.TrimSpace(v.InstructionExcerpt) == "" || !utf8.ValidString(v.Provenance+v.InstructionExcerpt) || strings.ContainsRune(v.Provenance+v.InstructionExcerpt, 0) || len(v.Roles) < 1 || len(v.Roles) > 2 {
			return ErrOrchestratorInvalid
		}
		roles := map[string]bool{}
		for _, role := range v.Roles {
			if !skillRole(role) || roles[role] {
				return ErrOrchestratorInvalid
			}
			roles[role] = true
		}
		seen[key] = true
	}
	assignments := map[string]bool{}
	for _, a := range value.Assignments {
		key := a.Role + "/" + a.TaskID + "/" + a.SkillID
		if !skillRole(a.Role) || a.TaskID != "" && !designIDPattern.MatchString(a.TaskID) || !designIDPattern.MatchString(a.SkillID) || a.SkillRevision < 1 || assignments[key] {
			return ErrOrchestratorInvalid
		}
		assignments[key] = true
	}
	return nil
}

func validateOrchestratorSkillSuggestions(snapshot OrchestratorContext, proposal *OrchestratorProposal) error {
	if err := validateOrchestratorSkillsContext(snapshot.Skills, snapshot.ProjectID); err != nil {
		return err
	}
	if proposal == nil || len(proposal.SuggestedSkillAssignments) == 0 {
		return nil
	}
	if proposal.Target != "plan" || proposal.Plan == nil || snapshot.Skills == nil || len(proposal.SuggestedSkillAssignments) > 32 {
		return errors.New("Skill recommendations need a plan and exact saved catalogue references.")
	}
	roles := map[string]string{}
	for _, task := range proposal.Plan.Tasks {
		role := task.Role
		if role == "" {
			role = "coding"
		}
		roles[task.ID] = role
	}
	versions := map[string]OrchestratorSkillSummary{}
	for _, v := range snapshot.Skills.Versions {
		versions[skillKey(v.ID, v.Revision)] = v
	}
	seen := map[string]bool{}
	perTask := map[string]int{}
	for _, a := range proposal.SuggestedSkillAssignments {
		v, found := versions[skillKey(a.SkillID, a.SkillRevision)]
		key := a.Role + "/" + a.TaskID + "/" + a.SkillID
		matches := false
		for _, role := range v.Roles {
			matches = matches || role == a.Role
		}
		if !found || !skillRole(a.Role) || roles[a.TaskID] != a.Role || a.SkillDigest != v.Digest || seen[key] || !matches || strings.TrimSpace(a.Reason) == "" || len(a.Reason) > 1000 || !validArtifactStoredText(a.Reason) {
			return errors.New("A recommended skill must match an available exact version, task role and reason.")
		}
		seen[key] = true
		perTask[a.TaskID]++
		if perTask[a.TaskID] > 16 {
			return errors.New("Recommend at most 16 exact skills for one task.")
		}
	}
	return nil
}
