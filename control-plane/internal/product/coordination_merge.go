package product

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
)

// Reconciliation is deliberately structural, not a claim of semantic agreement.
// A keyed record is indivisible: independently editing two fields of the same
// requirement/task still requires a decision. Unkeyed arrays are also atomic.
type coordinationCells map[string]json.RawMessage

var coordinationMetadata = map[string]bool{
	"projectId": true, "version": true, "revision": true, "briefRevision": true,
	"specificationRevision": true, "specificationDigest": true, "sketchRevision": true,
	"status": true, "digest": true, "updatedAt": true,
	"prototypeReleaseId": true, "prototypeCommit": true,
}

func coordinationDocument(snapshot OrchestratorContext, proposal *OrchestratorProposal, target string) any {
	switch target {
	case "specification":
		if proposal != nil {
			return proposal.Specification
		}
		return snapshot.Specification
	case "design-plan":
		if proposal != nil {
			return proposal.DesignPlan
		}
		return snapshot.DesignPlan
	case "plan":
		if proposal != nil {
			return proposal.Plan
		}
		return snapshot.Plan
	}
	return nil
}

func coordinationObject(value any) map[string]json.RawMessage {
	raw, _ := json.Marshal(value)
	var out map[string]json.RawMessage
	_ = json.Unmarshal(raw, &out)
	return out
}

func coordinationKeyed(raw json.RawMessage) ([]string, map[string]json.RawMessage, bool) {
	var rows []json.RawMessage
	if len(raw) == 0 || raw[0] != '[' || json.Unmarshal(raw, &rows) != nil {
		return nil, nil, false
	}
	keys, out := []string{}, map[string]json.RawMessage{}
	for _, row := range rows {
		var item struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		if json.Unmarshal(row, &item) != nil {
			return nil, nil, false
		}
		if item.ID == "" {
			item.ID = item.Name
		}
		if item.ID == "" || strings.ContainsAny(item.ID, "/~") || out[item.ID] != nil {
			return nil, nil, false
		}
		keys = append(keys, item.ID)
		out[item.ID] = row
	}
	return keys, out, true
}

func coordinationDiff(base, proposed map[string]json.RawMessage) coordinationCells {
	out := coordinationCells{}
	for field, next := range proposed {
		if coordinationMetadata[field] || reflect.DeepEqual(base[field], next) {
			continue
		}
		oldKeys, oldRows, oldOK := coordinationKeyed(base[field])
		newKeys, newRows, newOK := coordinationKeyed(next)
		// Reordering existing IDs is an intentional whole-collection edit. Do
		// not discard it while turning content changes into keyed cells.
		oldCommon, newCommon := []string{}, []string{}
		for _, id := range oldKeys {
			if _, ok := newRows[id]; ok {
				oldCommon = append(oldCommon, id)
			}
		}
		for _, id := range newKeys {
			if _, ok := oldRows[id]; ok {
				newCommon = append(newCommon, id)
			}
		}
		expectedOrder := append([]string{}, oldCommon...)
		for _, id := range newKeys {
			if _, ok := oldRows[id]; !ok {
				expectedOrder = append(expectedOrder, id)
			}
		}
		if !reflect.DeepEqual(oldCommon, newCommon) || !reflect.DeepEqual(expectedOrder, newKeys) {
			out[field] = next
			continue
		}
		if oldOK && newOK {
			for id, old := range oldRows {
				if !reflect.DeepEqual(old, newRows[id]) {
					out[field+"/"+id] = newRows[id]
				}
			}
			for id, value := range newRows {
				if _, ok := oldRows[id]; !ok {
					out[field+"/"+id] = value
				}
			}
		} else {
			out[field] = next
		}
	}
	return out
}

func coordinationCell(object map[string]json.RawMessage, path string) json.RawMessage {
	parts := strings.SplitN(path, "/", 2)
	if len(parts) == 1 {
		return object[path]
	}
	_, rows, ok := coordinationKeyed(object[parts[0]])
	if !ok {
		return nil
	}
	return rows[parts[1]]
}

func coordinationPaths(cells coordinationCells) []string {
	out := make([]string, 0, len(cells))
	for path := range cells {
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

func coordinationOverlaps(a, b coordinationCells) []string {
	out := []string{}
	for x, left := range a {
		for y, right := range b {
			if x == y && reflect.DeepEqual(left, right) {
				continue
			}
			if x == y || strings.HasPrefix(x, y+"/") || strings.HasPrefix(y, x+"/") {
				out = append(out, x)
			}
		}
	}
	sort.Strings(out)
	return out
}

func coordinationRebase(turn storedOrchestratorTurn, current OrchestratorContext) (OrchestratorAnswer, []string) {
	answer := *cloneOrchestratorAnswer(turn.Answer)
	target := answer.Proposal.Target
	base := coordinationObject(coordinationDocument(turn.Context, nil, target))
	next := coordinationObject(coordinationDocument(turn.Context, answer.Proposal, target))
	live := coordinationObject(coordinationDocument(current, nil, target))
	changes := coordinationDiff(base, next)
	conflicts := []string{}
	// A different dependency document is not a harmless metadata revision. The
	// original proposal was never reviewed/generated against that new contract.
	if target != "specification" && len(coordinationDiff(coordinationObject(turn.Context.Specification), coordinationObject(current.Specification))) != 0 {
		return answer, []string{"specification"}
	}
	if target == "design-plan" && (turn.Context.SketchRevision != current.SketchRevision || turn.Context.DesignPlan.PrototypeReleaseID != current.DesignPlan.PrototypeReleaseID || turn.Context.DesignPlan.PrototypeCommit != current.DesignPlan.PrototypeCommit) {
		return answer, []string{"design-dependency"}
	}
	for _, path := range coordinationPaths(changes) {
		old, actual, desired := coordinationCell(base, path), coordinationCell(live, path), changes[path]
		if !reflect.DeepEqual(actual, old) && !reflect.DeepEqual(actual, desired) {
			conflicts = append(conflicts, path)
			continue
		}
		parts := strings.SplitN(path, "/", 2)
		if len(parts) == 1 {
			live[path] = desired
			continue
		}
		keys, rows, ok := coordinationKeyed(live[parts[0]])
		if !ok {
			conflicts = append(conflicts, path)
			continue
		}
		if _, found := rows[parts[1]]; !found && desired != nil {
			keys = append(keys, parts[1])
		}
		rows[parts[1]] = desired
		values := []json.RawMessage{}
		for _, key := range keys {
			if rows[key] != nil {
				values = append(values, rows[key])
			}
		}
		live[parts[0]], _ = json.Marshal(values)
	}
	if len(conflicts) != 0 {
		return answer, conflicts
	}
	// Current metadata remains current; the existing canonical engine increments
	// revisions and invalidates approvals. Reconciliation grants neither approval
	// nor permission to change task completion status.
	if target != "plan" {
		live["status"] = json.RawMessage(`"draft"`)
		live["digest"] = json.RawMessage(`""`)
	}
	raw, _ := json.Marshal(live)
	switch target {
	case "specification":
		var value ProjectSpecification
		_ = json.Unmarshal(raw, &value)
		answer.Proposal.Specification = &value
	case "design-plan":
		var value ProjectDesignPlan
		_ = json.Unmarshal(raw, &value)
		answer.Proposal.DesignPlan = &value
	case "plan":
		var value ProjectPlan
		_ = json.Unmarshal(raw, &value)
		answer.Proposal.Plan = &value
	}
	return answer, conflicts
}
