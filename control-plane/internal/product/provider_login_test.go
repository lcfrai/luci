package product

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type testLoginProcess func() error

func (run testLoginProcess) Run() error { return run() }

func awaitProviderStatus(t *testing.T, s *Server, id, status string) ProviderLoginJob {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, job := range s.providerLoginSnapshots() {
			if job.ID == id && job.Status == status {
				return job
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("login never reached %s: %+v", status, s.providerLoginSnapshots())
	return ProviderLoginJob{}
}

func TestProviderLoginDeviceCodeFlowDuplicateAndSafeCompletion(t *testing.T) {
	s := &Server{}
	defer s.closeProviderLogins()
	started := make(chan struct{})
	finish := make(chan struct{})
	calls := 0
	factory := func(ctx context.Context, command ProviderLoginCommand, writer io.Writer) (ProviderLoginProcess, error) {
		calls++
		if command.Name != "az" || !reflect.DeepEqual(command.Args, []string{"login", "--use-device-code", "--output", "none"}) {
			t.Fatalf("unexpected command: %+v", command)
		}
		if !strings.Contains(strings.Join(command.Env, " "), "AZURE_CORE_LOGIN_EXPERIENCE_V2=off") {
			t.Fatal("hidden interactive subscription prompt not disabled")
		}
		if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > 10*time.Minute || time.Until(deadline) < 9*time.Minute {
			t.Fatal("login lacks a bounded lifetime")
		}
		return testLoginProcess(func() error {
			_, _ = writer.Write([]byte("To sign in, open https://microsoft.com/device"))
			_, _ = writer.Write([]byte("login and enter the code ABCD12345 to authenticate.\n"))
			_, _ = writer.Write([]byte(`{"accessToken":"PRIVATE-ACCESS-TOKEN","subscription":"PRIVATE-SUBSCRIPTION"}`))
			close(started)
			select {
			case <-finish:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}), nil
	}
	job, err := s.startProviderLogin("azure", factory)
	if err != nil {
		t.Fatal(err)
	}
	<-started
	waiting := awaitProviderStatus(t, s, job.ID, "waiting")
	if waiting.VerificationURL != "https://microsoft.com/devicelogin" || waiting.UserCode != "ABCD12345" {
		t.Fatalf("device flow missing instructions: %+v", waiting)
	}
	duplicate, err := s.startProviderLogin("azure", factory)
	if err != nil || duplicate.ID != job.ID || calls != 1 {
		t.Fatal("duplicate sign-in launched another process")
	}
	mux := http.NewServeMux()
	s.registerProviderLogin(mux)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/v1/setup/cloud/login", nil))
	if strings.Contains(w.Body.String(), "PRIVATE-") {
		t.Fatal("raw credential or account output leaked")
	}
	close(finish)
	completed := awaitProviderStatus(t, s, job.ID, "succeeded")
	if completed.VerificationURL != "" || completed.UserCode != "" {
		t.Fatal("completed job retained an expired device code")
	}
}

func TestProviderLoginCloseCancelsOwnedProcessAndClearsInstructions(t *testing.T) {
	s := &Server{}
	started := make(chan struct{})
	job, err := s.startProviderLogin("aws", func(ctx context.Context, command ProviderLoginCommand, writer io.Writer) (ProviderLoginProcess, error) {
		if command.Name != "aws" || !reflect.DeepEqual(command.Args, []string{"login", "--no-cli-pager"}) {
			t.Fatal("wrong AWS command")
		}
		return testLoginProcess(func() error {
			_, _ = writer.Write([]byte("https://signin.aws.amazon.com/oauth?response_type=code&client_id=cli&state=random\n"))
			close(started)
			<-ctx.Done()
			return ctx.Err()
		}), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	<-started
	awaitProviderStatus(t, s, job.ID, "waiting")
	s.closeProviderLogins()
	completed := awaitProviderStatus(t, s, job.ID, "cancelled")
	if completed.VerificationURL != "" || completed.UserCode != "" {
		t.Fatal("cancelled job retained login instructions")
	}
}

func TestProviderLoginMissingCLIAndFailuresDoNotExposeRawErrors(t *testing.T) {
	s := &Server{}
	defer s.closeProviderLogins()
	job, err := s.startProviderLogin("aws", func(context.Context, ProviderLoginCommand, io.Writer) (ProviderLoginProcess, error) {
		return nil, &exec.Error{Name: "SECRET-PATH", Err: exec.ErrNotFound}
	})
	if err != nil || job.Status != "failed" || job.ActionURL != "https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html" || strings.Contains(job.Message, "SECRET") {
		t.Fatalf("missing CLI not actionable: %+v %v", job, err)
	}
	job, err = s.startProviderLogin("aws", func(context.Context, ProviderLoginCommand, io.Writer) (ProviderLoginProcess, error) {
		return testLoginProcess(func() error { return errors.New("SECRET-CREDENTIAL") }), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	job = awaitProviderStatus(t, s, job.ID, "failed")
	encoded, _ := json.Marshal(s.providerLoginSnapshots())
	if strings.Contains(string(encoded), "SECRET") {
		t.Fatal("process error leaked")
	}
	if _, err = s.startProviderLogin("shell", nil); err == nil {
		t.Fatal("arbitrary executable provider accepted")
	}
}

func TestProviderLoginOnlyExposesAllowlistedAuthInstructions(t *testing.T) {
	for _, tc := range []struct {
		provider, address string
		allowed           bool
	}{
		{"azure", "https://microsoft.com/devicelogin", true},
		{"azure", "https://aka.ms/devicelogin", true},
		{"azure", "https://login.microsoftonline.com/common/oauth2/deviceauth", true},
		{"azure", "https://microsoft.com.evil.example/devicelogin", false},
		{"azure", "https://evil.example/devicelogin", false},
		{"azure", "https://microsoft.com/devicelogin?token=secret", false},
		{"azure", "https://microsoft.com/another-path", false},
		{"aws", "https://signin.aws.amazon.com/oauth?response_type=code&code_challenge=abc", true},
		{"aws", "https://us-east-1.signin.aws.amazon.com/oauth", true},
		{"aws", "https://signin.aws.amazon.com.evil.example/oauth", false},
		{"aws", "https://signin.aws.amazon.com/oauth?access_token=secret", false},
		{"aws", "https://signin.aws.amazon.com/oauth?code=secret", false},
		{"aws", "https://user:secret@signin.aws.amazon.com/oauth", false},
		{"aws", "http://signin.aws.amazon.com/oauth", false},
		{"aws", "https://signin.aws.amazon.com:8443/oauth", false},
	} {
		if allowedProviderLoginURL(tc.provider, tc.address) != tc.allowed {
			t.Errorf("unexpected auth URL decision for %s", tc.address)
		}
	}
	var mu sync.Mutex
	var updates []string
	writer := &providerLoginWriter{provider: "azure", update: func(address, code string) { mu.Lock(); updates = append(updates, address+" "+code); mu.Unlock() }}
	_, _ = writer.Write([]byte("https://evil.example/access_token=secret enter the code PRIVATE123\n"))
	if len(updates) != 0 {
		t.Fatal("untrusted verification URL reached UI")
	}
	_, _ = writer.Write([]byte(strings.Repeat("x", 32<<10)))
	if len(writer.buffer) > 16<<10 {
		t.Fatal("raw output buffer was not bounded")
	}
}
