package platform

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func emailInput() DataEmailInputConfiguration {
	return DataEmailInputConfiguration{Host: "smtp.example.test", Port: 465, Username: "smtp-user", Password: "private-smtp-password", FromAddress: "app@example.test", SenderName: "Example App", ApplicationOrigin: "http://localhost:3000"}
}
func emailSuccess(context.Context, dataEmailPrivateConfiguration, string) (dataEmailSMTPResult, error) {
	return dataEmailSMTPResult{Accepted: true}, nil
}
func saveEmail(t *testing.T, s *Service, revision int, input DataEmailInputConfiguration) DataEmailStatus {
	t.Helper()
	out, e := s.dataEmail(context.Background(), "owned", DataEmailRequest{Action: "save", ExpectedRevision: revision, Configuration: &input}, emailSuccess)
	if e != nil {
		t.Fatal(e)
	}
	return out
}

type emailFake struct {
	mu          sync.Mutex
	env         map[string][]string
	applyCalls  int
	failApply   int
	badReadback bool
}

func newEmailFake(t *testing.T) (*Service, *dataFake, *emailFake) {
	t.Helper()
	s, f := newDataFake(t)
	m := &emailFake{env: map[string][]string{}}
	base := s.opts.Runner
	s.opts.Runner = func(ctx context.Context, exe string, args []string, dir string, input []byte) ([]byte, error) {
		if len(args) == 4 && args[0] == "inspect" && args[3] == "{{json .Config.Env}}" {
			m.mu.Lock()
			v := append([]string(nil), m.env[args[1]]...)
			bad := m.badReadback
			m.mu.Unlock()
			if bad {
				v = append(v, "GOTRUE_SMTP_PASS=unexpected-private-value")
			}
			b, _ := json.Marshal(v)
			return b, nil
		}
		isUp := false
		envPath := ""
		for n, arg := range args {
			if arg == "--env-file" && n+1 < len(args) {
				envPath = args[n+1]
			}
			isUp = isUp || arg == "up"
		}
		if isUp {
			m.mu.Lock()
			m.applyCalls++
			fail := m.failApply > 0
			if fail {
				m.failApply--
			}
			m.mu.Unlock()
			if fail {
				return nil, errors.New("fixture failure containing private-smtp-password must not escape")
			}
		}
		b, e := base(ctx, exe, args, dir, input)
		if e == nil && isUp && envPath != "" {
			data, e := os.ReadFile(envPath)
			if e != nil {
				return nil, e
			}
			values := map[string]string{}
			for _, line := range strings.Split(string(data), "\n") {
				k, v, ok := strings.Cut(line, "=")
				if ok {
					if strings.HasPrefix(v, "'") && strings.HasSuffix(v, "'") {
						v = strings.ReplaceAll(v[1:len(v)-1], "\\'", "'")
					}
					if strings.HasPrefix(v, "\"") && strings.HasSuffix(v, "\"") {
						v = strings.NewReplacer("\\\\", "\\", "\\\"", "\"", "\\$", "$").Replace(v[1 : len(v)-1])
					}
					values[k] = v
				}
			}
			var install dataInstallation
			_ = s.read("owned-data-installation", &install)
			actual := []string{"API_EXTERNAL_URL=" + values["API_EXTERNAL_URL"], "GOTRUE_SITE_URL=" + values["SITE_URL"], "GOTRUE_URI_ALLOW_LIST=" + values["ADDITIONAL_REDIRECT_URLS"], "GOTRUE_MAILER_AUTOCONFIRM=false"}
			for _, key := range []string{"HOST", "PORT", "USER", "PASS", "ADMIN_EMAIL", "SENDER_NAME"} {
				actual = append(actual, "GOTRUE_SMTP_"+key+"="+values["SMTP_"+key])
			}
			m.mu.Lock()
			m.env[s.dataName(install, "auth")] = actual
			m.mu.Unlock()
		}
		return b, e
	}
	return s, f, m
}
func TestDataEmailSaveIsPrivateReadOnlyAndProjectScoped(t *testing.T) {
	s, f, _ := newEmailFake(t)
	input := emailInput()
	before := len(f.calls)
	w := request(t, s, "POST", "/v1/platform/projects/owned/data/email", "admin", DataEmailRequest{Action: "save", Configuration: &input})
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	if len(f.calls) != before {
		t.Fatal("saving unexpectedly touched Docker")
	}
	for _, role := range []string{"admin", "member"} {
		w = request(t, s, "GET", "/v1/platform/projects/owned/data/email", role, nil)
		if w.Code != 200 || strings.Contains(w.Body.String(), input.Password) || strings.Contains(w.Body.String(), `"password"`) {
			t.Fatal("private email credential escaped GET")
		}
	}
	w = request(t, s, "POST", "/v1/platform/projects/owned/data/email", "member", DataEmailRequest{Action: "check", ExpectedRevision: 1})
	if w.Code != 403 {
		t.Fatal("member can use credentials", w.Code)
	}
	w = request(t, s, "GET", "/v1/platform/projects/foreign/data/email", "admin", nil)
	if w.Code != 404 {
		t.Fatal("foreign project visible")
	}
	path, _ := s.path("owned-data-email")
	info, e := os.Stat(path)
	if e != nil {
		t.Fatal(e)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatal("email state is not private")
	}
	input.Password = ""
	out := saveEmail(t, s, 1, input)
	if !out.Configuration.HasPassword || out.Revision != 2 {
		t.Fatal("same-destination credential was not retained")
	}
	input.Host = "smtp.other.test"
	if _, e = s.dataEmail(context.Background(), "owned", DataEmailRequest{Action: "save", ExpectedRevision: 2, Configuration: &input}, emailSuccess); e == nil {
		t.Fatal("retained credential could be forwarded to another host")
	}
	input = emailInput()
	if _, e = s.dataEmail(context.Background(), "owned", DataEmailRequest{Action: "save", ExpectedRevision: 0, Configuration: &input}, emailSuccess); e == nil {
		t.Fatal("stale save accepted")
	}
}
func TestDataEmailValidatesTLSHeadersAndRedirectOrigins(t *testing.T) {
	s, _, _ := newEmailFake(t)
	cases := []func(*DataEmailInputConfiguration){func(c *DataEmailInputConfiguration) { c.Port = 587 }, func(c *DataEmailInputConfiguration) { c.Host = "https://smtp.example.test" }, func(c *DataEmailInputConfiguration) { c.Host = "smtp..example.test" }, func(c *DataEmailInputConfiguration) { c.FromAddress = "Name <app@example.test>" }, func(c *DataEmailInputConfiguration) { c.SenderName = "Luci\r\nBcc: victim@example.test" }, func(c *DataEmailInputConfiguration) { c.Password = "one\ntwo" }, func(c *DataEmailInputConfiguration) { c.ApplicationOrigin = "http://attacker.localhost:3000" }, func(c *DataEmailInputConfiguration) { c.ApplicationOrigin = "http://localhost:8730" }, func(c *DataEmailInputConfiguration) { c.ApplicationOrigin = "https://user:password@app.example.test" }, func(c *DataEmailInputConfiguration) { c.ApplicationOrigin = "https://app.example.test/path" }}
	for n, alter := range cases {
		input := emailInput()
		alter(&input)
		if _, e := s.validateDataEmailConfiguration("owned", input, nil); e == nil {
			t.Fatalf("invalid setting accepted case%d", n)
		}
	}
	for _, origin := range []string{"http://127.0.0.1:4000", "http://" + strings.Repeat("a", 64) + ".localhost:8731", "https://app.example.test"} {
		input := emailInput()
		input.ApplicationOrigin = origin
		if _, e := s.validateDataEmailConfiguration("owned", input, nil); e != nil {
			t.Fatal(e)
		}
	}
}
func TestDataEmailApplyVerifiesActualOwnedAuthAndSendsNoEmail(t *testing.T) {
	s, f, m := newEmailFake(t)
	installData(t, s)
	saveEmail(t, s, 0, emailInput())
	calls := 0
	check := func(_ context.Context, c dataEmailPrivateConfiguration, to string) (dataEmailSMTPResult, error) {
		calls++
		if to != "" {
			t.Fatal("apply sent an email")
		}
		if c.Password != emailInput().Password {
			t.Fatal("credential changed")
		}
		return dataEmailSMTPResult{}, nil
	}
	before := len(f.calls)
	out, e := s.dataEmail(context.Background(), "owned", DataEmailRequest{Action: "apply", ExpectedRevision: 1}, check)
	if e != nil || out.AppliedRevision != 1 || out.Status != "configured" || out.Connection.Status != "passed" || calls != 1 {
		t.Fatal("apply was not verified", e, out)
	}
	for _, args := range f.calls[before:] {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, " up ") && (!strings.Contains(joined, " --no-deps ") || args[len(args)-1] != "auth") {
			t.Fatal("email apply recreated other services")
		}
	}
	var install dataInstallation
	_ = s.read("owned-data-installation", &install)
	m.mu.Lock()
	m.env[s.dataName(install, "auth")] = []string{"GOTRUE_SMTP_HOST=manually-changed.example.test"}
	m.mu.Unlock()
	if _, e = s.dataEmail(context.Background(), "owned", DataEmailRequest{Action: "test", ExpectedRevision: 1, Recipient: "test@example.test", AcknowledgeSend: true}, check); e == nil {
		t.Fatal("test ignored changed Auth settings")
	}
	if calls != 1 {
		t.Fatal("mismatched Auth sent a test")
	}
}
func TestDataEmailFailureRollsBackPriorAuthAndRetainsDraft(t *testing.T) {
	s, _, m := newEmailFake(t)
	installData(t, s)
	saveEmail(t, s, 0, emailInput())
	if _, e := s.dataEmail(context.Background(), "owned", DataEmailRequest{Action: "apply", ExpectedRevision: 1}, emailSuccess); e != nil {
		t.Fatal(e)
	}
	input := emailInput()
	input.SenderName = "New name"
	saveEmail(t, s, 1, input)
	m.mu.Lock()
	m.failApply = 1
	m.mu.Unlock()
	out, e := s.dataEmail(context.Background(), "owned", DataEmailRequest{Action: "apply", ExpectedRevision: 2}, emailSuccess)
	if e == nil || out.AppliedRevision != 1 || out.Revision != 2 || out.Status != "configured" || !strings.Contains(out.Message, "restored and verified") {
		t.Fatal("prior Auth was not recovered", e, out)
	}
	var private dataEmailState
	_ = s.read("owned-data-email", &private)
	if private.Applied.SenderName == private.Desired.SenderName || private.RuntimeUncertain {
		t.Fatal("draft mistaken for applied settings")
	}
	if strings.Contains(out.Message, emailInput().Password) {
		t.Fatal("SMTP secret in error")
	}
	m.mu.Lock()
	m.failApply = 2
	m.mu.Unlock()
	out, e = s.dataEmail(context.Background(), "owned", DataEmailRequest{Action: "apply", ExpectedRevision: 2}, emailSuccess)
	if e == nil || out.Status != "attention" || s.EmailDeliveryStatus("owned") != "attention" {
		t.Fatal("failed recovery claimed readiness")
	}
}

