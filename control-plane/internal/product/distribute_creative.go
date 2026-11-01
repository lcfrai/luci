package product

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

const creativeSVGBytes = 12 << 20

var errCreative = errors.New("This composition cannot be generated completely. Choose a saved frame or pinned PNG/JPEG, shorten overflowing text, or use a type-only composition; your saved work is unchanged.")

// Only this editable brief crosses the request boundary. Captured visual data
// is derived from the exact saved Design revision, never accepted from a caller.
type LaunchCreativeBrief struct {
	Product    string `json:"product"`
	Headline   string `json:"headline"`
	Body       string `json:"body"`
	CTA        string `json:"cta"`
	Alt        string `json:"alt"`
	Format     string `json:"format"`
	Background string `json:"background"`
	Foreground string `json:"foreground"`
	Accent     string `json:"accent"`
	FrameID    string `json:"frameId"`
	AssetID    string `json:"assetId"`
}

type LaunchCreative struct {
	Version     int                 `json:"version"`
	Renderer    string              `json:"renderer"`
	Brief       LaunchCreativeBrief `json:"brief"`
	Frame       *DesignSceneFrame   `json:"frame,omitempty"`
	FrameDigest string              `json:"frameDigest,omitempty"`
	Assets      []DesignAsset       `json:"assets"`
}

func creativeText(s string, limit int, required bool) bool {
	return utf8.ValidString(s) && len(s) <= limit && (!required || strings.TrimSpace(s) != "") && validArtifactXMLText(s)
}

func validCreativeBrief(v LaunchCreativeBrief) bool {
	return creativeText(v.Product, 120, true) && creativeText(v.Headline, 240, true) && creativeText(v.Body, 800, false) && creativeText(v.CTA, 80, false) && creativeText(v.Alt, 1000, true) &&
		(v.Format == "wide" || v.Format == "square") && sceneColor.MatchString(v.Background) && v.Background != "transparent" && sceneColor.MatchString(v.Foreground) && v.Foreground != "transparent" && sceneColor.MatchString(v.Accent) && v.Accent != "transparent" &&
		(v.FrameID == "" || designIDPattern.MatchString(v.FrameID)) && (v.AssetID == "" || designIDPattern.MatchString(v.AssetID)) && !(v.FrameID != "" && v.AssetID != "")
}

func validLaunchCreative(row DistributeArtifact) bool {
	if row.Kind != "launch-creative" {
		return row.Creative == nil
	}
	v := row.Creative
	if v == nil || v.Version != 1 || v.Renderer != "luci-launch-svg-v1" || !validCreativeBrief(v.Brief) || v.Assets == nil || len(v.Assets) > 4 {
		return false
	}
	if (v.Frame == nil) != (v.Brief.FrameID == "") {
		return false
	}
	if v.Frame != nil {
		if v.Frame.ID != v.Brief.FrameID || v.FrameDigest != reviewDigest(*v.Frame) {
			return false
		}
		// Reuse the scene shape validator without inventing live interactions or
		// requiring unrelated destination frames/component-library definitions.
		frame := *v.Frame
		frame.Nodes = append([]DesignSceneNode{}, frame.Nodes...)
		for i := range frame.Nodes {
			frame.Nodes[i].ComponentID = ""
			frame.Nodes[i].Behavior = DesignSceneBehavior{Kind: "none"}
		}
		p := ProjectDesignPlan{Scene: &ProjectDesignScene{Version: 1, Frames: []DesignSceneFrame{frame}}, Screens: []DesignScreen{{ID: frame.ScreenID, States: []string{frame.State}}}, Assets: v.Assets}
		if validateDesignScene(p) != nil {
			return false
		}
	} else if v.FrameDigest != "" {
		return false
	}
	seen := map[string]bool{}
	for _, a := range v.Assets {
		if seen[a.ID] || !designIDPattern.MatchString(a.ID) || !validDesignRasterPin(a.Content) || !creativeText(a.Path, 1024, true) || !creativeText(a.Licence, 4000, false) {
			return false
		}
		seen[a.ID] = true
	}
	if v.Brief.AssetID != "" && (len(v.Assets) != 1 || v.Assets[0].ID != v.Brief.AssetID) {
		return false
	}
	if v.Frame == nil && v.Brief.AssetID == "" && len(v.Assets) != 0 {
		return false
	}
	raw, err := json.Marshal(v)
	return err == nil && len(raw) <= 144<<10
}

func (s *Server) creativeAuthority(ctx context.Context, id string, edit bool) error {
	if edit {
		_, err := s.researchAccess(ctx, id, false)
		return err
	}
	actor, _, ok := management.ActorFromContext(ctx)
	if !ok || actor.Disabled {
		return management.ErrExecutionAuthority
	}
	ref := management.ExecutionActorRef{UserID: actor.ID, ProjectID: id}
	if access, scoped := management.TeamAccessFromContext(ctx); scoped {
		ref.TeamID = access.Team.ID
	}
	return s.feedbackAccess(ctx, ref, false)
}

