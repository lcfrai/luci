//go:build windows

package management

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/managedmodel"
	"golang.org/x/sys/windows"
)

type managedModelObservationError struct {
	diagnostic managedmodel.ObservationDiagnostic
	cause      error
}

func (e *managedModelObservationError) Error() string { return "managed model observation unavailable" }
func (e *managedModelObservationError) Unwrap() error { return e.cause }
func (e *managedModelObservationError) ManagedModelObservationDiagnostic() *managedmodel.ObservationDiagnostic {
	copy := e.diagnostic
	return &copy
}

func managedObservationFailure(stage, reason string, cause error, status int) error {
	d := managedmodel.ObservationDiagnostic{Stage: stage, Reason: reason}
	if status >= 100 && status <= 599 {
		d.HTTPStatus = &status
	}
	return &managedModelObservationError{diagnostic: d, cause: cause}
}

func managedMetadataRequestFailure(stage, fallback string, cause error, status int) error {
	// Preserve a listener/connected-peer refusal through http.Client's url.Error.
	// url.Error implements net.Error even when its underlying error is semantic.
	if managedObservationDiagnostic(cause) != nil {
		return cause
	}
	reason := fallback
	var transport net.Error
	switch {
	case errors.Is(cause, context.Canceled):
		reason = "cancelled"
	case errors.Is(cause, context.DeadlineExceeded):
		reason = "timeout"
	case errors.As(cause, &transport) && transport.Timeout():
		reason = "timeout"
	case errors.Is(cause, windows.WSAECONNREFUSED):
		reason = "connect_refused"
	case errors.Is(cause, io.EOF), errors.Is(cause, io.ErrUnexpectedEOF), errors.Is(cause, windows.WSAECONNRESET), errors.Is(cause, windows.WSAECONNABORTED):
		reason = "connection_lost"
	}
	return managedObservationFailure(stage, reason, cause, status)
}

// The caller retains its existing single sample deadline and exact endpoint.
// A controlled client/verification callback lets tests exercise the real request
// and decode path without a socket, process, GPU query or model request.
func managedModelMetadata(ctx context.Context, client *http.Client, endpoint, path, body string, out any, verify func() error) (err error) {
	stage := ""
	switch path {
	case "/api/version":
		stage = "version"
	case "/api/tags":
		stage = "tags"
	case "/api/show":
		stage = "show"
	case "/api/ps":
		stage = "ps"
	default:
		return errors.New("unsupported managed metadata route")
	}
	method := http.MethodGet
	var input io.Reader
	if body != "" {
		method, input = http.MethodPost, strings.NewReader(body)
	}
	request, e := http.NewRequestWithContext(ctx, method, endpoint+path, input)
	if e != nil {
		return managedMetadataRequestFailure(stage, "request_failed", e, 0)
	}
	request.Header.Set("Content-Type", "application/json")
	response, e := client.Do(request)
	if e != nil {
		return managedMetadataRequestFailure(stage, "request_failed", e, 0)
	}
	defer func() {
		if e := response.Body.Close(); e != nil && err == nil {
			err = managedMetadataRequestFailure(stage, "response_close_failed", e, response.StatusCode)
		}
		if err == nil && ctx.Err() != nil {
			err = managedMetadataRequestFailure(stage, "request_failed", ctx.Err(), response.StatusCode)
		}
	}()
	if e = ctx.Err(); e != nil {
		return managedMetadataRequestFailure(stage, "request_failed", e, response.StatusCode)
	}
	if e = verify(); e != nil {
		return managedListenerObservation("listener", e, response.StatusCode)
	}
	if e = ctx.Err(); e != nil {
		return managedMetadataRequestFailure(stage, "request_failed", e, response.StatusCode)
	}
	if response.StatusCode != http.StatusOK {
		return managedObservationFailure(stage, "http_status", nil, response.StatusCode)
	}
	raw, e := io.ReadAll(io.LimitReader(response.Body, 256<<10+1))
	if e != nil {
		return managedMetadataRequestFailure(stage, "response_read_failed", e, response.StatusCode)
	}
	if len(raw) > 256<<10 {
		return managedObservationFailure(stage, "response_too_large", nil, response.StatusCode)
	}
	if e = json.Unmarshal(raw, out); e != nil {
		return managedObservationFailure(stage, "response_invalid", e, response.StatusCode)
	}
	return nil
}
