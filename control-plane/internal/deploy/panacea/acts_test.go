package panacea

import (
	"context"
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
)

// Task 04: the acts. Every method here is a write: one request, never retried, and each test
// counts the requests it made.

const (
	actsP    = "/api/v1/projects/engagement-portal"
	commit40 = "4f0c2a9d1b7e3c5a8f6d0b2e4c6a8f1d3b5e7c9a"
)

// fixtureSentence is a recorded error body's "error" sentence, as Panacea wrote it.
func fixtureSentence(t *testing.T, name string) string {
	t.Helper()
	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(loadFixture(t, name), &body); err != nil || body.Error == "" {
		t.Fatalf("fixture %s has no error sentence: %v", name, err)
	}
	return body.Error
}

// oneRequest fails unless rs saw exactly the one request want.
func oneRequest(t *testing.T, rs *recorded, want string) {
	t.Helper()
	if got := rs.requests(); len(got) != 1 || got[0] != want {
		t.Errorf("requests = %q, want exactly [%q]", got, want)
	}
}

// TestShipSendsExactlyBranchCommit kills a scan-mode override leaking into the body, and
// noCache:false sent where Panacea distinguishes absent (models/models.go:2349-2352).
func TestShipSendsExactlyBranchCommit(t *testing.T) {
	yes, no := true, false
	for _, c := range []struct {
		name    string
		noCache *bool
		want    string
	}{
		{"nil", nil, `{"branch":"feat/x","commit":"` + commit40 + `"}`},
		{"false", &no, `{"branch":"feat/x","commit":"` + commit40 + `"}`},
		{"true", &yes, `{"branch":"feat/x","commit":"` + commit40 + `","noCache":true}`},
	} {
		t.Run(c.name, func(t *testing.T) {
			rs := serveRecorded(t, map[string]fixture{"POST " + actsP + "/deploy": {file: "ship-202.json", status: 202}})
			res, err := rs.c.Ship(context.Background(), project, deploy.ShipRequest{Branch: "feat/x", Commit: commit40, NoCache: c.noCache})
			if err != nil || res.Run != "engagement-portal-deploy-9f3kq" {
				t.Fatalf("Ship = (%+v, %v), want the run ship-202.json names", res, err)
			}
			oneRequest(t, rs, "POST "+actsP+"/deploy")
			if got := rs.sent(); len(got) != 1 || got[0] != c.want {
				t.Errorf("body = %q, want exactly %s", got, c.want)
			}
		})
	}
}

// TestShipRefusesSymbolicCommit kills shipping whatever the branch holds at clone time: a
// commit that is not 40 lowercase hex, or no branch, is refused before any request.
func TestShipRefusesSymbolicCommit(t *testing.T) {
	rs := serveRecorded(t, nil)
	for _, req := range []deploy.ShipRequest{
		{Branch: "feat/x", Commit: "main"},
		{Branch: "feat/x", Commit: "4F0C2A9D1B7E3C5A8F6D0B2E4C6A8F1D3B5E7C9A"},
		{Branch: "feat/x", Commit: commit40[:39]},
		{Branch: "", Commit: commit40},
	} {
		_, err := rs.c.Ship(context.Background(), project, req)
		if deploy.KindOf(err) != deploy.KindRejected {
			t.Errorf("Ship(%+v) = %v, want KindRejected", req, err)
		}
	}
	if got := rs.requests(); len(got) != 0 {
		t.Errorf("a refused Ship reached Panacea: %q", got)
	}
}

// TestShipOneAttemptOn503 kills a retried deploy: a retried POST is a second deploy.
func TestShipOneAttemptOn503(t *testing.T) {
	rs := serveRecorded(t, map[string]fixture{
		"POST " + actsP + "/deploy": {body: `{"error":"service unavailable"}`, status: 503},
	})
	_, err := rs.c.Ship(context.Background(), project, deploy.ShipRequest{Branch: "feat/x", Commit: commit40})
	wantKind(t, err, deploy.KindUnavailable, 503, "")
	oneRequest(t, rs, "POST "+actsP+"/deploy")
}

