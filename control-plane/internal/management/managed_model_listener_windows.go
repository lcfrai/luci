//go:build windows

package management

import (
	"encoding/binary"
	"errors"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

type managedModelListenerError struct {
	reason string
	cause  error
}

func (e *managedModelListenerError) Error() string { return "managed model listener unavailable" }
func (e *managedModelListenerError) Unwrap() error {
	// Only a successfully decoded absent port may mean os.ErrNotExist. A native
	// census/wait/Job error with that code is still unknown ownership evidence.
	if e.reason == "listener_missing" {
		return e.cause
	}
	return nil
}

func managedListenerBranch(reason string, cause error) error {
	return &managedModelListenerError{reason: reason, cause: cause}
}

func managedListenerReason(err error) string {
	var branch *managedModelListenerError
	if errors.As(err, &branch) && managedListenerReasonAllowed(branch.reason) {
		return branch.reason
	}
	return "ownership_unavailable"
}

func managedListenerObservation(stage string, cause error, status int) error {
	return managedObservationFailure(stage, managedListenerReason(cause), cause, status)
}

// Keep the existing two-call, one-MiB census bound. In particular, a resized
// table is identified but NOT retried or accepted as ownership evidence.
func managedListenerTable(query func([]byte, *uint32) uint32) ([]byte, error) {
	var size uint32
	code := query(nil, &size)
	if code != uint32(windows.ERROR_INSUFFICIENT_BUFFER) || size < 4 || size > 1<<20 {
		return nil, managedListenerBranch("listener_table_size_failed", windows.Errno(code))
	}
	buf := make([]byte, size)
	code = query(buf, &size)
	if code == uint32(windows.ERROR_INSUFFICIENT_BUFFER) {
		return nil, managedListenerBranch("listener_table_resize_required", windows.Errno(code))
	}
	if code != 0 {
		return nil, managedListenerBranch("listener_table_read_failed", windows.Errno(code))
	}
	return buf, nil
}

func managedListenerPID(buf []byte, port int) (uint32, error) {
	if len(buf) < 4 {
		return 0, managedListenerBranch("listener_table_invalid", nil)
	}
	count := binary.LittleEndian.Uint32(buf[:4])
	if uint64(count)*24+4 > uint64(len(buf)) {
		return 0, managedListenerBranch("listener_table_invalid", nil)
	}
	var pid uint32
	for i := uint32(0); i < count; i++ {
		row := buf[4+i*24 : 4+(i+1)*24]
		if int(binary.BigEndian.Uint16(row[8:10])) != port {
			continue
		}
		if row[4] != 127 || row[5] != 0 || row[6] != 0 || row[7] != 1 {
			return 0, managedListenerBranch("listener_address_mismatch", nil)
		}
		if pid != 0 {
			return 0, managedListenerBranch("listener_multiple", nil)
		}
		pid = binary.LittleEndian.Uint32(row[20:24])
		if pid == 0 {
			return 0, managedListenerBranch("listener_pid_unavailable", nil)
		}
	}
	if pid == 0 {
		return 0, managedListenerBranch("listener_missing", os.ErrNotExist)
	}
	return pid, nil
}

func (s *windowsManagedModelService) listenerFailure(result managedModelSample, err error) (managedModelSample, error) {
	if errors.Is(err, os.ErrNotExist) {
		if !s.metadataReady && time.Now().Before(s.startupDeadline) {
			result.failure = "starting"
			return result, nil
		}
		if !s.metadataReady {
			result.failure = "startup_timeout"
		} else {
			result.failure = "listener_ownership_unavailable"
		}
	} else {
		result.failure = "listener_ownership_unavailable"
	}
	return result, managedListenerObservation("listener", err, 0)
}
