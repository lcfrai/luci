package platform

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func incompleteInstallFixture(t *testing.T, legacy bool) (*Service, *dataFake, dataInstallation, dataSecret) {
	t.Helper()
	s, f := newDataFake(t)
	if e := s.changeData("owned", func(v *DataStatus) { v.Operation = &DataOperation{Action: "install", Status: "running"} }); e != nil {
		t.Fatal(e)
	}
	if e := s.runDataAction(context.Background(), "owned", dataActionRequest{Action: "install", AcknowledgeResources: true}); e != nil {
		t.Fatal(e)
	}
	if e := s.changeData("owned", func(v *DataStatus) { v.Operation.Status = "succeeded" }); e != nil {
		t.Fatal(e)
	}
	var previous dataInstallation
	if e := s.read("owned-data-installation", &previous); e != nil {
		t.Fatal(e)
	}
	secret, e := s.dataCredentials(previous)
	if e != nil {
		t.Fatal(e)
	}
	if e := s.runDataAction(context.Background(), "owned", dataActionRequest{Action: "stop"}); e != nil {
		t.Fatal(e)
	}
	previous.InitializationState = "pending"
	if legacy {
		previous.InitializationState = ""
	}
	if e := s.write("owned-data-installation", previous); e != nil {
		t.Fatal(e)
	}
	runner := s.opts.Runner
	s.opts.Runner = func(ctx context.Context, exe string, args []string, dir string, input []byte) ([]byte, error) {
		if len(args) > 0 && args[0] == "logs" {
			return []byte("Success. You can now start the database server using:\npsql: error: /docker-entrypoint-initdb.d/init-scripts/98-webhooks.sql: Permission denied\n"), nil
		}
		if len(args) > 0 && args[0] == "stop" {
			f.mu.Lock()
			defer f.mu.Unlock()
			name := args[len(args)-1]
			value := f.containers[name]
			value.State.Running = false
			f.containers[name] = value
			f.stops = append(f.stops, name)
			return nil, nil
		}
		return runner(ctx, exe, args, dir, input)
	}
	return s, f, previous, secret
}

func TestIncompleteInstallRecoveryPreservesOriginalAndPromotesOnlyHealthyCandidate(t *testing.T) {
	s, f, previous, secret := incompleteInstallFixture(t, true)
	originalVolume := f.volumes[s.resource("owned", previous.StackID+"-postgres-data")]
	originalVolumeCopy := map[string]string{}
	for k, v := range originalVolume {
		originalVolumeCopy[k] = v
	}
	transport := s.opts.HTTPClient.Transport
	beforePromotion := map[string]bool{}
	s.opts.HTTPClient.Transport = dataTransport(func(r *http.Request) (*http.Response, error) {
		var active dataInstallation
		if e := s.read("owned-data-installation", &active); e != nil {
			t.Fatal(e)
		}
		if active.StackID == previous.StackID {
			beforePromotion[r.URL.Path] = true
		}
		return transport.RoundTrip(r)
	})
	if e := s.recoverDataInstallation(context.Background(), "owned", previous); e != nil {
		t.Fatal(e)
	}
	var active dataInstallation
	if e := s.read("owned-data-installation", &active); e != nil {
		t.Fatal(e)
	}
	if active.StackID == previous.StackID || active.InitializationState != "complete" || active.storageTenant() != previous.storageTenant() || len(active.Previous) != 1 || active.Previous[0] != previous.StackID {
		t.Fatal("fresh installation did not preserve namespace and retained original")
	}
	currentSecret, e := s.dataCredentials(active)
	if e != nil || !reflect.DeepEqual(currentSecret, secret) {
		t.Fatal("first-install recovery changed account keys")
	}
	for _, endpoint := range []string{"/auth/v1/health", "/rest/v1/", "/storage/v1/status"} {
		if !beforePromotion[endpoint] {
			t.Fatalf("%s was not checked before promotion", endpoint)
		}
	}
	if !reflect.DeepEqual(originalVolumeCopy, f.volumes[s.resource("owned", previous.StackID+"-postgres-data")]) {
		t.Fatal("original volume ownership changed")
	}
	for _, name := range dataServiceNames {
		if f.containers[s.dataName(previous, name)].State.Running {
			t.Fatal("recovery started the original partial stack")
		}
	}
	projectNetwork := s.resource("owned", "net")
	for _, name := range []string{"api-gw", "db"} {
		if f.attachments[s.dataName(previous, name)][projectNetwork] || !f.attachments[s.dataName(active, name)][projectNetwork] {
			t.Fatal("application network retained ambiguous old and fresh aliases")
		}
	}
	for _, sql := range f.sql {
		if sql != "SELECT 1;" {
			t.Fatal("recovery replayed bootstrap migrations or rewrote original SQL")
		}
	}
	status, e := s.loadData("owned")
	if e != nil || status.Status != "ready" || status.InitializationState != "complete" || status.CanRecoverInstall {
		t.Fatal("completed installation remains eligible for fresh recovery")
	}
}

