package platform

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Configuration and receipts are client-safe. Passwords exist only in the
// separate private state type and the owned Auth container's environment.
type DataEmailConfiguration struct {
	Host              string `json:"host"`
	Port              int    `json:"port"`
	Username          string `json:"username"`
	FromAddress       string `json:"fromAddress"`
	SenderName        string `json:"senderName"`
	ApplicationOrigin string `json:"applicationOrigin"`
	HasPassword       bool   `json:"hasPassword"`
}
type DataEmailReceipt struct {
	Status     string `json:"status"`
	Revision   int    `json:"revision"`
	CheckedAt  string `json:"checkedAt,omitempty"`
	AcceptedAt string `json:"acceptedAt,omitempty"`
}
type DataEmailStatus struct {
	ProjectID         string                 `json:"projectId"`
	Status            string                 `json:"status"`
	Message           string                 `json:"message"`
	Revision          int                    `json:"revision"`
	AppliedRevision   int                    `json:"appliedRevision"`
	Configuration     DataEmailConfiguration `json:"configuration"`
	ApplicationOrigin string                 `json:"applicationOrigin"`
	OriginSource      string                 `json:"originSource"`
	Connection        DataEmailReceipt       `json:"connection"`
	Test              DataEmailReceipt       `json:"test"`
	Operation         *DataOperation         `json:"operation,omitempty"`
}
type DataEmailInputConfiguration struct {
	Host              string `json:"host"`
	Port              int    `json:"port"`
	Username          string `json:"username"`
	Password          string `json:"password,omitempty"`
	FromAddress       string `json:"fromAddress"`
	SenderName        string `json:"senderName"`
	ApplicationOrigin string `json:"applicationOrigin"`
}
type DataEmailRequest struct {
	Action           string                       `json:"action"`
	ExpectedRevision int                          `json:"expectedRevision"`
	Configuration    *DataEmailInputConfiguration `json:"configuration,omitempty"`
	Recipient        string                       `json:"recipient,omitempty"`
	AcknowledgeSend  bool                         `json:"acknowledgeSend,omitempty"`
}
type dataEmailPrivateConfiguration struct {
	Host, Username, Password, FromAddress, SenderName, ApplicationOrigin string
	Port                                                                 int
}
type dataEmailState struct {
	ProjectID, StableTenant, Status, Message string
	Revision, AppliedRevision                int
	Desired, Applied                         *dataEmailPrivateConfiguration
	RuntimeUncertain                         bool
	Connection, Test                         DataEmailReceipt
	Operation                                *DataOperation
}

// A nonnil selection overrides the applied configuration only for one explicit
// owned Auth reconfiguration. It is never persisted as an applied configuration
// before healthy readback, nor silently replayed after a controller restart.
type dataEmailRuntimeSelection struct {
	Configuration *dataEmailPrivateConfiguration
}

// Compose dotenv double quotes preserve comment/space bytes. Escape backslash,
// quotation marks and dollar expansion, including a final literal backslash.
// Quote only these SMTP values, leaving upstream asset interpolation unchanged.
func dataEmailEnvironmentLiteral(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\"", "\\\"")
	value = strings.ReplaceAll(value, "$", "\\$")
	return "\"" + value + "\""
}

