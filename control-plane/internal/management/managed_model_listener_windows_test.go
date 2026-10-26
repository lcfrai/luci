//go:build windows

package management

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// Pure TCP-table/callback and inert original-kernel fixtures only. These tests
// do not query TCP state, open a process, or start a provider/GPU/model.
func managedListenerTestTable(port int, pids ...uint32) []byte {
	table := make([]byte, 4+24*len(pids))
	binary.LittleEndian.PutUint32(table[:4], uint32(len(pids)))
	for i, pid := range pids {
		row := table[4+24*i : 4+24*(i+1)]
		binary.LittleEndian.PutUint32(row[:4], 2)
		copy(row[4:8], []byte{127, 0, 0, 1})
		binary.BigEndian.PutUint16(row[8:10], uint16(port))
		binary.LittleEndian.PutUint32(row[20:24], pid)
	}
	return table
}

func requireManagedListenerDiagnostic(t *testing.T, err error, reason string) {
	t.Helper()
	d := managedObservationDiagnostic(managedListenerObservation("listener", err, 0))
	if err == nil || managedListenerReason(err) != reason || d == nil || d.Stage != "listener" || d.Reason != reason {
		t.Fatal("actual listener branch lost its sanitized projection")
	}
	if errors.Is(err, os.ErrNotExist) != (reason == "listener_missing") {
		t.Fatal("unknown native failure promoted to absent-port evidence")
	}
}

func TestManagedListenerTableKeepsTwoCallBoundAndFixedQueryReasons(t *testing.T) {
	for _, test := range []struct {
		name, reason string
		size         uint32
		first, last  uint32
		calls        int
	}{
		{"valid", "", 28, 122, 0, 2},
		{"probe_error", "listener_table_size_failed", 28, 5, 0, 1},
		{"probe_success_without_buffer", "listener_table_size_failed", 28, 0, 0, 1},
		{"probe_short", "listener_table_size_failed", 3, 122, 0, 1},
		{"probe_oversize", "listener_table_size_failed", (1 << 20) + 1, 122, 0, 1},
		{"read_resize", "listener_table_resize_required", 28, 122, 122, 2},
		{"read_error", "listener_table_read_failed", 28, 122, 5, 2},
		{"read_native_not_found", "listener_table_read_failed", 28, 122, 2, 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			buf, err := managedListenerTable(func(buf []byte, size *uint32) uint32 {
				calls++
				if calls == 1 {
					if buf != nil || *size != 0 {
						t.Fatal("initial census changed")
					}
					*size = test.size
					return test.first
				}
				if calls != 2 || len(buf) != int(test.size) || len(buf) > 1<<20 {
					t.Fatal("census retried or allocation bound changed")
				}
				copy(buf, managedListenerTestTable(24567, 55))
				if test.last == 122 {
					*size = test.size + 24
				}
				return test.last
			})
			if calls != test.calls {
				t.Fatal("unexpected native query count")
			}
			if test.reason == "" {
				if err != nil || len(buf) != 28 {
					t.Fatal("valid fixed census refused", err)
				}
				return
			}
			if err == nil || buf != nil || managedListenerReason(err) != test.reason || errors.Is(err, os.ErrNotExist) {
				t.Fatal("unknown census confused with absent listener")
			}
			requireManagedListenerDiagnostic(t, err, test.reason)
		})
	}
}

