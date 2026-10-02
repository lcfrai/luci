package product

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
)

// v4.3.23 OpenAPI documents /applications/{uuid}/envs and deletion by env UUID.
// Its request schemas omit flags accepted by the pinned implementation:
// https://github.com/coollabsio/coolify/blob/e2e2d4010bcd590084b66d6f748f3eec8e2bbee9/app/Http/Controllers/Api/ApplicationsController.php
// create_env/update_env_by_uuid explicitly accept is_runtime, is_buildtime and
// comment. Default is_buildtime is TRUE: never rely on provider defaults here.
// The model also creates a preview copy with the same ownership comment; remove
// those owned copies before deploying. Manual/vendor entries are never adopted.
type coolifyRuntimeVariable struct {
	UUID        string `json:"uuid"`
	Key         string `json:"key"`
	IsPreview   bool   `json:"is_preview"`
	IsLiteral   bool   `json:"is_literal"`
	IsRuntime   *bool  `json:"is_runtime"`
	IsBuildtime *bool  `json:"is_buildtime"`
	IsShared    bool   `json:"is_shared"`
	Comment     string `json:"comment"`
}

type managedApplicationVariables struct {
	ProjectID         string   `json:"projectId"`
	ApplicationID     string   `json:"applicationId"`
	TargetFingerprint string   `json:"targetFingerprint"`
	Keys              []string `json:"keys"`
}

func (c *CoolifyClient) runtimeVariables(ctx context.Context, applicationID string) ([]coolifyRuntimeVariable, error) {
	if !coolifyID.MatchString(applicationID) {
		return nil, errors.New("invalid application configuration target")
	}
	variables := []coolifyRuntimeVariable{}
	if err := c.request(ctx, http.MethodGet, "/applications/"+applicationID+"/envs", nil, nil, &variables); err != nil {
		return nil, err
	}
	if len(variables) > 2000 {
		return nil, errors.New("the provider returned too many application variables")
	}
	seen := map[string]bool{}
	for _, variable := range variables {
		if !coolifyID.MatchString(variable.UUID) || !applicationVariableName.MatchString(variable.Key) || seen[variable.UUID] {
			return nil, errors.New("the provider returned invalid application variable metadata")
		}
		seen[variable.UUID] = true
	}
	return variables, nil
}

func runtimeVariableMatches(variable coolifyRuntimeVariable, marker string) bool {
	return !variable.IsPreview && variable.Comment == marker && variable.IsLiteral && !variable.IsShared && variable.IsRuntime != nil && *variable.IsRuntime && variable.IsBuildtime != nil && !*variable.IsBuildtime
}

