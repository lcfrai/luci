package dockerbe

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/lcfrai/luci/control-plane/internal/backend"
)

// A controlled RoundTripper captures the production SDK bytes without opening
// a socket. The existing probe supplies only Info/ImageInspect/Create replies.
func TestCloud027ProductionCheckWire(t *testing.T) {
	stage := os.Getenv("CLOUD027_WIRE_STAGE")
	if stage == "" {
		t.Fatal("CLOUD027_WIRE_STAGE is required")
	}
	var input struct {
		Spec                                               backend.CheckSpec
		ImageID, ImageReference, StorageDriver, APIVersion string
		ImageVirtualSize                                   int64
	}
	raw, err := os.ReadFile(filepath.Join(stage, "probe-input.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &input); err != nil {
		t.Fatal(err)
	}
	spec := input.Spec
	if got := backend.CheckCommandDigest(spec); got != spec.Identity.CommandDigest {
		t.Fatalf("production digest %s != recorded lease %s", got, spec.Identity.CommandDigest)
	}
	// Marshal the actual production CheckSpec struct in its digest-normalized
	// form. No handwritten mirror of its JSON representation is used.
	digestSpec := spec
	digestSpec.Identity.EngineID, digestSpec.Identity.InstanceID, digestSpec.Identity.CommandDigest = "", "", ""
	digestSpec.Args = append([]string{}, spec.Args...)
	digestRaw, err := json.Marshal(digestSpec)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(digestRaw)
	if hex.EncodeToString(sum[:]) != backend.CheckCommandDigest(spec) {
		t.Fatal("Go JSON digest parity failed")
	}
	if err = os.WriteFile(filepath.Join(stage, "production-command-digest-input.json"), digestRaw, 0600); err != nil {
		t.Fatal(err)
	}
	_, f, _ := newCheckProbe(t)
	f.info.ID, f.info.Driver = spec.Identity.EngineID, input.StorageDriver
	f.image.ID, f.image.Size = input.ImageID, input.ImageVirtualSize
	var calls []string
	var createRaw []byte
	transport := checkWireTransport(func(req *http.Request) (*http.Response, error) {
		calls = append(calls, req.Method+" "+req.URL.Path)
		status := http.StatusOK
		var value any
		switch {
		case req.Method == "HEAD" && req.URL.Path == "/_ping":
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Api-Version": []string{input.APIVersion}}, Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
		case req.Method == "GET" && req.URL.Path == "/v"+input.APIVersion+"/info":
			value = f.info
		case req.Method == "GET" && req.URL.Path == "/v"+input.APIVersion+"/images/"+input.ImageReference+"/json":
			value = f.image
		case req.Method == "POST" && req.URL.Path == "/v"+input.APIVersion+"/containers/create":
			if createRaw != nil {
				return nil, fmt.Errorf("duplicate controlled Create")
			}
			createRaw, err = io.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			if req.URL.Query().Get("name") != checkName(spec.Identity) {
				return nil, fmt.Errorf("wrong Create name")
			}
			var body container.CreateRequest
			if err = json.Unmarshal(createRaw, &body); err != nil {
				return nil, err
			}
			value, err = f.ContainerCreate(req.Context(), body.Config, body.HostConfig, body.NetworkingConfig, nil, req.URL.Query().Get("name"))
			if err != nil {
				return nil, err
			}
			status = http.StatusCreated
		default:
			return nil, fmt.Errorf("unexpected SDK RPC %s %s", req.Method, req.URL.Path)
		}
		body, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(string(body))), Request: req}, nil
	})
	sdk, err := client.NewClientWithOpts(client.WithHost(f.endpoint), client.WithVersion(input.APIVersion), client.WithHTTPClient(&http.Client{Transport: transport}))
	if err != nil {
		t.Fatal(err)
	}
	defer sdk.Close()
	b, err := New(Config{Instance: spec.Identity.InstanceID, ResourceImage: input.ImageReference, Client: sdk})
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := b.PrepareCheck(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := b.CreateCheck(context.Background(), prepared)
	if err != nil {
		t.Fatal(err)
	}
	if ref == "" || len(f.creates) != 1 || len(f.starts) != 0 || len(f.removes) != 0 {
		t.Fatal("unexpected controlled lifecycle")
	}
	if err = os.WriteFile(filepath.Join(stage, "production-create-wire.json"), createRaw, 0600); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.MarshalIndent(map[string]any{"calls": calls, "commandDigest": backend.CheckCommandDigest(spec), "apiVersion": input.APIVersion, "creates": len(f.creates), "starts": len(f.starts), "removes": len(f.removes), "realDockerOperations": 0, "networkConnections": 0, "scope": "Controlled production PrepareCheck/CreateCheck/SDK capture; not the uncaptured original request"}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(stage, "production-probe-result.json"), append(encoded, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}
