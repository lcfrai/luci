package deploymentkube

import (
	"context"
	"errors"
	"reflect"
	"sort"

	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type requiredReference struct {
	kind, name string
	keys       []string
	services   []deploymentplan.Service
}

var ErrReferenceChanged = errors.New("The reviewed imported resource identity or selected key metadata changed.")

func requiredReferences(blueprint deploymentplan.Blueprint) []requiredReference {
	rows := map[string]requiredReference{}
	for _, service := range blueprint.Services {
		for name, keys := range deploymentplan.TelemetrySecretKeys(service) {
			key := "secret/" + name
			row := rows[key]
			row.kind, row.name = "secret", name
			for _, selected := range keys {
				present := false
				for _, old := range row.keys {
					present = present || old == selected
				}
				if !present {
					row.keys = append(row.keys, selected)
				}
			}
			rows[key] = row
		}
		if service.Model != nil {
			key := "persistent-volume-claim/" + service.Model.ClaimName
			row := rows[key]
			row.kind, row.name = "persistent-volume-claim", service.Model.ClaimName
			row.services = append(row.services, service)
			rows[key] = row
		}
		for _, ref := range service.Environment {
			key := ref.Kind + "/" + ref.ResourceName
			row := rows[key]
			row.kind, row.name = ref.Kind, ref.ResourceName
			present := false
			for _, name := range row.keys {
				if name == ref.Key {
					present = true
				}
			}
			if !present {
				row.keys = append(row.keys, ref.Key)
			}
			rows[key] = row
		}
	}
	keys := make([]string, 0, len(rows))
	for key := range rows {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]requiredReference, 0, len(keys))
	for _, key := range keys {
		row := rows[key]
		sort.Strings(row.keys)
		out = append(out, row)
	}
	return out
}

func referenceDigest(row ReferenceSnapshot) string { row.MetadataDigest = ""; return digest(row) }

// Pure validation for private reviewed-record loading. Legacy omitted bindings
// remain readable, but cannot pass new Apply when a runtime reference is used.
func ValidateReferenceBinding(request Request) error { return canonicalReferenceSnapshots(&request) }

func canonicalReferenceSnapshots(request *Request) error {
	required := requiredReferences(request.Blueprint)
	if len(required) > 64 || len(request.References) > 64 {
		return ErrRefused
	}
	// Legacy receipts remain readable. New Apply explicitly requires the complete
	// binding; Observe marks missing historical bindings unreviewed, not healthy.
	if len(request.References) == 0 {
		request.References = nil
		return nil
	}
	if len(required) != len(request.References) {
		return ErrRefused
	}
	rows := append([]ReferenceSnapshot(nil), request.References...)
	sort.Slice(rows, func(i, j int) bool { return rows[i].Kind+"/"+rows[i].Name < rows[j].Kind+"/"+rows[j].Name })
	for index, row := range rows {
		need := required[index]
		if row.Kind != need.kind || row.Name != need.name || row.Namespace != request.Blueprint.Infrastructure.Namespace || row.NamespaceUID == "" || len(row.NamespaceUID) > 128 || row.TargetFingerprint != request.Identity.TargetFingerprint || row.UID == "" || len(row.UID) > 128 || row.ResourceVersion == "" || len(row.ResourceVersion) > 128 || !reflect.DeepEqual(row.Keys, need.keys) || row.MetadataDigest != referenceDigest(row) {
			return ErrRefused
		}
		if row.Kind == "persistent-volume-claim" {
			if !claimCompatible(row, need.services) {
				return ErrRefused
			}
		} else if row.Phase != "" || len(row.AccessModes) != 0 || row.VolumeMode != "" || row.VolumeName != "" {
			return ErrRefused
		}
	}
	request.References = rows
	return nil
}

func claimCompatible(row ReferenceSnapshot, services []deploymentplan.Service) bool {
	if row.Phase != string(corev1.ClaimBound) || row.VolumeMode != string(corev1.PersistentVolumeFilesystem) || row.VolumeName == "" || len(row.VolumeName) > 253 || len(row.AccessModes) == 0 || len(row.AccessModes) > 4 {
		return false
	}
	shared, single, singlePod := false, false, false
	seen := map[string]bool{}
	for _, mode := range row.AccessModes {
		if seen[mode] {
			return false
		}
		seen[mode] = true
		switch mode {
		case "ReadOnlyMany", "ReadWriteMany":
			shared = true
		case "ReadWriteOnce":
			single = true
		case "ReadWriteOncePod":
			single = true
			singlePod = true
		default:
			return false
		}
	}
	if !shared && !single {
		return false
	}
	// RWO only promises single-node access, not co-location of independently
	// scheduled services. This connector has no reviewed co-location capability,
	// so nonshared RWO and RWOP both require one distinct consuming service.
	if singlePod || !shared {
		consumers := map[string]bool{}
		for _, service := range services {
			consumers[service.ID] = true
			if service.Replicas != 1 || service.DeploymentStrategy != "recreate" {
				return false
			}
		}
		if len(consumers) != 1 {
			return false
		}
	}
	for _, service := range services {
		if !shared && (service.Replicas != 1 || service.DeploymentStrategy != "recreate") {
			return false
		}
	}
	return true
}

