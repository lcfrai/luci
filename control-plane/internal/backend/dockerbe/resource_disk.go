package dockerbe

import (
	"context"
	"errors"
	"github.com/docker/docker/api/types/system"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
	"math"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type daemonDiskCache struct {
	disk         resourcebudget.Disk
	engine, root string
	failedAt     time.Time
}

// Disk capacity is needed within five seconds at admission. A four-second
// engine-bound cache leaves one sampler interval for publication, while the
// one-second native host and Docker API observations remain unchanged. Failed
// WSL probes are cached only as failure, so they never grant capacity.
const daemonDiskProbeInterval = 4 * time.Second

func (c *daemonDiskCache) matches(info system.Info, now time.Time) bool {
	if c == nil || c.engine != info.ID || c.root != info.DockerRootDir {
		return false
	}
	at := c.disk.ObservedAt
	if !c.failedAt.IsZero() {
		at = c.failedAt
	}
	return !at.IsZero() && !at.After(now) && now.Sub(at) < daemonDiskProbeInterval
}

func (b *Backend) observeDaemonDisk(ctx context.Context, info system.Info) *resourcebudget.Disk {
	if cached := b.resourceDisk; cached.matches(info, time.Now()) {
		if !cached.failedAt.IsZero() {
			return nil
		}
		d := cached.disk
		return &d
	}
	d, e := probeDaemonDisk(ctx, b.api, info)
	if e != nil {
		if ctx.Err() == nil {
			b.resourceDisk = &daemonDiskCache{engine: info.ID, root: info.DockerRootDir, failedAt: time.Now()}
		}
		return nil
	}
	b.resourceDisk = &daemonDiskCache{disk: d, engine: info.ID, root: info.DockerRootDir}
	return &d
}

var daemonRootPattern = regexp.MustCompile(`^/[A-Za-z0-9._/-]+$`)
var daemonIDPattern = regexp.MustCompile(`^[A-Za-z0-9:-]{1,128}$`)

func validDaemonDiskTarget(root, id string) bool {
	return daemonRootPattern.MatchString(root) && path.Clean(root) == root && root != "/" && daemonIDPattern.MatchString(id)
}

// This fixed read-only script binds the observed filesystem to the engine ID
// returned by Docker's API. /proc/<dockerd>/root resolves the daemon's mount
// namespace, not an arbitrary distro root or the Windows host's C: drive.
const desktopDiskScript = `set -eu
PATH=/usr/sbin:/usr/bin:/sbin:/bin
export PATH
root=$1
expected=$2
for proc in /proc/[0-9]*; do
  command=
  IFS= read -r command < "$proc/comm" 2>/dev/null || continue
  [ "$command" = dockerd ] || continue
  target="$proc/root$root"
  [ -f "$target/engine-id" ] || continue
  observed=
  IFS= read -r observed < "$target/engine-id" || [ -n "$observed" ] || continue
  [ "$observed" = "$expected" ] || continue
  printf '%s\n' "$observed"
  LC_ALL=C df -Pk "$target" | tail -n 1
  exit 0
done
exit 73
`

func parseDaemonDiskOutput(raw []byte, engineID string) (resourcebudget.Disk, error) {
	d := resourcebudget.Disk{Role: "docker", Basis: "windows-wsl-matching-engine-filesystem"}
	if len(raw) > 4096 {
		return d, errors.New("daemon capacity response too large")
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 2 || strings.TrimSpace(lines[0]) != engineID {
		return d, errors.New("daemon capacity identity mismatch")
	}
	f := strings.Fields(lines[1])
	if len(f) < 6 {
		return d, errors.New("invalid daemon filesystem capacity")
	}
	total, e1 := strconv.ParseUint(f[1], 10, 64)
	available, e2 := strconv.ParseUint(f[3], 10, 64)
	if e1 != nil || e2 != nil || total == 0 || available > total || total > math.MaxUint64/1024 {
		return d, errors.New("invalid daemon filesystem capacity")
	}
	total *= 1024
	available *= 1024
	d.TotalBytes = &total
	d.AvailableBytes = &available
	d.ObservedAt = time.Now().UTC()
	return d, nil
}