func (s *Service) loadDataEmail(id string) (dataEmailState, error) {
	v := dataEmailState{ProjectID: id, Status: "unconfigured", Message: "Connect your SMTP provider to enable application signup confirmation and password-recovery email.", Connection: DataEmailReceipt{Status: "not-checked"}, Test: DataEmailReceipt{Status: "not-sent"}}
	e := s.read(id+"-data-email", &v)
	if errors.Is(e, os.ErrNotExist) {
		e = nil
	}
	if v.ProjectID != id {
		return v, errors.New("email project binding mismatch")
	}
	return v, e
}
func safeDataEmailConfiguration(c *dataEmailPrivateConfiguration) DataEmailConfiguration {
	if c == nil {
		return DataEmailConfiguration{Port: 465, SenderName: "Luci"}
	}
	return DataEmailConfiguration{Host: c.Host, Port: c.Port, Username: c.Username, FromAddress: c.FromAddress, SenderName: c.SenderName, ApplicationOrigin: c.ApplicationOrigin, HasPassword: c.Password != ""}
}
func (s *Service) DataEmailStatus(id string) (DataEmailStatus, error) {
	s.mu.Lock()
	v, e := s.loadDataEmail(id)
	busy := s.busy[id]
	s.mu.Unlock()
	if e != nil {
		return DataEmailStatus{}, e
	}
	out := DataEmailStatus{ProjectID: id, Status: v.Status, Message: v.Message, Revision: v.Revision, AppliedRevision: v.AppliedRevision, Configuration: safeDataEmailConfiguration(v.Desired), ApplicationOrigin: safeDataEmailConfiguration(v.Desired).ApplicationOrigin, OriginSource: "local", Connection: v.Connection, Test: v.Test, Operation: v.Operation}
	if domain, e := s.loadDataDomain(id); e != nil {
		return out, e
	} else if domain.Request.AppOrigin != "" && domain.Status != "stopped" && domain.Status != "not-configured" {
		out.ApplicationOrigin, out.OriginSource = domain.Request.AppOrigin, "public-domain"
	}
	if v.Applied != nil {
		var i dataInstallation
		if s.read(id+"-data-installation", &i) != nil || i.ProjectID != id || i.storageTenant() != v.StableTenant {
			out.Status, out.Message = "attention", "The applied email settings belong to a different data installation. Apply them explicitly to this project's current owned stack."
		}
	}
	if v.RuntimeUncertain {
		out.Status = "attention"
	}
	if v.Operation != nil && v.Operation.Status == "running" && !busy {
		copy := *v.Operation
		copy.Status, copy.Stage = "interrupted", "interrupted"
		copy.Message = "Luci stopped during email setup. Reapply to verify Auth settings; no email is automatically retried."
		out.Operation, out.Status, out.Message = &copy, "attention", copy.Message
		if copy.Action == "test" {
			out.Test.Status = "uncertain"
		}
	}
	return out, nil
}
func (s *Service) EmailDeliveryStatus(id string) string {
	v, e := s.DataEmailStatus(id)
	if e != nil || v.Status == "attention" {
		return "attention"
	}
	if v.AppliedRevision == 0 {
		return "unconfigured"
	}
	if v.Test.Status == "accepted" && v.Test.Revision == v.AppliedRevision {
		return "test-accepted"
	}
	return "configured"
}
func (s *Service) registerDataEmail(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/platform/projects/{id}/data/email", func(w http.ResponseWriter, r *http.Request) {
		p, ok := s.project(w, r)
		if !ok {
			return
		}
		v, e := s.DataEmailStatus(p.ID)
		if e != nil {
			problem(w, 500, "data_email_state_unavailable", "Private email settings could not be read.")
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		respond(w, 200, v)
	})
	mux.HandleFunc("POST /v1/platform/projects/{id}/data/email", s.dataEmailAction)
}
func validDataEmailAddress(v string) bool {
	a, e := mail.ParseAddress(v)
	return e == nil && a.Address == v && len(v) <= 254 && !strings.ContainsAny(v, "\x00\r\n")
}
func canonicalDataEmailApplicationOrigin(v string) (string, error) {
	if strings.HasPrefix(v, "https://") {
		return canonicalDataOrigin(v)
	}
	u, e := url.Parse(v)
	if e != nil || u.Scheme != "http" || u.User != nil || u.RawPath != "" || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Opaque != "" {
		return "", errors.New("Use an HTTPS application origin or a local HTTP application address without a path.")
	}
	host := u.Hostname()
	capability := strings.HasSuffix(host, ".localhost") && dataDigestPattern.MatchString(strings.TrimSuffix(host, ".localhost"))
	if host != "127.0.0.1" && host != "localhost" && !capability {
		return "", errors.New("Use localhost or the app's isolated .localhost address for local email redirects.")
	}
	port := u.Port()
	if port != "" {
		n, e := strconv.Atoi(port)
		if e != nil || n < 1 || n > 65535 || n == 8730 {
			return "", errors.New("Use the application's port, separate from the Luci control port.")
		}
	}
	return "http://" + u.Host, nil
}
func (s *Service) validateDataEmailConfiguration(id string, in DataEmailInputConfiguration, prior *dataEmailPrivateConfiguration) (dataEmailPrivateConfiguration, error) {
	c := dataEmailPrivateConfiguration{Host: strings.ToLower(strings.TrimSpace(in.Host)), Port: in.Port, Username: in.Username, Password: in.Password, FromAddress: in.FromAddress, SenderName: in.SenderName, ApplicationOrigin: in.ApplicationOrigin}
	if c.Port != 465 {
		return c, errors.New("Use secure SMTP on port 465. This native Auth version does not enforce STARTTLS on other ports.")
	}
	if !dataDomainName.MatchString(c.Host) || strings.Contains(c.Host, "..") || net.ParseIP(c.Host) != nil || !strings.Contains(c.Host, ".") {
		return c, errors.New("Use your SMTP provider's DNS server name without a URL or port.")
	}
	for _, label := range strings.Split(c.Host, ".") {
		if len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return c, errors.New("The SMTP server name is invalid.")
		}
	}
	if c.Password == "" && prior != nil && prior.Host == c.Host && prior.Username == c.Username {
		c.Password = prior.Password
	}
	if len(c.Username) < 1 || len(c.Username) > 320 || len(c.Password) < 1 || len(c.Password) > 2048 || strings.ContainsAny(c.Username+c.Password, "\x00\r\n") {
		return c, errors.New("Supply an SMTP username and password. A changed server or username requires its own password.")
	}
	if !validDataEmailAddress(c.FromAddress) || len(c.SenderName) < 1 || len(c.SenderName) > 128 || strings.ContainsAny(c.SenderName, "\x00\r\n") {
		return c, errors.New("Use a valid sender email address and a sender name of up to 128 characters.")
	}
	domain, e := s.loadDataDomain(id)
	if e != nil {
		return c, e
	}
	if domain.Request.AppOrigin != "" && domain.Status != "stopped" && domain.Status != "not-configured" {
		if c.ApplicationOrigin != "" && c.ApplicationOrigin != domain.Request.AppOrigin {
			return c, errors.New("The published data connection owns the application redirect origin. Change it in the public connection settings.")
		}
		c.ApplicationOrigin = domain.Request.AppOrigin
	}
	c.ApplicationOrigin, e = canonicalDataEmailApplicationOrigin(c.ApplicationOrigin)
	return c, e
}
func (s *Service) dataEmailConfiguration(i dataInstallation, values map[string]string, override *dataEmailRuntimeSelection) error {
	v, e := s.loadDataEmail(i.ProjectID)
	if e != nil {
		return e
	}
	c := v.Applied
	if c != nil && v.StableTenant != i.storageTenant() {
		return errors.New("email settings belong to a different data tenant")
	}
	if override != nil {
		c = override.Configuration
	}
	if c == nil {
		return nil
	}
	values["SMTP_HOST"], values["SMTP_PORT"], values["SMTP_USER"], values["SMTP_PASS"] = c.Host, "465", c.Username, c.Password
	values["SMTP_ADMIN_EMAIL"], values["SMTP_SENDER_NAME"] = c.FromAddress, c.SenderName
	domain, _, e := s.dataDomainConfiguration(i)
	if e != nil {
		return e
	}
	if domain == "" {
		values["SITE_URL"], values["ADDITIONAL_REDIRECT_URLS"] = c.ApplicationOrigin, c.ApplicationOrigin+"/**"
	}
	return nil
}
func (s *Service) beginDataEmail(id string, in DataEmailRequest, limit int) (dataEmailState, error) {
	if in.ExpectedRevision < 0 || in.ExpectedRevision >= 1_000_000_000 {
		return dataEmailState{}, &dataFailure{"The saved email revision is invalid."}
	}
	switch in.Action {
	case "save", "check", "apply", "test", "disable":
	default:
		return dataEmailState{}, &dataFailure{"Choose save, check, apply, test or disable."}
	}
	if in.Action == "test" && (!in.AcknowledgeSend || !validDataEmailAddress(in.Recipient)) {
		return dataEmailState{}, &dataFailure{"Enter a valid recipient and explicitly acknowledge sending one test email."}
	}
	if !s.claim(id, limit) {
		return dataEmailState{}, &dataFailure{"A platform operation is already running for this project."}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.loadDataEmail(id)
	if e != nil || v.Revision != in.ExpectedRevision {
		delete(s.busy, id)
		return v, &dataFailure{"The saved email settings changed. Refresh before retrying."}
	}
	if in.Action == "save" {
		if in.Configuration == nil {
			delete(s.busy, id)
			return v, &dataFailure{"Supply the email settings to save."}
		}
		c, e := s.validateDataEmailConfiguration(id, *in.Configuration, v.Desired)
		if e != nil {
			delete(s.busy, id)
			return v, &dataFailure{e.Error()}
		}
		v.Desired = &c
		v.Revision++
		v.Status = "saved"
		v.Message = "Email settings saved privately. Check the connection and apply them to Auth; saving does not send email."
		v.Connection = DataEmailReceipt{Status: "not-checked"}
		if e = s.write(id+"-data-email", v); e != nil {
			delete(s.busy, id)
			return v, e
		}
		return v, nil
	}
	if (in.Action == "check" || in.Action == "apply") && v.Desired == nil {
		delete(s.busy, id)
		return v, &dataFailure{"Save your SMTP settings first."}
	}
	if in.Action == "test" && (v.Applied == nil || v.AppliedRevision != v.Revision || v.RuntimeUncertain) {
		delete(s.busy, id)
		return v, &dataFailure{"Apply and verify the current saved settings before sending a test email."}
	}
	if in.Action == "disable" && v.Desired == nil && v.Applied == nil {
		delete(s.busy, id)
		return v, &dataFailure{"Email is already unconfigured."}
	}
	v.Operation = &DataOperation{ID: randomID("data-email-op-"), Action: in.Action, Status: "running", Stage: "queued", Message: "Email operation accepted. No test message is sent unless explicitly requested.", CreatedAt: now(), UpdatedAt: now()}
	if in.Action == "apply" || in.Action == "disable" {
		v.RuntimeUncertain = true
	}
	if in.Action == "test" {
		v.Test = DataEmailReceipt{Status: "pending", Revision: v.AppliedRevision}
	}
	if e = s.write(id+"-data-email", v); e != nil {
		delete(s.busy, id)
		return v, e
	}
	return v, nil
}
func (s *Service) dataEmailAction(w http.ResponseWriter, r *http.Request) {
	p, ok := s.project(w, r)
	if !ok {
		return
	}
	if !admin(r) {
		problem(w, 403, "admin_required", "An Admin manages application email credentials and sending.")
		return
	}
	var in DataEmailRequest
	if !body(w, r, &in) {
		return
	}
	v, e := s.beginDataEmail(p.ID, in, operationLimit(r))
	if e != nil {
		problem(w, 409, "data_email_not_admitted", dataError(e))
		return
	}
	if in.Action == "save" {
		s.release(p.ID)
		status, _ := s.DataEmailStatus(p.ID)
		respond(w, 200, status)
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer s.release(p.ID)
		ctx, cancel := context.WithTimeout(s.ctx, 6*time.Minute)
		defer cancel()
		e := s.runDataEmail(ctx, p.ID, in, v, nativeDataEmailSMTP)
		s.finishDataEmail(p.ID, e)
	}()
	status, _ := s.DataEmailStatus(p.ID)
	respond(w, 202, status)
}

// Internal synchronous entry point shares admission and ownership checks with
// HTTP. The transport seam permits actual disposable TLS SMTP fixtures in tests.
func (s *Service) dataEmail(ctx context.Context, id string, in DataEmailRequest, transport dataEmailTransport) (DataEmailStatus, error) {
	if _, e := s.opts.ResolveProject(id); e != nil {
		return DataEmailStatus{}, e
	}
	v, e := s.beginDataEmail(id, in, 1)
	if e != nil {
		return DataEmailStatus{}, e
	}
	defer s.release(id)
	if in.Action != "save" {
		e = s.runDataEmail(ctx, id, in, v, transport)
		s.finishDataEmail(id, e)
	}
	out, readErr := s.DataEmailStatus(id)
	if e != nil {
		return out, e
	}
	return out, readErr
}
func (s *Service) changeDataEmail(id string, update func(*dataEmailState)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.loadDataEmail(id)
	if e != nil {
		return e
	}
	update(&v)
	return s.write(id+"-data-email", v)
}
func (s *Service) finishDataEmail(id string, e error) {
	_ = s.changeDataEmail(id, func(v *dataEmailState) {
		if e != nil {
			v.Message = dataError(e)
		}
		if v.Operation != nil {
			v.Operation.Status = "succeeded"
			v.Operation.Stage = "complete"
			if e != nil {
				v.Operation.Status = "failed"
			}
			v.Operation.Message = v.Message
			v.Operation.UpdatedAt = now()
		}
	})
}
func (s *Service) emailInstallation(ctx context.Context, id string) (dataInstallation, dataSecret, error) {
	var i dataInstallation
	if s.read(id+"-data-installation", &i) != nil || i.ProjectID != id {
		return i, dataSecret{}, &dataFailure{"Install and start this project's data stack before applying or testing email."}
	}
	if e := s.verifyDataOwnership(ctx, i, false); e != nil {
		return i, dataSecret{}, &dataFailure{"The owned data resources could not be verified. No email configuration was changed."}
	}
	secret, e := s.dataCredentials(i)
	if e != nil {
		return i, secret, e
	}
	info, e := s.inspect(ctx, id, "data-"+i.StackID+"-auth", s.dataName(i, "auth"))
	if e != nil || !info.State.Running || info.State.Health.Status == "starting" || info.State.Health.Status == "unhealthy" {
		return i, secret, &dataFailure{"Start or refresh the owned Auth service before applying or testing email."}
	}
	return i, secret, nil
}
func (s *Service) expectedDataEmailEnvironment(i dataInstallation, secret dataSecret, c *dataEmailPrivateConfiguration) (map[string]string, error) {
	api, e := s.browserDataConfigurationURL(i, secret)
	if e != nil {
		return nil, e
	}
	values := map[string]string{"SMTP_HOST": "", "SMTP_PORT": "587", "SMTP_USER": "", "SMTP_PASS": "", "SMTP_ADMIN_EMAIL": "", "SMTP_SENDER_NAME": "Luci", "SITE_URL": "http://localhost:3000", "ADDITIONAL_REDIRECT_URLS": "", "API_EXTERNAL_URL": api + "/auth/v1", "ENABLE_EMAIL_AUTOCONFIRM": "false"}
	if domain, origin, e := s.dataDomainConfiguration(i); e != nil {
		return nil, e
	} else if domain != "" {
		values["API_EXTERNAL_URL"] = domain + "/auth/v1"
		values["SITE_URL"] = origin
		values["ADDITIONAL_REDIRECT_URLS"] = origin + "/**"
	}
	if e = s.dataEmailConfiguration(i, values, &dataEmailRuntimeSelection{Configuration: c}); e != nil {
		return nil, e
	}
	result := map[string]string{"API_EXTERNAL_URL": values["API_EXTERNAL_URL"], "GOTRUE_SITE_URL": values["SITE_URL"], "GOTRUE_URI_ALLOW_LIST": values["ADDITIONAL_REDIRECT_URLS"], "GOTRUE_MAILER_AUTOCONFIRM": "false"}
	for _, name := range []string{"HOST", "PORT", "USER", "PASS", "ADMIN_EMAIL", "SENDER_NAME"} {
		result["GOTRUE_SMTP_"+name] = values["SMTP_"+name]
	}
	return result, nil
}
func (s *Service) verifyDataEmailRuntime(ctx context.Context, i dataInstallation, secret dataSecret, c *dataEmailPrivateConfiguration) error {
	info, e := s.inspect(ctx, i.ProjectID, "data-"+i.StackID+"-auth", s.dataName(i, "auth"))
	if e != nil || !info.State.Running || info.State.Health.Status == "starting" || info.State.Health.Status == "unhealthy" {
		return errors.New("owned Auth is not healthy")
	}
	manifest, e := reviewedDataAssets()
	if e != nil || info.Config.Image != manifest.Images["supabase/gotrue:v2.196.0"] {
		return errors.New("owned Auth image changed")
	}
	// The environment is examined only in process memory; neither it nor a
	// credential-derived fingerprint is exposed in DTOs, labels or diagnostics.
	b, e := s.docker(ctx, "inspect", s.dataName(i, "auth"), "--format", "{{json .Config.Env}}")
	if e != nil {
		return e
	}
	var raw []string
	if json.Unmarshal(b, &raw) != nil {
		return errors.New("Auth configuration readback failed")
	}
	actual := map[string]string{}
	for _, entry := range raw {
		k, v, ok := strings.Cut(entry, "=")
		if ok {
			if _, duplicate := actual[k]; duplicate {
				return errors.New("duplicate Auth environment setting")
			}
			actual[k] = v
		}
	}
	expected, e := s.expectedDataEmailEnvironment(i, secret, c)
	if e != nil {
		return e
	}
	for k, v := range expected {
		if actual[k] != v {
			return errors.New("Auth configuration was not applied")
		}
	}
	_, e = s.dataHTTP(ctx, i, secret, "GET", "/auth/v1/health", nil, "")
	return e
}
func (s *Service) runDataEmail(ctx context.Context, id string, in DataEmailRequest, v dataEmailState, transport dataEmailTransport) error {
	if in.Action == "check" {
		_, e := transport(ctx, *v.Desired, "")
		status := "passed"
		message := "Verified TLS and SMTP authentication. No email was sent; apply these settings to Auth next."
		if e != nil {
			status = "failed"
			message = dataError(e)
		}
		writeErr := s.changeDataEmail(id, func(current *dataEmailState) {
			current.Connection = DataEmailReceipt{Status: status, Revision: v.Revision, CheckedAt: now()}
			current.Message = message
		})
		if e != nil {
			return e
		}
		return writeErr
	}
	i, secret, e := s.emailInstallation(ctx, id)
	if e != nil {
		return e
	}
	if v.Applied != nil && v.StableTenant != i.storageTenant() {
		return &dataFailure{"Email belongs to a different data tenant. The retained settings were not sent or applied."}
	}
	if in.Action == "test" {
		if e = s.verifyDataEmailRuntime(ctx, i, secret, v.Applied); e != nil {
			_ = s.changeDataEmail(id, func(current *dataEmailState) { current.RuntimeUncertain = true; current.Test.Status = "failed" })
			return &dataFailure{"Auth no longer matches the applied email settings. Apply and verify them before sending a test."}
		}
		result, e := transport(ctx, *v.Applied, in.Recipient)
		status := "failed"
		message := "The test message was not accepted."
		if result.Uncertain {
			status = "uncertain"
			message = "The test's acceptance is uncertain. Check the recipient's inbox before explicitly retrying."
		}
		if result.Accepted {
			status = "accepted"
			message = "The SMTP server accepted the test email. Check the recipient's inbox; acceptance does not prove delivery."
		}
		writeErr := s.changeDataEmail(id, func(current *dataEmailState) {
			current.Test = DataEmailReceipt{Status: status, Revision: v.AppliedRevision, CheckedAt: now()}
			if result.Accepted {
				current.Test.AcceptedAt = now()
			}
			current.Message = message
		})
		if e != nil {
			return e
		}
		return writeErr
	}
	candidate := v.Desired
	if in.Action == "disable" {
		candidate = nil
	} else {
		_, e = transport(ctx, *candidate, "")
		if e != nil {
			// A rejected draft connection does not prove the previous Auth
			// settings are still applied. Only owned runtime readback may clear
			// uncertainty left by an interrupted or manually changed service.
			priorVerified := s.verifyDataEmailRuntime(ctx, i, secret, v.Applied) == nil
			_ = s.changeDataEmail(id, func(current *dataEmailState) {
				current.RuntimeUncertain = !priorVerified
				current.Connection = DataEmailReceipt{Status: "failed", Revision: v.Revision, CheckedAt: now()}
			})
			return e
		}
		_ = s.changeDataEmail(id, func(current *dataEmailState) {
			current.Connection = DataEmailReceipt{Status: "passed", Revision: v.Revision, CheckedAt: now()}
		})
	}
	apply := func(applyCtx context.Context, c *dataEmailPrivateConfiguration) error {
		if e := s.verifyDataOwnership(applyCtx, i, false); e != nil {
			return e
		}
		if e := s.prepareDataStackEmail(i, secret, &dataEmailRuntimeSelection{Configuration: c}); e != nil {
			return e
		}
		if _, e := s.compose(applyCtx, i, "up", "-d", "--no-deps", "--wait", "--wait-timeout", "240", "auth"); e != nil {
			return e
		}
		return s.verifyDataEmailRuntime(applyCtx, i, secret, c)
	}
	if e = apply(ctx, candidate); e != nil {
		// Recovery keeps the original private settings and volumes, and gets its
		// own bounded context after the failed request's deadline or cancellation.
		recovery, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		rollbackErr := apply(recovery, v.Applied)
		_ = s.changeDataEmail(id, func(current *dataEmailState) {
			current.RuntimeUncertain = rollbackErr != nil
			if rollbackErr != nil {
				current.Status = "attention"
			} else if current.Applied != nil {
				current.Status = "configured"
			} else {
				current.Status = "saved"
			}
		})
		if rollbackErr != nil {
			return &dataFailure{"Email application and recovery could not be verified. Owned data volumes are retained. Reapply after restoring Auth health; no test email was sent."}
		}
		return &dataFailure{"Email application failed. The previous Auth settings were restored and verified; the saved draft is retained. No test email was sent."}
	}
	return s.changeDataEmail(id, func(current *dataEmailState) {
		current.Applied = candidate
		current.StableTenant = i.storageTenant()
		current.RuntimeUncertain = false
		current.Test = DataEmailReceipt{Status: "not-sent"}
		if candidate == nil {
			current.Desired = nil
			current.Revision++
			current.AppliedRevision = 0
			current.Status = "unconfigured"
			current.Message = "SMTP credentials were removed and the owned Auth service verified. Signup confirmation and recovery email are unavailable until configured."
			current.Connection = DataEmailReceipt{Status: "not-checked"}
		} else {
			current.AppliedRevision = current.Revision
			current.Status = "configured"
			current.Message = "Secure SMTP settings are applied to the healthy owned Auth service for signup confirmation and password recovery. Send an explicit test to check SMTP acceptance."
		}
	})
}
