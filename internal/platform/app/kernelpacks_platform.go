package app

import (
	"time"

	"orbyte/internal/platform/config"
	"orbyte/internal/platform/module"
)

func referenceMasterdataKernelPackManifests() []module.Manifest {
	seededAt := time.Now().UTC()
	return []module.Manifest{
		referenceMasterdataKernelPackManifest(seededAt),
	}
}

func masterdataKernelPackManifests() []module.Manifest {
	return []module.Manifest{masterdataKernelPackManifest()}
}

func organizationStructureKernelPackManifests() []module.Manifest {
	return []module.Manifest{organizationStructureKernelPackManifest()}
}

func platformCoreKernelPackManifests() []module.Manifest {
	httpDefinition, _ := config.NewService().Definition("platform.http")
	acpDefinition, _ := config.NewService().Definition("platform.acp")
	mcpDefinition, _ := config.NewService().Definition("platform.mcp")
	return []module.Manifest{platformCoreKernelPackManifest(httpDefinition, acpDefinition, mcpDefinition)}
}

func identityKernelPackManifests() []module.Manifest {
	authDefinition, _ := config.NewService().Definition("identity.auth")
	return []module.Manifest{identityKernelPackManifest(authDefinition)}
}

func documentsKernelPackManifests() []module.Manifest {
	return []module.Manifest{documentsKernelPackManifest()}
}

func analyticsKernelPackManifests() []module.Manifest {
	return []module.Manifest{analyticsKernelPackManifest()}
}

func monitoringKernelPackManifests() []module.Manifest {
	return []module.Manifest{monitoringKernelPackManifest()}
}

func integrationKernelPackManifests() []module.Manifest {
	return []module.Manifest{integrationKernelPackManifest()}
}

func commercialCoreKernelPackManifests() []module.Manifest {
	return []module.Manifest{commercialCoreKernelPackManifest()}
}

func procurementCoreKernelPackManifests() []module.Manifest {
	return []module.Manifest{procurementCoreKernelPackManifest()}
}

func inventoryCoreKernelPackManifests() []module.Manifest {
	return []module.Manifest{inventoryCoreKernelPackManifest()}
}

func planningCoreKernelPackManifests() []module.Manifest {
	return []module.Manifest{planningCoreKernelPackManifest()}
}
