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
	"regexp"
	"sort"
	"strings"
)

// Trusted platform integration only. No HTTP request may supply these values.
type CloudDataBindingRequest struct {
	ProjectID, TargetID, TargetFingerprint, ServerUUID, ApplicationID, AppOrigin, Framework string
}

// Even the anon key and public URL are omitted from snapshots/release history.
// Private database and service-role values have no path through this contract.
type CloudDataBindingSnapshot struct {
	Installed           bool   `json:"-"`
	ProjectID           string `json:"-"`
	StableTenant        string `json:"-"`
	TargetID            string `json:"-"`
	TargetFingerprint   string `json:"-"`
	ServerUUID          string `json:"-"`
	DomainStatus        string `json:"-"`
	DomainRevision      int    `json:"-"`
	ConfigurationDigest string `json:"-"`
	PublicURL           string `json:"-"`
	ApplicationOrigin   string `json:"-"`
	PublishableKey      string `json:"-"`
}

type cloudApplicationVariable struct {
	Value     string
	Buildtime bool
}

func validDataClientFramework(framework string) bool {
	return framework == "" || framework == "none" || framework == "vite" || framework == "nextjs"
}

func dataClientAliases(framework string) []string {
	switch framework {
	case "vite":
		return []string{"VITE_SUPABASE_URL", "VITE_SUPABASE_ANON_KEY"}
	case "nextjs":
		return []string{"NEXT_PUBLIC_SUPABASE_URL", "NEXT_PUBLIC_SUPABASE_ANON_KEY"}
	default:
		return nil
	}
}

func cloudDataFingerprint(data CloudDataBindingSnapshot, framework string) string {
	if data.PublicURL == "" {
		return ""
	}
	encoded, _ := json.Marshal([]string{data.ProjectID, data.StableTenant, data.TargetID, data.TargetFingerprint, data.ServerUUID, fmt.Sprint(data.DomainRevision), data.ConfigurationDigest, data.PublicURL, data.ApplicationOrigin, data.PublishableKey, framework})
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func (s *Server) cloudApplicationDataSnapshot(ctx context.Context, row CloudRelease) (CloudDataBindingSnapshot, error) {
	if !validDataClientFramework(row.DataClientFramework) {
		return CloudDataBindingSnapshot{}, errors.New("choose runtime-only, Vite or Next.js browser data variables")
	}
	explicit := len(dataClientAliases(row.DataClientFramework)) > 0
	if s.cloudDataBindings == nil {
		if explicit {
			return CloudDataBindingSnapshot{}, errors.New("publish and verify this project's Data HTTPS domain before choosing browser build variables")
		}
		return CloudDataBindingSnapshot{}, nil
	}
	data, err := s.cloudDataBindings(ctx, CloudDataBindingRequest{ProjectID: row.ProjectID, TargetID: row.TargetID, TargetFingerprint: row.TargetFingerprint, ServerUUID: row.ServerUUID, ApplicationID: row.ApplicationID, AppOrigin: row.Domain, Framework: row.DataClientFramework})
	if err != nil {
		return CloudDataBindingSnapshot{}, errors.New("this project's Data domain or active installation changed; publish and verify it before deploying")
	}
	configured := data.DomainStatus != "" && data.DomainStatus != "not-configured" || data.DomainRevision != 0 || data.PublicURL != "" || data.ConfigurationDigest != ""
	if !configured {
		if explicit {
			return CloudDataBindingSnapshot{}, errors.New("publish and verify this project's Data HTTPS domain before choosing browser build variables")
		}
		return CloudDataBindingSnapshot{}, nil
	}
	publicURL, originErr := publicDataOrigin(data.PublicURL)
	appOrigin, appErr := publicDataOrigin(row.Domain)
	if !data.Installed || data.DomainStatus != "verifiedHTTPS" || data.ProjectID != row.ProjectID || !coolifyID.MatchString(data.StableTenant) || data.TargetID != row.TargetID || data.TargetFingerprint != row.TargetFingerprint || data.ServerUUID != row.ServerUUID || data.DomainRevision < 1 || data.DomainRevision >= 1_000_000_000 || !hexDigest(data.ConfigurationDigest) || originErr != nil || appErr != nil || publicURL != data.PublicURL || data.ApplicationOrigin != appOrigin || !validDataAnon(data.PublishableKey) {
		return CloudDataBindingSnapshot{}, errors.New("verify the exact active Data HTTPS domain, application origin and deployment server before deploying")
	}
	return data, nil
}

func hexDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32 && value == strings.ToLower(value)
}

