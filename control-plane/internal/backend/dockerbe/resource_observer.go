package dockerbe

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/client"
	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

// Resource observation is optional so existing backends and test doubles do
// not pretend to provide measurements they cannot obtain.
type resourceClient interface {
	Info(context.Context) (system.Info, error)
	ContainerStatsOneShot(context.Context, string) (container.StatsResponseReader, error)
}
type imageResourceClient interface {
	ImageInspect(context.Context, string, ...client.ImageInspectOption) (image.InspectResponse, error)
}
type resourceCPUPoint struct {
	usage uint64
	at    time.Time
}

var _ resourcebudget.ExecutionObserver = (*Backend)(nil)

func (b *Backend) ObserveResources(ctx context.Context) resourcebudget.ExecutionSnapshot {
	b.resourceMu.Lock()
	defer b.resourceMu.Unlock()
	s := resourcebudget.ExecutionSnapshot{Status: "unavailable", Scope: "execution-daemon", Containers: []resourcebudget.ContainerUsage{}, Unavailable: []resourcebudget.Issue{}}
	api, ok := b.api.(resourceClient)
	if !ok {
		s.Unavailable = append(s.Unavailable, resourcebudget.Issue{Metric: "execution", Reason: "Docker client does not support resource observation"})
		return s
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	info, e := api.Info(ctx)
	if e != nil {
		s.Unavailable = append(s.Unavailable, resourcebudget.Issue{Metric: "execution", Reason: "Docker daemon resource information is unavailable"})
		return s
	}
	s.EngineID, s.Platform, s.StorageDriver = info.ID, info.OSType, info.Driver
	if info.MemTotal > 0 {
		v := uint64(info.MemTotal)
		s.TotalMemoryBytes = &v
	}
	if info.NCPU > 0 && info.NCPU < math.MaxInt64/1000 {
		v := int64(info.NCPU) * 1000
		s.CPUMillis = &v
	}
	s.DaemonDisk, s.HostStorage = b.observeStorage(ctx, info)
	if s.HostStorage != nil && s.HostStorage.State != "confirmed" {
		s.Unavailable = append(s.Unavailable, resourcebudget.Issue{Metric: "execution.hostStorage", Reason: s.HostStorage.Message})
	}
	if s.DaemonDisk == nil {
		s.Unavailable = append(s.Unavailable, resourcebudget.Issue{Metric: "execution.daemonDisk", Reason: "Docker storage volume is not observable on the control-plane host"})
	}
	imageName := b.resourceImage
	if imageName == "" {
		imageName = "luci/sandbox:dev"
	}
	if images, ok := b.api.(imageResourceClient); ok {
		if image, e := images.ImageInspect(ctx, imageName); e == nil && image.Size > 0 {
			v := uint64(image.Size)
			s.ImageVirtualSizeBytes = &v
			s.ImageSizeBasis = "configured-image-virtual-size; conservative clone reservation, not observed allocation"
		}
	}
	if s.ImageVirtualSizeBytes == nil {
		s.Unavailable = append(s.Unavailable, resourcebudget.Issue{Metric: "execution.imageVirtualSizeBytes", Reason: "configured worker image size is unavailable; observation does not pull images"})
	}
	// Inventory the entire daemon, not just agent sessions: previews, checks,
	// data stacks and unrelated workloads all consume the same capacity.
	rows, e := b.api.ContainerList(ctx, container.ListOptions{All: true})
	if e != nil {
		s.Unavailable = append(s.Unavailable, resourcebudget.Issue{Metric: "execution.containers", Reason: "complete active-container inventory is unavailable"})
		return s
	}
	active := 0
	seen := map[string]bool{}
	for _, row := range rows {
		if row.State != "running" && row.State != "paused" && row.State != "restarting" && row.State != "created" {
			continue
		}
		if _, marker := row.Labels[backend.LabelLaunchFence]; marker && row.State == "created" {
			// Label presence alone is not exemption from capacity. Inspect the
			// exact immutable ID and require the full inert scratch shape. These
			// permanent, never-executable markers still occupy measured disk;
			// the separate fence ledger/inventory limits their count.
			inspected, inspectErr := b.api.ContainerInspect(ctx, row.ID)
			if inspectErr == nil && inspected.ID == row.ID && inertFenceContainer(inspected) {
				continue
			}
		}
		active++
		if active > 128 || ctx.Err() != nil {
			s.Unavailable = append(s.Unavailable, resourcebudget.Issue{Metric: "execution.containers", Reason: "active-container observation exceeded its bounded sample budget"})
			s.Status = "unavailable"
			s.ObservedAt = time.Now().UTC()
			return s
		}
		usage := resourcebudget.ContainerUsage{ID: row.ID, Kind: containerResourceKind(row.Labels), Owned: row.Labels[backend.LabelInstance] == b.instance || row.Labels["ai.lcfr.luci.platform.instance"] == b.instance}
		if usage.Kind == "agent" {
			usage.SandboxID = row.Labels[backend.LabelSandboxID]
		}
		inspect, e := b.api.ContainerInspect(ctx, row.ID)
		if row.State == "created" && e == nil && resourceContainerNeverStarted(row.ID, row.Labels, inspect) {
			// A verified never-started container has no live CPU/RAM demand or
			// meaningful stats. Its storage remains in the measured disk bound.
			// Do not emit a zero-usage row: absence from the live inventory keeps
			// every owned pending allocation charged by the admission ledger.
			continue
		}
		if e == nil && inspect.ContainerJSONBase != nil && inspect.HostConfig != nil {
			if inspect.HostConfig.Memory > 0 {
				v := uint64(inspect.HostConfig.Memory)
				usage.MemoryLimitBytes = &v
			}
			if inspect.HostConfig.NanoCPUs > 0 {
				v := inspect.HostConfig.NanoCPUs / 1_000_000
				usage.CPULimitMillis = &v
			} else if inspect.HostConfig.CPUQuota > 0 && inspect.HostConfig.CPUPeriod > 0 && inspect.HostConfig.CPUQuota <= math.MaxInt64/1000 {
				v := inspect.HostConfig.CPUQuota * 1000 / inspect.HostConfig.CPUPeriod
				usage.CPULimitMillis = &v
			}
		} else {
			s.Unavailable = append(s.Unavailable, resourcebudget.Issue{Metric: "execution.containerLimits", Reason: "an active container's hard limits could not be inspected"})
		}
		stats, e := api.ContainerStatsOneShot(ctx, row.ID)
		if e == nil && stats.Body != nil {
			var data container.StatsResponse
			limited := io.LimitReader(stats.Body, 1<<20)
			decodeErr := json.NewDecoder(limited).Decode(&data)
			_ = stats.Body.Close()
			if decodeErr == nil && (data.ID == "" || data.ID == row.ID) && !data.Read.IsZero() && time.Since(data.Read) <= 5*time.Second && !data.Read.After(time.Now().Add(time.Second)) {
				usage.ObservedAt = data.Read
				if stats.OSType == "linux" {
					v := data.MemoryStats.Usage
					usage.MemoryUsageBytes = &v
					working := v
					if inactive, ok := data.MemoryStats.Stats["inactive_file"]; ok && inactive <= v {
						working -= inactive
					} else if inactive, ok := data.MemoryStats.Stats["total_inactive_file"]; ok && inactive <= v {
						working -= inactive
					}
					usage.WorkingSetBytes = &working
				} else if stats.OSType == "windows" {
					v := data.MemoryStats.PrivateWorkingSet
					usage.MemoryUsageBytes = &v
					usage.WorkingSetBytes = &v
				}
				if previous, ok := b.resourceCPU[row.ID]; ok && data.Read.After(previous.at) && data.CPUStats.CPUUsage.TotalUsage >= previous.usage {
					units := float64(data.CPUStats.CPUUsage.TotalUsage - previous.usage)
					if stats.OSType == "windows" {
						units *= 100
					}
					millis := units / float64(data.Read.Sub(previous.at)) * 1000
					if millis >= 0 && millis <= math.MaxInt64 {
						v := int64(millis)
						usage.CPUUsageMillis = &v
					}
				}
				if b.resourceCPU == nil {
					b.resourceCPU = map[string]resourceCPUPoint{}
				}
				b.resourceCPU[row.ID] = resourceCPUPoint{data.CPUStats.CPUUsage.TotalUsage, data.Read}
				seen[row.ID] = true
			}
		}
		if usage.MemoryUsageBytes == nil {
			s.Unavailable = append(s.Unavailable, resourcebudget.Issue{Metric: "execution.containerUsage", Reason: "an active container's current memory usage is unavailable"})
		}
		if usage.MemoryLimitBytes == nil || usage.CPULimitMillis == nil {
			s.Unavailable = append(s.Unavailable, resourcebudget.Issue{Metric: "execution.containerLimits", Reason: "an active container has an unknown or unbounded memory or CPU limit"})
		}
		s.Containers = append(s.Containers, usage)
	}
	for id := range b.resourceCPU {
		if !seen[id] {
			delete(b.resourceCPU, id)
		}
	}
	s.InventoryComplete = true
	s.ObservedAt = time.Now().UTC()
	if s.TotalMemoryBytes != nil && s.CPUMillis != nil {
		s.Status = "ready"
		if len(s.Unavailable) > 0 {
			s.Status = "partial"
		}
	}
	return s
}

func resourceContainerNeverStarted(id string, labels map[string]string, inspect container.InspectResponse) bool {
	if _, marker := labels[backend.LabelLaunchFence]; marker {
		return false // Only the separate full inert-fence shape check exempts markers.
	}
	if inspect.Config != nil {
		if _, marker := inspect.Config.Labels[backend.LabelLaunchFence]; marker {
			return false
		}
	}
	if id == "" || inspect.ContainerJSONBase == nil || inspect.ID != id || inspect.State == nil {
		return false
	}
	s := inspect.State
	if s.Status != "created" || s.Running || s.Paused || s.Restarting || s.Dead || s.Pid != 0 || inspect.RestartCount != 0 {
		return false
	}
	if s.StartedAt == "" {
		return true
	}
	started, err := time.Parse(time.RFC3339Nano, s.StartedAt)
	return err == nil && started.IsZero()
}

func containerResourceKind(labels map[string]string) string {
	if _, marked := labels[assemblyLabel]; marked {
		if _, valid := assemblyResourceIdentity(labels); valid {
			return "document-image-assembly"
		}
		return "other" // mixed/foreign purpose labels cannot deduplicate a worker.
	}
	switch {
	case labels[backend.LabelManaged] == "true":
		return "agent"
	case labels["luci.check"] == "true":
		return "check"
	case labels["luci.preview"] == "true":
		return "preview"
	case labels[documentLabel] == "true":
		return "document-extraction"
	case labels["ai.lcfr.luci.platform.resource"] != "":
		return "project-service"
	default:
		return "other"
	}
}
