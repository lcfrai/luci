package deploymentplan

import (
	"fmt"
	kubevalidation "k8s.io/apimachinery/pkg/util/validation"
	"sort"
)

func validateImagePullSecrets(service *Service) error {
	if len(service.ImagePullSecrets) > 8 {
		return fmt.Errorf("choose at most eight explicit existing image pull Secrets")
	}
	seen := map[string]bool{}
	for _, name := range service.ImagePullSecrets {
		if len(kubevalidation.IsDNS1123Subdomain(name)) != 0 || seen[name] {
			return fmt.Errorf("image pull Secrets need unique exact names")
		}
		seen[name] = true
	}
	sort.Strings(service.ImagePullSecrets)
	return nil
}

// Only selected key metadata is captured, never registry credential values.
func ServiceSecretKeys(service Service) map[string][]string {
	out := TelemetrySecretKeys(service)
	for _, name := range service.ImagePullSecrets {
		out[name] = uniqueSorted(append(out[name], ".dockerconfigjson"))
	}
	return out
}
