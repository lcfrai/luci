package product

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"time"
)

const personalArchiveLimit = 128
const personalArchiveBytes = 64 << 20

// The index contains correlation digests, never additional project prose.
// Snapshot files are immutable. Only a resolution overlay may advance an
// already recorded handoff; it cannot create or resend a project request.
type PersonalArchiveKey struct {
	TurnID        string `json:"turnId"`
	KeyDigest     string `json:"keyDigest"`
	RequestDigest string `json:"requestDigest"`
}
type PersonalArchiveResolution struct {
	TurnID  string          `json:"turnId"`
	Handoff PersonalHandoff `json:"handoff"`
}
type PersonalArchive struct {
	ID            string                      `json:"id"`
	LastMessageAt string                      `json:"lastMessageAt"`
	Bytes         int                         `json:"bytes"`
	Digest        string                      `json:"digest"`
	Keys          []PersonalArchiveKey        `json:"keys"`
	Resolutions   []PersonalArchiveResolution `json:"resolutions,omitempty"`
}
type PersonalArchiveChoice struct {
	ID            string `json:"id"`
	LastMessageAt string `json:"lastMessageAt"`
	Turns         int    `json:"turns"`
}

func personalArchiveTime(value string) (time.Time, error) { return time.Parse(time.RFC3339Nano, value) }
func validPersonalArchives(rows []PersonalArchive) bool {
	if len(rows) > personalArchiveLimit {
		return false
	}
	ids, turns, keys := map[string]bool{}, map[string]bool{}, map[string]bool{}
	total := 0
	for _, row := range rows {
		if !prototypeReleaseID.MatchString(row.ID) || ids[row.ID] || !hexDigest(row.Digest) || row.Bytes < 1 || row.Bytes > personalFileLimit || len(row.Keys) < 1 || len(row.Keys) > personalTurnLimit || len(row.Resolutions) > len(row.Keys) {
			return false
		}
		ids[row.ID] = true
		total += row.Bytes
		if _, err := personalArchiveTime(row.LastMessageAt); err != nil {
			return false
		}
		for _, key := range row.Keys {
			if !prototypeReleaseID.MatchString(key.TurnID) || turns[key.TurnID] || !hexDigest(key.KeyDigest) || !hexDigest(key.RequestDigest) || keys[key.KeyDigest] {
				return false
			}
			turns[key.TurnID], keys[key.KeyDigest] = true, true
		}
		resolved := map[string]bool{}
		for _, resolution := range row.Resolutions {
			found := false
			for _, key := range row.Keys {
				found = found || key.TurnID == resolution.TurnID
			}
			if !found || resolved[resolution.TurnID] {
				return false
			}
			resolved[resolution.TurnID] = true
		}
	}
	return total <= personalArchiveBytes
}
func personalArchiveChoices(row PersonalConversation) []PersonalArchiveChoice {
	result := []PersonalArchiveChoice{}
	for _, archive := range row.Archives {
		result = append(result, PersonalArchiveChoice{ID: archive.ID, LastMessageAt: archive.LastMessageAt, Turns: len(archive.Keys)})
	}
	return result
}
func (s *Server) personalArchivePath(scope, id string, create bool) (string, error) {
	if !prototypeReleaseID.MatchString(id) {
		return "", ErrPersonalUnavailable
	}
	// Use the same private ordinary-file and ancestor gates as the active ledger.
	return s.personalPath(reviewDigest([]string{"personal-archive", scope, id}), create)
}
func (s *Server) readPersonalArchiveChoice(owner PersonalConversation, id string) (PersonalConversation, error) {
	for _, archive := range owner.Archives {
		if archive.ID != id {
			continue
		}
		path, err := s.personalArchivePath(owner.Scope, id, false)
		if err != nil {
			return PersonalConversation{}, ErrPersonalUnavailable
		}
		var row PersonalConversation
		if externalReadJSON(path, personalFileLimit, &row) != nil || len(row.Archives) != 0 || row.ConversationID != id || reviewDigest(row) != archive.Digest {
			return PersonalConversation{}, ErrPersonalUnavailable
		}
		row, err = validatePersonalConversation(row, owner.Scope)
		if err != nil || len(row.Turns) != len(archive.Keys) {
			return PersonalConversation{}, ErrPersonalUnavailable
		}
		raw, err := json.MarshalIndent(row, "", "  ")
		if err != nil || len(raw)+1 != archive.Bytes {
			return PersonalConversation{}, ErrPersonalUnavailable
		}
		for i, turn := range row.Turns {
			key := archive.Keys[i]
			if turn.ID != key.TurnID || turn.KeyDigest != key.KeyDigest || turn.RequestDigest != key.RequestDigest || turn.Status == "reserved" {
				return PersonalConversation{}, ErrPersonalUnavailable
			}
		}
		for _, resolution := range archive.Resolutions {
			for i := range row.Turns {
				turn := &row.Turns[i]
				if turn.ID != resolution.TurnID {
					continue
				}
				original := turn.Handoff
				next := resolution.Handoff
				if original == nil || original.Status == "forwarded" || next.ID != original.ID || next.ProjectID != original.ProjectID || next.Message != original.Message || next.ProjectRevision != original.ProjectRevision || next.ProjectContextDigest != original.ProjectContextDigest || next.ProjectRequestKey != original.ProjectRequestKey || next.RequestDigest != original.RequestDigest || next.Status != "unknown" && next.Status != "forwarded" || next.Status == "forwarded" && (!prototypeReleaseID.MatchString(next.ProjectTurnID) || !prototypeReleaseID.MatchString(next.ProjectJobID)) {
					return PersonalConversation{}, ErrPersonalUnavailable
				}
				turn.Handoff = &next
			}
		}
		return row, nil
	}
	return PersonalConversation{}, ErrPersonalUnavailable
}
func (s *Server) personalArchivedByID(row PersonalConversation, id string) (*PersonalTurn, error) {
	for _, archive := range row.Archives {
		for _, key := range archive.Keys {
			if key.TurnID == id {
				original, err := s.readPersonalArchiveChoice(row, archive.ID)
				if err != nil {
					return nil, err
				}
				for _, turn := range original.Turns {
					if turn.ID == id {
						return &turn, nil
					}
				}
				return nil, ErrPersonalUnavailable
			}
		}
	}
	return nil, nil
}
func (s *Server) personalArchivedByKey(row PersonalConversation, digest string) (*PersonalTurn, error) {
	for _, archive := range row.Archives {
		for _, key := range archive.Keys {
			if key.KeyDigest == digest {
				return s.personalArchivedByID(row, key.TurnID)
			}
		}
	}
	return nil, nil
}

