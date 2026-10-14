package management

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/dockerinfo"
)

type dependencySystemRunner struct {
	environment []string
	dir         string
}

func (dependencySystemRunner) platform() string { return dependencyHostPlatform() }
func (r dependencySystemRunner) engineInfo(ctx context.Context, executable string) (dockerinfo.Info, error) {
	return dockerinfo.Probe(ctx, executable, "", dependencyProcessEnvironment(r.environment), false)
}
func (dependencySystemRunner) installationLease(ctx context.Context) (func(), error) {
	return acquireDependencyHostInstallation(ctx)
}
func dependencyEnvironmentValue(environment []string, name string) string {
	for i := len(environment) - 1; i >= 0; i-- {
		key, value, ok := strings.Cut(environment[i], "=")
		if ok && strings.EqualFold(key, name) {
			return value
		}
	}
	return ""
}
func (r dependencySystemRunner) lookup(name string) (string, error) {
	allowed := name == "winget"
	for _, definition := range dependencyCatalogue {
		allowed = allowed || definition.executable == name
	}
	if !allowed {
		return "", errors.New("unsupported dependency executable")
	}
	extensions := []string{""}
	if runtime.GOOS == "windows" {
		extensions = []string{".exe"}
		if name == "az" {
			extensions = append(extensions, ".cmd")
		}
	}
	for _, directory := range filepath.SplitList(dependencyEnvironmentValue(r.environment, "PATH")) {
		// Never resolve from the working project, a relative PATH segment, or
		// a shell/batch wrapper (except Azure's specifically handled az.cmd).
		if !filepath.IsAbs(directory) {
			continue
		}
		for _, extension := range extensions {
			path := filepath.Join(directory, name+extension)
			info, err := os.Stat(path)
			if err == nil && !info.IsDir() && (runtime.GOOS == "windows" || info.Mode()&0111 != 0) {
				return path, nil
			}
		}
	}
	return "", exec.ErrNotFound
}

func dependencyProcessEnvironment(base []string) []string {
	allowed := map[string]bool{
		"PATH": true, "SYSTEMROOT": true, "WINDIR": true, "LOCALAPPDATA": true,
		"APPDATA": true, "USERPROFILE": true, "HOME": true, "TEMP": true, "TMP": true,
		"PROGRAMFILES": true, "PROGRAMFILES(X86)": true, "PROGRAMDATA": true,
		"DOCKER_HOST": true, "DOCKER_CONTEXT": true, "DOCKER_TLS_VERIFY": true, "DOCKER_CERT_PATH": true,
	}
	environment := []string{}
	for _, entry := range base {
		key, _, valid := strings.Cut(entry, "=")
		if valid && allowed[strings.ToUpper(key)] {
			environment = append(environment, entry)
		}
	}
	return environment
}

type dependencyOutput struct {
	mu   sync.Mutex
	data []byte
}

func (w *dependencyOutput) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	remaining := (64 << 10) - len(w.data)
	if remaining > len(p) {
		remaining = len(p)
	}
	if remaining > 0 {
		w.data = append(w.data, p[:remaining]...)
	}
	return len(p), nil // Drain installer output, retaining only a bounded prefix.
}

func (r dependencySystemRunner) run(ctx context.Context, executable string, args ...string) dependencyCommandResult {
	if !filepath.IsAbs(executable) {
		return dependencyCommandResult{Err: errors.New("dependency executable must be absolute")}
	}
	output := &dependencyOutput{}
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Dir = r.dir
	cmd.Env = dependencyProcessEnvironment(r.environment)
	cmd.Stdout, cmd.Stderr = output, output
	cmd.WaitDelay = 2 * time.Second
	release, err := startOwnedProcess(cmd)
	if err != nil {
		return dependencyCommandResult{Err: err}
	}
	defer release()
	err = cmd.Wait()
	result := dependencyCommandResult{Output: string(output.data), Err: err}
	if cmd.ProcessState != nil {
		result.Code = uint32(cmd.ProcessState.ExitCode())
	}
	return result
}
