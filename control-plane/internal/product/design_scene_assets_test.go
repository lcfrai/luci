package product

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

func sceneAssetPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	var out bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(&out, img); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}
func sceneAssetCommit(t *testing.T, dir, path string, data []byte) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, path), data, 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "--", path}, {"-c", "user.name=Image fixture", "-c", "user.email=image@example.com", "commit", "-m", "Pin image"}} {
		if _, err := RunCommand(context.Background(), "git", args, dir); err != nil {
			t.Fatal(err)
		}
	}
	out, err := RunCommand(context.Background(), "git", []string{"rev-parse", "HEAD"}, dir)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(out)
}
func TestDesignRasterUsesImmutableBlobNotCurrentPath(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "immutable-image")
	data := sceneAssetPNG(t, 3, 2)
	commit := sceneAssetCommit(t, p.Path, "hero.png", data)
	pin, actual, err := s.designRaster(context.Background(), p.Path, "hero.png", commit)
	if err != nil || !bytes.Equal(data, actual) || pin.Bytes != len(data) || pin.Width != 3 || pin.Height != 2 {
		t.Fatal("binding", pin, err)
	}
	changed := sceneAssetPNG(t, 7, 4)
	sceneAssetCommit(t, p.Path, "hero.png", changed)
	if err := os.Remove(filepath.Join(p.Path, "hero.png")); err != nil {
		t.Fatal(err)
	}
	next, actual, err := s.designRaster(context.Background(), p.Path, "hero.png", commit)
	if err != nil || next != pin || !bytes.Equal(actual, data) {
		t.Fatal("live path changed approved bytes", err)
	}
	other := createJourneyProject(t, mux, "foreign-image")
	if _, _, err := s.designRaster(context.Background(), other.Path, "hero.png", commit); err == nil {
		t.Fatal("foreign project object accepted")
	}
}
func TestDesignRasterRejectsUnsafeOversizeMalformedAndUnsupported(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "invalid-images")
	for name, data := range map[string][]byte{"oversize.png": bytes.Repeat([]byte{'a'}, designRasterBytes), "broken.png": []byte("\x89PNG\r\n\x1a\n"), "vector.svg": []byte(`<svg onload="alert(1)"/>`), "wide.png": sceneAssetPNG(t, 4097, 1)} {
		commit := sceneAssetCommit(t, p.Path, name, data)
		if _, _, err := s.designRaster(context.Background(), p.Path, name, commit); err == nil {
			t.Fatal("invalid image accepted", name)
		}
	}
	commit := sceneAssetCommit(t, p.Path, "safe.png", sceneAssetPNG(t, 2, 2))
	for _, path := range []string{"../safe.png", ".env", ".git/config", "safe.png\x00", "safe.png\\other"} {
		if _, _, err := s.designRaster(context.Background(), p.Path, path, commit); err == nil {
			t.Fatal("unsafe path accepted", path)
		}
	}
	if _, _, err := s.designRaster(context.Background(), p.Path, "safe.png", "HEAD"); err == nil {
		t.Fatal("mutable ref accepted")
	}
}
func TestDesignRasterRejectsSymlinkTreeAndTruncatedRunner(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "raster-runner-bounds")
	commit := sceneAssetCommit(t, p.Path, "safe.png", sceneAssetPNG(t, 2, 2))
	original := s.runner
	for _, mode := range []string{"120000", "160000", "040000"} {
		s.runner = func(ctx context.Context, name string, args []string, dir string) (string, error) {
			for _, arg := range args {
				if arg == "ls-tree" {
					return mode + " blob " + strings.Repeat("a", 40) + "\tsafe.png\x00", nil
				}
			}
			return original(ctx, name, args, dir)
		}
		if _, _, err := s.designRaster(context.Background(), p.Path, "safe.png", commit); err == nil {
			t.Fatal("non-regular object accepted", mode)
		}
	}
	s.runner = func(ctx context.Context, name string, args []string, dir string) (string, error) {
		for i, arg := range args {
			if arg == "cat-file" && i+1 < len(args) && args[i+1] == "blob" {
				return "truncated", nil
			}
		}
		return original(ctx, name, args, dir)
	}
	if _, _, err := s.designRaster(context.Background(), p.Path, "safe.png", commit); err == nil {
		t.Fatal("truncated runner output accepted")
	}
}