func (s *Server) ContinuePersonalConversation(ctx context.Context, revision int, conversationID string, access PortfolioGoalAccess) (PersonalView, error) {
	scope, err := s.PersonalScope(ctx)
	if err != nil {
		return PersonalView{}, err
	}
	s.personalMu.Lock()
	row, err := s.readPersonal(scope)
	if err == nil && (row.Revision != revision || row.ConversationID != conversationID) {
		err = ErrPersonalChanged
	}
	if err == nil && len(row.Turns) == 0 {
		s.personalMu.Unlock()
		return s.PersonalView(ctx, 0, access)
	}
	if err == nil {
		for _, turn := range row.Turns {
			if turn.Status == "reserved" {
				err = ErrOrchestratorBusy
			}
		}
	}
	if err == nil && len(row.Archives) >= personalArchiveLimit {
		err = ErrPersonalCapacity
	}
	if err == nil {
		snapshot := row
		snapshot.Archives = nil
		raw, marshalErr := json.MarshalIndent(snapshot, "", "  ")
		raw = append(raw, '\n')
		if marshalErr != nil || len(raw) > personalFileLimit {
			err = ErrPersonalCapacity
		} else {
			archive := PersonalArchive{ID: row.ConversationID, LastMessageAt: row.Turns[len(row.Turns)-1].CreatedAt, Bytes: len(raw), Digest: reviewDigest(snapshot), Keys: []PersonalArchiveKey{}}
			for _, turn := range row.Turns {
				archive.Keys = append(archive.Keys, PersonalArchiveKey{TurnID: turn.ID, KeyDigest: turn.KeyDigest, RequestDigest: turn.RequestDigest})
			}
			candidate := append(append([]PersonalArchive{}, row.Archives...), archive)
			if !validPersonalArchives(candidate) {
				err = ErrPersonalCapacity
			} else {
				path, pathErr := s.personalArchivePath(scope, archive.ID, true)
				err = pathErr
				if err == nil {
					file, createErr := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
					if errors.Is(createErr, os.ErrExist) {
						// An interrupted rollover may have saved this exact immutable
						// snapshot. Verify it; never overwrite an uncertain partial file.
						_, err = s.readPersonalArchiveChoice(PersonalConversation{Scope: scope, Archives: []PersonalArchive{archive}}, archive.ID)
					} else if createErr != nil {
						err = createErr
					} else {
						err = restrictPrivateFile(path)
						if err == nil {
							var n int
							n, err = file.Write(raw)
							if err == nil && n != len(raw) {
								err = ErrPersonalUnavailable
							}
						}
						if err == nil {
							err = file.Sync()
						}
						closeErr := file.Close()
						if err == nil {
							err = closeErr
						}
					}
				}
				if err == nil {
					row.Archives = candidate
					row.Turns = []PersonalTurn{}
					row.ConversationID = newID("personal-chat-")
					err = s.savePersonal(&row)
				}
			}
		}
	}
	s.personalMu.Unlock()
	if err != nil {
		return PersonalView{}, err
	}
	return s.PersonalView(ctx, 0, access)
}

func (s *Server) completeArchivedPersonalHandoff(row *PersonalConversation, turnID, handoffID, projectTurnID, projectJobID string) (bool, error) {
	turn, err := s.personalArchivedByID(*row, turnID)
	if err != nil || turn == nil {
		return false, err
	}
	if turn.Handoff == nil || turn.Handoff.ID != handoffID {
		return true, ErrPersonalChanged
	}
	next := *turn.Handoff
	if next.Status == "forwarded" {
		if next.ProjectTurnID == projectTurnID && next.ProjectJobID == projectJobID {
			return true, nil
		}
		return true, ErrPersonalChanged
	}
	if projectTurnID == "" {
		next.Status = "unknown"
	} else {
		next.Status, next.ProjectTurnID, next.ProjectJobID = "forwarded", projectTurnID, projectJobID
	}
	for i := range row.Archives {
		archive := &row.Archives[i]
		for _, key := range archive.Keys {
			if key.TurnID != turnID {
				continue
			}
			for j := range archive.Resolutions {
				if archive.Resolutions[j].TurnID == turnID {
					archive.Resolutions[j].Handoff = next
					return true, s.savePersonal(row)
				}
			}
			archive.Resolutions = append(archive.Resolutions, PersonalArchiveResolution{TurnID: turnID, Handoff: next})
			return true, s.savePersonal(row)
		}
	}
	return true, ErrPersonalUnavailable
}
