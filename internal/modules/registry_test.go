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

func TestForProfileRejectsUnconfiguredBusinessProfiles(t *testing.T) {
	for _, profile := range []string{ProfileClinic, ProfileOMS} {
		if _, err := ForProfile(profile); err == nil {
			t.Fatalf("expected unconfigured profile %q to fail", profile)
		}
	}
}
