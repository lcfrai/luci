package product

import (
	"encoding/json"
	"io"
	"strings"
	"testing"
)

type referenceBodyFixture struct {
	io.Reader
	closed bool
}

func (b *referenceBodyFixture) Close() error { b.closed = true; return nil }
func TestKubernetesResponseBudgetRejectsExcessAndClosesOriginal(t *testing.T) {
	for _, row := range []struct {
		input     string
		wantError bool
	}{{"1234", false}, {"12345", true}} {
		original := &referenceBodyFixture{Reader: strings.NewReader(row.input)}
		body := &boundedKubernetesBody{body: original, remaining: 4}
		data, err := io.ReadAll(body)
		if (err != nil) != row.wantError || string(data) != "1234" {
			t.Fatal("bounded response read changed", string(data), err)
		}
		if body.Close() != nil || !original.closed {
			t.Fatal("original body not closed")
		}
	}
}
func TestKubernetesReferenceDescriptorsCannotCarryResourceValues(t *testing.T) {
	row := KubernetesReferenceDescriptor{Kind: "secret", Name: "access", Namespace: "apps", UID: "original", ResourceVersion: "2", Keys: []string{"token"}, KeysRead: true}
	raw, err := json.Marshal(row)
	if err != nil || strings.Contains(string(raw), `"data"`) || strings.Contains(string(raw), `"value"`) {
		t.Fatal("descriptor has resource-value channel")
	}
	if !strings.Contains(string(raw), `"keys":["token"]`) {
		t.Fatal("selected key metadata unavailable")
	}
}
