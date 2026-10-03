package platform

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Runner never places runtime secrets in arguments. Input may contain a source
// archive; output is bounded even when an application writes continuously.
type Runner func(context.Context, string, []string, string, []byte) ([]byte, error)
type boundedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.Len() {
		return 0, errors.New("command output exceeds limit")
	}
	return b.Buffer.Write(p)
}
func commandRunner(dockerHost string) Runner {
	return func(ctx context.Context, exe string, args []string, dir string, input []byte) ([]byte, error) {
		cmd := exec.CommandContext(ctx, exe, args...)
		hide(cmd)
		cmd.Dir = dir
		cmd.Stdin = bytes.NewReader(input)
		cmd.WaitDelay = 3 * time.Second
		// Keep normal OS/Docker discovery, but never inherit provider/model credentials
		// or repository-controlled GIT_* configuration from the host environment.
		allowed := map[string]bool{"PATH": true, "SYSTEMROOT": true, "WINDIR": true, "SYSTEMDRIVE": true, "COMSPEC": true, "TEMP": true, "TMP": true, "TMPDIR": true, "HOME": true, "USERPROFILE": true, "LOCALAPPDATA": true, "APPDATA": true, "PROGRAMFILES": true, "PROGRAMFILES(X86)": true, "PROGRAMW6432": true, "PROGRAMDATA": true, "ALLUSERSPROFILE": true, "DOCKER_CONFIG": true, "DOCKER_CONTEXT": true, "DOCKER_CERT_PATH": true, "DOCKER_TLS_VERIFY": true}
		for _, v := range os.Environ() {
			key, _, _ := strings.Cut(v, "=")
			if allowed[strings.ToUpper(key)] {
				cmd.Env = append(cmd.Env, v)
			}
		}
		cmd.Env = append(cmd.Env, "GIT_TERMINAL_PROMPT=0", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
		if dockerHost != "" {
			cmd.Env = append(cmd.Env, "DOCKER_HOST="+dockerHost)
		}
		out := &boundedBuffer{limit: 64 << 20}
		cmd.Stdout = out
		cmd.Stderr = io.Discard
		if exe == "docker" && len(args) > 0 && args[0] == "logs" {
			cmd.Stderr = out
		}
		if err := cmd.Run(); err != nil {
			return nil, errors.New("platform command failed")
		}
		return out.Bytes(), nil
	}
}
func providerHTTPClient() *http.Client {
	return &http.Client{Timeout: 12 * time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}
