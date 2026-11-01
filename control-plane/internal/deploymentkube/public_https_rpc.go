package deploymentkube

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"k8s.io/client-go/rest"
)

// Same bounded authenticated transport/opaque error handling as preparation
// RPCs. This closed extension adds only the typed owned HTTPS profile's objects.
// No arbitrary URL/API group, manifest, retry, body recreation or exec surface.
func PublicHTTPSResourceRPC(ctx context.Context, target Target, method, kind, name string, input, output any) error {
	resource, group := "", "/api/v1"
	switch kind {
	case "PersistentVolumeClaim":
		resource = "persistentvolumeclaims"
	case "Secret":
		resource = "secrets"
	case "ConfigMap":
		resource = "configmaps"
	case "Service":
		resource = "services"
	case "Deployment":
		resource, group = "deployments", "/apis/apps/v1"
	default:
		return ErrRefused
	}
	if method != http.MethodGet && method != http.MethodPost && method != http.MethodDelete && method != http.MethodPatch || method == http.MethodPatch && kind != "Deployment" || (method == http.MethodPost) != (name == "") || name != "" && !token.MatchString(name) || !token.MatchString(target.Namespace) {
		return ErrRefused
	}
	client, e := preparationREST(target, 15*time.Second, 1<<20)
	if e != nil {
		return e
	}
	path := group + "/namespaces/" + target.Namespace + "/" + resource
	if name != "" {
		path += "/" + name
	}
	request := client.Verb(method).AbsPath(path).MaxRetries(0).WarningHandler(rest.NoWarnings{})
	if input != nil {
		raw, e := json.Marshal(input)
		if e != nil || len(raw) > 128<<10 {
			return ErrRefused
		}
		media := "application/json"
		if method == http.MethodPatch {
			media = "application/merge-patch+json"
		}
		request = request.SetHeader("Content-Type", media).Body(nonReplayablePreparationBody{Reader: bytes.NewReader(raw)})
	}
	stream, e := request.Stream(ctx)
	if e != nil {
		if errors.Is(e, ErrPreparationAbsent) {
			return ErrPreparationAbsent
		}
		return ErrUnknown
	}
	raw, readErr := io.ReadAll(io.LimitReader(stream, (1<<20)+1))
	closeErr := stream.Close()
	if readErr != nil || closeErr != nil || len(raw) > 1<<20 || ctx.Err() != nil {
		return ErrUnknown
	}
	if output != nil && json.Unmarshal(raw, output) != nil {
		return ErrUnknown
	}
	return nil
}
