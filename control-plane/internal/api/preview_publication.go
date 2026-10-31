package api

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/distribution/reference"
	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/previewruntime"
)

func validRuntimePublicationRepository(v string) bool {
	if len(v) < 3 || len(v) > 240 || strings.ContainsAny(v, "@\\\x00\r\n") {
		return false
	}
	n, err := reference.ParseNormalizedNamed(v)
	return err == nil && n.String() == v && reference.IsNameOnly(n) && strings.Contains(v, "/") && reference.Domain(n) != "localhost" && (strings.Contains(reference.Domain(n), ".") || strings.Contains(reference.Domain(n), ":"))
}
func (a *API) publicationAuthority(ctx context.Context, req previewruntime.Request) error {
	if req.Actor.Local || req.Actor.UserID == "" || req.ValidateSource == nil {
		return management.ErrExecutionAuthority
	}
	authority, err := a.authorizePreview(ctx, req.Actor, true)
	if err != nil {
		return management.ErrExecutionAuthority
	}
	if req.Actor.TeamID != "" && (authority.Team == nil || !authority.Team.Permissions.Secrets) {
		return management.ErrExecutionAuthority
	}
	fresh := management.WithExecutionAuthority(ctx, authority)
	if actor, err := management.CaptureDeploymentActor(fresh, req.ProjectID); err != nil || actor != req.Actor {
		return management.ErrExecutionAuthority
	}
	return req.ValidateSource(ctx)
}

func (a *API) buildAndPublishPreview(ctx context.Context, req previewruntime.Request, p previewruntime.Publication, publish func(previewruntime.Outcome) error) (previewruntime.Outcome, error) {
	if req.Kind != "" || !validRuntimePublicationRepository(p.Repository) || !checkHex(p.RegistryBinding, 32) || len(p.RegistryAuth) > 32<<10 || strings.ContainsAny(p.RegistryAuth, "\x00\r\n") {
		return previewruntime.Outcome{Identity: req.Identity, Phase: "blocked"}, errors.New("invalid reviewed image publication")
	}
	if _, ok := a.deps.Backend.(backend.ImagePublisher); !ok {
		return previewruntime.Outcome{Identity: req.Identity, Phase: "blocked"}, backend.ErrUnsupported
	}
	if err := a.publicationAuthority(ctx, req); err != nil {
		return previewruntime.Outcome{Identity: req.Identity, Phase: "blocked"}, err
	}
	// Reuse every existing prepared-context/base-image/admission/build gate. No
	// serving container is allocated by this internal entry point.
	return a.startPreviewPipeline(ctx, req, publish, &p)
}

func (g *executionAdmission) bindPreviewPublication(ctx context.Context, before previewExecutionLease, repository, binding string) (previewExecutionLease, error) {
	var out previewExecutionLease
	if !validRuntimePublicationRepository(repository) || !checkHex(binding, 32) {
		return out, errors.New("invalid registry descriptor")
	}
	err := g.mutate(ctx, func(l *executionLeaseLedger) error {
		for i, v := range l.Previews {
			if v.Scope != g.scope || v.ID != before.ID {
				continue
			}
			if v != before || v.Kind != "preview-build" || v.State != "reserved" || v.PublicationRepository != "" || v.PublicationBinding != "" {
				return errors.New("original publication reservation changed")
			}
			v.PublicationRepository, v.PublicationBinding = repository, binding
			v.Revision++
			v.UpdatedAt = time.Now().UTC()
			if !validPreviewExecutionLease(v) {
				return errors.New("invalid publication reservation")
			}
			l.Previews[i], out = v, v
			return nil
		}
		return errPreviewLeaseAbsent
	})
	return out, err
}

func (a *API) publicationMutation(ctx context.Context, req previewruntime.Request, p previewruntime.Publication, before previewExecutionLease) error {
	if before.Kind != "preview-build" || before.State != "publishing" || before.Actor != req.Actor || before.PublicationRepository != p.Repository || before.PublicationBinding != p.RegistryBinding || before.Identity.ProjectID != req.ProjectID || before.Identity.ReleaseID != req.ReleaseID || before.Identity.Commit != req.Commit || before.Identity.SourceDigest != req.SourceDigest {
		return management.ErrExecutionAuthority
	}
	if err := a.publicationAuthority(ctx, req); err != nil {
		return err
	}
	authority, err := a.authorizePreview(ctx, req.Actor, true)
	if err != nil {
		return err
	}
	if before.Resources.MemoryMiB > authority.Policy.CheckMemoryMiB || before.Resources.CPUMillis > authority.Policy.CheckCPUMillis || authority.Policy.MaxConcurrentChecksPerUser < 1 {
		return management.ErrExecutionAuthority
	}
	current, err := a.execution.previewLease(ctx, before.ID)
	if err != nil {
		return err
	}
	if current != before {
		return errors.New("original publication phase changed")
	}
	return ctx.Err()
}