// ReadReferences never returns API error text or resource values. It performs
// bounded named reads under repeated caller-owned authority checks. Namespace
// identity is observed, not an atomic precondition on later Kubernetes writes.
func ReadReferences(ctx context.Context, target Target, blueprint deploymentplan.Blueprint, authorize func(context.Context) error) ([]ReferenceSnapshot, error) {
	required := requiredReferences(blueprint)
	if len(required) == 0 {
		return nil, nil
	}
	if len(required) > 64 || target.Client == nil || target.Namespace != blueprint.Infrastructure.Namespace || target.NamespaceUID == "" || authorize == nil {
		return nil, ErrRefused
	}
	checkNamespace := func() error {
		if ctx.Err() != nil || authorize(ctx) != nil {
			return ErrRefused
		}
		namespace, err := target.Client.CoreV1().Namespaces().Get(ctx, target.Namespace, metav1.GetOptions{})
		if err != nil || namespace == nil || string(namespace.UID) != target.NamespaceUID || namespace.DeletionTimestamp != nil || authorize(ctx) != nil {
			return ErrRefused
		}
		return nil
	}
	if checkNamespace() != nil {
		return nil, ErrRefused
	}
	rows := make([]ReferenceSnapshot, 0, len(required))
	for _, need := range required {
		if ctx.Err() != nil || authorize(ctx) != nil {
			return nil, ErrRefused
		}
		row := ReferenceSnapshot{Kind: need.kind, Name: need.name, Namespace: target.Namespace, NamespaceUID: target.NamespaceUID, TargetFingerprint: target.Fingerprint, Keys: append([]string(nil), need.keys...)}
		switch need.kind {
		case "secret":
			object, err := target.Client.CoreV1().Secrets(target.Namespace).Get(ctx, need.name, metav1.GetOptions{})
			if err != nil || object == nil || object.Name != need.name || object.Namespace != target.Namespace || object.DeletionTimestamp != nil {
				return nil, ErrRefused
			}
			for _, key := range need.keys {
				if _, exists := object.Data[key]; !exists {
					return nil, ErrRefused
				}
			}
			row.UID, row.ResourceVersion = string(object.UID), object.ResourceVersion
		case "config-map":
			object, err := target.Client.CoreV1().ConfigMaps(target.Namespace).Get(ctx, need.name, metav1.GetOptions{})
			if err != nil || object == nil || object.Name != need.name || object.Namespace != target.Namespace || object.DeletionTimestamp != nil {
				return nil, ErrRefused
			}
			for _, key := range need.keys {
				if _, exists := object.Data[key]; !exists {
					return nil, ErrRefused
				}
			}
			row.UID, row.ResourceVersion = string(object.UID), object.ResourceVersion
		case "persistent-volume-claim":
			object, err := target.Client.CoreV1().PersistentVolumeClaims(target.Namespace).Get(ctx, need.name, metav1.GetOptions{})
			if err != nil || object == nil || object.Name != need.name || object.Namespace != target.Namespace || object.DeletionTimestamp != nil {
				return nil, ErrRefused
			}
			row.UID, row.ResourceVersion, row.Phase, row.VolumeName = string(object.UID), object.ResourceVersion, string(object.Status.Phase), object.Spec.VolumeName
			row.VolumeMode = "Filesystem"
			if object.Spec.VolumeMode != nil {
				row.VolumeMode = string(*object.Spec.VolumeMode)
			}
			for _, mode := range object.Spec.AccessModes {
				row.AccessModes = append(row.AccessModes, string(mode))
			}
			sort.Strings(row.AccessModes)
			if !claimCompatible(row, need.services) {
				return nil, ErrRefused
			}
		default:
			return nil, ErrRefused
		}
		if row.UID == "" || len(row.UID) > 128 || row.ResourceVersion == "" || len(row.ResourceVersion) > 128 || authorize(ctx) != nil {
			return nil, ErrRefused
		}
		row.MetadataDigest = referenceDigest(row)
		rows = append(rows, row)
	}
	if checkNamespace() != nil {
		return nil, ErrRefused
	}
	return rows, nil
}

func (c *Connector) validateReferences(ctx context.Context, target Target, request Request, service *deploymentplan.Service, mutation bool) error {
	blueprint := request.Blueprint
	if service != nil {
		blueprint.Services = []deploymentplan.Service{*service}
	}
	actual, err := ReadReferences(ctx, target, blueprint, func(ctx context.Context) error { return c.authorize(ctx, request, mutation) })
	if err != nil {
		return ErrRefused
	}
	expected := request.References
	if service != nil {
		wanted := map[string]bool{}
		for _, ref := range requiredReferences(blueprint) {
			wanted[ref.kind+"/"+ref.name] = true
		}
		expected = nil
		for _, ref := range request.References {
			if wanted[ref.Kind+"/"+ref.Name] {
				copy := ref
				for _, need := range requiredReferences(blueprint) {
					if need.kind == ref.Kind && need.name == ref.Name {
						copy.Keys = need.keys
					}
				}
				copy.MetadataDigest = referenceDigest(copy)
				expected = append(expected, copy)
			}
		}
	}
	if !reflect.DeepEqual(actual, expected) {
		return ErrReferenceChanged
	}
	return nil
}