// TestShip500IsUnavailableWithoutSentence kills rendering Panacea's internal error text
// ("failed to start deployment: "+err.Error(), handlers/projects.go:2991); task 09 writes its own.
func TestShip500IsUnavailableWithoutSentence(t *testing.T) {
	rs := serveRecorded(t, map[string]fixture{"POST " + actsP + "/deploy": {file: "ship-500-timeout.json", status: 500}})
	_, err := rs.c.Ship(context.Background(), project, deploy.ShipRequest{Branch: "feat/x", Commit: commit40})
	wantKind(t, err, deploy.KindUnavailable, 500, "")
	oneRequest(t, rs, "POST "+actsP+"/deploy")
}

// TestShip402IsARefusalWithSentence kills a paused budget (projects.go:2856) or a risk block
// (:2916-2918) read as an outage with no sentence: task 09's abandon branches would never match,
// the reservation would linger 15 minutes, and the reason would be lost (review I2).
func TestShip402IsARefusalWithSentence(t *testing.T) {
	for _, file := range []string{"ship-402-budget.json", "ship-402-risk.json"} {
		t.Run(file, func(t *testing.T) {
			rs := serveRecorded(t, map[string]fixture{"POST " + actsP + "/deploy": {file: file, status: 402}})
			_, err := rs.c.Ship(context.Background(), project, deploy.ShipRequest{Branch: "main", Commit: commit40})
			wantKind(t, err, deploy.KindConflict, 402, fixtureSentence(t, file))
			oneRequest(t, rs, "POST "+actsP+"/deploy")
		})
	}
}

// TestBuiltErrorsCarryTheirStatus kills a built error left at Status 0, which the cockpit reads
// as "did not answer in time" of an answer that arrived, or set to a 5xx, which says "could not
// start the build" after Panacea said "deployment started" (re-review M-4).
func TestBuiltErrorsCarryTheirStatus(t *testing.T) {
	rs := serveRecorded(t, map[string]fixture{
		"POST " + actsP + "/deploy": {status: 202,
			body: `{"message":"deployment started","data":{"project":"engagement-portal","status":"running","pipelineRun":""}}`},
		"POST " + actsP + "/deployments/d1/rollback": {
			body: `{"message":"rollback initiated to d1","data":{"id":"d2","projectName":"engagement-portal","revision":"","status":"rolling-back","image":"i","createdAt":"2026-09-23T11:02:09Z","rollbackWarmth":"warm"}}`},
	})
	ctx := context.Background()
	res, err := rs.c.Ship(ctx, project, deploy.ShipRequest{Branch: "feat/x", Commit: commit40})
	wantKind(t, err, deploy.KindUnavailable, 202, "")
	if res.Run != "" {
		t.Errorf("an unnamed run read as %q", res.Run)
	}
	rb, err := rs.c.Rollback(ctx, project, "d1")
	wantKind(t, err, deploy.KindUnavailable, 200, "")
	if rb.SourceDeploymentID != "" {
		t.Errorf("a rollback answered for d2 still reported source %q", rb.SourceDeploymentID)
	}
	if n := len(rs.requests()); n != 2 {
		t.Errorf("%d requests, want one Ship and one Rollback", n)
	}
	if log := rs.log.String(); !strings.Contains(log, "did not name the run") || !strings.Contains(log, "d2") {
		t.Errorf("the log does not say what was wrong with either answer:\n%s", log)
	}
}

