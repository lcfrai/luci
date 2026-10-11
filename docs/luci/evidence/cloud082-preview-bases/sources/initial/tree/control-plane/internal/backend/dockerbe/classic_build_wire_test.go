package dockerbe

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/build"
	"github.com/docker/docker/client"
)

// Docker v28 build_routes.go rejects non-nil SecurityOpt on Linux: that build
// parameter supports Windows credential specs only. Exercise the pinned SDK's
// actual wire projection of production options, not just a fake's acceptance.
func TestClassicBuildWireSupportsLinuxAndRetainsLimits(t *testing.T) {
	for _, kind := range []string{"runtime", "preview"} {
		t.Run(kind, func(t *testing.T) {
			var options build.ImageBuildOptions
			if kind == "runtime" {
				b, f, spec := newRuntimeImageProbe(t)
				p := runtimeFixturePrepare(t, b, spec)
				id := runtimeFixtureOutput(f, p)
				f.buildStream = runtimeFixtureAux(t, id) + runtimeFixtureSuccess(t, id)
				if _, err := b.BuildRuntimeImage(context.Background(), p); err != nil {
					t.Fatal(err)
				}
				options = f.buildOptions[0]
			} else {
				// The preview path has its own production build-option construction.
				b, f, spec := newPreviewProbe(t)
				p, err := b.PreparePreview(context.Background(), spec)
				if err != nil {
					t.Fatal(err)
				}
				id := previewBuiltImage(f, p)
				aux, _ := json.Marshal(map[string]any{"aux": map[string]string{"ID": id}})
				f.stream = string(aux) + "\n" + previewStreamFrame("Successfully built "+strings.TrimPrefix(id, "sha256:")[:12]+"\n")
				if _, err = b.BuildPreview(context.Background(), p, allowPreviewMutation); err != nil {
					t.Fatal(err)
				}
				options = f.buildOptions[0]
			}
			seen := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seen = true
				if r.Method != "POST" || !strings.HasSuffix(r.URL.Path, "/build") {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
					http.Error(w, "unexpected", 400)
					return
				}
				q := r.URL.Query()
				if _, present := q["securityopt"]; present {
					http.Error(w, "security options are not supported on this platform", 400)
					return
				}
				if q.Get("version") != "1" || q.Get("memory") == "" || q.Get("memory") == "0" || q.Get("memswap") != q.Get("memory") || q.Get("cpuperiod") != "100000" || q.Get("cpuquota") == "" || q.Get("cpuquota") == "0" {
					t.Errorf("missing classic hard limits: %v", q)
					http.Error(w, "missing limits", 400)
					return
				}
				if _, err := io.Copy(io.Discard, r.Body); err != nil {
					t.Error(err)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, "{}\n")
			}))
			defer server.Close()
			c, err := client.NewClientWithOpts(client.WithHost(server.URL), client.WithVersion("1.51"))
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			response, err := c.ImageBuild(context.Background(), strings.NewReader("fixture body"), options)
			if err != nil {
				t.Fatal("production build options rejected by the pinned Linux API contract", err)
			}
			defer response.Body.Close()
			if _, err = io.Copy(io.Discard, response.Body); err != nil {
				t.Fatal(err)
			}
			if !seen {
				t.Fatal("SDK request not observed")
			}
		})
	}
}
