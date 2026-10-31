package product

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/deploymentkube"
	"github.com/lcfrai/luci/control-plane/internal/management"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevalidation "k8s.io/apimachinery/pkg/util/validation"
)

const kubernetesResponseLimit = 8 << 20

type boundedKubernetesBody struct {
	body      io.ReadCloser
	remaining int64
}

func (b *boundedKubernetesBody) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if b.remaining == 0 {
		var extra [1]byte
		n, err := b.body.Read(extra[:])
		if n > 0 {
			return 0, ErrKubernetesReview
		}
		return 0, err
	}
	if int64(len(p)) > b.remaining {
		p = p[:b.remaining]
	}
	n, err := b.body.Read(p)
	b.remaining -= int64(n)
	return n, err
}
func (b *boundedKubernetesBody) Close() error { return b.body.Close() }

type boundedKubernetesTransport struct{ next http.RoundTripper }

func (t boundedKubernetesTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	response, err := t.next.RoundTrip(r)
	if err != nil {
		return nil, err
	}
	if response == nil || response.Body == nil {
		return nil, ErrKubernetesReview
	}
	if response.ContentLength > kubernetesResponseLimit {
		_ = response.Body.Close()
		return nil, ErrKubernetesReview
	}
	response.Body = &boundedKubernetesBody{body: response.Body, remaining: kubernetesResponseLimit}
	return response, nil
}

type KubernetesReferenceRead struct {
	Kind string `json:"kind"`
	Name string `json:"name,omitempty"`
}
type KubernetesReferenceDescriptor struct {
	Kind            string   `json:"kind"`
	Name            string   `json:"name"`
	Namespace       string   `json:"namespace"`
	UID             string   `json:"uid"`
	ResourceVersion string   `json:"resourceVersion"`
	Keys            []string `json:"keys,omitempty"`
	Phase           string   `json:"phase,omitempty"`
	AccessModes     []string `json:"accessModes,omitempty"`
	VolumeMode      string   `json:"volumeMode,omitempty"`
	KeysRead        bool     `json:"keysRead"`
	Partial         bool     `json:"partial"`
}
type KubernetesReferenceView struct {
	ProjectID         string                          `json:"projectId"`
	TargetID          string                          `json:"targetId"`
	TargetFingerprint string                          `json:"targetFingerprint"`
	NamespaceUID      string                          `json:"namespaceUid"`
	Namespace         string                          `json:"namespace"`
	Items             []KubernetesReferenceDescriptor `json:"items"`
	Partial           bool                            `json:"partial"`
}

