package modules

import "testing"

func TestKnownProfiles(t *testing.T) {
	items := KnownProfiles()
	if len(items) != 3 {
		t.Fatalf("expected 3 profiles, got %+v", items)
	}
	if items[0] != ProfileAll || items[1] != ProfileClinic || items[2] != ProfileOMS {
		t.Fatalf("unexpected profiles: %+v", items)
	}
}

func TestProfileHelpers(t *testing.T) {
	all, err := ForProfile(ProfileAll)
	if err != nil || len(all) == 0 {
		t.Fatalf("expected all profile manifests, got len=%d err=%v", len(all), err)
	}
	clinic, err := ForProfile(ProfileClinic)
	if err != nil || len(clinic) == 0 {
		t.Fatalf("expected clinic profile manifests, got len=%d err=%v", len(clinic), err)
	}
	if _, err := ForProfile(ProfileOMS); err == nil {
		t.Fatal("expected oms profile to be unconfigured")
	}
	if _, err := ForProfile("unknown"); err == nil {
		t.Fatal("expected unknown profile error")
	}

	cloned := cloneManifests(clinic)
	cloned[0].Key = "changed"
	if clinicManifests()[0].Key == "changed" {
		t.Fatal("expected cloneManifests to copy slice values")
	}
	if len(allManifests()) == 0 || len(omsManifests()) != 0 {
		t.Fatal("expected manifest helper behavior")
	}
	if _, err := requireConfiguredProfile("empty", nil); err == nil {
		t.Fatal("expected requireConfiguredProfile to fail on empty profile")
	}
}
