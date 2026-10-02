package product

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"golang.org/x/crypto/ssh"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type DeployKeyOperation struct {
	ID                string `json:"id"`
	ProjectID         string `json:"projectId"`
	TargetID          string `json:"targetId"`
	TargetFingerprint string `json:"targetFingerprint"`
	Repository        string `json:"repository"`
	PublicKey         string `json:"publicKey"`
	PrivateKeyUUID    string `json:"privateKeyUuid,omitempty"`
	KeyAttempted      bool   `json:"keyAttempted,omitempty"`
	Status            string `json:"status"`
	Message           string `json:"message"`
}
type githubDeployKey struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Key      string `json:"key"`
	ReadOnly bool   `json:"read_only"`
}

func githubDeploymentRepository(raw string) (string, string, error) {
	clean, err := safeDeliveryURL(raw)
	if err != nil {
		return "", "", err
	}
	u, err := url.Parse(clean)
	if err != nil || u.Host != "github.com" {
		return "", "", errors.New("use the private repository's HTTPS github.com URL")
	}
	parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	if len(parts) != 2 {
		return "", "", errors.New("use https://github.com/owner/repository")
	}
	name := strings.TrimSuffix(parts[1], ".git")
	if !githubOwner.MatchString(parts[0]) || !githubRepo.MatchString(name) || name == "." || name == ".." {
		return "", "", errors.New("invalid GitHub repository")
	}
	return parts[0], name, nil
}
func samePublicKey(a, b string) bool {
	pa, _, _, _, ea := ssh.ParseAuthorizedKey([]byte(a))
	pb, _, _, _, eb := ssh.ParseAuthorizedKey([]byte(b))
	return ea == nil && eb == nil && string(pa.Marshal()) == string(pb.Marshal())
}

