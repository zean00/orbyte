package modules

import (
	"fmt"

	platformmodule "clinic/internal/platform/module"
	// modulegen:imports
)

const (
	ProfileAll    = "all"
	ProfileClinic = "clinic"
	ProfileOMS    = "oms"
)

func KnownProfiles() []string {
	return []string{ProfileAll, ProfileClinic, ProfileOMS}
}

func ForProfile(profile string) ([]platformmodule.Manifest, error) {
	switch profile {
	case "", ProfileAll:
		return cloneManifests(allManifests()), nil
	case ProfileClinic:
		return requireConfiguredProfile(ProfileClinic, clinicManifests())
	case ProfileOMS:
		return requireConfiguredProfile(ProfileOMS, omsManifests())
	default:
		return nil, fmt.Errorf("unknown domain profile %q", profile)
	}
}

func allManifests() []platformmodule.Manifest {
	items := []platformmodule.Manifest{
		// modulegen:manifests
	}
	return cloneManifests(items)
}

func clinicManifests() []platformmodule.Manifest {
	return []platformmodule.Manifest{}
}

func omsManifests() []platformmodule.Manifest {
	return []platformmodule.Manifest{}
}

func cloneManifests(manifests []platformmodule.Manifest) []platformmodule.Manifest {
	return append([]platformmodule.Manifest{}, manifests...)
}

func requireConfiguredProfile(profile string, manifests []platformmodule.Manifest) ([]platformmodule.Manifest, error) {
	items := cloneManifests(manifests)
	if len(items) == 0 {
		return nil, fmt.Errorf("domain profile %q is not configured yet", profile)
	}
	return items, nil
}