// TestRollbackCarriesWarmth kills dropping the only hint of how long the flip takes: warm is
// the alias flip (clients/kubernetes.go:5962-5989), cold the legacy image patch (:6029-6037).
func TestRollbackCarriesWarmth(t *testing.T) {
	const warm, cold = "engagement-portal-deploy-7k2xq", "engagement-portal-deploy-3mz8d"
	rs := serveRecorded(t, map[string]fixture{
		"POST " + actsP + "/deployments/" + warm + "/rollback": {file: "rollback-warm.json"},
		"POST " + actsP + "/deployments/" + cold + "/rollback": {file: "rollback-cold.json"},
		"POST " + actsP + "/deployments/d-odd/rollback": {
			body: `{"data":{"id":"d-odd","status":"rolling-back","image":"i","createdAt":"2026-09-23T11:02:09Z","rollbackWarmth":"tepid","coldStartSeconds":7}}`},
		// Panacea writes 0 or rollbackColdStartSeconds (clients/kubernetes.go:5970, :6036), never
		// a negative; a negative is no time at all, not "ready 5 s ago" (named survivor Y1).
		"POST " + actsP + "/deployments/d-neg/rollback": {
			body: `{"data":{"id":"d-neg","status":"rolling-back","image":"i","createdAt":"2026-09-23T11:02:09Z","rollbackWarmth":"cold","coldStartSeconds":-5}}`},
	})
	ctx := context.Background()
	for _, c := range []struct {
		id   string
		want deploy.RollbackResult
	}{
		{warm, deploy.RollbackResult{SourceDeploymentID: warm, Image: "panaceacr.azurecr.io/engagement-portal/app:4f0c2a9",
			Warmth: deploy.WarmthWarm}},
		{cold, deploy.RollbackResult{SourceDeploymentID: cold, Image: "panaceacr.azurecr.io/engagement-portal/app:9b1e7c3",
			Warmth: deploy.WarmthCold, ColdStartSeconds: 20}},
		// A word this code does not know is unknown, never a guess, and carries no seconds.
		{"d-odd", deploy.RollbackResult{SourceDeploymentID: "d-odd", Image: "i", Warmth: deploy.WarmthUnknown}},
		{"d-neg", deploy.RollbackResult{SourceDeploymentID: "d-neg", Image: "i", Warmth: deploy.WarmthCold}},
	} {
		got, err := rs.c.Rollback(ctx, project, c.id)
		if err != nil || got != c.want {
			t.Errorf("Rollback(%s) = (%+v, %v), want %+v", c.id, got, err, c.want)
		}
	}
	if n := len(rs.requests()); n != 4 {
		t.Errorf("%d requests, want 4", n)
	}
}

// TestRollbackRefusalsVerbatim kills a generic "rollback failed", which hides "redeploy from
// source instead" (handlers/projects.go:1603-1604), "no image tag" (:1575) and "not found" (:1570).
func TestRollbackRefusalsVerbatim(t *testing.T) {
	for _, c := range []struct {
		file   string
		status int
		kind   deploy.Kind
	}{
		{"rollback-409-pruned.json", 409, deploy.KindConflict},
		{"rollback-400-noimage.json", 400, deploy.KindRejected},
		{"rollback-404.json", 404, deploy.KindNotFound},
	} {
		t.Run(c.file, func(t *testing.T) {
			rs := serveRecorded(t, map[string]fixture{"POST " + actsP + "/deployments/d1/rollback": {file: c.file, status: c.status}})
			_, err := rs.c.Rollback(context.Background(), project, "d1")
			wantKind(t, err, c.kind, c.status, fixtureSentence(t, c.file))
			oneRequest(t, rs, "POST "+actsP+"/deployments/d1/rollback")
		})
	}
}

// TestCancelOutcomes kills an error toast for an undo that raced the build's end
// (already_completed, handlers/projects.go:3669-3674), and a word this code does not know read
// as either outcome. already_completed is confirmed by one re-read of the run (review t04 I2):
// here it has finished, so the answer is AlreadyFinished with Panacea's sentence. The unknown
// word is logged at warn, naming the run and the word (review t04 M4, named survivor RV1).
func TestCancelOutcomes(t *testing.T) {
	const run = "engagement-portal-deploy-9f3kq"
	rs := serveRecorded(t, map[string]fixture{
		"POST " + actsP + "/pipelines/" + run + "/cancel": {file: "cancel-cancelled.json"},
		"POST " + actsP + "/pipelines/r-done/cancel":      {file: "cancel-already.json"},
		"GET " + actsP + "/pipelines/r-done":              {file: "run-succeeded.json"},
		"POST " + actsP + "/pipelines/r-odd/cancel":       {body: `{"data":{"status":"paused","message":"?"}}`},
	})
	ctx := context.Background()
	got, err := rs.c.Cancel(ctx, project, run)
	if want := (deploy.CancelResult{Outcome: deploy.Cancelled, Message: "Deployment cancelled successfully"}); err != nil || got != want {
		t.Errorf("Cancel(running) = (%+v, %v), want %+v", got, err, want)
	}
	got, err = rs.c.Cancel(ctx, project, "r-done")
	if want := (deploy.CancelResult{Outcome: deploy.AlreadyFinished, Message: "Pipeline run is already Succeeded"}); err != nil || got != want {
		t.Errorf("Cancel(finished) = (%+v, %v), want %+v", got, err, want)
	}
	got, err = rs.c.Cancel(ctx, project, "r-odd")
	wantKind(t, err, deploy.KindUnavailable, 200, "")
	if got != (deploy.CancelResult{}) {
		t.Errorf("an unknown cancel word read as %+v", got)
	}
	want := []string{"POST " + actsP + "/pipelines/" + run + "/cancel", "POST " + actsP + "/pipelines/r-done/cancel",
		"GET " + actsP + "/pipelines/r-done", "POST " + actsP + "/pipelines/r-odd/cancel"}
	if got := rs.requests(); !slices.Equal(got, want) {
		t.Errorf("requests = %q\nwant       %q", got, want)
	}
	if !warnedWith(rs.log.String(), " run=r-odd ", " word=paused ") {
		t.Errorf("no warn line names the run and the unknown word:\n%s", rs.log.String())
	}
}

