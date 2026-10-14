package main

import (
	"context"
	"errors"
	"testing"
)

type fakeAzureARM struct {
	resources map[string]azureARMResource
	reads     []string
	fail      string
}

func (f *fakeAzureARM) Read(_ context.Context, path, _ string, out *azureARMResource) error {
	f.reads = append(f.reads, path)
	if path == f.fail {
		return errors.New("fixture outage")
	}
	*out = f.resources[path]
	return nil
}

func azureARMFixture() (e2bOperatorConfig, *fakeAzureARM) {
	subscription := "12345678-1234-1234-1234-123456789abc"
	diskID := "/subscriptions/" + subscription + "/resourcegroups/luci/providers/microsoft.compute/disks/luci-data"
	controllerID := "/subscriptions/" + subscription + "/resourcegroups/luci/providers/microsoft.compute/virtualmachines/controller-one"
	runtimeID := "/subscriptions/" + subscription + "/resourcegroups/luci/providers/microsoft.compute/virtualmachinescalesets/runtime-one/virtualMachines/0"
	selected := e2bOperatorConfig{}
	selected.AzureMount = &azureDurableMount{ControllerInstanceID: controllerID}
	selected.Store.AzureDataDiskID = diskID
	selected.Deployment.AzureSubscriptionID = subscription
	selected.Deployment.AzureScaleSetID = "/subscriptions/" + subscription + "/resourcegroups/luci/providers/microsoft.compute/virtualmachinescalesets/runtime-one"
	selected.Deployment.AzureInstanceID = "0"
	disk := azureARMResource{ID: diskID, ManagedBy: controllerID}
	disk.Properties.DiskState = "Attached"
	one := 1
	disk.Properties.MaxShares = &one
	controller := azureARMResource{ID: controllerID}
	controller.Properties.ProvisioningState = "Succeeded"
	runtime := azureARMResource{ID: runtimeID}
	runtime.Properties.ProvisioningState = "Succeeded"
	return selected, &fakeAzureARM{resources: map[string]azureARMResource{diskID: disk, controllerID: controller, runtimeID: runtime}}
}

func TestAzureARMRequiresExactControllerDiskAndRuntimeHostReadback(t *testing.T) {
	selected, reader := azureARMFixture()
	if err := verifyAzureARMOwnership(context.Background(), selected, reader); err != nil {
		t.Fatal(err)
	}
	if len(reader.reads) != 3 || reader.reads[0] != selected.Store.AzureDataDiskID || reader.reads[1] != selected.AzureMount.ControllerInstanceID {
		t.Fatal("ARM readback did not bind the disk and controller first", reader.reads)
	}
	ordinary := reader.resources[selected.Store.AzureDataDiskID]
	ordinary.Properties.MaxShares = nil
	reader.resources[selected.Store.AzureDataDiskID] = ordinary
	if err := verifyAzureARMOwnership(context.Background(), selected, reader); err != nil {
		t.Fatal("nonshared disk with implicit maxShares default was refused", err)
	}
	for _, changed := range []func(*e2bOperatorConfig, *fakeAzureARM){
		func(s *e2bOperatorConfig, f *fakeAzureARM) { f.fail = s.Store.AzureDataDiskID },
		func(s *e2bOperatorConfig, f *fakeAzureARM) {
			r := f.resources[s.Store.AzureDataDiskID]
			r.ManagedBy += "-other"
			f.resources[s.Store.AzureDataDiskID] = r
		},
		func(s *e2bOperatorConfig, f *fakeAzureARM) {
			r := f.resources[s.Store.AzureDataDiskID]
			r.Properties.DiskState = "Unattached"
			f.resources[s.Store.AzureDataDiskID] = r
		},
		func(s *e2bOperatorConfig, f *fakeAzureARM) {
			r := f.resources[s.Store.AzureDataDiskID]
			r.ManagedByExtended = []string{s.AzureMount.ControllerInstanceID, "other-vm"}
			f.resources[s.Store.AzureDataDiskID] = r
		},
		func(s *e2bOperatorConfig, f *fakeAzureARM) {
			r := f.resources[s.Store.AzureDataDiskID]
			shared := 2
			r.Properties.MaxShares = &shared
			f.resources[s.Store.AzureDataDiskID] = r
		},
		func(s *e2bOperatorConfig, f *fakeAzureARM) {
			r := f.resources[s.AzureMount.ControllerInstanceID]
			r.Properties.ProvisioningState = "Failed"
			f.resources[s.AzureMount.ControllerInstanceID] = r
		},
		func(s *e2bOperatorConfig, f *fakeAzureARM) {
			f.fail = s.Deployment.AzureScaleSetID + "/virtualMachines/0"
		},
	} {
		bad, fake := azureARMFixture()
		changed(&bad, fake)
		if err := verifyAzureARMOwnership(context.Background(), bad, fake); err == nil {
			t.Fatal("unverified disk/controller/runtime was accepted")
		}
	}
}
