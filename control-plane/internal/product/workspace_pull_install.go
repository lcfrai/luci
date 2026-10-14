package product

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

// Reserve the private staging volume before making copies of the two held
// payloads. This is host disk demand, separate from later Docker admission.
func (s *Server) pullStageCapacity(dir string, sizes ...int64) error {
	if err := prepareWorkspaceStagingDir(dir); err != nil {
		return err
	}
	var required uint64 = 4 << 30 // Existing minimum free storage reserve.
	for _, size := range sizes {
		if size <= 0 || size > 4<<30 || required > 10<<30-uint64(size) {
			return fmt.Errorf("runtime transfer size is outside its verified bound")
		}
		required += uint64(size)
	}
	available := s.workspaceTransferAvailable
	if available == nil {
		available = func(path string) (uint64, error) {
			disk, err := resourcebudget.ObserveDisk(resourcebudget.DiskPath{Role: "workspace-transfer", Path: path})
			if err != nil || disk.AvailableBytes == nil {
				return 0, fmt.Errorf("workspace transfer volume capacity unavailable")
			}
			return *disk.AvailableBytes, nil
		}
	}
	free, err := available(dir)
	if err != nil {
		return err
	}
	if free < required {
		return fmt.Errorf("Waiting for storage: verified runtime transfer needs %d MiB including the 4096 MiB free reserve; %d MiB is available", required>>20, free>>20)
	}
	return nil
}

