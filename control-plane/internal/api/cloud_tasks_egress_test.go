package api

import "testing"

func TestCloudModelDestinationMustMatchOperatorEgress(t *testing.T) {
	cloud := &CloudTaskRuntime{ModelEgressDomains: []string{"api.openai.com", "api.anthropic.com"}}
	for _, tc := range []struct {
		harness string
		env     map[string]string
		want    bool
	}{
		{"codex", map[string]string{}, true},
		{"claude-code", map[string]string{}, true},
		{"codex", map[string]string{"OPENAI_BASE_URL": "https://api.openai.com/v1"}, true},
		{"codex", map[string]string{"OPENAI_BASE_URL": "https://unlisted.example/v1"}, false},
		{"claude-code", map[string]string{"ANTHROPIC_BASE_URL": "http://api.anthropic.com"}, false},
		{"codex", map[string]string{"OPENAI_BASE_URL": "https://api.openai.com.evil.example/v1"}, false},
		{"codex", map[string]string{"OPENAI_BASE_URL": "https://user:secret@api.openai.com/v1"}, false},
		{"codex", map[string]string{"OPENAI_BASE_URL": "https://api.openai.com:8443/v1"}, false},
		{"shell", map[string]string{}, false},
	} {
		if got := cloud.allowsModelEndpoint(tc.harness, tc.env); got != tc.want {
			t.Fatalf("harness=%s env=%v allowed=%t want=%t", tc.harness, tc.env, got, tc.want)
		}
	}
	if (&CloudTaskRuntime{}).allowsModelEndpoint("codex", nil) {
		t.Fatal("unconfigured model egress admitted")
	}
}
