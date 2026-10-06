package product

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalReleaseUsesPrivatePinnedRuntimeConfigurationAndRemovesItsEnvFile(t *testing.T) {
	for _, failRun := range []bool{false, true} {
		t.Run(map[bool]string{false: "healthy", true: "start-failure"}[failRun], func(t *testing.T) {
			s, mux := journeyFixture(t)
			s.registerApplicationConfiguration(mux)
			p := createJourneyProject(t, mux, "configured-preview")
			base := "/v1/product/projects/" + p.ID
			if err := os.WriteFile(filepath.Join(p.Path, "Dockerfile"), []byte("FROM scratch\n"), 0600); err != nil {
				t.Fatal(err)
			}
			commitPreviewFixture(t, p.Path)
			secret := "fixture-private-value=with spaces#literal"
			journeyRequest(t, mux, "POST", base+"/configuration", map[string]any{"expectedRevision": 0, "updates": []map[string]any{
				{"name": "APP_SECRET", "value": secret}, {"name": "APP_MODE", "value": "fixture"}, {"name": "APP_EMPTY", "value": ""},
			}}, 200)
			t.Setenv("ANTHROPIC_API_KEY", "host-model-key-must-not-be-injected")
			t.Setenv("APP_EMPTY", "host-value-must-not-be-inherited")
			health := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) }))
			defer health.Close()
			envPath := ""
			runs := 0
			s.runner = func(ctx context.Context, executable string, args []string, dir string) (string, error) {
				if executable != "docker" {
					return RunCommand(ctx, executable, args, dir)
				}
				if strings.Contains(strings.Join(args, " "), secret) {
					t.Fatal("application secret entered process argv")
				}
				switch args[0] {
				case "build":
					if strings.Contains(strings.Join(args, " "), "--env") || strings.Contains(strings.Join(args, " "), "--build-arg") {
						t.Fatal("runtime configuration was injected at build time")
					}
					if err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
						if err != nil || entry.IsDir() {
							return err
						}
						data, readErr := os.ReadFile(path)
						if strings.Contains(string(data), secret) {
							t.Fatal("build context contains private configuration")
						}
						return readErr
					}); err != nil {
						t.Fatal(err)
					}
					// Saving a new revision during build must not alter this release.
					journeyRequest(t, mux, "POST", base+"/configuration", map[string]any{"expectedRevision": 1, "updates": []map[string]any{{"name": "APP_SECRET", "value": "newer-configuration-value"}}}, 200)
					return "built", nil
				case "run":
					runs++
					for i, arg := range args {
						if arg == "--env-file" && i+1 < len(args) {
							envPath = args[i+1]
						}
					}
					if envPath == "" || filepath.Clean(filepath.Dir(envPath)) != filepath.Clean(s.stateDir) {
						t.Fatal("runtime env file is not outside the project in private state")
					}
					assertPrivateFile(t, envPath)
					data, err := os.ReadFile(envPath)
					if err != nil || string(data) != "APP_EMPTY=\nAPP_MODE=fixture\nAPP_SECRET="+secret+"\n" {
						t.Fatalf("unpinned or nonliteral environment: %q %v", data, err)
					}
					if strings.Contains(string(data), "host-model-key") {
						t.Fatal("host model credentials leaked to application")
					}
					if failRun {
						return "", errors.New("daemon rejected " + secret)
					}
					return strings.Repeat("d", 64), nil
				case "port":
					if _, err := os.Stat(envPath); !errors.Is(err, os.ErrNotExist) {
						t.Fatal("runtime env file survived the Docker run invocation")
					}
					return strings.TrimPrefix(health.URL, "http://"), nil
				default:
					t.Fatalf("unexpected Docker operation %v", args)
					return "", nil
				}
			}
			status := 201
			if failRun {
				status = 422
			}
			response := journeyRequest(t, mux, "POST", base+"/releases", map[string]any{"target": "local"}, status)
			var row Release
			if err := json.Unmarshal(response.Body.Bytes(), &row); err != nil {
				t.Fatal(err)
			}
			if row.ConfigurationRevision != 1 || runs != 1 {
				t.Fatalf("release lost pinned configuration revision: %#v", row)
			}
			if _, err := os.Stat(envPath); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("private environment file was not removed")
			}
			metadata := journeyRequest(t, mux, "GET", base+"/configuration", nil, 200).Body.String()
			history := journeyRequest(t, mux, "GET", base+"/releases", nil, 200).Body.String()
			stored, err := os.ReadFile(s.releaseFile(p.ID))
			if err != nil {
				t.Fatal(err)
			}
			for _, data := range []string{response.Body.String(), metadata, history, string(stored)} {
				if strings.Contains(data, secret) || strings.Contains(data, "newer-configuration-value") {
					t.Fatal("configuration value escaped through release/configuration metadata")
				}
			}
		})
	}
}

func TestPreviewConfigurationRejectsEnvFileInjectionWithoutWriting(t *testing.T) {
	s, _ := journeyFixture(t)
	for _, values := range []map[string]string{{"BAD=NAME": "value"}, {"APP_VALUE": "one\nOTHER=two"}, {"APP_VALUE": "one\rOTHER=two"}, {"APP_VALUE": "one\x00two"}} {
		if path, err := s.writePreviewEnvironment(values); err == nil || path != "" {
			t.Fatal("unsafe environment record was written")
		}
	}
	if path, err := s.writePreviewEnvironment(nil); err != nil || path != "" {
		t.Fatal("empty configuration created a file")
	}
	files, err := filepath.Glob(filepath.Join(s.stateDir, "preview-env-*"))
	if err != nil || len(files) != 0 {
		t.Fatalf("rejected environment left files: %v %v", files, err)
	}
}
