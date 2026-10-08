// Package resourcebudget measures capacity and usage. It does not choose worker
// sizes or scheduling policy. A missing observation is never a measured zero.
package resourcebudget

import (
	"context"
	"time"
)

type Issue struct {
	Metric string `json:"metric"`
	Reason string `json:"reason"`
}

type Memory struct {
	PhysicalTotalBytes     *uint64 `json:"physicalTotalBytes"`
	PhysicalAvailableBytes *uint64 `json:"physicalAvailableBytes"`
	// Effective values include the control-plane's visible cgroup limits on
	// Linux. They do not establish the capacity of a remote Docker daemon.
	EffectiveTotalBytes     *uint64 `json:"effectiveTotalBytes"`
	EffectiveAvailableBytes *uint64 `json:"effectiveAvailableBytes"`
	LimitSource             string  `json:"limitSource"`
	CommitLimitBytes        *uint64 `json:"commitLimitBytes"`
	CommitAvailableBytes    *uint64 `json:"commitAvailableBytes"`
}

type CPU struct {
	LogicalProcessors       *int     `json:"logicalProcessors"`
	EffectiveMillis         *int64   `json:"effectiveMillis"`
	LimitSource             string   `json:"limitSource"`
	UtilizationPercent      *float64 `json:"utilizationPercent"`
	PressureSomeAvg10       *float64 `json:"pressureSomeAvg10"`
	MemoryPressureSomeAvg10 *float64 `json:"memoryPressureSomeAvg10"`
	IOPressureSomeAvg10     *float64 `json:"ioPressureSomeAvg10"`
}

type Disk struct {
	ObservedAt     time.Time `json:"observedAt"`
	Basis          string    `json:"basis"`
	Role           string    `json:"role"`
	TotalBytes     *uint64   `json:"totalBytes"`
	AvailableBytes *uint64   `json:"availableBytes"`
}

type ProcessUsage struct {
	PID                int     `json:"pid"`
	Role               string  `json:"role"`
	StartID            string  `json:"startId"`
	ResidentBytes      *uint64 `json:"residentBytes"`
	PrivateCommitBytes *uint64 `json:"privateCommitBytes"`
}

type HostSnapshot struct {
	ObservedAt  time.Time      `json:"observedAt"`
	Status      string         `json:"status"`
	Platform    string         `json:"platform"`
	Scope       string         `json:"scope"`
	Memory      Memory         `json:"memory"`
	CPU         CPU            `json:"cpu"`
	Disks       []Disk         `json:"disks"`
	Processes   []ProcessUsage `json:"processes"`
	Unavailable []Issue        `json:"unavailable"`
}

// ContainerUsage contains no container name, image, command or environment.
// IDs are for internal reconciliation; callers must authorize any disclosure.
// Owned means the observer recognizes this Luci instance, not a specific team.
type ContainerUsage struct {
	ID               string    `json:"id"`
	SandboxID        string    `json:"sandboxId,omitempty"`
	Owned            bool      `json:"owned"`
	Kind             string    `json:"kind"`
	MemoryLimitBytes *uint64   `json:"memoryLimitBytes"`
	CPULimitMillis   *int64    `json:"cpuLimitMillis"`
	MemoryUsageBytes *uint64   `json:"memoryUsageBytes"`
	WorkingSetBytes  *uint64   `json:"workingSetBytes"`
	CPUUsageMillis   *int64    `json:"cpuUsageMillis"`
	ObservedAt       time.Time `json:"observedAt"`
}

type ExecutionSnapshot struct {
	ObservedAt            time.Time `json:"observedAt"`
	Status                string    `json:"status"`
	Scope                 string    `json:"scope"`
	EngineID              string    `json:"engineId,omitempty"`
	Platform              string    `json:"platform"`
	StorageDriver         string    `json:"storageDriver"`
	ImageVirtualSizeBytes *uint64   `json:"imageVirtualSizeBytes"`
	ImageSizeBasis        string    `json:"imageSizeBasis"`
	DaemonDisk            *Disk     `json:"daemonDisk"`
	// Windows Desktop guest free space does not establish physical room for
	// its dynamic VHD. This separate bound never claims engine/VHD binding.
	HostStorage *HostStorageBound `json:"hostStorage,omitempty"`
	// Daemon allocation is not observed free memory in the Docker VM.
	TotalMemoryBytes  *uint64          `json:"totalMemoryBytes"`
	CPUMillis         *int64           `json:"cpuMillis"`
	Containers        []ContainerUsage `json:"containers"`
	InventoryComplete bool             `json:"inventoryComplete"`
	Unavailable       []Issue          `json:"unavailable"`
}

type Snapshot struct {
	Host      HostSnapshot      `json:"host"`
	Execution ExecutionSnapshot `json:"execution"`
}

// ExecutionObserver is an optional backend capability. A sampler calls it in
// the background; request handlers read the sampler's cached snapshot only.
type ExecutionObserver interface {
	ObserveResources(context.Context) ExecutionSnapshot
}

type DiskPath struct{ Role, Path string }

// An additional process must be registered with its creation identity. A bare
// PID is insufficient because the OS can reuse it after that process exits.
type OwnedProcess struct {
	PID           int
	Role, StartID string
}

type ProbeOptions struct {
	Disks []DiskPath
	// DisksFunc resolves reconfigured project roots in the background sampler.
	// It must be quick and must not perform OS probes itself.
	DisksFunc      func() []DiskPath
	OwnedProcesses []OwnedProcess
}

func ptr[T any](v T) *T { return &v }

func (s HostSnapshot) Fresh(now time.Time, maxAge time.Duration) bool {
	return !s.ObservedAt.IsZero() && !s.ObservedAt.After(now) && now.Sub(s.ObservedAt) <= maxAge
}

func (s ExecutionSnapshot) Fresh(now time.Time, maxAge time.Duration) bool {
	return !s.ObservedAt.IsZero() && !s.ObservedAt.After(now) && now.Sub(s.ObservedAt) <= maxAge
}