func (s *Server) prepareDeployKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TargetID   string `json:"targetId"`
		Repository string `json:"repositoryHTTPSGithub"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	owner, repo, err := githubDeploymentRepository(req.Repository)
	if err != nil {
		journeyError(w, 400, "repository_invalid", err.Error())
		return
	}
	target, err := s.deploymentTarget(req.TargetID)
	if err != nil {
		journeyError(w, 409, "target_unavailable", "Connect a deployment target first.")
		return
	}
	fingerprint := targetFingerprint(target)
	sum := sha256.Sum256([]byte(p.ID + "\x00" + target.ID + "\x00" + owner + "/" + repo))
	id := "deploy-key-" + hex.EncodeToString(sum[:16])
	file := filepath.Join(s.stateDir, "deploy-keys", id+".json")
	if !s.reserveDelivery(id) {
		journeyError(w, 409, "deploy_key_busy", "This deployment-key connection is already running.")
		return
	}
	defer s.releaseDelivery(id)
	client, err := NewCoolifyClient(target.BaseURL, target.Token)
	if err != nil {
		journeyError(w, 400, "target_invalid", err.Error())
		return
	}
	row := DeployKeyOperation{}
	data, err := os.ReadFile(file)
	created := os.IsNotExist(err)
	if err != nil && !created {
		journeyError(w, 500, "deploy_key_state_unavailable", "Could not read deployment-key state.")
		return
	}
	if !created && (len(data) > 32768 || json.Unmarshal(data, &row) != nil || row.ID != id || row.ProjectID != p.ID || row.TargetFingerprint != fingerprint) {
		journeyError(w, 409, "deploy_key_state_conflict", "The saved key operation belongs to different target settings or is invalid.")
		return
	}
	var privatePEM []byte
	if created || !row.KeyAttempted {
		public, private, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			journeyError(w, 500, "key_generation_failed", "Could not generate a deployment key.")
			return
		}
		block, err := ssh.MarshalPrivateKey(private, "Luci "+id)
		if err != nil {
			journeyError(w, 500, "key_generation_failed", "Could not encode a deployment key.")
			return
		}
		privatePEM = pem.EncodeToMemory(block)
		publicSSH, _ := ssh.NewPublicKey(public)
		row = DeployKeyOperation{ID: id, ProjectID: p.ID, TargetID: target.ID, TargetFingerprint: fingerprint, Repository: "https://github.com/" + owner + "/" + repo + ".git", PublicKey: strings.TrimSpace(string(ssh.MarshalAuthorizedKey(publicSSH))), Status: "preparing", Message: "Preparing read-only GitHub deployment access."}
		if err = atomicJSON(file, row); err != nil {
			journeyError(w, 500, "deploy_key_reservation_failed", "Could not save key intent; no remote change was attempted.")
			return
		}
	}
	defer func() {
		for i := range privatePEM {
			privatePEM[i] = 0
		}
	}()
	fail := func(message string) {
		row.Status = "attention"
		row.Message = message
		_ = atomicJSON(file, row)
		journeyJSON(w, 409, row)
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	// Check GitHub access first, so a signed-out account creates no remote key.
	output, err := s.runner(ctx, "gh", []string{"api", "--hostname", "github.com", "repos/" + owner + "/" + repo + "/keys", "--paginate", "--slurp"}, s.stateDir)
	if err != nil {
		fail("Sign in to GitHub with repository administration rights, then retry. You may also configure an existing Coolify private-key UUID manually.")
		return
	}
	var pages [][]githubDeployKey
	if len(output) > 1<<20 || json.Unmarshal([]byte(output), &pages) != nil {
		fail("GitHub returned an unreadable deploy-key list; no key was uploaded.")
		return
	}
	keys, err := client.PrivateKeys(ctx)
	if err != nil {
		fail("Could not inspect Coolify's keys. Reconnect this target before retrying.")
		return
	}
	foundRemote := false
	for _, key := range keys {
		if key.Name == id {
			if !samePublicKey(key.PublicKey, row.PublicKey) || row.PrivateKeyUUID != "" && row.PrivateKeyUUID != key.UUID {
				fail("A Coolify key with this operation's name has different key material. Resolve that conflict manually.")
				return
			}
			row.PrivateKeyUUID = key.UUID
			foundRemote = true
			break
		}
	}
	if row.PrivateKeyUUID != "" && !foundRemote {
		fail("The previously registered Coolify key is missing. Restore it or configure another key explicitly; access is not ready.")
		return
	}
	if row.PrivateKeyUUID == "" {
		if len(privatePEM) == 0 {
			fail("A prior key creation was interrupted and no matching remote key was found. No replacement key was generated; supply an existing Coolify key UUID or resolve the recorded operation.")
			return
		}
		row.KeyAttempted = true
		if err = atomicJSON(file, row); err != nil {
			journeyError(w, 500, "deploy_key_reservation_failed", "Could not reserve key registration; no remote key was created.")
			return
		}
		row.PrivateKeyUUID, err = client.CreatePrivateKey(ctx, id, string(privatePEM))
		if err != nil {
			fail("Coolify did not confirm the private-key registration. Retry to reconcile the same public key; no duplicate key will be generated.")
			return
		}
		row.Status = "key_registered"
		row.Message = "Coolify key registered; connecting read-only repository access."
		if err = atomicJSON(file, row); err != nil {
			journeyError(w, 500, "deploy_key_state_failed", "A remote key may exist. Retry to reconcile its recorded operation name.")
			return
		}
	}
	found := false
	for _, page := range pages {
		for _, key := range page {
			if key.Title == id || samePublicKey(key.Key, row.PublicKey) {
				if !samePublicKey(key.Key, row.PublicKey) || !key.ReadOnly {
					fail("A matching GitHub deploy-key name/key has unexpected access. Resolve it before continuing.")
					return
				}
				found = true
			}
		}
	}
	if !found {
		input := filepath.Join(s.stateDir, ".deploy-key-input-"+newID("")+".json")
		if err = atomicJSON(input, map[string]any{"title": id, "key": row.PublicKey, "read_only": true}); err != nil {
			fail("Could not prepare the read-only deploy-key request.")
			return
		}
		defer os.Remove(input)
		response, err := s.runner(ctx, "gh", []string{"api", "--hostname", "github.com", "repos/" + owner + "/" + repo + "/keys", "--method", "POST", "--input", input}, s.stateDir)
		var key githubDeployKey
		if err != nil || json.Unmarshal([]byte(response), &key) != nil || key.ID <= 0 || !samePublicKey(key.Key, row.PublicKey) || !key.ReadOnly {
			fail("GitHub did not confirm read-only access. Retry to reconcile the existing key before any new upload.")
			return
		}
	}
	row.Status = "ready"
	row.Message = "Read-only repository access is registered with GitHub and the selected Coolify target."
	if err = atomicJSON(file, row); err != nil {
		journeyError(w, 500, "deploy_key_state_failed", "Access was registered but its status could not be saved. Retry to reconcile it.")
		return
	}
	journeyJSON(w, 200, row)
}
