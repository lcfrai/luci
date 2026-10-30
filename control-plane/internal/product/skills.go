package product

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/lcfrai/luci/control-plane/internal/registry"
)

const skillRegistryBytesLimit = 1 << 20
const selectedSkillsBytesLimit = 64 << 10

// Instructions are project content, not capabilities or trusted system policy.
// Provenance is the editor's declaration, never an independently verified origin.
type ProjectSkillVersion struct {
	ID           string   `json:"id"`
	Revision     int      `json:"revision"`
	Title        string   `json:"title"`
	Provenance   string   `json:"provenance"`
	Roles        []string `json:"roles"`
	Instructions string   `json:"instructions"`
	Digest       string   `json:"digest"`
}

type ProjectSkillAssignment struct {
	Role          string `json:"role"`
	TaskID        string `json:"taskId,omitempty"`
	SkillID       string `json:"skillId"`
	SkillRevision int    `json:"skillRevision"`
}

type ProjectSkillRegistry struct {
	ProjectID   string                   `json:"projectId"`
	Revision    int                      `json:"revision"`
	Versions    []ProjectSkillVersion    `json:"versions"`
	Assignments []ProjectSkillAssignment `json:"assignments"`
}

type ProjectSkillsSave struct {
	ExpectedPlanRevision *int `json:"expectedPlanRevision,omitempty"`
	ExpectedRevision     int  `json:"expectedRevision"`
	// At most one new immutable version per save. No client digest or revision.
	Skill       *ProjectSkillInput       `json:"skill,omitempty"`
	Assignments []ProjectSkillAssignment `json:"assignments"`
}

type ProjectSkillInput struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Provenance   string   `json:"provenance"`
	Roles        []string `json:"roles"`
	Instructions string   `json:"instructions"`
}

func currentSkills(state journeyState) ProjectSkillRegistry {
	if state.Skills == nil {
		return ProjectSkillRegistry{ProjectID: state.Brief.ProjectID, Versions: []ProjectSkillVersion{}, Assignments: []ProjectSkillAssignment{}}
	}
	// Request-local copies must not mutate saved slices while preparing a draft.
	var out ProjectSkillRegistry
	data, _ := json.Marshal(state.Skills)
	_ = json.Unmarshal(data, &out)
	return out
}

func skillVersionDigest(skill ProjectSkillVersion) string {
	skill.Digest = ""
	return reviewDigest(skill)
}

func skillRole(role string) bool              { return role == "coding" || role == "reviewer" }
func skillKey(id string, revision int) string { return fmt.Sprintf("%s:%d", id, revision) }

func validateSkillRegistry(saved ProjectSkillRegistry) error {
	if !prototypeReleaseID.MatchString(saved.ProjectID) || saved.Revision < 0 || saved.Versions == nil || saved.Assignments == nil || len(saved.Versions) > 128 || len(saved.Assignments) > 64 {
		return errors.New("Skills need a project, nonnegative revision, at most 128 versions and 64 explicit assignments.")
	}
	versions := map[string]ProjectSkillVersion{}
	latest := map[string]int{}
	for _, skill := range saved.Versions {
		if !designIDPattern.MatchString(skill.ID) || skill.Revision != latest[skill.ID]+1 || strings.TrimSpace(skill.Title) == "" || len(skill.Title) > 200 || strings.TrimSpace(skill.Provenance) == "" || len(skill.Provenance) > 2000 || strings.TrimSpace(skill.Instructions) == "" || len(skill.Instructions) > 16<<10 || !utf8.ValidString(skill.Instructions) || strings.ContainsRune(skill.Instructions, 0) || len(skill.Roles) < 1 || len(skill.Roles) > 2 {
			return errors.New("Skill versions need sequential revisions, a title, declared provenance, roles and 1–16 KiB of instructions.")
		}
		roles := map[string]bool{}
		for _, role := range skill.Roles {
			if !skillRole(role) || roles[role] {
				return errors.New("Skill roles must be unique coding or reviewer roles.")
			}
			roles[role] = true
		}
		if skill.Digest != skillVersionDigest(skill) {
			return errors.New("A saved skill version digest does not match its content.")
		}
		latest[skill.ID] = skill.Revision
		versions[skillKey(skill.ID, skill.Revision)] = skill
	}
	seen := map[string]bool{}
	for _, assignment := range saved.Assignments {
		skill, ok := versions[skillKey(assignment.SkillID, assignment.SkillRevision)]
		key := assignment.Role + "/" + assignment.TaskID + "/" + assignment.SkillID
		if !ok || !skillRole(assignment.Role) || assignment.TaskID != "" && !designIDPattern.MatchString(assignment.TaskID) || seen[key] {
			return errors.New("Assignments need unique role/task/skill selections and an existing exact skill revision.")
		}
		matches := false
		for _, role := range skill.Roles {
			matches = matches || role == assignment.Role
		}
		if !matches {
			return errors.New("The selected skill does not support the assignment's role.")
		}
		seen[key] = true
	}
	data, err := json.Marshal(saved)
	if err != nil || len(data) > skillRegistryBytesLimit {
		return errors.New("The skill registry exceeds 1 MiB. Reduce instructions or split the project.")
	}
	return nil
}

