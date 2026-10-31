package api

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/coder/websocket"
)

type terminalFrames struct {
	frames [][]byte
	final  error
	reads  int
}

func (f *terminalFrames) Reader(context.Context) (websocket.MessageType, io.Reader, error) {
	f.reads++
	if len(f.frames) == 0 {
		return 0, nil, f.final
	}
	v := f.frames[0]
	f.frames = f.frames[1:]
	return websocket.MessageBinary, bytes.NewReader(v), nil
}
func TestTaskTerminalPassiveObserverRequiresTypedExitAndNormalEOF(t *testing.T) {
	for _, kind := range []string{"zero", "nonzero", "unknown", "stdout-lookalike", "missing-eof", "extra-after-exit", "oversized-exit", "cancelled"} {
		t.Run(kind, func(t *testing.T) {
			frames := &terminalFrames{frames: [][]byte{[]byte("\x00opaque output"), []byte("\x03exit status 0")}, final: websocket.CloseError{Code: websocket.StatusNormalClosure}}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch kind {
			case "nonzero":
				frames.frames[1] = []byte("\x03exit status 1")
			case "unknown":
				frames.frames[1] = []byte("\x03unknown")
			case "stdout-lookalike":
				frames.frames = [][]byte{[]byte("\x00exit status 0")}
			case "missing-eof":
				frames.final = errors.New("transport lost")
			case "extra-after-exit":
				frames.frames = append(frames.frames, []byte("\x00late bytes"))
			case "oversized-exit":
				frames.frames[1] = append([]byte{3}, bytes.Repeat([]byte("x"), 65)...)
			case "cancelled":
				cancel()
			}
			exit, eof := taskTerminalReadEOF(ctx, frames)
			if (eof && exit.Successful()) != (kind == "zero") {
				t.Fatal("unobserved or failed terminal result promoted", kind, eof, exit.Successful())
			}
		})
	}
}

func TestTaskTerminalObservationBudgetIsOriginalAndNeverRenewed(t *testing.T) {
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	created := now.Add(-29 * time.Minute)
	deadline, ok := taskTerminalObservationDeadline(created, now)
	if !ok || deadline.Sub(now) != time.Minute {
		t.Fatal("remaining observation budget was renewed", deadline, ok)
	}
	for _, invalid := range []time.Time{time.Time{}, now.Add(time.Second), now.Add(-30 * time.Minute), now.Add(-31 * time.Minute)} {
		if _, ok := taskTerminalObservationDeadline(invalid, now); ok {
			t.Fatal("invalid/expired observation admitted", invalid)
		}
	}
	later, ok := taskTerminalObservationDeadline(created, now.Add(30*time.Second))
	if !ok || !later.Equal(deadline) {
		t.Fatal("a later wait changed the original absolute deadline")
	}
	if _, ok := taskTerminalObservationDeadline(created, deadline); ok {
		t.Fatal("expired clock gained a new wait")
	}
}