// warnedWith reports whether one level=WARN line of log holds every part.
func warnedWith(log string, parts ...string) bool {
	for _, line := range strings.Split(log, "\n") {
		if !strings.Contains(line, "level=WARN") {
			continue
		}
		all := true
		for _, p := range parts {
			all = all && strings.Contains(line, p)
		}
		if all {
			return true
		}
	}
	return false
}

// TestCancelUnstartedIsNotFinished kills an undo in a run's first seconds reported as the benign
// AlreadyFinished (review t04 I2). CancelDeploy cancels only Pending or Running
// (handlers/projects.go:3669), and a run Tekton has not yet given a condition reads "Unknown"
// (clients/pipeline.go:1097-1100), so Panacea answers already_completed and nothing is stopped.
// Cancel re-reads the run through Run, a structured read, not Panacea's sentence: still pending,
// or started since, is KindConflict with notStartedSentence; a word this code does not know, or a
// re-read that fails, is KindUnavailable. Each is Status 200, the cancel's answer, and is logged
// at warn naming the run. The cancel is sent once, never retried.
func TestCancelUnstartedIsNotFinished(t *testing.T) {
	const cancel = "POST " + actsP + "/pipelines/r-new/cancel"
	for _, c := range []struct {
		name    string
		run     fixture
		kind    deploy.Kind
		message string
	}{
		{"not started", fixture{file: "run-unstarted.json"}, deploy.KindConflict, notStartedSentence},
		{"started since", fixture{file: "run-running.json"}, deploy.KindConflict, notStartedSentence},
		{"unknown word", fixture{body: `{"data":{"name":"r-new","status":"Paused"}}`}, deploy.KindUnavailable, ""},
		{"re-read fails", fixture{body: `{"error":"upstream connect error"}`, status: 503}, deploy.KindUnavailable, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			rs := serveRecorded(t, map[string]fixture{
				cancel:                              {file: "cancel-unstarted.json"},
				"GET " + actsP + "/pipelines/r-new": c.run,
			})
			got, err := rs.c.Cancel(context.Background(), project, "r-new")
			wantKind(t, err, c.kind, 200, c.message)
			if got != (deploy.CancelResult{}) {
				t.Errorf("a cancel that stopped nothing read as %+v", got)
			}
			reqs := rs.requests()
			if len(reqs) < 2 || reqs[0] != cancel || slices.Index(reqs[1:], cancel) >= 0 || reqs[1] != "GET "+actsP+"/pipelines/r-new" {
				t.Errorf("requests = %q, want the cancel once, then the run's re-read", reqs)
			}
			if !warnedWith(rs.log.String(), " run=r-new ") {
				t.Errorf("no warn line names the run:\n%s", rs.log.String())
			}
		})
	}
}