func cloudApplicationVariables(row CloudRelease, snapshot ApplicationConfigurationSnapshot, data CloudDataBindingSnapshot) (map[string]cloudApplicationVariable, error) {
	values := map[string]cloudApplicationVariable{}
	for name, value := range snapshot.Values {
		values[name] = cloudApplicationVariable{Value: value}
	}
	if data.PublicURL == "" {
		if row.DataBindingFingerprint != "" || row.DataDomainRevision != 0 || len(dataClientAliases(row.DataClientFramework)) > 0 {
			return nil, errors.New("verified Data binding disappeared before configuration")
		}
		return values, nil
	}
	if row.DataBindingFingerprint != cloudDataFingerprint(data, row.DataClientFramework) || row.DataDomainRevision != data.DomainRevision {
		return nil, errors.New("the release Data binding snapshot changed")
	}
	generated := map[string]string{"SUPABASE_URL": data.PublicURL, "SUPABASE_PUBLIC_URL": data.PublicURL, "SUPABASE_ANON_KEY": data.PublishableKey, "SUPABASE_PUBLISHABLE_KEY": data.PublishableKey}
	aliases := dataClientAliases(row.DataClientFramework)
	if len(aliases) == 2 {
		generated[aliases[0]], generated[aliases[1]] = data.PublicURL, data.PublishableKey
	}
	// No native network has been proved for a provider application. Do not send
	// the source workspace's private credentials to this arbitrary target host.
	for _, name := range []string{"SUPABASE_SERVICE_ROLE_KEY", "SUPABASE_SERVICE_KEY", "SUPABASE_DB_URL"} {
		if _, present := values[name]; present {
			return nil, errors.New("private native Data variables require a verified application network; remove them before public cloud deployment")
		}
	}
	for name, value := range generated {
		if _, collision := values[name]; collision {
			return nil, errors.New("a Data variable conflicts with private application configuration; resolve it before deploying")
		}
		values[name] = cloudApplicationVariable{Value: value, Buildtime: containsString(aliases, name)}
	}
	return values, nil
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func requireDataClientDockerfileArgs(source, framework string) error {
	aliases := dataClientAliases(framework)
	if len(aliases) == 0 {
		return nil
	}
	// Fail closed for build-stage scope. Global ARG declarations and ARGs in a
	// later runtime stage do not supply a preceding Next/Vite build. Standard
	// package-manager builds and direct next/vite builds are recognized; custom
	// wrappers must expose an explicit framework build step for this contract.
	build := regexp.MustCompile(`(?i)(?:^|[\s;&|])(?:npm|pnpm|yarn|bun)\s+(?:run\s+)?build(?:[:\s;&|]|$)|\b(?:npx\s+)?(?:next|vite)\s+build(?:[\s;&|]|$)`)
	stageArgs := map[string]bool{}
	stages := map[string]map[string]bool{}
	stageName := ""
	stageStarted := false
	logical := ""
	for _, line := range strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasSuffix(line, "\\") {
			logical += strings.TrimSuffix(line, "\\") + " "
			continue
		}
		line, logical = logical+line, ""
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch strings.ToUpper(fields[0]) {
		case "FROM":
			if stageName != "" {
				stages[stageName] = stageArgs
			}
			stageArgs, stageName, stageStarted = map[string]bool{}, "", true
			base := 1
			for base < len(fields) && strings.HasPrefix(fields[base], "--") {
				base++
			}
			if base < len(fields) {
				for name := range stages[strings.ToLower(fields[base])] {
					stageArgs[name] = true
				}
			}
			if len(fields) >= 4 && strings.EqualFold(fields[len(fields)-2], "AS") {
				stageName = strings.ToLower(fields[len(fields)-1])
			}
		case "ARG":
			if stageStarted {
				name := strings.SplitN(fields[1], "=", 2)[0]
				if containsString(aliases, name) {
					stageArgs[name] = true
				}
			}
		case "RUN":
			if build.MatchString(strings.TrimSpace(line[len(fields[0]):])) && stageArgs[aliases[0]] && stageArgs[aliases[1]] {
				return nil
			}
		}
	}
	return errors.New("declare both public Data Dockerfile ARG inputs before an explicit npm, pnpm, yarn, bun, next or vite build step in the framework build stage; commit and push before deploying")
}