// This explicit POST inspects only an existing target. Listing uses Kubernetes
// PartialObjectMetadata, never a bulk list of Secret/ConfigMap values. A named
// selection reads its keys in memory; only key names and identity are returned.
func (s *Server) ReadKubernetesReferences(ctx context.Context, actor management.ExecutionActorRef, targetID string, input KubernetesReferenceRead) (KubernetesReferenceView, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if s.kubernetesAuthority(ctx, actor, false, true, 0) != nil {
		return KubernetesReferenceView{}, management.ErrExecutionAuthority
	}
	resource := ""
	switch input.Kind {
	case "persistent-volume-claim":
		resource = "persistentvolumeclaims"
	case "secret":
		resource = "secrets"
	case "config-map":
		resource = "configmaps"
	default:
		return KubernetesReferenceView{}, ErrKubernetesReview
	}
	if input.Name != "" && len(kubevalidation.IsDNS1123Subdomain(input.Name)) != 0 {
		return KubernetesReferenceView{}, ErrKubernetesReview
	}
	_, ledger, err := s.readKubernetesTargets(actor.ProjectID)
	if err != nil {
		return KubernetesReferenceView{}, ErrKubernetesReview
	}
	var selected KubernetesTargetDescriptor
	for _, row := range ledger.Targets {
		if row.ID == targetID {
			selected = row.KubernetesTargetDescriptor
		}
	}
	if selected.ID == "" {
		return KubernetesReferenceView{}, ErrKubernetesReview
	}
	target, err := s.resolveKubernetesTarget(ctx, deploymentkube.Identity{ProjectID: actor.ProjectID, TargetID: selected.ID, TargetFingerprint: selected.Fingerprint})
	if err != nil {
		return KubernetesReferenceView{}, ErrKubernetesReview
	}
	check := func() error {
		if s.kubernetesAuthority(ctx, actor, false, true, 0) != nil {
			return management.ErrExecutionAuthority
		}
		current, err := s.resolveKubernetesTarget(ctx, deploymentkube.Identity{ProjectID: actor.ProjectID, TargetID: selected.ID, TargetFingerprint: selected.Fingerprint})
		if err != nil || current.NamespaceUID != selected.NamespaceUID {
			return ErrKubernetesReview
		}
		namespace, err := current.Client.CoreV1().Namespaces().Get(ctx, current.Namespace, metav1.GetOptions{})
		if err != nil || namespace == nil || string(namespace.UID) != selected.NamespaceUID || namespace.DeletionTimestamp != nil {
			return ErrKubernetesReview
		}
		return s.kubernetesAuthority(ctx, actor, false, true, 0)
	}
	if err := check(); err != nil {
		return KubernetesReferenceView{}, err
	}
	view := KubernetesReferenceView{ProjectID: actor.ProjectID, TargetID: selected.ID, TargetFingerprint: selected.Fingerprint, NamespaceUID: selected.NamespaceUID, Namespace: selected.Namespace, Items: []KubernetesReferenceDescriptor{}}
	if input.Name == "" {
		rest := target.Client.CoreV1().RESTClient()
		if rest == nil {
			return KubernetesReferenceView{}, ErrKubernetesReview
		}
		stream, err := rest.Get().Namespace(target.Namespace).Resource(resource).Param("limit", "64").SetHeader("Accept", "application/json;as=PartialObjectMetadataList;g=meta.k8s.io;v=v1").Stream(ctx)
		if err != nil {
			return KubernetesReferenceView{}, ErrKubernetesReview
		}
		data, readErr := io.ReadAll(io.LimitReader(stream, (1<<20)+1))
		closeErr := stream.Close()
		if readErr != nil || closeErr != nil || len(data) > 1<<20 {
			return KubernetesReferenceView{}, ErrKubernetesReview
		}
		var list metav1.PartialObjectMetadataList
		if json.Unmarshal(data, &list) != nil || list.Kind != "PartialObjectMetadataList" || list.APIVersion != "meta.k8s.io/v1" || len(list.Items) > 64 {
			return KubernetesReferenceView{}, ErrKubernetesReview
		}
		view.Partial = list.Continue != ""
		for _, row := range list.Items {
			if row.DeletionTimestamp != nil {
				continue
			}
			view.Items = append(view.Items, KubernetesReferenceDescriptor{Kind: input.Kind, Name: row.Name, Namespace: row.Namespace, UID: string(row.UID), ResourceVersion: row.ResourceVersion})
		}
	} else {
		row := KubernetesReferenceDescriptor{Kind: input.Kind, Name: input.Name, Namespace: target.Namespace}
		switch input.Kind {
		case "secret":
			object, err := target.Client.CoreV1().Secrets(target.Namespace).Get(ctx, input.Name, metav1.GetOptions{})
			if err != nil || object == nil || object.Name != input.Name || object.Namespace != target.Namespace || object.DeletionTimestamp != nil {
				return KubernetesReferenceView{}, ErrKubernetesReview
			}
			row.UID, row.ResourceVersion = string(object.UID), object.ResourceVersion
			row.KeysRead = true
			for key := range object.Data {
				row.Keys = append(row.Keys, key)
			}
		case "config-map":
			object, err := target.Client.CoreV1().ConfigMaps(target.Namespace).Get(ctx, input.Name, metav1.GetOptions{})
			if err != nil || object == nil || object.Name != input.Name || object.Namespace != target.Namespace || object.DeletionTimestamp != nil {
				return KubernetesReferenceView{}, ErrKubernetesReview
			}
			row.UID, row.ResourceVersion = string(object.UID), object.ResourceVersion
			row.KeysRead = true
			for key := range object.Data {
				row.Keys = append(row.Keys, key)
			}
		case "persistent-volume-claim":
			object, err := target.Client.CoreV1().PersistentVolumeClaims(target.Namespace).Get(ctx, input.Name, metav1.GetOptions{})
			if err != nil || object == nil || object.Name != input.Name || object.Namespace != target.Namespace || object.DeletionTimestamp != nil {
				return KubernetesReferenceView{}, ErrKubernetesReview
			}
			row.UID, row.ResourceVersion, row.Phase = string(object.UID), object.ResourceVersion, string(object.Status.Phase)
			row.VolumeMode = "Filesystem"
			if object.Spec.VolumeMode != nil {
				row.VolumeMode = string(*object.Spec.VolumeMode)
			}
			for _, mode := range object.Spec.AccessModes {
				row.AccessModes = append(row.AccessModes, string(mode))
			}
			sort.Strings(row.AccessModes)
		}
		sort.Strings(row.Keys)
		if len(row.Keys) > 64 {
			row.Keys = row.Keys[:64]
			row.Partial = true
			view.Partial = true
		}
		view.Items = append(view.Items, row)
	}
	for _, row := range view.Items {
		if row.Namespace != target.Namespace || row.Name == "" || len(kubevalidation.IsDNS1123Subdomain(row.Name)) != 0 || row.UID == "" || len(row.UID) > 128 || row.ResourceVersion == "" || len(row.ResourceVersion) > 128 {
			return KubernetesReferenceView{}, ErrKubernetesReview
		}
		for _, key := range row.Keys {
			if len(kubevalidation.IsConfigMapKey(key)) != 0 {
				return KubernetesReferenceView{}, ErrKubernetesReview
			}
		}
		if row.Phase != "" && row.Phase != "Bound" && row.Phase != "Pending" && row.Phase != "Lost" {
			return KubernetesReferenceView{}, ErrKubernetesReview
		}
		if row.VolumeMode != "" && row.VolumeMode != "Filesystem" && row.VolumeMode != "Block" {
			return KubernetesReferenceView{}, ErrKubernetesReview
		}
		if len(row.AccessModes) > 4 {
			return KubernetesReferenceView{}, ErrKubernetesReview
		}
		for _, mode := range row.AccessModes {
			if mode != "ReadWriteOnce" && mode != "ReadWriteOncePod" && mode != "ReadOnlyMany" && mode != "ReadWriteMany" {
				return KubernetesReferenceView{}, ErrKubernetesReview
			}
		}
	}
	sort.Slice(view.Items, func(i, j int) bool { return view.Items[i].Name < view.Items[j].Name })
	if err := check(); err != nil {
		return KubernetesReferenceView{}, err
	}
	return view, nil
}