// TestNotFoundIsTheConnectorsSentence kills showing Panacea's 404 sentence for Ship, Cancel and
// Run (review t04 M2). TriggerDeploy (handlers/projects.go:2953-2957), CancelDeploy (:3657-3661)
// and GetPipelineRunStatus (:3588-3592) answer 404 with err.Error() of a live Kubernetes Get for
// ANY error, so an API-server timeout reaches the user as "not found" holding an internal
// address. Each keeps KindNotFound and the 404, takes the connector's own sentence, and logs
// Panacea's at warn. The IDOR sentence (:3600, :3664) is replaced too: the connector does not
// parse which one came (plan 7P-12).
func TestNotFoundIsTheConnectorsSentence(t *testing.T) {
	rs := serveRecorded(t, map[string]fixture{
		"POST " + actsP + "/deploy":               {file: "ship-404-transient.json", status: 404},
		"POST " + actsP + "/pipelines/r-1/cancel": {file: "cancel-404-transient.json", status: 404},
		"GET " + actsP + "/pipelines/r-1":         {body: `{"error":"pipeline run not found for this project"}`, status: 404},
	})
	ctx := context.Background()
	_, err := rs.c.Ship(ctx, project, deploy.ShipRequest{Branch: "feat/x", Commit: commit40})
	wantKind(t, err, deploy.KindNotFound, 404, "Panacea did not find the project engagement-portal.")
	_, err = rs.c.Cancel(ctx, project, "r-1")
	wantKind(t, err, deploy.KindNotFound, 404, "Panacea did not find run r-1 in this project.")
	_, err = rs.c.Run(ctx, project, "r-1")
	wantKind(t, err, deploy.KindNotFound, 404, "Panacea did not find run r-1 in this project.")
	log := rs.log.String()
	for _, f := range []string{"ship-404-transient.json", "cancel-404-transient.json"} {
		if s := fixtureSentence(t, f); !warnedWith(log, strconv.Quote(s)) {
			t.Errorf("no warn line carries Panacea's sentence %q:\n%s", s, log)
		}
	}
	if !warnedWith(log, strconv.Quote("pipeline run not found for this project")) {
		t.Errorf("no warn line carries the run read's sentence:\n%s", log)
	}
}

// TestActsRefuseBadSegmentsBeforeRequest is TestEmptySegmentRefusedBeforeRequest for the writes:
// an empty, "." or ".." project, run, deployment, branch or group names a different route (a mux
// cleans a dot segment with a redirect, which do never follows), so each call is KindRejected
// with no request. Kills an act that skips segment for its own id (named survivor Z1: Cancel's
// run; Z1b: Rollback's deployment).
func TestActsRefuseBadSegmentsBeforeRequest(t *testing.T) {
	rs := serveRecorded(t, map[string]fixture{})
	ctx := context.Background()
	c := rs.c
	errs := map[string]error{}
	_, errs["Ship project"] = c.Ship(ctx, "", deploy.ShipRequest{Branch: "feat/x", Commit: commit40})
	_, errs["Cancel empty"] = c.Cancel(ctx, project, "")
	_, errs["Cancel dot"] = c.Cancel(ctx, project, ".")
	_, errs["Rollback dotdot"] = c.Rollback(ctx, project, "..")
	_, errs["Rollback empty"] = c.Rollback(ctx, project, "")
	_, errs["AutoDeploy"] = c.AutoDeploy(ctx, project, "")
	_, errs["SetAutoDeploy"] = c.SetAutoDeploy(ctx, project, ".", true)
	_, errs["Posture"] = c.Posture(ctx, project, "..")
	_, errs["SetPosture"] = c.SetPosture(ctx, project, "", deploy.PostureChange{Mode: deploy.PosturePublic})
	_, errs["Access"] = c.Access(ctx, "")
	errs["Grant"] = c.Grant(ctx, ".", deploy.GrantRequest{Grantee: "a.b@kpmg.com.au", Role: deploy.RoleViewer})
	errs["Revoke empty"] = c.Revoke(ctx, project, "")
	errs["Revoke dotdot"] = c.Revoke(ctx, project, "..")
	_, errs["SetURLPrefix"] = c.SetURLPrefix(ctx, project, "", "uat-test")
	_, errs["ClearURLPrefix"] = c.ClearURLPrefix(ctx, project, ".")
	for name, err := range errs {
		if k := deploy.KindOf(err); k != deploy.KindRejected {
			t.Errorf("%s: Kind %q (%v), want rejected", name, k, err)
		}
	}
	if got := rs.requests(); len(got) != 0 {
		t.Errorf("requests = %q, want none", got)
	}
}