func TestDataEmailRejectedDraftDoesNotClearUnverifiedAuth(t *testing.T) {
	s, _, fixture := newEmailFake(t)
	installData(t, s)
	saveEmail(t, s, 0, emailInput())
	if _, e := s.dataEmail(context.Background(), "owned", DataEmailRequest{Action: "apply", ExpectedRevision: 1}, emailSuccess); e != nil {
		t.Fatal(e)
	}
	var installation dataInstallation
	_ = s.read("owned-data-installation", &installation)
	fixture.mu.Lock()
	fixture.env[s.dataName(installation, "auth")] = []string{"GOTRUE_SMTP_HOST=changed.example.test"}
	fixture.mu.Unlock()
	rejected := func(context.Context, dataEmailPrivateConfiguration, string) (dataEmailSMTPResult, error) {
		return dataEmailSMTPResult{}, &dataFailure{"SMTP authentication was rejected."}
	}
	out, e := s.dataEmail(context.Background(), "owned", DataEmailRequest{Action: "apply", ExpectedRevision: 1}, rejected)
	if e == nil || out.Status != "attention" || s.EmailDeliveryStatus("owned") != "attention" {
		t.Fatal("rejected draft incorrectly verified changed Auth", e, out)
	}
	if out.AppliedRevision != 1 || out.Connection.Status != "failed" {
		t.Fatal("previous receipt or failed connection was lost")
	}
}

