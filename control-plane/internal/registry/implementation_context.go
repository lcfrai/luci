package registry

// ImplementationContext records reviewed coding inputs, not task completion.
type ImplementationContext struct {
	ProjectID             string   `json:"projectId"`
	WorktreeID            string   `json:"worktreeId"`
	BriefRevision         int      `json:"briefRevision"`
	BriefDigest           string   `json:"briefDigest"`
	PlanRevision          int      `json:"planRevision"`
	PlanDigest            string   `json:"planDigest"`
	SpecificationRevision int      `json:"specificationRevision"`
	SpecificationDigest   string   `json:"specificationDigest"`
	DesignPlanRevision    int      `json:"designPlanRevision"`
	DesignPlanDigest      string   `json:"designPlanDigest"`
	TaskIDs               []string `json:"taskIds"`
	TaskTitles            []string `json:"taskTitles"`
	RequirementIDs        []string `json:"requirementIds"`
	SourceCommit          string   `json:"sourceCommit"`
	SourceDigest          string   `json:"sourceDigest"`
	SourceDirty           bool     `json:"sourceDirty"`
}

func (c *ImplementationContext) Clone() *ImplementationContext {
	if c == nil {
		return nil
	}
	out := *c
	out.TaskIDs = cloneStrings(c.TaskIDs)
	out.TaskTitles = cloneStrings(c.TaskTitles)
	out.RequirementIDs = cloneStrings(c.RequirementIDs)
	return &out
}
