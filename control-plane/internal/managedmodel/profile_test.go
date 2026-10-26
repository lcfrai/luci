package managedmodel

import "testing"

func TestManagedExecutionProfilesAreExplicitAndKeepResourceLimits(t *testing.T) {
	for _, tc := range []struct {
		mode, canonical, cache string
		flash                  bool
	}{
		{"", "gpu", "q8_0", true},
		{"gpu", "gpu", "q8_0", true},
		{"cpu", "cpu", "f16", false},
	} {
		t.Run(tc.canonical+tc.mode, func(t *testing.T) {
			mode, err := NormalizeExecutionMode(tc.mode)
			profile, profileErr := ProfileForExecutionMode(tc.mode)
			if err != nil || profileErr != nil || mode != tc.canonical || profile != (Profile{16384, tc.cache, tc.flash, 1, 7168, 6656, 20}) {
				t.Fatal("mode changed profile or resource contract", mode, profile, err, profileErr)
			}
			if mode == "gpu" && profile != FixedProfile() {
				t.Fatal("legacy GPU profile changed")
			}
		})
	}
	for _, mode := range []string{"auto", "CPU", " cpu ", "gpu,cpu"} {
		if _, err := NormalizeExecutionMode(mode); err == nil {
			t.Fatalf("unknown mode %q became an execution profile", mode)
		}
		if _, err := ProfileForExecutionMode(mode); err == nil {
			t.Fatalf("unknown mode %q received a profile", mode)
		}
	}
}
