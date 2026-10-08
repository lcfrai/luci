//go:build windows

package dockerbe

import (
	"bytes"
	"context"
	"errors"
	"github.com/docker/docker/api/types/system"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
	"golang.org/x/sys/windows"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

type capacityOutput struct{ bytes.Buffer }

func (b *capacityOutput) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 4096 {
		return 0, io.ErrShortBuffer
	}
	return b.Buffer.Write(p)
}

func probeDaemonDisk(ctx context.Context, api APIClient, info system.Info) (resourcebudget.Disk, error) {
	endpoint, ok := api.(interface{ DaemonHost() string })
	if !ok || !strings.HasPrefix(endpoint.DaemonHost(), "npipe://") || info.OSType != "linux" || !strings.Contains(strings.ToLower(info.OperatingSystem), "docker desktop") || !validDaemonDiskTarget(info.DockerRootDir, info.ID) {
		return resourcebudget.Disk{}, errors.New("capacity probe requires local Docker Desktop Linux execution")
	}
	systemDirectory, e := windows.GetSystemDirectory()
	if e != nil {
		return resourcebudget.Disk{}, e
	}
	// Invoke the Windows system executable, never a workspace/PATH substitute.
	cmd := exec.CommandContext(ctx, filepath.Join(systemDirectory, "wsl.exe"), "--distribution", "docker-desktop", "--user", "root", "--exec", "/bin/sh", "-c", desktopDiskScript, "luci-capacity", info.DockerRootDir, info.ID)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	var output capacityOutput
	cmd.Stdout = &output
	cmd.Stderr = io.Discard
	if e := cmd.Run(); e != nil {
		return resourcebudget.Disk{}, errors.New("Docker Desktop's matching daemon filesystem could not be measured through WSL")
	}
	return parseDaemonDiskOutput(output.Bytes(), info.ID)
}
