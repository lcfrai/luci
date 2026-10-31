//go:build linux || darwin

package hub

import "testing"

func TestNormalZeroRequiresCompletedOriginalWaitNotManualFinish(t *testing.T) {
	for _, kind := range []string{"normal", "nonzero", "no-wait", "manual-close"} {
		h := &Hub{done: make(chan struct{}), subs: map[int]*subscriber{}, waitObserved: kind != "no-wait", normalZero: kind != "nonzero"}
		if kind == "manual-close" {
			h.finish("exit status 0")
		} else {
			h.finishWithWait("diagnostic text has no authority", true)
		}
		if h.NormalZero() != (kind == "normal") {
			t.Fatal("diagnostic/manual lifecycle invented a completed normal Wait", kind)
		}
	}
}