func (s *Server) reconcileCloudApplicationConfiguration(ctx context.Context, row CloudRelease, client *CoolifyClient, snapshot ApplicationConfigurationSnapshot) error {
	if row.ConfigurationRevision != snapshot.Revision || !validApplicationValues(snapshot.Values) || !coolifyID.MatchString(row.ApplicationID) {
		return errors.New("the release application configuration snapshot is invalid")
	}
	sum := sha256.Sum256([]byte(row.ProjectID + "\x00" + row.TargetFingerprint + "\x00" + row.ApplicationID))
	path, err := s.privateApplicationStatePath("application-managed-" + hex.EncodeToString(sum[:]))
	if err != nil {
		return err
	}
	managed := managedApplicationVariables{ProjectID: row.ProjectID, ApplicationID: row.ApplicationID, TargetFingerprint: row.TargetFingerprint, Keys: []string{}}
	s.mu.RLock()
	file, readErr := os.Open(path)
	if readErr == nil {
		data, err := io.ReadAll(io.LimitReader(file, (64<<10)+1))
		_ = file.Close()
		if err != nil || len(data) > 64<<10 || json.Unmarshal(data, &managed) != nil {
			readErr = errors.New("private application variable ownership is unreadable")
		}
	}
	s.mu.RUnlock()
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return errors.New("private application variable ownership is unavailable")
	}
	if managed.ProjectID != row.ProjectID || managed.ApplicationID != row.ApplicationID || managed.TargetFingerprint != row.TargetFingerprint || len(managed.Keys) > 200 {
		return errors.New("private application variable ownership does not match this release")
	}
	owned := map[string]bool{}
	for _, key := range managed.Keys {
		if !applicationVariableName.MatchString(key) || owned[key] {
			return errors.New("private application variable ownership is invalid")
		}
		owned[key] = true
	}
	if len(owned) == 0 && len(snapshot.Values) == 0 {
		return nil
	}
	prefix := "luci-runtime:" + row.ProjectID + ":" + hex.EncodeToString(sum[:8]) + ":"
	marker := prefix + fmt.Sprint(snapshot.Revision)
	variables, err := client.runtimeVariables(ctx, row.ApplicationID)
	if err != nil {
		return err
	}
	// Preflight all collisions before changing any provider state. Previously
	// managed names alone do not authorize altering an externally replaced entry.
	production := map[string]coolifyRuntimeVariable{}
	for _, variable := range variables {
		if variable.IsPreview {
			continue
		}
		if _, duplicate := production[variable.Key]; duplicate {
			return errors.New("the provider has ambiguous application variable names")
		}
		production[variable.Key] = variable
		if _, wanted := snapshot.Values[variable.Key]; wanted && (!owned[variable.Key] || !strings.HasPrefix(variable.Comment, prefix) || variable.IsShared) {
			return errors.New("a configured variable is managed outside Luci; choose a different name or resolve that provider entry explicitly")
		}
	}
	// Persist intended ownership before any create/update. If the response is
	// lost, readback can reconcile the exact revision marker without reading
	// secret values or blindly issuing another mutation.
	for key := range snapshot.Values {
		owned[key] = true
	}
	managed.Keys = make([]string, 0, len(owned))
	for key := range owned {
		managed.Keys = append(managed.Keys, key)
	}
	sort.Strings(managed.Keys)
	s.mu.Lock()
	err = atomicJSON(path, managed)
	s.mu.Unlock()
	if err != nil {
		return errors.New("could not save private application variable ownership; no variables were sent")
	}
	names := make([]string, 0, len(snapshot.Values))
	for name := range snapshot.Values {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		current, exists := production[name]
		if exists && runtimeVariableMatches(current, marker) {
			continue
		}
		method := http.MethodPost
		if exists {
			method = http.MethodPatch
		}
		payload := map[string]any{"key": name, "value": snapshot.Values[name], "is_preview": false, "is_literal": true, "is_multiline": false, "is_shown_once": true, "is_runtime": true, "is_buildtime": false, "comment": marker}
		mutationErr := client.request(ctx, method, "/applications/"+row.ApplicationID+"/envs", nil, payload, nil)
		observed, observeErr := client.runtimeVariables(ctx, row.ApplicationID)
		if observeErr != nil {
			return errors.New("application configuration could not be confirmed; no deployment started")
		}
		confirmed := 0
		for _, variable := range observed {
			if variable.Key == name && runtimeVariableMatches(variable, marker) {
				confirmed++
			}
		}
		if confirmed != 1 {
			if mutationErr != nil {
				return errors.New("the provider did not confirm the runtime configuration update; no deployment started")
			}
			return errors.New("the provider did not confirm runtime-only configuration; no deployment started")
		}
		variables = observed
	}
	// Delete only names in the persisted ownership ledger AND our exact owner
	// namespace. This includes Coolify's automatic preview copies, never manual
	// preview entries. An absent UUID after a lost delete response is success.
	variables, err = client.runtimeVariables(ctx, row.ApplicationID)
	if err != nil {
		return err
	}
	for _, variable := range variables {
		_, desired := snapshot.Values[variable.Key]
		if !owned[variable.Key] || !strings.HasPrefix(variable.Comment, prefix) || variable.IsShared || (!variable.IsPreview && desired) {
			continue
		}
		_ = client.request(ctx, http.MethodDelete, "/applications/"+row.ApplicationID+"/envs/"+variable.UUID, nil, nil, nil)
		observed, err := client.runtimeVariables(ctx, row.ApplicationID)
		if err != nil {
			return errors.New("removed application configuration could not be confirmed; no deployment started")
		}
		for _, present := range observed {
			if present.UUID == variable.UUID {
				return errors.New("the provider did not confirm removal of managed configuration; no deployment started")
			}
		}
	}
	final, err := client.runtimeVariables(ctx, row.ApplicationID)
	if err != nil {
		return err
	}
	for _, name := range names {
		count := 0
		for _, variable := range final {
			if variable.Key == name && runtimeVariableMatches(variable, marker) {
				count++
			}
		}
		if count != 1 {
			return errors.New("runtime-only application configuration changed before deployment")
		}
	}
	for _, variable := range final {
		if owned[variable.Key] && strings.HasPrefix(variable.Comment, prefix) && (variable.IsPreview || !containsApplicationVariable(snapshot.Values, variable.Key)) {
			return errors.New("obsolete managed application configuration remains on the provider")
		}
	}
	managed.Keys = names
	s.mu.Lock()
	err = atomicJSON(path, managed)
	s.mu.Unlock()
	if err != nil {
		return errors.New("could not save confirmed application configuration ownership")
	}
	return nil
}

func containsApplicationVariable(values map[string]string, name string) bool {
	_, ok := values[name]
	return ok
}
