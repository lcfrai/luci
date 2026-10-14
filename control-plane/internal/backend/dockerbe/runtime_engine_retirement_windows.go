package dockerbe

import (
	"context"
	"errors"
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
	probe, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	systemDirectory, err := windows.GetSystemDirectory()
	if err != nil {
		return backend.RuntimeImageEngineState{}, backend.ErrUnavailable
	}
	cmd := exec.CommandContext(probe, filepath.Join(systemDirectory, "wsl.exe"), "--list", "--running", "--quiet")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	var output capacityOutput // 4 KiB cap; never buffer arbitrary child output
	cmd.Stdout, cmd.Stderr = &output, io.Discard
	err = cmd.Run()
	if err != nil || probe.Err() != nil {
		return backend.RuntimeImageEngineState{}, errors.Join(err, backend.ErrUnavailable)
	}
	running, err := dockerDesktopWSLRunning(output.Bytes())
	if err != nil {
		return backend.RuntimeImageEngineState{}, err
	}
	info, infoErr := cli.Info(probe)
	if running {
		if infoErr != nil || info.ID == "" || info.OSType != "linux" || !strings.Contains(strings.ToLower(info.OperatingSystem), "docker desktop") || probe.Err() != nil {
			return backend.RuntimeImageEngineState{}, backend.ErrUnavailable
		}
		return backend.RuntimeImageEngineState{State: "running", EngineID: info.ID}, nil
	}
	if infoErr == nil || probe.Err() != nil {
		return backend.RuntimeImageEngineState{}, backend.ErrUnavailable
	}
	// A transient API outage is not an engine stop. Recheck the host boundary
	// after the failed Info request before retaining a stopped receipt.
	cmd = exec.CommandContext(probe, filepath.Join(systemDirectory, "wsl.exe"), "--list", "--running", "--quiet")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output.Reset()
	cmd.Stdout, cmd.Stderr = &output, io.Discard
	if cmd.Run() != nil || probe.Err() != nil {
		return backend.RuntimeImageEngineState{}, backend.ErrUnavailable
	}
	running, err = dockerDesktopWSLRunning(output.Bytes())
	if err != nil || running {
		return backend.RuntimeImageEngineState{}, backend.ErrUnavailable
	}
	return backend.RuntimeImageEngineState{State: "stopped"}, nil
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
