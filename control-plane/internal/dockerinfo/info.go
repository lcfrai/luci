// Package dockerinfo reads the selected Docker engine without invoking the
// Docker CLI's info formatter, which enumerates installed CLI plugins.
package dockerinfo

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

type Info struct {
	ID            string
	OSType        string
	ServerVersion string
	MemTotal      int64
	NCPU          int
	Swarm         struct{ LocalNodeState string }
}

// Probe uses Docker's own selected-transport proxy. It therefore preserves
// Docker contexts (including SSH/TLS) and DOCKER_HOST without guessing a
// local socket or exposing endpoint credentials. It sends one bounded GET to
// the daemon; it never asks the CLI to render `docker info` or list plugins.
// Set ignoreInheritedHost where the caller's existing command runner discards
// ambient DOCKER_HOST in favor of an explicit selected host.
func Probe(ctx context.Context, executable, selectedHost string, environment []string, ignoreInheritedHost bool) (Info, error) {
	var result Info
	if executable == "" {
		return result, errors.New("Docker executable is unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "system", "dial-stdio")
	cmd.WaitDelay = 2 * time.Second
	cmd.Env = selectedEnvironment(environment, selectedHost, ignoreInheritedHost)
	cmd.Stderr = io.Discard // Context paths and transport errors may be private.
	hideWindow(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return result, err
	}
	defer stdin.Close()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return result, err
	}
	if err = cmd.Start(); err != nil {
		return result, err
	}
	settled := false
	defer func() {
		if !settled {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://api.moby.localhost/info", nil)
	if err = request.Write(stdin); err != nil {
		return result, errors.New("selected Docker engine did not accept the info request")
	}
	response, err := http.ReadResponse(bufio.NewReader(stdout), request)
	if err != nil {
		return result, errors.New("selected Docker engine did not answer")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return result, errors.New("selected Docker engine refused the info request")
	}
	const limit = 1 << 20
	data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil || len(data) > limit || json.Unmarshal(data, &result) != nil {
		return Info{}, errors.New("selected Docker engine returned invalid bounded info")
	}
	// Windows npipe CloseWrite sends an EOF message. Closing before the daemon's
	// answer can abort the request, so retire our write side only after reading
	// the complete bounded response.
	_ = stdin.Close()
	if err = cmd.Wait(); err != nil {
		settled = true
		return Info{}, errors.New("selected Docker transport did not settle")
	}
	settled = true
	return result, nil
}

func selectedEnvironment(base []string, selectedHost string, ignoreInheritedHost bool) []string {
	allowed := map[string]bool{"PATH": true, "SYSTEMROOT": true, "WINDIR": true, "SYSTEMDRIVE": true, "COMSPEC": true, "TEMP": true, "TMP": true, "TMPDIR": true, "HOME": true, "USERPROFILE": true, "LOCALAPPDATA": true, "APPDATA": true, "PROGRAMFILES": true, "PROGRAMFILES(X86)": true, "PROGRAMW6432": true, "PROGRAMDATA": true, "ALLUSERSPROFILE": true, "DOCKER_CONFIG": true, "DOCKER_CONTEXT": true, "DOCKER_CERT_PATH": true, "DOCKER_TLS_VERIFY": true, "DOCKER_HOST": true}
	selected := make([]string, 0, len(base)+1)
	for _, item := range base {
		key, _, ok := strings.Cut(item, "=")
		if ok && allowed[strings.ToUpper(key)] && !(ignoreInheritedHost && strings.EqualFold(key, "DOCKER_HOST")) && !(selectedHost != "" && strings.EqualFold(key, "DOCKER_HOST")) {
			selected = append(selected, item)
		}
	}
	if selectedHost != "" {
		selected = append(selected, "DOCKER_HOST="+selectedHost)
	}
	return selected
}

func Environment() []string { return os.Environ() }
