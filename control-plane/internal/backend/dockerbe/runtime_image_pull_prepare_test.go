package dockerbe

import (
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/runtimeoci"
)

func TestPullAdmissionChargesAllRetainedCompressedAndFinalBytes(t *testing.T) {
	var m runtimeoci.PullManifest
	m.Image.Layers = []runtimeoci.PullLayer{
		{CompressedBytes: 600 << 20, UncompressedBytes: 1500 << 20},
		{CompressedBytes: 300 << 20, UncompressedBytes: 700 << 20},
	}
	m.Bootstrap.Bytes = 64 << 20
	m.Bootstrap.Layers = []runtimeoci.BootstrapLayer{{UncompressedBytes: 96 << 20}}
	main, boot, err := pullAdmissionMiB(m, 1024)
	if err != nil || main != 600+300+1500+700+256+512 || boot != 1024 {
		t.Fatalf("charge main=%d bootstrap=%d err=%v", main, boot, err)
	}
	m.Image.Layers[1].UncompressedBytes = runtimeoci.MaxPullImageBytes
	if _, _, err := pullAdmissionMiB(m, 1024); err == nil {
		t.Fatal("aggregate expanded image overflow accepted")
	}
	m.Image.Layers[1].UncompressedBytes = 700 << 20
	m.Image.Layers[1].CompressedBytes = runtimeoci.MaxPullStoreBytes
	if _, _, err := pullAdmissionMiB(m, 1024); err == nil {
		t.Fatal("aggregate compressed temp overflow accepted")
	}
}
