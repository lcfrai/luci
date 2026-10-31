package management

import (
	"encoding/json"
	"errors"
	"net/http"
)

func personalResponseProjects(raw []byte) ([]string, error) {
	if len(raw) > 5<<20 {
		return nil, errors.New("private response over limit")
	}
	var root any
	if json.Unmarshal(raw, &root) != nil {
		return nil, errors.New("invalid private response")
	}
	seen, nodes := map[string]bool{}, 0
	add := func(value any) error {
		id, ok := value.(string)
		if ok && id == "" {
			return nil
		} // First-use preferences have no compute project.
		if !ok || !teamProjectIDPattern.MatchString(id) {
			return errors.New("invalid private project reference")
		}
		seen[id] = true
		if len(seen) > 256 {
			return errors.New("private response project limit")
		}
		return nil
	}
	var walk func(any, int) error
	walk = func(value any, depth int) error {
		nodes++
		if depth > 16 || nodes > 100000 {
			return errors.New("private response nesting limit")
		}
		switch typed := value.(type) {
		case map[string]any:
			for key, child := range typed {
				if key == "projectId" || key == "computeProjectId" {
					if err := add(child); err != nil {
						return err
					}
				}
				if key == "projectIds" && child != nil {
					ids, ok := child.([]any)
					if !ok || len(ids) > 16 {
						return errors.New("invalid private source set")
					}
					for _, id := range ids {
						if err := add(id); err != nil {
							return err
						}
					}
				}
				if err := walk(child, depth+1); err != nil {
					return err
				}
			}
		case []any:
			for _, child := range typed {
				if err := walk(child, depth+1); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := walk(root, 0); err != nil {
		return nil, err
	}
	ids := []string{}
	for id := range seen {
		ids = append(ids, id)
	}
	return ids, nil
}
func (g *Gateway) recheckTeamPersonal(w http.ResponseWriter, capture *teamCapturedResponse, teamID, actorID string) {
	if capture.overflow {
		fail(w, 503, "personal_response_unavailable", "The private response exceeded its bound.")
		return
	}
	if capture.status < 200 || capture.status >= 300 {
		copyTeamResponse(w, capture)
		return
	}
	ids, err := personalResponseProjects(capture.body.Bytes())
	if err != nil {
		fail(w, 503, "personal_response_unavailable", "The private response could not be verified.")
		return
	}
	for _, id := range ids {
		access, err := g.resolveTeamAccess(teamID, actorID, id)
		if err != nil || !access.Permissions.Read {
			fail(w, 403, "team_access_changed", "Project access changed. The private response was withheld. Saved changes are not rolled back; check the original request when access is restored.")
			return
		}
	}
	copyTeamResponse(w, capture)
}
