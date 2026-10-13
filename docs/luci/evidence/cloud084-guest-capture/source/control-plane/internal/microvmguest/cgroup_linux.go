//go:build linux

package microvmguest

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend/e2bbe"
)

const taskCgroupParent = "/sys/fs/cgroup/luci-tasks"
const kernelLaunchReceipt = "launch-kernel.json"
const cgroup2SuperMagic = 0x63677270

// FrozenTask is produced only by exact kernel identity and freeze observation.
// It does not establish a successful upstream exit or release VM ownership.
// There is deliberately no exported unfreeze or caller-supplied boolean path.
type FrozenTask struct {
	identity  e2bbe.CaptureIdentity
	directory string
	kernel    taskKernel
}

func (f FrozenTask) Identity() e2bbe.CaptureIdentity { return f.identity }

type taskKernel interface {
	bootID() (string, error)
	selfGroup() (string, error)
	inode(string) (uint64, error)
	read(string, string) (string, error)
	write(string, string, string) error
}
type nativeTaskKernel struct{}

func (nativeTaskKernel) bootID() (string, error) {
	b, e := os.ReadFile("/proc/sys/kernel/random/boot_id")
	return strings.TrimSpace(string(b)), e
}
func (nativeTaskKernel) selfGroup() (string, error) {
	b, e := os.ReadFile("/proc/self/cgroup")
	if e != nil {
		return "", e
	}
	value := ""
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "0::") {
			if value != "" {
				return "", ErrRefused
			}
			value = strings.TrimPrefix(line, "0::")
		}
	}
	if value == "" || !strings.HasPrefix(value, "/") || filepath.Clean(value) != value {
		return "", ErrRefused
	}
	return value, nil
}
func (nativeTaskKernel) inode(path string) (uint64, error) {
	info, e := os.Lstat(path)
	if e != nil {
		return 0, e
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || st.Uid != 0 || info.Mode().Perm()&0022 != 0 {
		return 0, ErrRefused
	}
	var fs syscall.Statfs_t
	if e = syscall.Statfs(path, &fs); e != nil {
		return 0, e
	}
	if fs.Type != cgroup2SuperMagic {
		return 0, ErrRefused
	}
	return st.Ino, nil
}
func (nativeTaskKernel) read(path, name string) (string, error) {
	f, e := openKernelControl(path, name, os.O_RDONLY)
	if e != nil {
		return "", e
	}
	defer f.Close()
	b := make([]byte, 4097)
	n, e := f.Read(b)
	if e != nil || n > 4096 {
		return "", ErrRefused
	}
	return strings.TrimSpace(string(b[:n])), nil
}
func (nativeTaskKernel) write(path, name, value string) error {
	f, e := openKernelControl(path, name, os.O_WRONLY)
	if e != nil {
		return e
	}
	_, e = f.WriteString(value)
	ce := f.Close()
	if e != nil {
		return e
	}
	return ce
}
func openKernelControl(path, name string, flags int) (*os.File, error) {
	switch name {
	case "cgroup.procs", "cgroup.freeze", "cgroup.events":
	default:
		return nil, ErrRefused
	}
	fd, e := syscall.Open(filepath.Join(path, name), flags|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if e != nil {
		return nil, e
	}
	return os.NewFile(uintptr(fd), name), nil
}

// BindTaskCgroup runs in the trusted root bootstrap, after the one-shot launch
// reservation, before Exec drops privileges. The entire process is moved, so
// every later sandboxd/harness descendant inherits the undelegated task group.
func BindTaskCgroup(ctx context.Context, p Prepared, identity e2bbe.CaptureIdentity) (e2bbe.CaptureIdentity, error) {
	if os.Geteuid() != 0 || validatePrepared(p) != nil || ctx.Err() != nil {
		return identity, ErrRefused
	}
	if identity.Identity != p.manifest.Binding.Identity || identity.InputBundleDigest != p.archiveSHA256 || identity.SourceDigest != p.manifest.Binding.SourceDigest || identity.ContextDigest != p.manifest.Binding.ContextDigest {
		return identity, ErrRefused
	}
	if _, e := os.Lstat(filepath.Join(p.directory, "launch-reserved.json")); e != nil {
		return identity, ErrRefused
	}
	k := nativeTaskKernel{}
	if _, e := k.inode(taskCgroupParent); e != nil {
		return identity, e
	}
	// Exact cgroup mount root is part of the trusted Linux template. A delegated
	// subtree or v1 namespace is not silently treated as the same path boundary.
	mount, e := os.ReadFile("/proc/self/mountinfo")
	if e != nil {
		return identity, e
	}
	rootMount := false
	for _, line := range strings.Split(string(mount), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 7 && fields[3] == "/" && fields[4] == "/sys/fs/cgroup" && strings.Contains(line, " - cgroup2 ") {
			rootMount = true
		}
	}
	if !rootMount {
		return identity, ErrRefused
	}
	identity.Version = 1
	identity.ProcessPID = uint32(os.Getpid())
	identity.CgroupRelativePath = "luci-tasks/" + identity.Identity.OperationID
	identity.BootID, e = k.bootID()
	if e != nil {
		return identity, e
	}
	path := filepath.Join("/sys/fs/cgroup", identity.CgroupRelativePath)
	// Validate all non-kernel inputs before creating a group; zero inode is only
	// this local preparation value, never an accepted controller receipt.
	identity.CgroupInode = 1
	if e = e2bbe.ValidateCaptureIdentityShape(identity); e != nil {
		return identity, e
	}
	if e = os.Mkdir(path, 0700); e != nil {
		return identity, e
	}
	identity.CgroupInode, e = k.inode(path)
	if e != nil {
		return identity, e
	}
	if e = writeJSON(filepath.Join(p.directory, kernelLaunchReceipt), identity); e != nil {
		return identity, e
	}
	if e = ctx.Err(); e != nil {
		return identity, e
	}
	if e = k.write(path, "cgroup.procs", strconv.Itoa(os.Getpid())); e != nil {
		return identity, e
	}
	current, e := k.selfGroup()
	if e != nil || current != "/"+identity.CgroupRelativePath {
		return identity, ErrRefused
	}
	return identity, nil
}

// FreezeTaskGroup runs in the root collector, outside the task's group. The
// controller must separately bind this exact prior launch receipt to its
// authenticated native exit before asking for capture. Freezing is not exit.
func FreezeTaskGroup(ctx context.Context, identity e2bbe.CaptureIdentity) (FrozenTask, error) {
	if os.Geteuid() != 0 {
		return FrozenTask{}, ErrRefused
	}
	return freezeTaskGroup(ctx, identity, nativeTaskKernel{})
}
func freezeTaskGroup(ctx context.Context, identity e2bbe.CaptureIdentity, k taskKernel) (FrozenTask, error) {
	if e := e2bbe.ValidateCaptureIdentityShape(identity); e != nil {
		return FrozenTask{}, e
	}
	path := filepath.Join("/sys/fs/cgroup", identity.CgroupRelativePath)
	task := FrozenTask{identity: identity, directory: path, kernel: k}
	if e := task.verifyIdentity(ctx); e != nil {
		return FrozenTask{}, e
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if e := k.write(path, "cgroup.freeze", "1"); e != nil {
		return FrozenTask{}, e
	}
	// Cancellation or an unavailable freeze never unfreezes the task or proves
	// cleanup. Preserve the VM/charge and reconcile the exact owned operation.
	timer := time.NewTicker(10 * time.Millisecond)
	defer timer.Stop()
	for {
		if e := task.verifyIdentity(bounded); e != nil {
			return FrozenTask{}, e
		}
		text, e := k.read(path, "cgroup.events")
		if e != nil {
			return FrozenTask{}, e
		}
		frozen, e := kernelFrozen(text)
		if e != nil {
			return FrozenTask{}, e
		}
		if frozen {
			if e = task.verifyFrozen(bounded); e != nil {
				return FrozenTask{}, e
			}
			return task, nil
		}
		select {
		case <-bounded.Done():
			return FrozenTask{}, bounded.Err()
		case <-timer.C:
		}
	}
}
func kernelFrozen(value string) (bool, error) {
	seen := false
	frozen := false
	for _, line := range strings.Split(value, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return false, ErrRefused
		}
		switch fields[0] {
		case "frozen":
			if seen || (fields[1] != "0" && fields[1] != "1") {
				return false, ErrRefused
			}
			seen = true
			frozen = fields[1] == "1"
		case "populated":
			if fields[1] != "0" && fields[1] != "1" {
				return false, ErrRefused
			}
		default:
			return false, ErrRefused
		}
	}
	if !seen {
		return false, ErrRefused
	}
	return frozen, nil
}
func (f FrozenTask) verifyIdentity(ctx context.Context) error {
	if e := ctx.Err(); e != nil {
		return e
	}
	if f.kernel == nil || f.directory != filepath.Join("/sys/fs/cgroup", f.identity.CgroupRelativePath) {
		return ErrRefused
	}
	boot, e := f.kernel.bootID()
	if e != nil || boot != f.identity.BootID {
		return ErrRefused
	}
	inode, e := f.kernel.inode(f.directory)
	if e != nil || inode != f.identity.CgroupInode {
		return ErrRefused
	}
	self, e := f.kernel.selfGroup()
	if e != nil || self == "/"+f.identity.CgroupRelativePath || strings.HasPrefix(self, "/"+f.identity.CgroupRelativePath+"/") {
		return ErrRefused
	}
	return nil
}
func (f FrozenTask) verifyFrozen(ctx context.Context) error {
	if e := f.verifyIdentity(ctx); e != nil {
		return e
	}
	requested, e := f.kernel.read(f.directory, "cgroup.freeze")
	if e != nil || requested != "1" {
		return ErrRefused
	}
	events, e := f.kernel.read(f.directory, "cgroup.events")
	if e != nil {
		return e
	}
	frozen, e := kernelFrozen(events)
	if e != nil || !frozen {
		return fmt.Errorf("exact task freeze not observed: %w", errors.Join(ErrRefused, e))
	}
	return nil
}
