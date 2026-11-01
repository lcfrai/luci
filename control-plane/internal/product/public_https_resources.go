package product

import (
	"context"
	"errors"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/deploymentkube"
	"github.com/lcfrai/luci/control-plane/internal/project"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func publicHTTPSActual(kind string) (runtime.Object, metav1.Object, error) {
	switch kind {
	case "PersistentVolumeClaim":
		r := &corev1.PersistentVolumeClaim{}
		return r, r, nil
	case "Secret":
		r := &corev1.Secret{}
		return r, r, nil
	case "ConfigMap":
		r := &corev1.ConfigMap{}
		return r, r, nil
	case "Deployment":
		r := &appsv1.Deployment{}
		return r, r, nil
	case "Service":
		r := &corev1.Service{}
		return r, r, nil
	}
	return nil, nil, ErrPublicHTTPS
}
func publicHTTPSCreationStep(item *PublicHTTPSResource) (bool, error) {
	if item == nil || item.UID != "" && !item.Attempted || item.DeleteAttempted || item.AbsentObserved {
		return false, ErrPublicHTTPS
	}
	if item.Attempted {
		if item.UID == "" {
			return false, ErrPublicHTTPS
		}
		return false, nil
	}
	return true, nil
}
func publicHTTPSHasUnattempted(row storedPublicHTTPS) bool {
	for _, item := range row.Resources {
		if !item.Attempted {
			return true
		}
	}
	return false
}
func (s *Server) createPublicHTTPSObjects(ctx context.Context, p project.Project, row storedPublicHTTPS) error {
	objects, e := deploymentkube.PublicHTTPSObjects(publicHTTPSSpec(row), publicHTTPSCredentialData(row))
	if e != nil {
		return e
	}
	for _, desired := range objects {
		if e = s.publicHTTPSGuard(ctx, row, true, true); e != nil {
			return e
		}
		target, e := s.publicHTTPSTarget(ctx, row)
		if e != nil {
			return e
		}
		item := publicHTTPSResource(&row, desired.Kind)
		create, stepErr := publicHTTPSCreationStep(item)
		if stepErr != nil {
			return stepErr
		}
		if !create {
			actual, meta, e := publicHTTPSActual(desired.Kind)
			if e != nil {
				return e
			}
			if s.publicHTTPSRPC(ctx, target, "GET", desired.Kind, desired.Name, nil, actual) != nil || !deploymentkube.PublicHTTPSOwned(publicHTTPSSpec(row), meta, item.UID) || !deploymentkube.PublicHTTPSMatches(desired.Object, actual) || s.publicHTTPSGuard(ctx, row, true, true) != nil {
				return ErrPublicHTTPS
			}
			continue
		}
		if e = s.publicHTTPSRPC(ctx, target, "GET", desired.Kind, desired.Name, nil, nil); !errors.Is(e, deploymentkube.ErrPreparationAbsent) {
			return ErrPublicHTTPS
		}
		row, e = s.publicHTTPSMutate(p, row.ID, row.Actor, func(r *storedPublicHTTPS) error {
			item := publicHTTPSResource(r, desired.Kind)
			if r.RetireAttempted || r.DesiredReplicas != 1 || item == nil || item.Attempted {
				return ErrPublicHTTPS
			}
			item.Attempted = true
			item.State = "unknown"
			if item.Kind == "PersistentVolumeClaim" {
				r.PVCDataRetained = true
			}
			return nil
		})
		if e != nil {
			return e
		}
		if e = s.publicHTTPSGuard(ctx, row, true, true); e != nil {
			return e
		}
		target, e = s.publicHTTPSMutationTarget(ctx, row)
		if e != nil {
			return e
		}
		if desired.Kind == "PersistentVolumeClaim" {
			class, e := target.Client.StorageV1().StorageClasses().Get(ctx, row.Plan.StorageClass, metav1.GetOptions{})
			if e != nil || class == nil || string(class.UID) != row.Plan.StorageClassUID || class.ResourceVersion != row.Plan.StorageClassVersion || class.DeletionTimestamp != nil {
				return ErrPublicHTTPS
			}
		}
		if e = s.publicHTTPSGuard(ctx, row, true, true); e != nil {
			return e
		}
		actual, meta, e := publicHTTPSActual(desired.Kind)
		if e != nil {
			return e
		}
		if e = s.publicHTTPSRPC(ctx, target, "POST", desired.Kind, "", desired.Object, actual); e != nil {
			return e
		}
		uid := string(meta.GetUID())
		if !validConnectionToken(uid, 128, false) {
			return ErrPublicHTTPS
		}
		// Only acknowledged server identity is retained; no later GET substitutes
		// correlation labels for a lost Create UID. Bookkeeping adds no new effect.
		row, e = s.publicHTTPSMutate(p, row.ID, row.Actor, func(r *storedPublicHTTPS) error {
			item := publicHTTPSResource(r, desired.Kind)
			if item == nil || !item.Attempted || item.UID != "" && item.UID != uid {
				return ErrPublicHTTPS
			}
			item.UID = uid
			item.State = "observed"
			return nil
		})
		if e != nil {
			return e
		}
		if !deploymentkube.PublicHTTPSOwned(publicHTTPSSpec(row), meta, uid) || !deploymentkube.PublicHTTPSMatches(desired.Object, actual) || s.publicHTTPSGuard(ctx, row, true, true) != nil {
			return ErrPublicHTTPS
		}
	}
	return nil
}
func (s *Server) publicHTTPSObservedObjects(ctx context.Context, p project.Project, row storedPublicHTTPS) (storedPublicHTTPS, bool, error) {
	spec := publicHTTPSSpec(row)
	credential := publicHTTPSCredentialData(row)
	if row.State == "retired" {
		credential = map[string][]byte{"tenant-id": []byte("retired"), "client-id": []byte("retired"), "client-secret": []byte("retired")}
	}
	desired, e := deploymentkube.PublicHTTPSObjects(spec, credential)
	if e != nil {
		return row, false, e
	}
	allReady := true
	for _, object := range desired {
		item := publicHTTPSResource(&row, object.Kind)
		if item == nil {
			return row, false, ErrPublicHTTPS
		}
		if row.RetireAttempted && !item.Attempted {
			continue
		}
		if !item.Attempted {
			return row, false, ErrPublicHTTPS
		}
		if e = s.publicHTTPSGuard(ctx, row, false, false); e != nil {
			return row, false, e
		}
		target, e := s.publicHTTPSTarget(ctx, row)
		if e != nil {
			return row, false, e
		}
		actual, meta, e := publicHTTPSActual(object.Kind)
		if e != nil {
			return row, false, e
		}
		e = s.publicHTTPSRPC(ctx, target, "GET", object.Kind, object.Name, nil, actual)
		if errors.Is(e, deploymentkube.ErrPreparationAbsent) && (item.DeleteAttempted || row.RetireAttempted && (item.UID == "" || object.Kind == "PersistentVolumeClaim")) {
			row, e = s.publicHTTPSMutate(p, row.ID, row.Actor, func(r *storedPublicHTTPS) error {
				v := publicHTTPSResource(r, object.Kind)
				if v == nil || v.UID != item.UID || !v.DeleteAttempted && !(r.RetireAttempted && (v.UID == "" || v.Kind == "PersistentVolumeClaim")) {
					return ErrPublicHTTPS
				}
				v.AbsentObserved = true
				v.State = "deleted"
				if v.Kind == "PersistentVolumeClaim" {
					r.PVCDataRetained = false
				}
				return nil
			})
			if e != nil {
				return row, false, e
			}
			continue
		}
		if item.UID == "" || e != nil || !deploymentkube.PublicHTTPSOwned(spec, meta, item.UID) || !deploymentkube.PublicHTTPSMatches(object.Object, actual) {
			return row, false, ErrPublicHTTPS
		}
		ready, state := true, "ready"
		switch v := actual.(type) {
		case *corev1.PersistentVolumeClaim:
			ready = v.Status.Phase == corev1.ClaimBound && v.Spec.VolumeName != ""
			if row.RetireAttempted {
				state = "retained"
			}
		case *appsv1.Deployment:
			if row.DesiredReplicas == 0 {
				ready = v.Status.Replicas == 0 && v.Status.AvailableReplicas == 0
			} else {
				ready = v.Status.ObservedGeneration >= v.Generation && v.Status.AvailableReplicas == 1 && v.Status.Replicas == 1
			}
		case *corev1.Service:
			ready = false
			if len(v.Status.LoadBalancer.Ingress) == 1 {
				candidate := v.Status.LoadBalancer.Ingress[0]
				if publicHTTPSIPv4(candidate.IP) && candidate.Hostname == "" {
					if row.Address != "" && row.Address != candidate.IP {
						return row, false, ErrPublicHTTPS
					}
					row, e = s.publicHTTPSMutate(p, row.ID, row.Actor, func(r *storedPublicHTTPS) error {
						if r.Address != "" && r.Address != candidate.IP {
							return ErrPublicHTTPS
						}
						r.Address = candidate.IP
						return nil
					})
					if e != nil {
						return row, false, e
					}
					ready = true
				}
			}
		}
		if !ready {
			state = "pending"
			allReady = false
		}
		row, e = s.publicHTTPSMutate(p, row.ID, row.Actor, func(r *storedPublicHTTPS) error {
			v := publicHTTPSResource(r, object.Kind)
			if v == nil || v.UID != item.UID {
				return ErrPublicHTTPS
			}
			v.State = state
			return nil
		})
		if e != nil {
			return row, false, e
		}
		if deploymentkube.PublicHTTPSNamespace(ctx, target) != nil || s.publicHTTPSAuthority(ctx, row.Actor, false, 0) != nil {
			return row, false, ErrPublicHTTPS
		}
	}
	return row, allReady, nil
}
func (s *Server) scalePublicHTTPS(ctx context.Context, p project.Project, row storedPublicHTTPS, actionKey string) error {
	if e := s.publicHTTPSGuard(ctx, row, row.DesiredReplicas == 1, false); e != nil {
		return e
	}
	target, e := s.publicHTTPSTarget(ctx, row)
	if e != nil {
		return e
	}
	item := publicHTTPSResource(&row, "Deployment")
	if item == nil || item.UID == "" {
		return ErrPublicHTTPS
	}
	var actual appsv1.Deployment
	if e = s.publicHTTPSRPC(ctx, target, "GET", "Deployment", item.Name, nil, &actual); e != nil || !deploymentkube.PublicHTTPSOwned(publicHTTPSSpec(row), &actual, item.UID) {
		return ErrPublicHTTPS
	}
	// The original UID+RV precondition makes a late older scale request unable to
	// override a newer accepted state. The marker binds reconciliation to this key.
	marker := reviewDigest(struct {
		ID, Key  string
		Replicas int32
	}{row.ID, actionKey, row.DesiredReplicas})
	patch := map[string]any{"metadata": map[string]any{"uid": item.UID, "resourceVersion": actual.ResourceVersion, "annotations": map[string]string{"luci.ai/https-action": marker}}, "spec": map[string]any{"replicas": row.DesiredReplicas}}
	if e = s.publicHTTPSGuard(ctx, row, row.DesiredReplicas == 1, false); e != nil {
		return e
	}
	target, e = s.publicHTTPSMutationTarget(ctx, row)
	if e != nil {
		return e
	}
	var acknowledged appsv1.Deployment
	if e = s.publicHTTPSRPC(ctx, target, "PATCH", "Deployment", item.Name, patch, &acknowledged); e != nil {
		return e
	}
	if string(acknowledged.UID) != item.UID || acknowledged.Spec.Replicas == nil || *acknowledged.Spec.Replicas != row.DesiredReplicas || acknowledged.Annotations["luci.ai/https-action"] != marker {
		return ErrPublicHTTPS
	}
	_, e = s.publicHTTPSMutate(p, row.ID, row.Actor, func(r *storedPublicHTTPS) error {
		if r.LastActionRequestKey != actionKey {
			return ErrPublicHTTPS
		}
		r.ScaleVersion = acknowledged.ResourceVersion
		return nil
	})
	return e
}
func (s *Server) retirePublicHTTPS(ctx context.Context, p project.Project, row storedPublicHTTPS) error {
	// Stop/remove only known original network/controller/config/credential UIDs.
	// Certificate/account storage is deliberately retained, never deleted here.
	for _, kind := range []string{"Deployment", "Service", "ConfigMap", "Secret"} {
		item := publicHTTPSResource(&row, kind)
		if item == nil || !item.Attempted {
			continue
		}
		if item.UID == "" {
			continue
		}
		if item.DeleteAttempted {
			continue
		}
		if e := s.publicHTTPSGuard(ctx, row, false, false); e != nil {
			return e
		}
		target, e := s.publicHTTPSTarget(ctx, row)
		if e != nil {
			return e
		}
		actual, meta, e := publicHTTPSActual(kind)
		if e != nil {
			return e
		}
		if e = s.publicHTTPSRPC(ctx, target, "GET", kind, item.Name, nil, actual); e != nil || !deploymentkube.PublicHTTPSOwned(publicHTTPSSpec(row), meta, item.UID) {
			return ErrPublicHTTPS
		}
		row, e = s.publicHTTPSMutate(p, row.ID, row.Actor, func(r *storedPublicHTTPS) error {
			v := publicHTTPSResource(r, kind)
			if v == nil || v.UID != item.UID || v.DeleteAttempted {
				return ErrPublicHTTPS
			}
			v.DeleteAttempted = true
			return nil
		})
		if e != nil {
			return e
		}
		if e = s.publicHTTPSGuard(ctx, row, false, false); e != nil {
			return e
		}
		target, e = s.publicHTTPSMutationTarget(ctx, row)
		if e != nil {
			return e
		}
		uid := meta.GetUID()
		policy := metav1.DeletePropagationForeground
		options := metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid}, PropagationPolicy: &policy}
		if e = s.publicHTTPSRPC(ctx, target, "DELETE", kind, item.Name, options, nil); e != nil {
			return e
		}
	}
	if row.DNSAcknowledged && !row.DNSDeleteAttempted {
		var record publicHTTPSDNSRecord
		if s.publicHTTPSARMRead(ctx, row, "A", &record) != nil || !publicHTTPSDNSMatches(row, record) {
			return ErrPublicHTTPS
		}
		var e error
		row, e = s.publicHTTPSMutate(p, row.ID, row.Actor, func(r *storedPublicHTTPS) error {
			if r.DNSDeleteAttempted || r.DNSRecordETag != record.ETag {
				return ErrPublicHTTPS
			}
			r.DNSDeleteAttempted = true
			return nil
		})
		if e != nil {
			return e
		}
		if e = s.publicHTTPSDNSMutation(ctx, row, true); e != nil {
			return e
		}
	}
	return nil
}
func publicHTTPSDelay(row *storedPublicHTTPS, slow bool) {
	interval := 30 * time.Second
	if slow {
		interval = 15 * time.Minute
	}
	row.NextObserveAt = time.Now().UTC().Add(interval).Format(time.RFC3339Nano)
}