func TestManagedListenerDecoderSeparatesAbsentForeignDuplicateAndUnknownPID(t *testing.T) {
	foreign := managedListenerTestTable(24567, 55)
	foreign[8] = 0
	truncated := managedListenerTestTable(24567, 55)[:8]
	for _, test := range []struct {
		name, reason string
		table        []byte
		missing      bool
	}{
		{"valid", "", managedListenerTestTable(24567, 55), false},
		{"empty", "listener_missing", managedListenerTestTable(24567), true},
		{"other_port", "listener_missing", managedListenerTestTable(24568, 55), true},
		{"foreign_address", "listener_address_mismatch", foreign, false},
		{"duplicate_same_owner", "listener_multiple", managedListenerTestTable(24567, 55, 55), false},
		{"duplicate_other_owner", "listener_multiple", managedListenerTestTable(24567, 55, 66), false},
		{"unknown_pid", "listener_pid_unavailable", managedListenerTestTable(24567, 0), false},
		{"short_header", "listener_table_invalid", []byte{1, 0, 0}, false},
		{"short_rows", "listener_table_invalid", truncated, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			pid, err := managedListenerPID(test.table, 24567)
			if test.reason == "" {
				if err != nil || pid != 55 {
					t.Fatal("exact loopback listener changed", err)
				}
				return
			}
			if pid != 0 || err == nil || managedListenerReason(err) != test.reason || errors.Is(err, os.ErrNotExist) != test.missing {
				t.Fatal("listener branch or absence semantics changed")
			}
			requireManagedListenerDiagnostic(t, err, test.reason)
		})
	}
}

type managedListenerFakeKernel struct {
	*managedModelFakeKernel
	waitErr, jobErr error
	waits, jobs     int
}

func (k *managedListenerFakeKernel) wait(handle windows.Handle, ms uint32) (bool, error) {
	k.waits++
	if handle != 3 || ms != 0 {
		return false, errors.New("wrong original wait")
	}
	return k.exited, k.waitErr
}
func (k *managedListenerFakeKernel) jobState(handle windows.Handle) (managedModelJobState, error) {
	k.jobs++
	if handle != 1 {
		return managedModelJobState{}, errors.New("wrong original Job")
	}
	return k.job, k.jobErr
}

func TestManagedListenerOwnerReasonsKeepExactOriginalHandleAndJob(t *testing.T) {
	for _, test := range []struct {
		name, reason string
		pid          uint32
		stopping     bool
		closing      bool
		exited       bool
		waitErr      bool
		jobErr       bool
		members      []uint32
	}{
		{name: "valid", pid: 55, members: []uint32{55}},
		{name: "valid_with_child", pid: 55, members: []uint32{55, 66}},
		{name: "stopping", reason: "owner_stopping", pid: 55, stopping: true},
		{name: "closing_handle", reason: "owner_stopping", pid: 55, closing: true},
		{name: "foreign_pid", reason: "owner_pid_mismatch", pid: 66, members: []uint32{55, 66}},
		{name: "wait_unknown", reason: "owner_wait_failed", pid: 55, waitErr: true},
		{name: "exited", reason: "owner_exited", pid: 55, exited: true},
		{name: "job_unknown", reason: "job_query_failed", pid: 55, jobErr: true},
		{name: "not_in_job", reason: "owner_not_in_job", pid: 55, members: []uint32{66}},
	} {
		t.Run(test.name, func(t *testing.T) {
			k := &managedListenerFakeKernel{managedModelFakeKernel: managedModelFake()}
			k.exited, k.job.pids = test.exited, test.members
			if test.waitErr {
				k.waitErr = errors.Join(os.ErrNotExist, errors.New("private native wait context"))
			}
			if test.jobErr {
				k.jobErr = errors.Join(os.ErrNotExist, errors.New("private native Job context"))
			}
			o := &managedModelProcessOwner{kernel: k, stopRequested: test.stopping}
			o.process.value, o.job.value, o.process.attempted = 3, 1, test.closing
			o.latest.Store(managedModelOwnerStatus{identity: managedModelIdentity{pid: 55, created: 100}})
			s := windowsManagedModelService{owner: o}
			err := s.ownedPID(test.pid)
			if test.reason == "" {
				if err != nil || k.waits != 1 || k.jobs != 1 {
					t.Fatal("verified original owner refused", err)
				}
			} else {
				requireManagedListenerDiagnostic(t, err, test.reason)
			}
			if (test.stopping || test.closing || test.pid != 55) && (k.waits != 0 || k.jobs != 0) {
				t.Fatal("stale owner reached native checks")
			}
			if k.terminated || len(k.closes) != 0 || o.process.value != 3 || o.job.value != 1 {
				t.Fatal("diagnostic mutated or retired original ownership")
			}
		})
	}
}

