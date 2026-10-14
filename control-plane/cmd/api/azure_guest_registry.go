package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
)

var azureGuestSourcePattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

// This fixed guest inspection runs only against the independently verified
// runtime ARM identity. It has no shell input, secret, upload or build branch.
func azureGuestRegistryScript(source string) string {
	return "set -eu\npython3 - <<'PY'\nimport hashlib,json,pathlib,subprocess\np=pathlib.Path('/opt/luci/guest-registry/receipt.json')\nr=json.loads(subprocess.check_output(['python3','/opt/luci/azure-embed/package/bin/guest_registry.py','ready','--source-commit','" + source + "'],timeout=180))\nprint('LUCI_AZURE_GUEST_SOURCE='+json.dumps({'sourceCommit':r['sourceCommit'],'fromImage':r['fromImage'],'imageDigest':r['imageDigest'],'receiptSHA256':hashlib.sha256(p.read_bytes()).hexdigest(),'taskStateRestored':False},separators=(',',':')))\nPY"
}

func azureGuestRegistryReceipt(data []byte, source, image string) error {
	const marker = "LUCI_AZURE_GUEST_SOURCE="
	var response struct {
		Value []struct {
			Message string `json:"message"`
		} `json:"value"`
	}
	if len(data) > 64<<10 || json.Unmarshal(data, &response) != nil {
		return errors.New("runtime guest source inspection is incomplete")
	}
	count := 0
	for _, row := range response.Value {
		for _, line := range strings.Split(row.Message, "\n") {
			if !strings.HasPrefix(line, marker) {
				continue
			}
			count++
			var value struct {
				Source            string `json:"sourceCommit"`
				Image             string `json:"fromImage"`
				Digest            string `json:"imageDigest"`
				Receipt           string `json:"receiptSHA256"`
				TaskStateRestored *bool  `json:"taskStateRestored"`
			}
			if json.Unmarshal([]byte(strings.TrimPrefix(line, marker)), &value) != nil || value.Source != source || value.Image != image ||
				!azureDigestPattern.MatchString(value.Digest) || !azureDigestPattern.MatchString(value.Receipt) ||
				image != "127.0.0.1:5055/luci/guest@sha256:"+value.Digest || value.TaskStateRestored == nil || *value.TaskStateRestored {
				return errors.New("runtime guest source differs from signed selection")
			}
		}
	}
	if count != 1 {
		return errors.New("runtime guest source inspection is ambiguous")
	}
	return nil
}

func (c *azureARMClient) inspectGuestRegistry(ctx context.Context, runtimeID, source, image string) error {
	if !strings.HasPrefix(image, "127.0.0.1:5055/luci/guest@sha256:") {
		return nil
	}
	if !azureGuestSourcePattern.MatchString(source) || !strings.HasPrefix(runtimeID, "/subscriptions/") || strings.ContainsAny(runtimeID, "?#") {
		return errors.New("runtime guest source inspection identity is invalid")
	}
	token, err := c.credential.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{azureARMOrigin + "/.default"}})
	if err != nil {
		return errors.New("runtime source inspection managed identity is unavailable")
	}
	body, _ := json.Marshal(map[string]any{"commandId": "RunShellScript", "script": []string{azureGuestRegistryScript(source)}})
	request, _ := http.NewRequestWithContext(ctx, http.MethodPost, azureARMOrigin+runtimeID+"/runCommand?api-version=2023-09-01", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+token.Token)
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return errors.New("runtime source inspection unavailable; no template mutation permitted")
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusOK {
		data, err := io.ReadAll(io.LimitReader(response.Body, (64<<10)+1))
		if err != nil {
			return errors.New("runtime source inspection read failed")
		}
		return azureGuestRegistryReceipt(data, source, image)
	}
	if response.StatusCode != http.StatusAccepted {
		return errors.New("runtime source inspection was not accepted")
	}
	operation, result := response.Header.Get("Azure-AsyncOperation"), response.Header.Get("Location")
	validURL := func(value string) bool {
		u, err := url.Parse(value)
		return err == nil && u.Scheme == "https" && u.Host == "management.azure.com" && u.User == nil && u.Fragment == "" &&
			strings.HasPrefix(strings.ToLower(u.Path), "/subscriptions/"+strings.ToLower(strings.Split(runtimeID, "/")[2])+"/")
	}
	if !validURL(operation) || !validURL(result) {
		return errors.New("runtime source inspection operation has no exact ARM scope")
	}
	read := func(address string) ([]byte, error) {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
		req.Header.Set("Authorization", "Bearer "+token.Token)
		resp, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(io.LimitReader(resp.Body, (64<<10)+1))
		if err != nil || resp.StatusCode != http.StatusOK || len(data) > 64<<10 {
			return nil, errors.New("runtime source inspection result incomplete")
		}
		return data, nil
	}
	for range 25 {
		select {
		case <-ctx.Done():
			return errors.New("runtime source inspection remains uncertain")
		case <-time.After(2 * time.Second):
		}
		data, err := read(operation)
		var status struct {
			Status string `json:"status"`
		}
		if err != nil || json.Unmarshal(data, &status) != nil {
			return errors.New("runtime source inspection readback uncertain")
		}
		if status.Status == "Succeeded" {
			data, err = read(result)
			if err != nil {
				return err
			}
			return azureGuestRegistryReceipt(data, source, image)
		}
		if status.Status != "InProgress" && status.Status != "Running" && status.Status != "Accepted" {
			return errors.New("runtime source inspection did not succeed")
		}
	}
	return errors.New("runtime source inspection remains uncertain; no template mutation permitted")
}
