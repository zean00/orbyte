package modules

import "testing"

func TestForProfileRejectsUnknownProfile(t *testing.T) {
	if _, err := ForProfile("unknown"); err == nil {
		t.Fatal("expected unknown profile error")
	}
}

func TestForProfileReturnsKnownProfiles(t *testing.T) {
	for _, profile := range []string{"", ProfileAll} {
		manifests, err := ForProfile(profile)
		if err != nil {
			t.Fatalf("profile %q failed: %v", profile, err)
		}
		if manifests == nil {
			t.Fatalf("profile %q returned nil manifests", profile)
		}
	}
}

func TestForProfileLoadsClinicAndRejectsUnconfiguredOMS(t *testing.T) {
	manifests, err := ForProfile(ProfileClinic)
	if err != nil {
		t.Fatalf("expected clinic profile to load: %v", err)
	}
	if len(manifests) == 0 {
		t.Fatal("expected clinic profile manifests")
	}
	if _, err := ForProfile(ProfileOMS); err == nil {
		t.Fatal("expected unconfigured oms profile to fail")
	}
}