func TestManagedListenerReasonsPreserveStartupWaitAndSafeProjection(t *testing.T) {
	for _, test := range []struct {
		name, want string
		before     bool
		ready      bool
		unknownPID bool
		wait       bool
	}{
		{"absent_before", "starting", true, false, false, true},
		{"absent_expired", "startup_timeout", false, false, false, false},
		{"absent_ready", "listener_ownership_unavailable", true, true, false, false},
		{"unknown_pid_before", "listener_ownership_unavailable", true, false, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			deadline := time.Now().Add(-time.Minute)
			if test.before {
				deadline = time.Now().Add(time.Minute)
			}
			s := windowsManagedModelService{startupDeadline: deadline, metadataReady: test.ready}
			var err error
			if test.unknownPID {
				_, err = managedListenerPID(managedListenerTestTable(24567, 0), 24567)
			} else {
				_, err = managedListenerPID(managedListenerTestTable(24567), 24567)
			}
			result, got := s.listenerFailure(managedModelSample{}, err)
			if result.failure != test.want || (got == nil) != test.wait || !s.startupDeadline.Equal(deadline) {
				t.Fatal("absence wait/deadline changed or unknown PID admitted")
			}
		})
	}
	// Representative table, original-owner and absence errors must also survive
	// HTTP wrapping and the response verification seam without becoming retries.
	for _, reason := range []string{"listener_table_resize_required", "owner_pid_mismatch", "listener_missing"} {
		t.Run(reason, func(t *testing.T) {
			for _, stage := range []string{"listener", "peer"} {
				cause := managedListenerBranch(reason, errors.New("private native context"))
				err := managedListenerObservation(stage, cause, 0)
				wrapped := &url.Error{Op: "Get", URL: "http://private-endpoint", Err: err}
				classified := managedMetadataRequestFailure("tags", "request_failed", wrapped, 0)
				s := windowsManagedModelService{startupDeadline: time.Now().Add(time.Minute)}
				result, got := s.metadataFailure(managedModelSample{}, classified)
				d := managedObservationDiagnostic(got)
				if got == nil || result.failure != "listener_ownership_unavailable" || d == nil || d.Stage != stage || d.Reason != reason {
					t.Fatal("typed listener branch was hidden or delayed")
				}
				raw, e := json.Marshal(d)
				if e != nil || strings.Contains(string(raw), "private") {
					t.Fatal("private native evidence escaped projection")
				}
				var out map[string]any
				client := &http.Client{Transport: managedObservationRoundTrip(func(r *http.Request) (*http.Response, error) {
					return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{}`)), Request: r}, nil
				})}
				post := managedModelMetadata(context.Background(), client, "http://127.0.0.1:24567", "/api/tags", "", &out, func() error { return cause })
				if detail := managedObservationDiagnostic(post); detail == nil || detail.Stage != "listener" || detail.Reason != reason || detail.HTTPStatus == nil || *detail.HTTPStatus != 200 {
					t.Fatal("post-response branch lost")
				}
			}
		})
	}
	if managedListenerReason(managedListenerBranch("private-reason", errors.New("private cause"))) != "ownership_unavailable" {
		t.Fatal("unknown native reason exposed")
	}
	for _, stage := range []string{"listener", "peer", "show"} {
		unknown := managedObservationFailure(stage, "private-reason", nil, 0)
		if managedObservationDiagnostic(unknown) != nil {
			t.Fatal("unknown observation reason exposed")
		}
	}
	if managedObservationDiagnostic(managedObservationFailure("tags", "owner_pid_mismatch", nil, 0)) != nil {
		t.Fatal("native ownership reason attached to unrelated metadata stage")
	}
}
