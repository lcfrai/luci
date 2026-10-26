package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestNativeManagedRequestUsesOneOwnerThenHTTPShutdownBudget(t *testing.T) {
	now := time.Now()
	clock := now
	signal := newManagedShutdownSignal(func() time.Time { return clock })
	token := strings.Repeat("x", 40)
	handler := withManagedShutdown(http.NotFoundHandler(), token, signal.request)
	request := httptest.NewRequest(http.MethodPost, "/v1/control-plane/shutdown", nil)
	request.RemoteAddr = "127.0.0.1:43123"
	request.Header.Set("X-Luci-Lifecycle-Token", token)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 200 {
		t.Fatal(response.Code)
	}
	select {
	case <-signal.done:
	case <-time.After(time.Second):
		t.Fatal("managed request did not enter the shared shutdown path")
	}
	expected := now.Add(35 * time.Second)
	if !signal.deadline.Equal(expected) {
		t.Fatal("wrong initial close budget")
	}
	clock = now.Add(17 * time.Second)
	signal.request()
	if !signal.deadline.Equal(expected) {
		t.Fatal("second trigger reset close budget")
	}
	ctx, cancel := context.WithDeadline(context.Background(), signal.deadline)
	defer cancel()
	var order []string
	err := finishManagedShutdown(ctx, func(c context.Context) error {
		order = append(order, "owners")
		deadline, _ := c.Deadline()
		if !deadline.Equal(expected) {
			t.Fatal("owner received a new deadline")
		}
		clock = now.Add(31 * time.Second)
		return nil
	}, func(c context.Context) error {
		order = append(order, "http")
		deadline, _ := c.Deadline()
		if !deadline.Equal(expected) || deadline.Sub(clock) != 4*time.Second {
			t.Fatal("HTTP received more than the original remaining budget")
		}
		return nil
	})
	if err != nil || !reflect.DeepEqual(order, []string{"owners", "http"}) {
		t.Fatal("managed HTTP shutdown preceded dependent retirement", order, err)
	}
}
