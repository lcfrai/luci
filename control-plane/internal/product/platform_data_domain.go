package product

import (
	"context"
	"errors"

	"github.com/lcfrai/luci/control-plane/internal/platform"
)

// PlatformDataDomains connects native resource ownership to provider ownership.
// The resolver closes over construction; browsers cannot replace these checks.
func (s *Server) PlatformDataDomains(resolve func() *platform.Service) *platform.DataDomainProvider {
	prerequisites := func(id string, host platform.DataDomainHost) (PublicDataDomainPrerequisites, error) {
		if resolve == nil || resolve() == nil || host.ProjectID != id {
			return PublicDataDomainPrerequisites{}, errors.New("the native data integration is unavailable")
		}
		service := resolve()
		return PublicDataDomainPrerequisites{
			Data: PublicDataHost{ProjectID: host.ProjectID, StableTenant: host.StableTenant, ApplicationNetwork: host.ApplicationNetwork, NetworkID: host.NetworkID, DockerEngineID: host.DockerEngineID, PublishableKey: host.PublishableKey},
			VerifyHost: func(ctx context.Context, server CoolifyServer) error {
				v := server.Settings
				if v == nil || v.Usable == nil || v.Reachable == nil || v.BuildOnly == nil || v.SwarmManager == nil || v.SwarmWorker == nil {
					return errors.New("the selected server's native Docker readiness is unavailable")
				}
				return service.VerifyDataDomainHost(ctx, id, host, platform.DataDomainServer{UUID: server.UUID, IP: server.IP, Usable: *v.Usable, Reachable: *v.Reachable, BuildServer: *v.BuildOnly, SwarmManager: *v.SwarmManager, SwarmWorker: *v.SwarmWorker})
			},
			VerifyHTTPS: func(ctx context.Context, receipt PublicDataDomainBinding) error {
				return service.VerifyDataDomainHTTPS(ctx, id, platformDomainReceipt(receipt))
			},
		}, nil
	}
	request := func(in platform.DataDomainRequest, host platform.DataDomainHost) (PublicDataDomainRequest, error) {
		if in.ProjectID != host.ProjectID || in.StableTenant != host.StableTenant {
			return PublicDataDomainRequest{}, errors.New("the native data request changed ownership")
		}
		target, err := s.deploymentTarget(in.TargetID)
		if err != nil {
			return PublicDataDomainRequest{}, errors.New("choose a connected deployment server")
		}
		binding, err := s.DataDomainBinding(in.ProjectID)
		if err != nil {
			return PublicDataDomainRequest{}, err
		}
		// Provider IDs and its independent revision come from trusted state.
		// The public counter protects native operation admission separately.
		return PublicDataDomainRequest{ProjectID: in.ProjectID, StableTenant: host.StableTenant, TargetID: in.TargetID, ServerUUID: target.ServerUUID, Domain: in.Domain, AppOrigin: in.AppOrigin, Environment: "production", AcknowledgePublic: in.AcknowledgePublic, ExpectedRevision: binding.Revision}, nil
	}
	return &platform.DataDomainProvider{
		Preflight: func(ctx context.Context, in platform.DataDomainRequest, host platform.DataDomainHost) error {
			mapped, err := request(in, host)
			if err != nil {
				return err
			}
			pre, err := prerequisites(in.ProjectID, host)
			if err != nil {
				return err
			}
			_, _, err = s.dataDomainPrerequisites(ctx, mapped, pre)
			return err
		},
		Publish: func(ctx context.Context, in platform.DataDomainRequest, host platform.DataDomainHost) (platform.DataDomainReceipt, error) {
			mapped, err := request(in, host)
			if err != nil {
				return platform.DataDomainReceipt{}, err
			}
			pre, err := prerequisites(in.ProjectID, host)
			if err != nil {
				return platform.DataDomainReceipt{}, err
			}
			binding, err := s.PublishDataDomain(ctx, mapped, pre)
			return platformDomainReceipt(binding), err
		},
		Refresh: func(ctx context.Context, id string, host platform.DataDomainHost) (platform.DataDomainReceipt, error) {
			pre, err := prerequisites(id, host)
			if err != nil {
				return platform.DataDomainReceipt{}, err
			}
			binding, err := s.RefreshDataDomain(ctx, id, pre)
			return platformDomainReceipt(binding), err
		},
		Stop: func(ctx context.Context, id string, host platform.DataDomainHost) (platform.DataDomainReceipt, error) {
			pre, err := prerequisites(id, host)
			if err != nil {
				return platform.DataDomainReceipt{}, err
			}
			binding, err := s.StopDataDomain(ctx, id, pre)
			return platformDomainReceipt(binding), err
		},
	}
}

func platformDomainReceipt(v PublicDataDomainBinding) platform.DataDomainReceipt {
	return platform.DataDomainReceipt{ProjectID: v.ProjectID, StableTenant: v.StableTenant, TargetID: v.TargetID, TargetFingerprint: v.TargetFingerprint, ServerUUID: v.ServerUUID, ProviderProjectUUID: v.ProviderProjectUUID, ProviderServiceUUID: v.ProviderServiceUUID, Domain: v.Domain, AppOrigin: v.AppOrigin, ApplicationNetwork: v.ApplicationNetwork, NetworkID: v.NetworkID, DockerEngineID: v.DockerEngineID, Status: v.Status, Message: v.Message, CheckedAt: v.CheckedAt, ConfigurationDigest: v.ConfigurationDigest, ProbePath: v.ProbePath, Revision: v.Revision}
}

func PlatformCloudDataBindings(resolve func() *platform.Service) func(context.Context, CloudDataBindingRequest) (CloudDataBindingSnapshot, error) {
	return func(ctx context.Context, req CloudDataBindingRequest) (CloudDataBindingSnapshot, error) {
		if resolve == nil || resolve() == nil {
			return CloudDataBindingSnapshot{}, errors.New("the native application data integration is unavailable")
		}
		service := resolve()
		v, err := service.CloudDataSnapshot(ctx, req.ProjectID)
		if err != nil {
			return CloudDataBindingSnapshot{}, err
		}
		if v.ProjectID != req.ProjectID {
			return CloudDataBindingSnapshot{}, errors.New("the native application data snapshot changed ownership")
		}
		return CloudDataBindingSnapshot{Installed: v.Installed, ProjectID: v.ProjectID, StableTenant: v.StableTenant, TargetID: v.TargetID, TargetFingerprint: v.TargetFingerprint, ServerUUID: v.ServerUUID, DomainStatus: v.DomainStatus, DomainRevision: v.DomainRevision, ConfigurationDigest: v.ConfigurationDigest, PublicURL: v.PublicURL, ApplicationOrigin: v.AppOrigin, PublishableKey: v.PublishableKey}, nil
	}
}
