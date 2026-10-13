package e2bbe

import (
	"crypto/tls"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func azureDeploymentFixture() Deployment {
	return Deployment{
		Provider: "azure", Region: "eastus", InstanceType: "Standard_D4ds_v7", Scope: "scope-one",
		HostGeneration: "boot-one", RuntimeRevision: RuntimeRevision,
		ManagementReceiptDigest: strings.Repeat("a", 64), RestoreReceiptDigest: strings.Repeat("b", 64),
		AzureSubscriptionID: "12345678-1234-1234-1234-123456789abc", AzureResourceGroup: "luci-runtime",
		AzureScaleSetID: "/subscriptions/12345678-1234-1234-1234-123456789abc/resourcegroups/luci-runtime/providers/microsoft.compute/virtualmachinescalesets/embed-host",
		AzureInstanceID: "0", AzureDataDiskID: "/subscriptions/12345678-1234-1234-1234-123456789abc/resourcegroups/luci-state/providers/microsoft.compute/disks/controller-data",
		AzureZone: "1", AzureGatewayHost: "embed.private.example",
	}
}

func TestAzureArtifactStoreRefusesChangedDataDiskIdentity(t *testing.T) {
	d := azureDeploymentFixture()
	sc := storeConfig(t)
	sc.AzureDataDiskID = d.AzureDataDiskID
	s, err := NewDirectoryStore(sc)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	sc.AzureDataDiskID = strings.Replace(d.AzureDataDiskID, "controller-data", "different-disk", 1)
	if _, err = NewDirectoryStore(sc); !errors.Is(err, ErrConflict) {
		t.Fatal("changed Azure data disk reopened the original store", err)
	}
	sc.Directory = filepath.Join(t.TempDir(), "new-store")
	sc.AzureDataDiskID = "invalid"
	if _, err = NewDirectoryStore(sc); !errors.Is(err, ErrUnsupported) {
		t.Fatal("malformed Azure data disk was accepted", err)
	}
}

func TestAzureProviderMetadataBindsDeploymentWithoutExposingARMIDs(t *testing.T) {
	d := azureDeploymentFixture()
	r := Record{Request: taskRequest("alpha", "one"), Deployment: d, RequestDigest: strings.Repeat("c", 64)}
	m := metadata(r)
	if m["luci.deployment"] != digest(d) {
		t.Fatal("Azure deployment was not bound to provider readback")
	}
	for _, value := range m {
		if strings.Contains(value, d.AzureSubscriptionID) || strings.Contains(value, d.AzureDataDiskID) {
			t.Fatal("private Azure resource ID entered guest-visible metadata")
		}
	}
	r.Deployment.AzureInstanceID = "1"
	if metadata(r)["luci.deployment"] == m["luci.deployment"] {
		t.Fatal("different Azure host identity reused the same metadata digest")
	}
}

func TestAzureDeploymentIdentityIsExactAndSeparateFromAWS(t *testing.T) {
	const endpoint = "https://embed.private.example"
	d := azureDeploymentFixture()
	if !validDeployment(Config{Deployment: d, Endpoint: endpoint, ClientCertificate: &tls.Certificate{}}) {
		t.Fatal("configured Azure identity was refused")
	}
	if validDeployment(Config{Deployment: d, Endpoint: endpoint}) {
		t.Fatal("Azure gateway accepted no client identity")
	}
	for name, change := range map[string]func(*Deployment){
		"subscription": func(d *Deployment) { d.AzureSubscriptionID = "other" },
		"scale set":    func(d *Deployment) { d.AzureScaleSetID += "/virtualmachines/0" },
		"disk": func(d *Deployment) {
			d.AzureDataDiskID = "/subscriptions/other/resourcegroups/luci-state/providers/microsoft.compute/disks/controller-data"
		},
		"zone":    func(d *Deployment) { d.AzureZone = "0" },
		"sku":     func(d *Deployment) { d.InstanceType = "m8i.xlarge" },
		"gateway": func(d *Deployment) { d.AzureGatewayHost = "public.example" },
		"receipt": func(d *Deployment) { d.RestoreReceiptDigest = "" },
	} {
		t.Run(name, func(t *testing.T) {
			changed := d
			change(&changed)
			if validDeployment(Config{Deployment: changed, Endpoint: endpoint, ClientCertificate: &tls.Certificate{}}) {
				t.Fatal("changed Azure boundary was accepted")
			}
		})
	}
	aws := Deployment{Provider: "aws", Region: "us-east-1", InstanceType: "m8i.xlarge", Scope: "scope-one", HostGeneration: "host-one", RuntimeRevision: RuntimeRevision, ManagementReceiptDigest: strings.Repeat("a", 64), RestoreReceiptDigest: strings.Repeat("b", 64)}
	if !validDeployment(Config{Deployment: aws}) {
		t.Fatal("existing AWS identity was refused")
	}
	aws.AzureZone = "1"
	if validDeployment(Config{Deployment: aws}) {
		t.Fatal("AWS accepted Azure-only fields")
	}
}
