package product

import (
	"context"
	"encoding/json"
	"errors"
	"golang.org/x/crypto/ssh"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestDeployKeyReconcilesAmbiguousWritesWithoutRetainingPrivateMaterial(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerCloudReleases(mux)
	p := createJourneyProject(t, mux, "private-app")
	var keys []CoolifyPrivateKey
	remoteCreates, githubCreates := 0, 0
	var githubKeys []githubDeployKey
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") != "Bearer "+testCoolifyToken {
			t.Error("missing Coolify authentication")
		}
		switch r.Method + " " + r.URL.Path {
		case "GET /api/v1/security/keys":
			_ = json.NewEncoder(w).Encode(keys)
		case "POST /api/v1/security/keys":
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			signer, err := ssh.ParsePrivateKey([]byte(body["private_key"]))
			if err != nil {
				t.Fatal("not an actual OpenSSH private key", err)
			}
			remoteCreates++
			keys = append(keys, CoolifyPrivateKey{UUID: "key-owned", Name: body["name"], PublicKey: strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))})
			// Provider accepted it but the response is ambiguous. A retry must look
			// up the saved public identity, never generate another private key.
			w.WriteHeader(500)
			_, _ = w.Write([]byte(body["private_key"]))
		default:
			t.Error("unexpected provider request", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	s.targets["target-test"] = DeploymentTarget{ID: "target-test", Name: "fixture", BaseURL: server.URL, Token: testCoolifyToken, ServerUUID: "server-test"}
	s.runner = func(_ context.Context, exe string, args []string, dir string) (string, error) {
		if exe != "gh" || dir != s.stateDir || !slices.Contains(args, "repos/example/private/keys") {
			t.Fatalf("unsafe deploy-key command: %s %q in %s", exe, args, dir)
		}
		if !slices.Contains(args, "POST") {
			data, _ := json.Marshal([][]githubDeployKey{githubKeys})
			return string(data), nil
		}
		input := args[slices.Index(args, "--input")+1]
		assertPrivateFile(t, input)
		data, err := os.ReadFile(input)
		if err != nil {
			return "", err
		}
		var req struct {
			Title    string `json:"title"`
			Key      string `json:"key"`
			ReadOnly bool   `json:"read_only"`
		}
		if json.Unmarshal(data, &req) != nil || !req.ReadOnly {
			t.Fatal("GitHub write access requested")
		}
		githubCreates++
		githubKeys = append(githubKeys, githubDeployKey{ID: 1, Title: req.Title, Key: req.Key, ReadOnly: true})
		return "", errors.New("ambiguous network failure must not be exposed")
	}
	path := "/v1/product/projects/" + p.ID + "/releases/deploy-key"
	req := map[string]string{"targetId": "target-test", "repositoryHTTPSGithub": "https://github.com/example/private.git"}
	first := journeyRequest(t, mux, "POST", path, req, 409)
	if strings.Contains(first.Body.String(), "PRIVATE KEY") {
		t.Fatal("provider echoed private key into response")
	}
	journeyRequest(t, mux, "POST", path, req, 409)           // recovers Coolify; GitHub accepts then fails response
	result := journeyRequest(t, mux, "POST", path, req, 200) // recovers GitHub without another upload
	var operation DeployKeyOperation
	_ = json.Unmarshal(result.Body.Bytes(), &operation)
	if operation.Status != "ready" || operation.PrivateKeyUUID != "key-owned" || remoteCreates != 1 || githubCreates != 1 {
		t.Fatalf("recovery duplicated or lost access: %+v remote=%d github=%d", operation, remoteCreates, githubCreates)
	}
	journeyRequest(t, mux, "POST", path, req, 200)
	if remoteCreates != 1 || githubCreates != 1 {
		t.Fatal("repeat created another key")
	}
	err := filepath.WalkDir(s.stateDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if strings.Contains(string(data), "PRIVATE KEY") {
				t.Error("private key persisted in local state", path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(p.Path, "deploy-keys")); !os.IsNotExist(err) {
		t.Fatal("provider state entered project Git")
	}
	keys = nil
	lost := journeyRequest(t, mux, "POST", path, req, 409)
	if strings.Contains(lost.Body.String(), `"status":"ready"`) {
		t.Fatal("deleted remote key remained ready")
	}
}

func TestDeployKeyPreflightFailureCanRetryWithoutAnExternalMutation(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerCloudReleases(mux)
	p := createJourneyProject(t, mux, "private-app")
	s.targets["target-test"] = DeploymentTarget{ID: "target-test", BaseURL: "http://127.0.0.1:1", Token: testCoolifyToken, ServerUUID: "server-test"}
	calls := 0
	s.runner = func(context.Context, string, []string, string) (string, error) {
		calls++
		return "", errors.New("not signed in")
	}
	path := "/v1/product/projects/" + p.ID + "/releases/deploy-key"
	req := map[string]string{"targetId": "target-test", "repositoryHTTPSGithub": "https://github.com/example/private"}
	for range 2 {
		w := journeyRequest(t, mux, "POST", path, req, 409)
		var row DeployKeyOperation
		_ = json.Unmarshal(w.Body.Bytes(), &row)
		if row.KeyAttempted {
			t.Fatal("signed-out account attempted a private-key upload")
		}
	}
	if calls != 2 {
		t.Fatal("retry did not recheck GitHub")
	}
}
