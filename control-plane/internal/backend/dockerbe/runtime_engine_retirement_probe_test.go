package dockerbe

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRuntimeEngineRetirementHangingInfoStillChecksSecondStoppedHostWitness(t *testing.T) {
	wslReads, infoReads := 0, 0
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	state, err := observeRuntimeImageEngineRetirement(ctx, 10*time.Millisecond,
		func(probe context.Context) (bool, error) {
			wslReads++
			if probe.Err() != nil {
				t.Fatal("WSL witness was called with an expired child context")
			}
			return false, nil
		},
		func(probe context.Context) (string, error) {
			infoReads++
			<-probe.Done() // a stopped engine's Info never answers
			return "", probe.Err()
		})
	if err != nil || state.State != "stopped" || state.EngineID != "" || wslReads != 2 || infoReads != 1 || ctx.Err() != nil {
		t.Fatal("Info consumed the fresh WSL witness budget", state, err, wslReads, infoReads, ctx.Err())
	}
}

func TestRuntimeEngineRetirementCancellationAndHostRestartKeepUnknown(t *testing.T) {
	t.Run("outer cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		reads := 0
		state, err := observeRuntimeImageEngineRetirement(ctx, time.Second,
			func(context.Context) (bool, error) { reads++; return false, nil },
			func(probe context.Context) (string, error) {
				cancel()
				<-probe.Done()
				return "", probe.Err()
			})
		if err == nil || state.State != "" || reads != 1 {
			t.Fatal("cancellation was mistaken for an engine-stop receipt", state, err, reads)
		}
	})
	t.Run("host restarted before second witness", func(t *testing.T) {
		reads := 0
		state, err := observeRuntimeImageEngineRetirement(context.Background(), 10*time.Millisecond,
			func(context.Context) (bool, error) { reads++; return reads == 2, nil },
			func(probe context.Context) (string, error) { <-probe.Done(); return "", probe.Err() })
		if err == nil || state.State != "" || reads != 2 {
			t.Fatal("a restarted host was mistaken for a stopped engine", state, err, reads)
		}
	})
	t.Run("API answered while WSL was absent", func(t *testing.T) {
		reads := 0
		state, err := observeRuntimeImageEngineRetirement(context.Background(), time.Second,
			func(context.Context) (bool, error) { reads++; return false, nil },
			func(context.Context) (string, error) { return "different-engine", nil })
		if err == nil || state.State != "" || reads != 1 {
			t.Fatal("a responsive API was mistaken for a stopped engine", state, err, reads)
		}
	})
}

func TestRuntimeEngineRetirementRunningHostRequiresResponsiveExactInfo(t *testing.T) {
	for _, tc := range []struct {
		name string
		info func(context.Context) (string, error)
		want string
	}{
		{"responsive", func(context.Context) (string, error) { return "engine-one", nil }, "running"},
		{"unavailable", func(context.Context) (string, error) { return "", errors.New("offline") }, ""},
		{"empty identity", func(context.Context) (string, error) { return "", nil }, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reads := 0
			state, err := observeRuntimeImageEngineRetirement(context.Background(), time.Second,
				func(context.Context) (bool, error) { reads++; return true, nil }, tc.info)
			if state.State != tc.want || reads != 1 || (tc.want == "running") != (err == nil) {
				t.Fatal(state, err, reads)
			}
		})
	}
}