func TestDataEmailTestsRequireExplicitRecipientAndHonestAcceptance(t *testing.T) {
	s, _, _ := newEmailFake(t)
	installData(t, s)
	saveEmail(t, s, 0, emailInput())
	_, _ = s.dataEmail(context.Background(), "owned", DataEmailRequest{Action: "apply", ExpectedRevision: 1}, emailSuccess)
	for _, in := range []DataEmailRequest{{Action: "test", ExpectedRevision: 1, Recipient: "test@example.test"}, {Action: "test", ExpectedRevision: 1, Recipient: "bad", AcknowledgeSend: true}} {
		if _, e := s.dataEmail(context.Background(), "owned", in, emailSuccess); e == nil {
			t.Fatal("unacknowledged or invalid email accepted")
		}
	}
	out, e := s.dataEmail(context.Background(), "owned", DataEmailRequest{Action: "test", ExpectedRevision: 1, Recipient: "test@example.test", AcknowledgeSend: true}, emailSuccess)
	if e != nil || out.Test.Status != "accepted" || out.Test.AcceptedAt == "" || s.EmailDeliveryStatus("owned") != "test-accepted" || !strings.Contains(out.Message, "does not prove delivery") {
		t.Fatal("dishonest acceptance receipt", e, out)
	}
	uncertain := func(context.Context, dataEmailPrivateConfiguration, string) (dataEmailSMTPResult, error) {
		return dataEmailSMTPResult{Uncertain: true}, &dataFailure{"Acceptance was uncertain."}
	}
	out, e = s.dataEmail(context.Background(), "owned", DataEmailRequest{Action: "test", ExpectedRevision: 1, Recipient: "test@example.test", AcknowledgeSend: true}, uncertain)
	if e == nil || out.Test.Status != "uncertain" || out.Test.AcceptedAt != "" {
		t.Fatal("lost SMTP receipt claimed acceptance")
	}
}
func TestDataEmailRestoreAndDomainConfigurationKeepSMTPPrivate(t *testing.T) {
	s, _, _ := newEmailFake(t)
	installData(t, s)
	saveEmail(t, s, 0, emailInput())
	_, e := s.dataEmail(context.Background(), "owned", DataEmailRequest{Action: "apply", ExpectedRevision: 1}, emailSuccess)
	if e != nil {
		t.Fatal(e)
	}
	var original dataInstallation
	_ = s.read("owned-data-installation", &original)
	next := dataInstallation{ProjectID: "owned", StackID: "restore-candidate", StorageTenantID: original.storageTenant(), APIPort: 45101}
	secret, _ := s.dataCredentials(original)
	if e = s.prepareDataStack(next, secret); e != nil {
		t.Fatal(e)
	}
	root, _ := s.dataRoot(next)
	b, _ := os.ReadFile(filepath.Join(root, "runtime.env"))
	if !strings.Contains(string(b), "SMTP_PASS="+dataEmailEnvironmentLiteral(emailInput().Password)) {
		t.Fatal("restore candidate lost email settings")
	}
	domain := dataDomainState{ProjectID: "owned", StableTenant: original.storageTenant(), Status: "pending", Request: DataDomainRequest{Domain: "https://data.example.test", AppOrigin: "https://app.example.test"}}
	_ = s.write("owned-data-domain", domain)
	if e = s.prepareDataStack(next, secret); e != nil {
		t.Fatal(e)
	}
	b, _ = os.ReadFile(filepath.Join(root, "runtime.env"))
	if !strings.Contains(string(b), "SITE_URL=https://app.example.test\n") || strings.Contains(string(b), "SITE_URL=http://localhost:3000") {
		t.Fatal("SMTP overwrote public Auth origin")
	}
	status, _ := s.DataEmailStatus("owned")
	if status.OriginSource != "public-domain" || status.ApplicationOrigin != "https://app.example.test" {
		t.Fatal("UI lost authoritative domain origin")
	}
	input := emailInput()
	if _, e = s.validateDataEmailConfiguration("owned", input, nil); e == nil {
		t.Fatal("client overrode authoritative domain origin")
	}
	// Candidate SMTP credentials are runtime state, never part of a portable
	// database/storage backup or agent/application environment bindings.
	w := request(t, s, "POST", "/v1/platform/projects/owned/data", "member", dataActionRequest{Action: "backup"})
	if w.Code != 202 {
		t.Fatal(w.Body)
	}
	data := waitData(t, s)
	_, archive, e := s.readDataBackup("owned", data.Backups[0].ID)
	if e != nil || strings.Contains(string(archive), emailInput().Password) {
		t.Fatal("SMTP credential escaped data backup")
	}
}
func TestDataEmailDisableRemovesCredentialsOnlyAfterVerifiedAuth(t *testing.T) {
	s, _, _ := newEmailFake(t)
	installData(t, s)
	saveEmail(t, s, 0, emailInput())
	_, _ = s.dataEmail(context.Background(), "owned", DataEmailRequest{Action: "apply", ExpectedRevision: 1}, emailSuccess)
	out, e := s.dataEmail(context.Background(), "owned", DataEmailRequest{Action: "disable", ExpectedRevision: 1}, emailSuccess)
	if e != nil || out.Status != "unconfigured" || out.AppliedRevision != 0 || out.Configuration.HasPassword || out.Revision != 2 {
		t.Fatal("SMTP removal was not verified", e, out)
	}
	path, _ := s.path("owned-data-email")
	b, _ := os.ReadFile(path)
	if strings.Contains(string(b), emailInput().Password) {
		t.Fatal("removed SMTP credential retained in state")
	}
}
func TestDataEmailInterruptedOperationsNeverAutomaticallySendOrApply(t *testing.T) {
	s, f, _ := newEmailFake(t)
	saveEmail(t, s, 0, emailInput())
	v, _ := s.loadDataEmail("owned")
	v.Operation = &DataOperation{Action: "test", Status: "running"}
	v.Test = DataEmailReceipt{Status: "pending", Revision: 1}
	_ = s.write("owned-data-email", v)
	before := len(f.calls)
	out, e := s.DataEmailStatus("owned")
	if e != nil || out.Operation.Status != "interrupted" || out.Test.Status != "uncertain" || len(f.calls) != before {
		t.Fatal("interrupted email replayed or claimed success")
	}
	if !s.claim("owned", 1) {
		t.Fatal("claim")
	}
	defer s.release("owned")
	if _, e = s.beginDataEmail("owned", DataEmailRequest{Action: "check", ExpectedRevision: 1}, 1); e == nil {
		t.Fatal("parallel platform operation admitted")
	}
}
func TestDataEmailActualComposePreservesCredentialBytes(t *testing.T) {
	if os.Getenv("LUCI_SMTP_COMPOSE_EVAL") != "1" {
		t.Skip("explicit read-only Compose evaluation")
	}
	if _, e := exec.LookPath("docker"); e != nil {
		t.Fatal(e)
	}
	s, _, _ := newEmailFake(t)
	installData(t, s)
	var install dataInstallation
	_ = s.read("owned-data-installation", &install)
	secret, _ := s.dataCredentials(install)
	root, _ := s.dataRoot(install)
	for revision, password := range []string{" leading $NOT_AN_ENV_VALUE # comment ' quote \\ trailing ", "final-backslash\\", "literal \\\" quote ' $dollar é"} {
		input := emailInput()
		input.Password = password
		saveEmail(t, s, revision, input)
		v, _ := s.loadDataEmail("owned")
		if e := s.prepareDataStackEmail(install, secret, &dataEmailRuntimeSelection{Configuration: v.Desired}); e != nil {
			t.Fatal(e)
		}
		cmd := exec.Command("docker", "compose", "--project-directory", root, "--env-file", filepath.Join(root, "runtime.env"), "-f", filepath.Join(root, "compose.yaml"), "config", "--format", "json")
		b, e := cmd.Output()
		if e != nil {
			t.Fatal("real Compose could not decode the owned SMTP configuration")
		}
		var config struct {
			Services map[string]struct{ Environment map[string]string }
		}
		if e = json.Unmarshal(b, &config); e != nil {
			t.Fatal("real Compose returned an unexpected environment shape", e)
		}
		// Docker Compose config deliberately doubles every dollar in its rendered
		// YAML/JSON for round-trip safety (cmd/compose/config.go escapeDollarSign).
		// Decode that rendering escape; it is not part of the container environment.
		actual := strings.ReplaceAll(config.Services["auth"].Environment["GOTRUE_SMTP_PASS"], "$$", "$")
		if actual != input.Password {
			first := 0
			for first < len(actual) && first < len(input.Password) && actual[first] == input.Password[first] {
				first++
			}
			t.Fatalf("real Compose changed fixture SMTP credential bytes: expectedLength=%d actualLength=%d firstDifference=%d", len(input.Password), len(actual), first)
		}
	}
}
