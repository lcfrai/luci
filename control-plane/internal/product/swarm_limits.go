package product

// These are bounded input/storage contracts, not promises of simultaneous
// execution. Actual starts also require current account/provider authority and
// observed resource capacity. Defaults and per-tick scheduling stay unchanged.
const (
	// A reviewed implementation context already selects at most 200 tasks.
	MaxSwarmParallelTasks = 200
	// The durable workspace queue contains at most 500 task entries.
	MaxSwarmScheduledTasks = 500
)