func validateSkillAssignmentTasks(saved ProjectSkillRegistry, tasks []ProjectTask) error {
	byID := map[string]string{}
	for _, task := range tasks {
		role := task.Role
		if role == "" {
			role = "coding"
		}
		byID[task.ID] = role
	}
	for _, assignment := range saved.Assignments {
		if assignment.TaskID != "" && byID[assignment.TaskID] != assignment.Role {
			return errors.New("A skill assignment's task was removed or changed role. Review skill assignments before starting work.")
		}
	}
	return nil
}

func applySkillsSave(state journeyState, req ProjectSkillsSave) (ProjectSkillRegistry, error) {
	saved := currentSkills(state)
	if req.ExpectedPlanRevision != nil && (*req.ExpectedPlanRevision < 0 || currentPlan(state, saved.ProjectID).Revision != *req.ExpectedPlanRevision) {
		return saved, errors.New("The implementation plan changed. Review current tasks before saving these skill assignments.")
	}
	if saved.Revision != req.ExpectedRevision {
		return saved, errors.New("The skill registry changed. Reload before saving.")
	}
	if req.Assignments == nil {
		return saved, errors.New("Send the complete assignment list; an explicit empty list removes assignments.")
	}
	if req.Skill != nil {
		next := 1
		for _, previous := range saved.Versions {
			if previous.ID == req.Skill.ID {
				next = previous.Revision + 1
			}
		}
		skill := ProjectSkillVersion{ID: req.Skill.ID, Revision: next, Title: req.Skill.Title, Provenance: req.Skill.Provenance, Roles: append([]string{}, req.Skill.Roles...), Instructions: req.Skill.Instructions}
		sort.Strings(skill.Roles)
		skill.Digest = skillVersionDigest(skill)
		saved.Versions = append(saved.Versions, skill)
	}
	saved.Revision++
	saved.Assignments = append([]ProjectSkillAssignment{}, req.Assignments...)
	sort.Slice(saved.Assignments, func(i, j int) bool {
		a, b := saved.Assignments[i], saved.Assignments[j]
		return a.Role+"/"+a.TaskID+"/"+a.SkillID < b.Role+"/"+b.TaskID+"/"+b.SkillID
	})
	if err := validateSkillRegistry(saved); err != nil {
		return saved, err
	}
	if err := validateSkillAssignmentTasks(saved, currentPlan(state, saved.ProjectID).Tasks); err != nil {
		return saved, err
	}
	// Validate each actual task before saving; the aggregate run budget is also
	// checked later for the exact selected task set, without silent truncation.
	preview := state
	preview.Skills = &saved
	for _, task := range currentPlan(state, saved.ProjectID).Tasks {
		if _, err := selectedSkills(preview, []string{task.ID}); err != nil {
			return saved, err
		}
	}
	return saved, nil
}

