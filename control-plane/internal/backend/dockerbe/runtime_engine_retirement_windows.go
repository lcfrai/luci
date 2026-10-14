package dockerbe

import (
	"context"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"

	"github.com/docker/docker/client"
	"github.com/lcfrai/luci/control-plane/internal/backend"
	"golang.org/x/sys/windows"
)

// Docker Desktop's WSL distribution is the selected local engine host. A
// failed Docker request alone cannot prove it stopped: the exact distribution
// must disappear from the running WSL inventory while that engine is also
// unreachable. This observer never stops Docker or any user workload.
func (b *Backend) ObserveRuntimeImageEngine(ctx context.Context) (backend.RuntimeImageEngineState, error) {
	cli, ok := b.api.(*client.Client)
	endpoint := ""
	if ok {
		endpoint = strings.ToLower(cli.DaemonHost())
	}
	if endpoint != "npipe:////./pipe/docker_engine" && endpoint != "npipe:////./pipe/dockerdesktoplinuxengine" {
		return backend.RuntimeImageEngineState{}, backend.ErrUnsupported
	}
	systemDirectory, err := windows.GetSystemDirectory()
	if err != nil {
		return backend.RuntimeImageEngineState{}, backend.ErrUnavailable
	}
	observeWSL := func(probe context.Context) (bool, error) {
		cmd := exec.CommandContext(probe, filepath.Join(systemDirectory, "wsl.exe"), "--list", "--running", "--quiet")
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		var output capacityOutput // 4 KiB cap; never buffer arbitrary child output
		cmd.Stdout, cmd.Stderr = &output, io.Discard
		if cmd.Run() != nil || probe.Err() != nil {
			return false, backend.ErrUnavailable
		}
		return dockerDesktopWSLRunning(output.Bytes())
	}
	observeInfo := func(probe context.Context) (string, error) {
		info, err := cli.Info(probe)
		if err != nil || probe.Err() != nil {
			return "", backend.ErrUnavailable
		}
		if info.ID == "" || info.OSType != "linux" || !strings.Contains(strings.ToLower(info.OperatingSystem), "docker desktop") {
			return "", nil // API answered, but not as the selected engine
		}
		return info.ID, nil
	}
	return observeRuntimeImageEngineRetirement(ctx, 2*time.Second, observeWSL, observeInfo)
}

func dockerDesktopWSLRunning(output []byte) (bool, error) {
	var value string
	utf16Output := false
	if len(output) >= 2 && output[0] == 0xff && output[1] == 0xfe {
		output = output[2:]
		utf16Output = true
	}
	if len(output) >= 2 && output[1] == 0 {
		utf16Output = true
	}
	if utf16Output {
		if len(output)%2 != 0 {
			return false, backend.ErrUnavailable
		}
		units := make([]uint16, len(output)/2)
		for i := range units {
			units[i] = uint16(output[2*i]) | uint16(output[2*i+1])<<8
		}
		value = string(utf16.Decode(units))
	} else {
		value = string(output)
	}
	for _, line := range strings.FieldsFunc(value, func(r rune) bool { return r == '\n' || r == '\r' || r == 0 }) {
		if strings.TrimSpace(line) == "docker-desktop" {
			return true, nil
		}
	}
	return false, nil
}
