//go:build windows

package api

import (
	"math"
	"testing"
	"time"
)

func TestDebianBinaryRawAbsoluteTickUsesCapturedOriginalDeadline(t *testing.T) {
	base := time.Unix(1700000000, 0).UTC()
	for _, row := range []struct{name string;tick uint64;now,deadline time.Time;want uint64;refuse bool}{
		{"floor", 1000, base, base.Add(1500*time.Microsecond), 1001, false},
		{"original-delay", 1000, base.Add(2*time.Second), base.Add(300*time.Second), 299000, false},
		{"maximum", 1000, base, base.Add(300*time.Second), 301000, false},
		{"zero-boot-tick", 0, base, base.Add(time.Millisecond), 1, false},
		{"expired", 1000, base, base, 0, true},
		{"late", 1000, base.Add(time.Second), base, 0, true},
		{"fraction-only", 1000, base, base.Add(time.Microsecond), 0, true},
		{"too-long", 1000, base, base.Add(300*time.Second+time.Nanosecond), 0, true},
		{"overflow", math.MaxUint64, base, base.Add(time.Millisecond), 0, true},
		{"missing-now", 1000, time.Time{}, base, 0, true},
		{"missing-deadline", 1000, base, time.Time{}, 0, true},
	} {
		t.Run(row.name, func(t *testing.T) {
			got, err := debianBinaryRawAbsoluteTick(row.tick, row.now, row.deadline)
			if row.refuse { if err == nil { t.Fatal("invalid captured scalar accepted", got) }; return }
			if err != nil || got != row.want { t.Fatal(got, err, row.want) }
		})
	}
	// Values are synthetic arithmetic inputs. This does not sample clocks,
	// initialize an owner or prove a tickFIRST/Go-nowSECOND native call order.
}
