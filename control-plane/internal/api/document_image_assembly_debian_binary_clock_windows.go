//go:build windows

package api

import (
	"math"
	"time"
)

// Pure arithmetic over already captured tickFIRST/Go-nowSECOND and the SAME
// original deadline. This function samples no clock and grants no authority.
func debianBinaryRawAbsoluteTick(tickFirst uint64, goNowSecond, originalDeadline time.Time) (uint64, error) {
	if goNowSecond.IsZero() || originalDeadline.IsZero() || !originalDeadline.After(goNowSecond) { return 0, errDebianBinaryRaw }
	remaining := originalDeadline.Sub(goNowSecond)
	if remaining > 300*time.Second { return 0, errDebianBinaryRaw }
	millis := uint64(remaining/time.Millisecond)
	if millis == 0 || millis > 300000 || tickFirst > math.MaxUint64-millis { return 0, errDebianBinaryRaw }
	return tickFirst+millis, nil
}