func (s *Server) recheckCloudApplicationData(ctx context.Context, row CloudRelease) error {
	data, err := s.cloudApplicationDataSnapshot(ctx, row)
	if err != nil || row.DataBindingFingerprint != cloudDataFingerprint(data, row.DataClientFramework) || row.DataDomainRevision != data.DomainRevision {
		return errors.New("Data domain, key or server changed while configuring the application; redeploy after verifying the current Data domain")
	}
	return nil
}

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

func runtimeVariableMatches(variable coolifyRuntimeVariable, marker string, buildtime bool) bool {
	return !variable.IsPreview && variable.Comment == marker && variable.IsLiteral && !variable.IsShared && variable.IsRuntime != nil && *variable.IsRuntime && variable.IsBuildtime != nil && *variable.IsBuildtime == buildtime
}

func (s *Server) reconcileCloudApplicationConfiguration(ctx context.Context, row CloudRelease, client *CoolifyClient, snapshot ApplicationConfigurationSnapshot, dataSnapshots ...CloudDataBindingSnapshot) error {
	if row.ConfigurationRevision != snapshot.Revision || !validApplicationValues(snapshot.Values) || !coolifyID.MatchString(row.ApplicationID) {
		return errors.New("the release application configuration snapshot is invalid")
	}
	data := CloudDataBindingSnapshot{}
	if len(dataSnapshots) > 1 {
		return errors.New("ambiguous Data configuration snapshot")
	}
	if len(dataSnapshots) == 1 {
		data = dataSnapshots[0]
	}
	desiredValues, err := cloudApplicationVariables(row, snapshot, data)
	if err != nil {
		return err
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
	if len(owned) == 0 && len(desiredValues) == 0 {
		return nil
	}
	prefix := "luci-runtime:" + row.ProjectID + ":" + hex.EncodeToString(sum[:8]) + ":"
	marker := prefix + fmt.Sprint(snapshot.Revision) + ":" + row.DataBindingFingerprint
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
		if _, wanted := desiredValues[variable.Key]; wanted && (!owned[variable.Key] || !strings.HasPrefix(variable.Comment, prefix) || variable.IsShared) {
			return errors.New("a configured variable is managed outside Luci; choose a different name or resolve that provider entry explicitly")
		}
	}
	// Persist intended ownership before any create/update. If the response is
	// lost, readback can reconcile the exact revision marker without reading
	// secret values or blindly issuing another mutation.
	for key := range desiredValues {
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
	names := make([]string, 0, len(desiredValues))
	for name := range desiredValues {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		current, exists := production[name]
		if exists && runtimeVariableMatches(current, marker, desiredValues[name].Buildtime) {
			continue
		}
		method := http.MethodPost
		if exists {
			method = http.MethodPatch
		}
		payload := map[string]any{"key": name, "value": desiredValues[name].Value, "is_preview": false, "is_literal": true, "is_multiline": false, "is_shown_once": true, "is_runtime": true, "is_buildtime": desiredValues[name].Buildtime, "comment": marker}
		mutationErr := client.request(ctx, method, "/applications/"+row.ApplicationID+"/envs", nil, payload, nil)
		observed, observeErr := client.runtimeVariables(ctx, row.ApplicationID)
		if observeErr != nil {
			return errors.New("application configuration could not be confirmed; no deployment started")
		}
		confirmed := 0
		for _, variable := range observed {
			if variable.Key == name && runtimeVariableMatches(variable, marker, desiredValues[name].Buildtime) {
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
		_, desired := desiredValues[variable.Key]
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
			if variable.Key == name && runtimeVariableMatches(variable, marker, desiredValues[name].Buildtime) {
				count++
			}
		}
		if count != 1 {
			return errors.New("runtime-only application configuration changed before deployment")
		}
	}
	for _, variable := range final {
		_, desired := desiredValues[variable.Key]
		if owned[variable.Key] && strings.HasPrefix(variable.Comment, prefix) && (variable.IsPreview || !desired) {
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
