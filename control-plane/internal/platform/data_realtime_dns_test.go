package platform

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDataRealtimeUsesDNSValidAliasOnEachPrivateStackNetwork(t *testing.T) {
	s, _ := newDataFake(t)
	var networks []string
	for _, stackID := range []string{"stack-first-dns", "stack-second-dns"} {
		install := dataInstallation{ProjectID: "owned", StackID: stackID, APIPort: 54321}
		if err := s.prepareDataStack(install, newDataSecret()); err != nil {
			t.Fatal(err)
		}
		root, err := s.dataRoot(install)
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(root, "compose.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		var compose struct {
			Services map[string]struct {
				Container string `yaml:"container_name"`
				Labels    map[string]string
				Networks  map[string]struct{ Aliases []string }
			}
			Networks map[string]struct {
				Name   string
				Labels map[string]string
			}
		}
		if err := yaml.Unmarshal(data, &compose); err != nil {
			t.Fatal(err)
		}
		realtime := compose.Services["realtime"]
		aliases := realtime.Networks["default"].Aliases
		if len(realtime.Networks) != 1 || len(aliases) != 1 || aliases[0] != "realtime-dev.realtime" {
			t.Fatal("Realtime did not receive its short private tenant alias")
		}
		if realtime.Container != s.dataName(install, "realtime") || realtime.Labels[labelProject] != "owned" || realtime.Labels[labelResource] != "data-"+stackID+"-realtime" {
			t.Fatal("DNS fix changed the owned container identity")
		}
		network := compose.Networks["default"]
		if network.Labels[labelResource] != "data-"+stackID || network.Name != s.resource("owned", stackID+"-net") {
			t.Fatal("Realtime alias is not scoped to the exact owned stack network")
		}
		networks = append(networks, network.Name)
		cds, err := os.ReadFile(filepath.Join(root, "volumes", "api", "envoy", "cds.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		var envoy struct {
			Resources []struct {
				Name       string
				Assignment struct {
					Endpoints []struct {
						Targets []struct {
							Endpoint struct {
								Address struct {
									Socket struct{ Address string } `yaml:"socket_address"`
								} `yaml:"address"`
							} `yaml:"endpoint"`
						} `yaml:"lb_endpoints"`
					} `yaml:"endpoints"`
				} `yaml:"load_assignment"`
			} `yaml:"resources"`
		}
		if err := yaml.Unmarshal(cds, &envoy); err != nil {
			t.Fatal(err)
		}
		found := false
		for _, cluster := range envoy.Resources {
			if cluster.Name != "realtime" {
				continue
			}
			found = true
			if len(cluster.Assignment.Endpoints) != 1 || len(cluster.Assignment.Endpoints[0].Targets) != 1 || cluster.Assignment.Endpoints[0].Targets[0].Endpoint.Address.Socket.Address != aliases[0] {
				t.Fatal("Envoy's actual generated Realtime cluster does not use the private alias")
			}
		}
		if !found {
			t.Fatal("generated Envoy Realtime cluster is missing")
		}
		for _, label := range strings.Split(aliases[0], ".") {
			if len(label) == 0 || len(label) > 63 {
				t.Fatal("Realtime cluster contains an invalid DNS label")
			}
		}
	}
	if networks[0] == networks[1] {
		t.Fatal("two stacks share a network for their common Realtime alias")
	}
}