func (a *API) publishBuiltPreview(ctx context.Context, req previewruntime.Request, p previewruntime.Publication, lease previewExecutionLease, result backend.PreviewBuildResult, publish func(previewruntime.Outcome) error, out previewruntime.Outcome) (previewruntime.Outcome, error) {
	var err error
	lease, err = a.execution.transitionPreview(ctx, lease, "built", "", result.ImageID)
	if err != nil {
		return out, err
	}
	out.Image, out.Phase, out.ResourcesHeld, out.CleanupConfirmed = result.ImageID, "built", true, false
	out.Message = "The source image is built; its original capacity remains reserved through registry publication."
	if err = publish(out); err != nil {
		return out, err
	}
	if err = a.publicationAuthority(ctx, req); err != nil {
		return out, err
	}
	// Persist before the first tag/push. A resumed controller only observes this
	// original receipt; it must never call BuildAndPublish with a retained ID.
	lease, err = a.execution.transitionPreview(ctx, lease, "publishing", "", result.ImageID)
	if err != nil {
		return out, err
	}
	out.Phase, out.Message = "publishing", "Publishing the exact reviewed image to the selected registry repository."
	if err = publish(out); err != nil {
		return out, err
	}
	target := backend.ImagePublication{Identity: lease.Identity, ImageID: result.ImageID, Repository: p.Repository, Tag: "luci-" + lease.ID, RegistryAuth: p.RegistryAuth}
	got, pushErr := a.deps.Backend.(backend.ImagePublisher).PublishPreviewImage(ctx, target, func(call context.Context) error { return a.publicationMutation(call, req, p, lease) })
	if !got.Settled {
		out.Phase, out.ReasonCode, out.Message = "unknown", "image_publication_unconfirmed", "The original registry publication is unconfirmed. Keep its identity and reservation; do not repeat the push."
		if err = publish(out); err != nil {
			return out, err
		}
		if pushErr == nil {
			pushErr = backend.ErrUnavailable
		}
		return out, pushErr
	}
	good := pushErr == nil && strings.HasPrefix(got.Digest, "sha256:") && checkHex(strings.TrimPrefix(got.Digest, "sha256:"), 32) && got.Reference == p.Repository+"@"+got.Digest
	digest := ""
	if good {
		digest = got.Digest
	}
	record, cancel := previewRecordContext(ctx)
	_, err = a.execution.finishPreviewPublication(record, lease, digest)
	cancel()
	if err != nil {
		return out, err
	}
	out.ResourcesHeld, out.CleanupConfirmed = false, true
	if good {
		out.ImmutableImage, out.Phase, out.Message = got.Reference, "published", "The original image push completed and its immutable registry digest was read back. Application readiness is separate."
	} else {
		out.Phase, out.ReasonCode, out.Message = "failed", "image_publication_failed", "The original push ended without a confirmed usable registry digest. No retry was started."
		if pushErr == nil {
			pushErr = backend.ErrUnavailable
		}
	}
	// Settle original receipts even if access disappeared after the RPC, but do
	// not publish a successful reference to a caller without current authority.
	if authErr := a.publicationAuthority(ctx, req); authErr != nil {
		return out, authErr
	}
	if err = publish(out); err != nil {
		return out, err
	}
	return out, pushErr
}

func (g *executionAdmission) finishPreviewPublication(ctx context.Context, before previewExecutionLease, digest string) (previewExecutionLease, error) {
	var out previewExecutionLease
	if digest != "" && (!strings.HasPrefix(digest, "sha256:") || !checkHex(strings.TrimPrefix(digest, "sha256:"), 32)) {
		return out, errors.New("invalid immutable image digest")
	}
	err := g.mutate(ctx, func(l *executionLeaseLedger) error {
		for i, v := range l.Previews {
			if v.Scope != g.scope || v.ID != before.ID {
				continue
			}
			if v != before || v.Kind != "preview-build" || v.State != "publishing" || v.Image == "" || v.PublicationBinding == "" {
				return errors.New("original publication settlement changed")
			}
			v.State, v.PublicationDigest, v.UpdatedAt = "finished", digest, time.Now().UTC()
			v.Revision++
			if !validPreviewExecutionLease(v) {
				return errors.New("invalid publication settlement")
			}
			l.Previews[i], out = v, v
			return nil
		}
		return errPreviewLeaseAbsent
	})
	return out, err
}
