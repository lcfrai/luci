package panacea

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
)

type answer struct {
	status int
	ctype  string
	body   string
	header map[string]string
}

func serve(a answer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		for k, v := range a.header {
			w.Header().Set(k, v)
		}
		if a.ctype != "" {
			w.Header().Set("Content-Type", a.ctype)
		}
		w.WriteHeader(a.status)
		_, _ = w.Write([]byte(a.body))
	}
}

func TestStatusTable(t *testing.T) {
	const jsonCT, textCT, htmlCT = "application/json", "text/plain; charset=utf-8", "text/html"
	ra := map[string]string{"Retry-After": "7"}
	rows := []struct {
		name    string
		a       answer
		kind    deploy.Kind
		message string
		retry   time.Duration
	}{
		{"3xx", answer{302, htmlCT, "<a>moved</a>", map[string]string{"Location": "/oauth2/sign_in"}}, deploy.KindUnavailable, "", 0},
		{"401 writeAPIKeyError", answer{401, jsonCT, `{"error":"api key revoked or expired"}` + "\n", nil}, deploy.KindUnauthorized, "api key revoked or expired", 0},
		{"401 http.Error text/plain", answer{401, textCT, `{"error":"unauthorized: missing or invalid authentication"}` + "\n", nil}, deploy.KindUnauthorized, "unauthorized: missing or invalid authentication", 0},
		{"403 rbac text/plain", answer{403, textCT, `{"error":"no access yet: your account is waiting for team approval — finish onboarding first"}`, nil}, deploy.KindForbidden, "no access yet: your account is waiting for team approval — finish onboarding first", 0},
		{"404", answer{404, jsonCT, `{"error":"project not found"}`, nil}, deploy.KindNotFound, "project not found", 0},
		{"404 router text", answer{404, textCT, "404 page not found\n", nil}, deploy.KindNotFound, "", 0},
		{"409", answer{409, jsonCT, `{"error":"a deploy is already running"}`, ra}, deploy.KindConflict, "a deploy is already running", 0},
		{"402", answer{402, jsonCT, `{"error":"new deploys are paused: monthly budget reached; new spend is paused."}`, nil}, deploy.KindConflict, "new deploys are paused: monthly budget reached; new spend is paused.", 0},
		{"400", answer{400, jsonCT, `{"error":"invalid branch"}`, nil}, deploy.KindRejected, "invalid branch", 0},
		{"400 message fallback", answer{400, jsonCT, `{"message":"env is required"}`, nil}, deploy.KindRejected, "env is required", 0},
		{"413", answer{413, jsonCT, `{"error":"body too large"}`, nil}, deploy.KindRejected, "body too large", 0},
		{"422", answer{422, jsonCT, `{"error":"unknown environment"}`, nil}, deploy.KindRejected, "unknown environment", 0},
		{"429", answer{429, jsonCT, `{"error":"slow down"}`, map[string]string{"Retry-After": "5"}}, deploy.KindUnavailable, "", 5 * time.Second},
		{"500", answer{500, jsonCT, `{"error":"failed to list deployments: boom"}`, ra}, deploy.KindUnavailable, "", 0},
		{"502 ingress HTML", answer{502, htmlCT, "<html><body>502 Bad Gateway</body></html>", ra}, deploy.KindUnavailable, "", 0},
		{"503 Retry-After", answer{503, jsonCT, `{"error":"kubernetes client unavailable"}`, ra}, deploy.KindUnavailable, "", 7 * time.Second},
		{"504", answer{504, textCT, "upstream timed out", nil}, deploy.KindUnavailable, "", 0},
		{"599", answer{599, jsonCT, `{"error":"odd"}`, nil}, deploy.KindUnavailable, "", 0},
		{"other 418", answer{418, jsonCT, `{"error":"teapot"}`, nil}, deploy.KindUnavailable, "", 0},
		{"other 405", answer{405, textCT, "Method Not Allowed", nil}, deploy.KindUnavailable, "", 0},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			hs := newHarness(t, serve(r.a))
			err := hs.c.do(context.Background(), "ship", http.MethodPost, "/api/v1/projects/p/deploy", map[string]string{}, nil)
			e := asDeployError(t, err)
			if e.Kind != r.kind || e.Message != r.message || e.RetryAfter != r.retry || e.Status != r.a.status || e.Op != "ship" {
				t.Errorf("got {%q %q retry %v status %d op %q}, want {%q %q retry %v status %d op ship}",
					e.Kind, e.Message, e.RetryAfter, e.Status, e.Op, r.kind, r.message, r.retry, r.a.status)
			}
			if e.Cause == nil {
				t.Error("Cause is nil; it carries the failure for the log")
			}
		})
	}
	t.Run("transport", func(t *testing.T) {
		hs := newHarness(t, serve(answer{status: 200}))
		hs.srv.Close()
		err := hs.c.do(context.Background(), "ship", http.MethodPost, "/api/v1/projects/p/deploy", map[string]string{}, nil)
		e := asDeployError(t, err)
		if e.Kind != deploy.KindUnavailable || e.Message != "" || e.RetryAfter != 0 || e.Status != 0 || e.Cause == nil {
			t.Errorf("closed server: %+v; want unavailable, status 0, a Cause", e)
		}
	})
}