func (s *Server) artifactDraftForRequestLocked(ctx context.Context, p project.Project, kind, runID string, brief *LaunchCreativeBrief) (DistributeArtifact, error) {
	if kind != "launch-creative" {
		if brief != nil {
			return DistributeArtifact{}, errCreative
		}
		return s.artifactDraftForKindLocked(ctx, p, kind, runID)
	}
	if brief == nil || !validCreativeBrief(*brief) || s.creativeAuthority(ctx, p.ID, true) != nil {
		return DistributeArtifact{}, errCreative
	}
	source, state, _, _, err := s.artifactSourceForKindLocked(ctx, p, kind, runID)
	if err != nil {
		return DistributeArtifact{}, err
	}
	plan := currentDesignPlan(state)
	v := &LaunchCreative{Version: 1, Renderer: "luci-launch-svg-v1", Brief: *brief, Assets: []DesignAsset{}}
	wanted := map[string]bool{}
	if brief.FrameID != "" {
		if plan.Scene == nil || validateDesignScene(plan) != nil {
			return DistributeArtifact{}, errCreative
		}
		for _, frame := range plan.Scene.Frames {
			if frame.ID == brief.FrameID {
				copy := frame
				v.Frame = &copy
				v.FrameDigest = reviewDigest(frame)
			}
		}
		if v.Frame == nil {
			return DistributeArtifact{}, errCreative
		}
		for _, node := range v.Frame.Nodes {
			if node.Kind == "image" {
				wanted[node.AssetID] = true
			}
		}
	} else if brief.AssetID != "" {
		wanted[brief.AssetID] = true
	}
	for _, asset := range plan.Assets {
		if wanted[asset.ID] {
			v.Assets = append(v.Assets, asset)
			delete(wanted, asset.ID)
		}
	}
	if len(wanted) != 0 {
		return DistributeArtifact{}, errCreative
	}
	ref := fmt.Sprintf("design:%d:%s", source.DesignRevision, source.DesignDigest)
	row := DistributeArtifact{ProjectID: p.ID, Kind: kind, Source: source, SourceDigest: reviewDigest(source), Title: "Launch creative: " + brief.Product, Creative: v,
		Sections: []DistributeArtifactSection{{ID: "creative-brief", Heading: "Launch creative brief", Body: "Composed artwork from the saved product context. A design frame is not an implemented product screenshot.", Bullets: []string{}, SourceRefs: []string{ref, fmt.Sprintf("gtm:%d:%s", source.GTMRevision, source.GTMDigest)}}},
		Gaps:     []string{"Artwork does not establish implemented behavior, customer outcomes, licence clearance or publication."}, Assumptions: []string{"Review every marketing claim and selected asset licence before external use."}, State: "draft"}
	if !validLaunchCreative(row) {
		return DistributeArtifact{}, errCreative
	}
	// Layout validation is independent of loading raster bytes; it rejects a
	// composition that would otherwise hide overflowing authored content.
	if _, err = renderLaunchCreative(row, nil, true); err != nil {
		return DistributeArtifact{}, err
	}
	return row, nil
}

func (s *Server) creativeSVG(ctx context.Context, p project.Project, row DistributeArtifact) ([]byte, error) {
	if !validLaunchCreative(row) || s.creativeAuthority(ctx, p.ID, false) != nil {
		return nil, errCreative
	}
	images := map[string][]byte{}
	for _, asset := range row.Creative.Assets {
		if ctx.Err() != nil || s.creativeAuthority(ctx, p.ID, false) != nil {
			return nil, errCreative
		}
		pin, raw, err := s.designRaster(ctx, p.Path, asset.Path, asset.Content.SourceCommit)
		if err != nil || pin != *asset.Content {
			return nil, errCreative
		}
		images[asset.ID] = raw
	}
	body, err := renderLaunchCreative(row, images, false)
	if err != nil || ctx.Err() != nil || s.creativeAuthority(ctx, p.ID, false) != nil {
		return nil, errCreative
	}
	return body, nil
}

func creativeFailure(w http.ResponseWriter) {
	journeyError(w, 422, "creative_unavailable", errCreative.Error())
}

func creativeSVGDigest(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func (s *Server) exportLaunchCreative(w http.ResponseWriter, r *http.Request) {
	if !artifactAccess(w, r, false) {
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	digest := r.URL.Query().Get("digest")
	if !hexDigest(digest) || s.creativeAuthority(r.Context(), p.ID, false) != nil {
		creativeFailure(w)
		return
	}
	s.mu.RLock()
	_, ledger, err := s.readArtifactLedger(p)
	var row DistributeArtifact
	if record := artifactRecord(&ledger, r.PathValue("artifactId")); err == nil && record != nil {
		row = record.Revisions[len(record.Revisions)-1]
	}
	s.mu.RUnlock()
	if err != nil || row.Digest != digest || row.ProjectID != p.ID || row.Creative == nil {
		creativeFailure(w)
		return
	}
	body, err := s.creativeSVG(r.Context(), p, row)
	if err != nil {
		creativeFailure(w)
		return
	}
	// Raster reads can wait on Git. Publish only the still-selected immutable row
	// under fresh access; later source edits do not erase historical artwork.
	s.mu.RLock()
	_, latest, e := s.readArtifactLedger(p)
	record := artifactRecord(&latest, row.ID)
	unchanged := e == nil && record != nil && record.Revisions[len(record.Revisions)-1].Digest == digest
	s.mu.RUnlock()
	if !unchanged || s.creativeAuthority(r.Context(), p.ID, false) != nil {
		creativeFailure(w)
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src data:; style-src 'none'; sandbox")
	w.Header().Set("X-Luci-Content-SHA256", creativeSVGDigest(body))
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="luci-launch-%s-r%d.svg"`, row.ID, row.Revision))
	w.Header().Set("Content-Length", fmt.Sprint(len(body)))
	_, _ = w.Write(body)
}