func TestDesignRasterRejectsDeclaredSizeAndForgedPin(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "raster-size-and-digest")
	data := sceneAssetPNG(t, 2, 2)
	commit := sceneAssetCommit(t, p.Path, "safe.png", data)
	pin, _, err := s.designRaster(context.Background(), p.Path, "safe.png", commit)
	if err != nil {
		t.Fatal(err)
	}
	original := s.runner
	for _, size := range []string{"0", "2097152", "99999"} {
		s.runner = func(ctx context.Context, name string, args []string, dir string) (string, error) {
			for i, arg := range args {
				if arg == "cat-file" && i+1 < len(args) && args[i+1] == "-s" {
					return size, nil
				}
			}
			return original(ctx, name, args, dir)
		}
		if _, _, err := s.designRaster(context.Background(), p.Path, "safe.png", commit); err == nil {
			t.Fatal("declared size accepted", size)
		}
	}
	s.runner = original
	for _, change := range []func(*DesignRasterPin){func(p *DesignRasterPin) { p.GitBlob = strings.Repeat("0", 40) }, func(p *DesignRasterPin) { p.Digest = strings.Repeat("0", 64) }, func(p *DesignRasterPin) { p.Width++ }, func(p *DesignRasterPin) { p.MimeType = "image/jpeg" }} {
		forged := pin
		change(&forged)
		plan := ProjectDesignPlan{Assets: []DesignAsset{{ID: "image", Path: "safe.png", Content: &forged}}}
		if s.validateDesignAssetBindings(context.Background(), p.Path, plan) == nil {
			t.Fatal("caller-claimed binding accepted", forged)
		}
	}
	if _, _, err := s.designRaster(context.Background(), p.Path, "safe.png", strings.Repeat("0", 40)); err == nil {
		t.Fatal("missing Git object silently substituted")
	}
}
func TestDesignAssetRetainedRevisionAndPinValidation(t *testing.T) {
	_, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "image-revisions")
	base := "/v1/product/projects/" + p.ID + "/design-plan"
	data := sceneAssetPNG(t, 3, 2)
	commit := sceneAssetCommit(t, p.Path, "hero.png", data)
	var binding struct {
		ProjectID string          `json:"projectId"`
		Path      string          `json:"path"`
		Content   DesignRasterPin `json:"content"`
	}
	json.Unmarshal(journeyRequest(t, mux, "GET", base+"/asset-binding?path=hero.png&commit="+commit, nil, 200).Body.Bytes(), &binding)
	plan := ProjectDesignPlan{ProjectID: p.ID, Status: "draft", Screens: []DesignScreen{}, Tokens: []DesignToken{}, Assets: []DesignAsset{{ID: "hero", Path: "hero.png", Content: &binding.Content}}}
	save := func(expected int, code int) {
		t.Helper()
		json.Unmarshal(journeyRequest(t, mux, "POST", base, map[string]any{"expectedRevision": expected, "expectedBriefRevision": 0, "expectedSpecificationRevision": 0, "plan": plan}, code).Body.Bytes(), &plan)
	}
	save(0, 200)
	pinned := *plan.Assets[0].Content
	plan.Assets[0].Content.Digest = strings.Repeat("0", 64)
	journeyRequest(t, mux, "POST", base, map[string]any{"expectedRevision": 1, "expectedBriefRevision": 0, "expectedSpecificationRevision": 0, "plan": plan}, 422)
	plan.Assets = []DesignAsset{}
	save(1, 200)
	sceneAssetCommit(t, p.Path, "hero.png", sceneAssetPNG(t, 8, 6))
	endpoint := base + "/1/assets/hero?digest=" + pinned.Digest
	w := journeyRequest(t, mux, "GET", endpoint, nil, 200)
	if !bytes.Equal(w.Body.Bytes(), data) || w.Header().Get("Cache-Control") != "private, no-store" || w.Header().Get("X-Content-Type-Options") != "nosniff" || w.Header().Get("Content-Type") != "image/png" {
		t.Fatal("old revision content/headers changed")
	}
	journeyRequest(t, mux, "GET", base+"/99/assets/hero?digest="+pinned.Digest, nil, 404)
	journeyRequest(t, mux, "GET", base+"/1/assets/hero?digest="+strings.Repeat("0", 64), nil, 404)
	// Read requests cannot mutate the saved version or working file.
	var current ProjectDesignPlan
	json.Unmarshal(journeyRequest(t, mux, "GET", base, nil, 200).Body.Bytes(), &current)
	if current.Revision != 2 || len(current.Assets) != 0 {
		t.Fatal("GET mutated saved design")
	}
	for _, access := range []management.TeamAccess{{Team: management.Team{ID: "team-test"}, ProjectID: p.ID}, {Team: management.Team{ID: "team-test"}, ProjectID: "foreign", Permissions: management.TeamPermissions{Read: true}}} {
		req := httptest.NewRequest("GET", endpoint, nil).WithContext(management.WithTeamAccess(context.Background(), access))
		res := httptest.NewRecorder()
		mux.ServeHTTP(res, req)
		if res.Code != 403 {
			t.Fatal("revoked/foreign read accepted", res.Code)
		}
		req = httptest.NewRequest("GET", base+"/asset-binding?"+url.Values{"path": {"hero.png"}, "commit": {commit}}.Encode(), nil).WithContext(management.WithTeamAccess(context.Background(), access))
		res = httptest.NewRecorder()
		mux.ServeHTTP(res, req)
		if res.Code != 403 {
			t.Fatal("revoked/foreign binding accepted", res.Code)
		}
	}
}
