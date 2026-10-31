package product

import (
	"context"
	"encoding/json"
	"errors"
	"os"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

type RuntimePreparationSummary struct {
	ID                string   `json:"id"`
	Digest            string   `json:"digest"`
	BlueprintRevision int      `json:"blueprintRevision"`
	Services          []string `json:"services"`
	CreatedAt         string   `json:"createdAt"`
	Finalized         bool     `json:"finalized"`
}
type runtimePreparationIndex struct {
	Version int                          `json:"version"`
	Actor   management.ExecutionActorRef `json:"actor"`
	Rows    []RuntimePreparationSummary  `json:"rows"`
}

func (s *Server) readRuntimePreparationIndex(actor management.ExecutionActorRef) (string, runtimePreparationIndex, error) {
	file, err := privateApplicationStatePathForRoot(s.config.ProjectsDir, s.stateDir, "runtime-preparation-index-"+reviewDigest(actor)[:32])
	empty := runtimePreparationIndex{Version: 1, Actor: actor, Rows: []RuntimePreparationSummary{}}
	if err != nil {
		return "", empty, err
	}
	var saved runtimePreparationIndex
	err = externalReadJSON(file, 256<<10, &saved)
	if errors.Is(err, os.ErrNotExist) {
		return file, empty, nil
	}
	if err != nil || saved.Version != 1 || saved.Actor != actor || saved.Rows == nil || len(saved.Rows) > 128 {
		return "", empty, ErrRuntimePreparation
	}
	seen := map[string]bool{}
	for _, row := range saved.Rows {
		if !runtimePreparationID(row.ID) || !hexDigest(row.Digest) || row.BlueprintRevision < 1 || !feedbackTimestamp(row.CreatedAt) || len(row.Services) < 1 || len(row.Services) > 32 || seen[row.ID] {
			return "", empty, ErrRuntimePreparation
		}
		seen[row.ID] = true
	}
	return file, saved, nil
}

// Caller holds runtimePreparationMu. Only finalized rows can leave the recent
// index. Immutable per-ID originals are never deleted or overwritten by trim.
func (s *Server) indexRuntimePreparation(row storedRuntimePreparation) error {
	file, index, err := s.readRuntimePreparationIndex(row.Plan.Actor)
	if err != nil {
		return err
	}
	summary := RuntimePreparationSummary{ID: row.Plan.ID, Digest: row.Plan.Digest, BlueprintRevision: row.Plan.BlueprintRevision, CreatedAt: row.Plan.CreatedAt, Services: []string{}, Finalized: row.SavedBlueprintRevision > 0}
	seen := map[string]bool{}
	add := func(id string) {
		if id != "" && !seen[id] {
			seen[id] = true
			summary.Services = append(summary.Services, id)
		}
	}
	for _, b := range row.Plan.Builds {
		add(b.Selection.ServiceID)
	}
	if row.Plan.Resources != nil {
		for _, v := range row.Plan.Resources.Volumes {
			add(v.ServiceID)
		}
		for _, id := range row.Plan.Resources.ImagePullServiceIDs {
			add(id)
		}
		for _, secret := range row.Plan.Resources.Secrets {
			add(secret.ServiceID)
		}
	}
	if len(summary.Services) == 0 || len(summary.Services) > 32 {
		return ErrRuntimePreparation
	}
	found := false
	for i, v := range index.Rows {
		if v.ID == summary.ID {
			if v.Digest != summary.Digest {
				return ErrRuntimePreparation
			}
			index.Rows[i] = summary
			found = true
		}
	}
	if !found {
		if len(index.Rows) >= 128 {
			removed := false
			for i, v := range index.Rows {
				if v.Finalized {
					index.Rows = append(index.Rows[:i], index.Rows[i+1:]...)
					removed = true
					break
				}
			}
			if !removed {
				return errors.New("Retained unresolved preparation capacity is full; keep original IDs and resolve existing work before reviewing more")
			}
		}
		index.Rows = append(index.Rows, summary)
	}
	raw, err := json.MarshalIndent(index, "", "  ")
	if err != nil || len(raw)+1 > 256<<10 {
		return ErrRuntimePreparation
	}
	return durableRunJSON(file, index)
}
func (s *Server) ListRuntimePreparations(ctx context.Context, actor management.ExecutionActorRef) ([]RuntimePreparationSummary, error) {
	if err := s.kubernetesAuthority(ctx, actor, false, true, 0); err != nil {
		return nil, err
	}
	_, index, err := s.readRuntimePreparationIndex(actor)
	if err != nil {
		return nil, err
	}
	if err = s.kubernetesAuthority(ctx, actor, false, true, 0); err != nil {
		return nil, err
	}
	return index.Rows, nil
}
