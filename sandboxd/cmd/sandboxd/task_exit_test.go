package main

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestTaskContainerZeroRequiresActualNormalWaitAndListenerSettlement(t *testing.T) {
	for _, scenario := range []string{"normal", "child-unsuccessful", "listener-failure", "shutdown-failure", "listener-unknown", "listener-timeout"} {
		t.Run(scenario, func(t *testing.T) {
			processDone := make(chan struct{})
			close(processDone)
			stop := make(chan struct{})
			serve := func() error {
				<-stop
				if scenario == "listener-failure" {
					return errors.New("listener failed")
				}
				if scenario == "listener-unknown" {
					return nil
				}
				return http.ErrServerClosed
			}
			shutdown := func(context.Context) error {
				if scenario != "listener-timeout" {
					close(stop)
				}
				if scenario == "shutdown-failure" {
					return errors.New("shutdown failed")
				}
				return nil
			}
			err := serveTaskSession(serve, shutdown, processDone, 5*time.Millisecond)
			if scenario == "listener-timeout" {
				close(stop)
			}
			code := taskContainerExitCode(scenario != "child-unsuccessful", err)
			if (code == 0) != (scenario == "normal") {
				t.Fatal("unknown/unsuccessful task acquired container-zero eligibility", scenario, code, err)
			}
		})
	}
}
