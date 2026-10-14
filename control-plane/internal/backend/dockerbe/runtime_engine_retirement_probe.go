package dockerbe

import (
	"context"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

// Each observation has its own deadline. In particular, a stopped Docker
// Desktop engine can make Info wait until its deadline; that timeout must not
// consume the time needed for a fresh, positive WSL host observation.
func observeRuntimeImageEngineRetirement(
	ctx context.Context,
	probeLimit time.Duration,
	wslRunning func(context.Context) (bool, error),
	engineInfo func(context.Context) (string, error),
) (backend.RuntimeImageEngineState, error) {
	unknown := backend.RuntimeImageEngineState{}
	observeWSL := func() (bool, error) {
		probe, cancel := context.WithTimeout(ctx, probeLimit)
		defer cancel()
		running, err := wslRunning(probe)
		if err != nil || probe.Err() != nil {
			return false, backend.ErrUnavailable
		}
		return running, nil
	}
	if ctx.Err() != nil || probeLimit <= 0 {
		return unknown, backend.ErrUnavailable
	}
	running, err := observeWSL()
	if err != nil {
		return unknown, err
	}
	probe, cancel := context.WithTimeout(ctx, probeLimit)
	engineID, infoErr := engineInfo(probe)
	infoDeadline := probe.Err()
	cancel()
	if ctx.Err() != nil {
		return unknown, backend.ErrUnavailable
	}
	if running {
		if infoErr != nil || infoDeadline != nil || engineID == "" {
			return unknown, backend.ErrUnavailable
		}
		return backend.RuntimeImageEngineState{State: "running", EngineID: engineID}, nil
	}
	if infoErr == nil {
		return unknown, backend.ErrUnavailable // API still answered; stop is unproved
	}
	// Neither a Docker timeout nor a single WSL response proves retirement.
	// The second bounded host read must still see the exact distro stopped.
	running, err = observeWSL()
	if err != nil || running || ctx.Err() != nil {
		return unknown, backend.ErrUnavailable
	}
	return backend.RuntimeImageEngineState{State: "stopped"}, nil
}
