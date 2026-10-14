package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAzureGuestRegistryExactSourceAndDigestRefusal(t *testing.T) {
	source, digest := strings.Repeat("a", 40), strings.Repeat("b", 64)
	image := "127.0.0.1:5055/luci/guest@sha256:" + digest
	value := map[string]any{"sourceCommit": source, "fromImage": image, "imageDigest": digest, "receiptSHA256": strings.Repeat("c", 64), "taskStateRestored": false}
	response := func(rows ...string) []byte {
		values := []map[string]string{}
		for _, row := range rows {
			values = append(values, map[string]string{"message": row})
		}
		data, _ := json.Marshal(map[string]any{"value": values})
		return data
	}
	marker := func() string { data, _ := json.Marshal(value); return "LUCI_AZURE_GUEST_SOURCE=" + string(data) }
	if err := azureGuestRegistryReceipt(response(marker()), source, image); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"sourceCommit", "fromImage", "imageDigest", "receiptSHA256", "taskStateRestored"} {
		old := value[field]
		value[field] = "changed"
		if azureGuestRegistryReceipt(response(marker()), source, image) == nil {
			t.Fatalf("accepted changed %s", field)
		}
		value[field] = old
	}
	if azureGuestRegistryReceipt(response(marker(), marker()), source, image) == nil {
		t.Fatal("accepted ambiguous inspector receipt")
	}
	if azureGuestRegistryReceipt(response("no source receipt"), source, image) == nil {
		t.Fatal("accepted missing source receipt")
	}
	if strings.Contains(azureGuestRegistryScript(source), "docker") || !strings.Contains(azureGuestRegistryScript(source), "'ready'") {
		t.Fatal("source inspection must retain read-only guest entrypoint")
	}
}