func TestIncompleteInstallRecoveryFailureRetainsSelectionAndStopsCandidate(t *testing.T) {
	for _, failAfterPromotion := range []bool{false, true} {
		t.Run(map[bool]string{false: "before-promotion", true: "after-promotion"}[failAfterPromotion], func(t *testing.T) {
			s, f, previous, _ := incompleteInstallFixture(t, false)
			transport := s.opts.HTTPClient.Transport
			storageCalls := 0
			s.opts.HTTPClient.Transport = dataTransport(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path == "/storage/v1/status" {
					storageCalls++
					if !failAfterPromotion || storageCalls == 2 {
						return nil, errors.New("private fixture API failed")
					}
				}
				return transport.RoundTrip(r)
			})
			if e := s.recoverDataInstallation(context.Background(), "owned", previous); e == nil {
				t.Fatal("unhealthy candidate was accepted")
			}
			var selected dataInstallation
			if e := s.read("owned-data-installation", &selected); e != nil || !reflect.DeepEqual(selected, previous) {
				t.Fatal("failed recovery replaced original selection")
			}
			var retained []string
			if e := s.read("owned-data-retained", &retained); e != nil || len(retained) != 1 {
				t.Fatal("failed candidate is not retained")
			}
			for _, name := range dataServiceNames {
				if f.containers[s.dataName(previous, name)].State.Running || f.containers[s.dataName(dataInstallation{ProjectID: "owned", StackID: retained[0]}, name)].State.Running {
					t.Fatal("failure left the original or candidate running")
				}
			}
			projectNetwork := s.resource("owned", "net")
			for _, name := range []string{"api-gw", "db"} {
				if !f.attachments[s.dataName(previous, name)][projectNetwork] || f.attachments[s.dataName(dataInstallation{ProjectID: "owned", StackID: retained[0]}, name)][projectNetwork] {
					t.Fatal("failed activation did not reinstate the original network selection")
				}
			}
			status, e := s.loadData("owned")
			if e != nil || status.Status != "stopped" || status.InitializationState != "pending" {
				t.Fatal("failure hid the stopped original installation")
			}
		})
	}
}

func TestIncompleteInstallRecoveryRejectsSuccessfulAndAmbiguousLegacyData(t *testing.T) {
	for _, reason := range []string{"complete", "backup", "restore", "running", "legacy-unknown", "legacy-completed", "pending-unknown", "pending-completed", "foreign-container", "foreign-volume"} {
		t.Run(reason, func(t *testing.T) {
			s, f, previous, _ := incompleteInstallFixture(t, strings.HasPrefix(reason, "legacy"))
			switch reason {
			case "complete":
				previous.InitializationState = "complete"
			case "backup":
				_ = s.changeData("owned", func(v *DataStatus) { v.Backups = []DataBackup{{ID: "existing-backup"}} })
			case "restore":
				previous.Previous = []string{"stack-successful"}
			case "running":
				name := s.dataName(previous, "db")
				value := f.containers[name]
				value.State.Running = true
				f.containers[name] = value
			case "foreign-container":
				name := s.dataName(previous, "db")
				value := f.containers[name]
				value.Config.Labels[labelProject] = "someone-else"
				f.containers[name] = value
			case "foreign-volume":
				f.volumes[s.resource("owned", previous.StackID+"-postgres-data")][labelProject] = "someone-else"
			case "legacy-unknown", "legacy-completed", "pending-unknown", "pending-completed":
				runner := s.opts.Runner
				s.opts.Runner = func(ctx context.Context, exe string, args []string, dir string, input []byte) ([]byte, error) {
					if len(args) > 0 && args[0] == "logs" {
						if strings.HasSuffix(reason, "completed") {
							return []byte("Success. You can now start the database server using:\npsql: /docker-entrypoint-initdb.d/init-scripts/98-webhooks.sql: Permission denied\nPostgreSQL init process complete\n"), nil
						}
						return []byte("database system is ready"), nil
					}
					return runner(ctx, exe, args, dir, input)
				}
			}
			if e := s.write("owned-data-installation", previous); e != nil {
				t.Fatal(e)
			}
			before := len(f.containers)
			if e := s.recoverDataInstallation(context.Background(), "owned", previous); e == nil {
				t.Fatal("successful or ambiguous prior data was replaced")
			}
			if len(f.containers) != before {
				t.Fatal("preflight failure allocated a candidate")
			}
		})
	}
}