func TestPaymentRequiredIsConflict(t *testing.T) {
	sentences := []string{
		`new deploys are paused: team "alpha" has reached its monthly budget; new deploys and LLM usage are paused until the next cycle or a raised budget (Team Admin → Billing).`,
		`production deploy blocked by the project's risk assessment: 2 critical requirements failing. Resolve the failing risk requirements on the project Risk page, then re-deploy.`,
	}
	for _, s := range sentences {
		body := `{"error":` + jsonString(s) + `}` + "\n"
		hs := newHarness(t, serve(answer{402, "application/json", body, nil}))
		err := hs.c.do(context.Background(), "ship", http.MethodPost, "/api/v1/projects/p/deploy", map[string]string{}, nil)
		e := asDeployError(t, err)
		if e.Kind != deploy.KindConflict || e.Status != 402 || e.Message != s {
			t.Errorf("402: {%q %d %q}; want conflict 402 with the sentence %q", e.Kind, e.Status, e.Message, s)
		}
	}
}

func TestFiveHundredSentenceNeverRendered(t *testing.T) {
	const sentence = "failed to list deployments: dial tcp 10.0.3.7:6443"
	for _, status := range []int{500, 502, 503, 504} {
		hs := newHarness(t, serve(answer{status, "application/json", `{"error":"` + sentence + `"}`, nil}))
		err := hs.c.do(context.Background(), "environments", http.MethodGet, "/api/v1/projects/p/deployments/latest", nil, nil)
		e := asDeployError(t, err)
		if e.Kind != deploy.KindUnavailable || e.Message != "" {
			t.Errorf("%d: {%q %q}; want unavailable with no message", status, e.Kind, e.Message)
		}
		for what, s := range map[string]string{"Error()": err.Error(), "Cause": e.Cause.Error()} {
			if strings.Contains(s, "10.0.3.7") {
				t.Errorf("%d: %s carries the cluster address: %s", status, what, s)
			}
		}
		if !strings.Contains(hs.log.String(), sentence) {
			t.Errorf("%d: the sentence is not in the Debug log:\n%s", status, hs.log.String())
		}
	}
}

func jsonString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func TestReadProblem(t *testing.T) {
	long := strings.Repeat("é", 400) // 800 bytes, trimmed to the first 250 runes
	cases := []struct{ body, want string }{
		{`{"error":"api key revoked or expired"}`, "api key revoked or expired"},
		{`  {"error":"  padded  "}  `, "padded"},
		{`{"message":"from message"}`, "from message"},
		{`{"error":"","message":"empty error falls back"}`, "empty error falls back"},
		{`{"error":{"code":7},"message":"object error falls back"}`, "object error falls back"},
		{`{"error":"wins","message":"loses"}`, "wins"},
		{`{"data":{}}`, ""},
		{`<html><body>502 Bad Gateway</body></html>`, ""},
		{`404 page not found`, ""},
		{``, ""},
		{`["error"]`, ""},
		{`{"error":"` + long + `"}`, strings.Repeat("é", 250)},
		{"{\"error\":\"bad \xff byte\"}", "bad � byte"},
	}
	for _, c := range cases {
		got := readProblem([]byte(c.body))
		if got != c.want {
			t.Errorf("readProblem(%.40q) = %.60q, want %.60q", c.body, got, c.want)
		}
		if len(got) > 500 || !utf8.ValidString(got) {
			t.Errorf("readProblem(%.40q) = %d bytes, valid UTF-8 %v", c.body, len(got), utf8.ValidString(got))
		}
	}
	odd := strings.Repeat("a", 499) + "é" // the rune straddles byte 500
	if got := readProblem([]byte(`{"error":"` + odd + `"}`)); got != strings.Repeat("a", 499) {
		t.Errorf("a rune straddling byte 500: got %d bytes, want the 499 before it", len(got))
	}
	exact := strings.Repeat("b", 500)
	if got := readProblem([]byte(`{"error":"` + exact + `"}`)); got != exact {
		t.Errorf("a 500-byte sentence: got %d bytes, want all 500", len(got))
	}
}

func TestMapStatusRetryAfterOnlyOn429And503(t *testing.T) {
	for status := 300; status < 600; status++ {
		e := mapStatus("op", status, "a sentence", 9*time.Second)
		want := time.Duration(0)
		if status == 429 || status == 503 {
			want = 9 * time.Second
		}
		if e.RetryAfter != want || e.Status != status || e.Op != "op" {
			t.Errorf("mapStatus(%d): RetryAfter %v, Status %d, Op %q; want %v, %d, op", status, e.RetryAfter, e.Status, e.Op, want, status)
		}
		if (status >= 500 || status == 429 || status < 400) && e.Message != "" {
			t.Errorf("mapStatus(%d) rendered the sentence %q", status, e.Message)
		}
	}
}

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		v    string
		want time.Duration
	}{
		{"", 0},
		{"5", 5 * time.Second},
		{" 2 ", 2 * time.Second},
		{"0", 0},
		{"-3", 0},
		{"soon", 0},
		{"1.5", 0},
		{now.Add(90 * time.Second).Format(http.TimeFormat), 90 * time.Second},
		{now.Add(-time.Minute).Format(http.TimeFormat), 0},
		{"99999999999999999999", 0},
	}
	for _, c := range cases {
		if got := parseRetryAfter(c.v, now); got != c.want {
			t.Errorf("parseRetryAfter(%q) = %v, want %v", c.v, got, c.want)
		}
	}
}
