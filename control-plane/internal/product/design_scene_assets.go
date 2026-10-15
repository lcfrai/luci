package product

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

// Strictly below the shared runner's stdout limit: saturation is never content.
const designRasterBytes = 2 << 20

var errDesignRaster = errors.New("The immutable PNG/JPEG asset is unavailable or invalid. Pin a regular project Git file below 2 MiB, at most 4096 pixels per side.")

func designAssetReadAllowed(ctx context.Context, id string) bool {
	if access, scoped := management.TeamAccessFromContext(ctx); scoped {
		return access.ProjectID == id && access.Permissions.Read && access.Team.ID != ""
	}
	return management.TeamRuntimeIDFromContext(ctx) == ""
}

// Only immutable tree objects are read. No working file, checkout, filter or ref mutation.
func (s *Server) designRaster(ctx context.Context, dir, path, commit string) (DesignRasterPin, []byte, error) {
	fail := func() (DesignRasterPin, []byte, error) { return DesignRasterPin{}, nil, errDesignRaster }
	if !deliverySHA.MatchString(commit) || path == "" || len(path) > 1024 || strings.ContainsAny(path, "\\\r\n\x00") || blockedProjectFile(path) || !validBackupEntry("files/"+path) {
		return fail()
	}
	kind, err := s.journeyGit(ctx, dir, "cat-file", "-t", commit)
	if err != nil || strings.TrimSpace(kind) != "commit" {
		return fail()
	}
	tree, err := s.journeyGit(ctx, dir, "ls-tree", "-z", commit, "--", ":(literal)"+path)
	if err != nil || !strings.HasSuffix(tree, "\x00") || strings.Count(tree, "\x00") != 1 {
		return fail()
	}
	parts := strings.SplitN(strings.TrimSuffix(tree, "\x00"), "\t", 2)
	if len(parts) != 2 || parts[1] != path {
		return fail()
	}
	header := strings.Fields(parts[0])
	if len(header) != 3 || (header[0] != "100644" && header[0] != "100755") || header[1] != "blob" || !deliverySHA.MatchString(header[2]) {
		return fail()
	}
	sizeText, err := s.journeyGit(ctx, dir, "cat-file", "-s", header[2])
	size, parseErr := strconv.Atoi(strings.TrimSpace(sizeText))
	if err != nil || parseErr != nil || size < 1 || size >= designRasterBytes {
		return fail()
	}
	raw, err := s.journeyGit(ctx, dir, "cat-file", "blob", header[2])
	if err != nil || len(raw) != size || ctx.Err() != nil {
		return fail()
	}
	data := []byte(raw)
	// Git's SHA-1 object identifier is a format binding; raw SHA-256 pins delivered content.
	object := sha1.New()
	_, _ = object.Write([]byte("blob " + strconv.Itoa(size) + "\x00"))
	_, _ = object.Write(data)
	if hex.EncodeToString(object.Sum(nil)) != header[2] {
		return fail()
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "png" && format != "jpeg") || config.Width < 1 || config.Height < 1 || config.Width > 4096 || config.Height > 4096 || config.Width*config.Height > 16<<20 {
		return fail()
	}
	// Configuration alone can accept a truncated body; validate the bounded raster too.
	if _, decodedFormat, err := image.Decode(bytes.NewReader(data)); err != nil || decodedFormat != format {
		return fail()
	}
	digest := sha256.Sum256(data)
	mime := "image/" + format
	return DesignRasterPin{SourceCommit: commit, GitBlob: header[2], Digest: hex.EncodeToString(digest[:]), MimeType: mime, Bytes: size, Width: config.Width, Height: config.Height}, data, nil
}

func (s *Server) validateDesignAssetBindings(ctx context.Context, dir string, plan ProjectDesignPlan) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	verified := map[string]DesignRasterPin{}
	for _, asset := range plan.Assets {
		if asset.Content == nil {
			continue
		}
		key := asset.Path + "\x00" + asset.Content.SourceCommit
		actual, found := verified[key]
		if !found {
			var err error
			actual, _, err = s.designRaster(ctx, dir, asset.Path, asset.Content.SourceCommit)
			if err != nil {
				return err
			}
			verified[key] = actual
		}
		if actual != *asset.Content {
			return errDesignRaster
		}
	}
	return ctx.Err()
}

func designAssetHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
}
func (s *Server) bindDesignAsset(w http.ResponseWriter, r *http.Request) {
	designAssetHeaders(w)
	if !designAssetReadAllowed(r.Context(), r.PathValue("id")) {
		problem(w, 403, "forbidden", "Current project read access is required.")
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	pin, _, err := s.designRaster(ctx, p.Path, r.URL.Query().Get("path"), r.URL.Query().Get("commit"))
	if err != nil {
		problem(w, 422, "asset_unavailable", errDesignRaster.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"projectId": p.ID, "path": r.URL.Query().Get("path"), "content": pin})
}
func (s *Server) getDesignAsset(w http.ResponseWriter, r *http.Request) {
	designAssetHeaders(w)
	if !designAssetReadAllowed(r.Context(), r.PathValue("id")) {
		problem(w, 403, "forbidden", "Current project read access is required.")
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	revision, err := strconv.Atoi(r.PathValue("revision"))
	if err != nil || revision < 1 || !hexDigest(r.URL.Query().Get("digest")) {
		problem(w, 400, "invalid_asset", "An exact saved revision and digest are required.")
		return
	}
	s.mu.RLock()
	state, readErr := s.readApprovedJourney(p.Path, p.ID)
	var plan *ProjectDesignPlan
	if readErr == nil {
		if state.DesignPlan != nil && state.DesignPlan.Revision == revision {
			copy := *state.DesignPlan
			plan = &copy
		}
		for _, candidate := range state.DesignPlanRevisions {
			if candidate.Revision == revision {
				copy := candidate
				plan = &copy
				break
			}
		}
	}
	s.mu.RUnlock()
	if readErr != nil {
		problem(w, 500, "history_unavailable", "Saved design history is unavailable.")
		return
	}
	if plan == nil || plan.ProjectID != p.ID {
		problem(w, 404, "revision_unavailable", "That exact design revision is not retained here.")
		return
	}
	var asset *DesignAsset
	for _, candidate := range plan.Assets {
		if candidate.ID == r.PathValue("assetId") {
			copy := candidate
			asset = &copy
			break
		}
	}
	if asset == nil || !validDesignRasterPin(asset.Content) || asset.Content.Digest != r.URL.Query().Get("digest") {
		problem(w, 404, "asset_unavailable", "That revision has no matching pinned image.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	pin, data, err := s.designRaster(ctx, p.Path, asset.Path, asset.Content.SourceCommit)
	if err != nil || pin != *asset.Content || ctx.Err() != nil {
		problem(w, 422, "asset_unavailable", errDesignRaster.Error())
		return
	}
	w.Header().Set("Content-Type", pin.MimeType)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