func TestDataInitializationReceiptIsCachedAndSuccessfulRefreshCompletesIt(t *testing.T) {
	s, f, previous, _ := incompleteInstallFixture(t, true)
	before := len(f.calls)
	response := request(t, s, "GET", "/v1/platform/projects/owned/data", "member", nil)
	if response.Code != 200 || len(f.calls) != before || !strings.Contains(response.Body.String(), `"initializationState":"unknown"`) {
		t.Fatal("status read probed Docker or invented a legacy initialization receipt")
	}
	// A real, successful health refresh is enough to retire an ambiguous legacy
	// initialization state. The receipt is not derived from a caller's claim.
	for _, name := range dataServiceNames {
		value := f.containers[s.dataName(previous, name)]
		value.State.Running = true
		f.containers[s.dataName(previous, name)] = value
	}
	if e := s.runDataAction(context.Background(), "owned", dataActionRequest{Action: "refresh"}); e != nil {
		t.Fatal(e)
	}
	var initialized dataInstallation
	if e := s.read("owned-data-installation", &initialized); e != nil || initialized.InitializationState != "complete" {
		t.Fatal("successful verified refresh did not retain its completed initialization receipt")
	}
	if e := s.runDataAction(context.Background(), "owned", dataActionRequest{Action: "stop"}); e != nil {
		t.Fatal(e)
	}
	if e := s.recoverDataInstallation(context.Background(), "owned", initialized); e == nil {
		t.Fatal("a stopped successful legacy installation was replaced")
	}
}

func TestIncompleteInstallRecoveryRequiresExplicitAcknowledgments(t *testing.T) {
	s, _, _, _ := incompleteInstallFixture(t, false)
	for _, payload := range []dataActionRequest{{Action: "recover-install", AcknowledgeResources: true}, {Action: "recover-install", ConfirmFreshInstall: true}} {
		response := request(t, s, "POST", "/v1/platform/projects/owned/data", "member", payload)
		if response.Code != 400 {
			t.Fatal("fresh recovery omitted an explicit acknowledgment")
		}
	}
}

func TestIncompleteInstallRecoveryReportsUnconfirmedCandidateStop(t *testing.T) {
	s, f, previous, _ := incompleteInstallFixture(t, false)
	transport := s.opts.HTTPClient.Transport
	s.opts.HTTPClient.Transport = dataTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/storage/v1/status" {
			return nil, errors.New("fixture candidate failed")
		}
		return transport.RoundTrip(r)
	})
	runner := s.opts.Runner
	s.opts.Runner = func(ctx context.Context, exe string, args []string, dir string, input []byte) ([]byte, error) {
		if len(args) > 0 && args[0] == "stop" {
			return nil, errors.New("fixture cannot stop candidate")
		}
		return runner(ctx, exe, args, dir, input)
	}
	e := s.recoverDataInstallation(context.Background(), "owned", previous)
	if e == nil || !strings.Contains(dataError(e), "could not confirm") {
		t.Fatal("failure did not disclose unconfirmed candidate cleanup")
	}
	status, e := s.loadData("owned")
	if e != nil || status.Status != "degraded" || status.URL != "" {
		t.Fatal("failure claimed stopped or published a candidate address")
	}
	var retained []string
	if e := s.read("owned-data-retained", &retained); e != nil || len(retained) != 1 {
		t.Fatal("candidate identity was lost after cleanup failure")
	}
	if !f.containers[s.dataName(dataInstallation{ProjectID: "owned", StackID: retained[0]}, "db")].State.Running {
		t.Fatal("fixture did not exercise a candidate that failed to stop")
	}
}