// installPullWorkspace is the ordinary SSH compatibility installer. It uses
// the verified v2 package only; the large store and bootstrap are transferred
// once each, separately from the small core and bounded metadata.
func (s *Server) installPullWorkspace(job WorkspaceInstall, t Tunnel) {
	ctx, cancel := context.WithTimeout(s.workspaceContext, 45*time.Minute)
	defer cancel()
	open := s.workspacePullCompanionOpen
	if open == nil {
		open = s.openWorkspacePullCompanion
	}
	c, err := open(ctx)
	if err != nil {
		s.finishWorkspaceInstall(job, "failed", "The Linux core and its matching runtime pull companion could not be verified. Reinstall the complete Luci package before retrying.")
		return
	}
	defer c.close()
	if c.core == nil || c.metadata == nil || c.pull == nil || c.pull.Store == nil || c.pull.Bootstrap == nil {
		s.finishWorkspaceInstall(job, "failed", "The verified runtime pull companion is incomplete. No remote installation started.")
		return
	}
	stagingDir := filepath.Join(s.stateDir, "workspace-transfer-staging")
	type transfer struct {
		source                       *os.File
		size                         int64
		hash, suffix, staged, remote string
	}
	m := c.pull.Manifest
	files := []transfer{
		{source: c.core, size: c.coreSize, hash: c.coreHash, suffix: ".tar.gz"},
		{source: c.metadata, size: c.metadataSize, hash: c.metadataHash, suffix: ".runtime-metadata.tar"},
		{source: c.pull.Store, size: m.RegistryStore.Bytes, hash: m.RegistryStore.SHA256, suffix: "." + m.RegistryStore.FileName},
		{source: c.pull.Bootstrap, size: m.Bootstrap.Bytes, hash: m.Bootstrap.SHA256, suffix: "." + m.Bootstrap.FileName},
	}
	if err := s.pullStageCapacity(stagingDir, files[0].size, files[1].size, files[2].size, files[3].size); err != nil {
		s.finishWorkspaceInstall(job, "failed", err.Error()+". No remote installation started.")
		return
	}
	for i := range files {
		files[i].staged, err = stageWorkspaceFile(ctx, files[i].source, files[i].size, files[i].hash, stagingDir, job.ID+"-pull")
		if err != nil {
			for j := range files {
				if files[j].staged != "" {
					_ = os.Remove(files[j].staged)
				}
			}
			s.finishWorkspaceInstall(job, "failed", "Could not stage the exact verified runtime files. No remote installation started.")
			return
		}
		files[i].remote = "/home/" + t.Request.User + "/" + job.ID + files[i].suffix
	}
	defer func() {
		for _, file := range files {
			_ = os.Remove(file.staged)
		}
	}()
	host := t.Request.Host
	if net.ParseIP(host) != nil && strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	for _, file := range files {
		args := append(sshArguments(t, true), file.staged, t.Request.User+"@"+host+":"+file.remote)
		if _, err := s.runner(ctx, "scp", args, ""); err != nil {
			s.finishWorkspaceInstall(job, "unknown", "A verified runtime transfer did not confirm completion. Inspect only this installation's remote temporary files before retrying.")
			return
		}
	}
	script := pullWorkspaceInstallScript(c, job, t, files[0].remote, files[1].remote, files[2].remote, files[3].remote)
	args := append(sshArguments(t, false), t.Request.User+"@"+t.Request.Host, "sudo -n /usr/bin/bash -c "+shellQuote(script))
	if _, err := s.runner(ctx, "ssh", args, ""); err != nil {
		s.finishWorkspaceInstall(job, "unknown", "Remote installation did not confirm completion. Inspect the selected server and its service before retrying.")
		return
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for i := 0; i < 20; i++ {
		if workspaceReady(ctx, t.APIURL) == nil {
			s.finishWorkspaceInstall(job, "ready", "Luci is ready on your machine. Activate the cloud workspace, then connect an upstream coding harness.")
			return
		}
		select {
		case <-ctx.Done():
			s.finishWorkspaceInstall(job, "unknown", "Installation stopped before readiness could be confirmed. Check the selected machine.")
			return
		case <-ticker.C:
		}
	}
	s.finishWorkspaceInstall(job, "unknown", "Installation ran, but the remote service has not answered. Check server logs before retrying.")
}

func pullWorkspaceInstallScript(c *workspaceCompanion, job WorkspaceInstall, t Tunnel, core, metadata, store, bootstrap string) string {
	m := c.pull.Manifest
	var script strings.Builder
	script.WriteString("set -euo pipefail\n")
	for _, item := range []struct{ hash, path string }{{c.coreHash, core}, {c.metadataHash, metadata}, {m.RegistryStore.SHA256, store}, {m.Bootstrap.SHA256, bootstrap}} {
		fmt.Fprintf(&script, "printf '%%s  %%s\\n' '%s' '%s' | sha256sum --check --status\n", item.hash, item.path)
	}
	fmt.Fprintf(&script, "test \"$(stat -c %%s -- '%s')\" = '%d'\ntest \"$(stat -c %%s -- '%s')\" = '%d'\n", store, m.RegistryStore.Bytes, bootstrap, m.Bootstrap.Bytes)
	fmt.Fprintf(&script, "install -d -m 755 /opt/luci /opt/luci/releases /opt/luci/releases/%s\n", job.ID)
	fmt.Fprintf(&script, "tar --extract --gzip --file '%s' --directory /opt/luci/releases/%s --no-same-owner\n", core, job.ID)
	fmt.Fprintf(&script, "printf '%%s  %%s\\n' '%s' '/opt/luci/releases/%s/runtime/runtime-payload-companion.json' | sha256sum --check --status\n", c.markerHash, job.ID)
	fmt.Fprintf(&script, "tar --extract --file '%s' --directory /opt/luci/releases/%s --no-same-owner\n", metadata, job.ID)
	for _, item := range []struct{ source, name, hash string }{{store, m.RegistryStore.FileName, m.RegistryStore.SHA256}, {bootstrap, m.Bootstrap.FileName, m.Bootstrap.SHA256}} {
		fmt.Fprintf(&script, "install -m 644 -- '%s' '/opt/luci/releases/%s/runtime/%s'\n", item.source, job.ID, item.name)
		fmt.Fprintf(&script, "printf '%%s  %%s\\n' '%s' '/opt/luci/releases/%s/runtime/%s' | sha256sum --check --status\n", item.hash, job.ID, item.name)
	}
	fmt.Fprintf(&script, `chmod 755 /opt/luci/releases/%s/luci
ln -sfn /opt/luci/releases/%s /opt/luci/current
cat > /etc/systemd/system/luci.service <<'UNIT'
[Unit]
Description=Luci cloud workspace
After=docker.service network-online.target
[Service]
User=%s
SupplementaryGroups=docker
Environment=HOME=/home/%s
Environment=LUCI_STATE_DIR=/home/%s/.local/state/luci
Environment=LUCI_PROJECTS_DIR=/home/%s/projects
Environment=LUCI_ADDR=127.0.0.1:8730
Environment=LUCI_SESSION_CPU_MILLIS=1000
Environment=LUCI_SESSION_MEMORY_MIB=1536
ExecStart=/opt/luci/current/luci
Restart=on-failure
UMask=0077
[Install]
WantedBy=multi-user.target
UNIT
install -d -m 700 -o %s /home/%s/projects /home/%s/.local/state/luci
systemctl daemon-reload
systemctl enable luci
systemctl restart luci
`, job.ID, job.ID, t.Request.User, t.Request.User, t.Request.User, t.Request.User, t.Request.User, t.Request.User, t.Request.User)
	for _, path := range []string{core, metadata, store, bootstrap} {
		fmt.Fprintf(&script, "rm -f -- '%s'\n", path)
	}
	return script.String()
}