func (s *Server) registerSkills(mux Registrar) {
	mux.HandleFunc("GET /v1/product/projects/{id}/skills", s.getProjectSkills)
	mux.HandleFunc("POST /v1/product/projects/{id}/skills", s.saveProjectSkills)
}

func (s *Server) getProjectSkills(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		problem(w, 500, "skills_unavailable", "The saved project skills could not be read.")
		return
	}
	journeyJSON(w, 200, currentSkills(state))
}

func (s *Server) saveProjectSkills(w http.ResponseWriter, r *http.Request) {
	var req ProjectSkillsSave
	if !journeyDecode(w, r, &req) {
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		problem(w, 500, "skills_unavailable", "The saved project skills could not be read.")
		return
	}
	if currentSkills(state).Revision != req.ExpectedRevision {
		problem(w, 409, "revision_conflict", "The skill registry changed. Reload before saving.")
		return
	}
	saved, err := applySkillsSave(state, req)
	if err != nil {
		problem(w, 422, "invalid_skills", err.Error())
		return
	}
	state.Skills = &saved
	if err = s.writeAttributedJourney(r.Context(), p.Path, state, contextMutation{"skills", "save"}); err != nil {
		problem(w, 500, "save_failed", err.Error())
		return
	}
	journeyJSON(w, 200, saved)
}

type selectedProjectSkill struct {
	registry.ImplementationSkill
	Title        string `json:"title"`
	Provenance   string `json:"provenance"`
	Instructions string `json:"instructions"`
}

func selectedSkills(state journeyState, taskIDs []string) ([]selectedProjectSkill, error) {
	saved := currentSkills(state)
	if err := validateSkillRegistry(saved); err != nil {
		return nil, err
	}
	if err := validateSkillAssignmentTasks(saved, currentPlan(state, saved.ProjectID).Tasks); err != nil {
		return nil, err
	}
	versions := map[string]ProjectSkillVersion{}
	for _, skill := range saved.Versions {
		versions[skillKey(skill.ID, skill.Revision)] = skill
	}
	wanted := map[string]bool{}
	for _, id := range taskIDs {
		wanted[id] = true
	}
	selected := []selectedProjectSkill{}
	for _, task := range currentPlan(state, saved.ProjectID).Tasks {
		if !wanted[task.ID] {
			continue
		}
		role := task.Role
		if role == "" {
			role = "coding"
		}
		seen := map[string]int{}
		for _, assignment := range saved.Assignments {
			if assignment.Role != role || assignment.TaskID != "" && assignment.TaskID != task.ID {
				continue
			}
			if revision, duplicate := seen[assignment.SkillID]; duplicate {
				if revision != assignment.SkillRevision {
					return nil, errors.New("Role and task selections disagree on a skill revision. Choose one explicit revision.")
				}
				continue
			}
			seen[assignment.SkillID] = assignment.SkillRevision
			skill := versions[skillKey(assignment.SkillID, assignment.SkillRevision)]
			selected = append(selected, selectedProjectSkill{ImplementationSkill: registry.ImplementationSkill{TaskID: task.ID, Role: role, ID: skill.ID, Revision: skill.Revision, Digest: skill.Digest}, Title: skill.Title, Provenance: skill.Provenance, Instructions: skill.Instructions})
		}
		if len(seen) > 16 {
			return nil, errors.New("A task may use at most 16 selected skills.")
		}
	}
	sort.Slice(selected, func(i, j int) bool {
		return selected[i].TaskID+"/"+selected[i].ID < selected[j].TaskID+"/"+selected[j].ID
	})
	data, err := json.MarshalIndent(selected, "", "  ")
	if err != nil || len(data) > selectedSkillsBytesLimit || len(selected) > 64 {
		return nil, errors.New("Selected skills exceed the 64 KiB context budget or 64 task-skill bindings. Narrow this run.")
	}
	return selected, nil
}
