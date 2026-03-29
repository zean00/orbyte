package app

import (
	"fmt"

	"orbyte/internal/platform/module"
)

type modulePack interface {
	Manifests() []module.Manifest
}

type staticModulePack struct {
	manifests func() []module.Manifest
}

func (p staticModulePack) Manifests() []module.Manifest {
	if p.manifests == nil {
		return nil
	}
	return append([]module.Manifest(nil), p.manifests()...)
}

func builtInModulePacks() []modulePack {
	return []modulePack{
		staticModulePack{manifests: referenceMasterdataKernelPackManifests},
		staticModulePack{manifests: masterdataKernelPackManifests},
		staticModulePack{manifests: platformCoreKernelPackManifests},
		staticModulePack{manifests: identityKernelPackManifests},
		staticModulePack{manifests: documentsKernelPackManifests},
		staticModulePack{manifests: commercialCoreKernelPackManifests},
		staticModulePack{manifests: procurementCoreKernelPackManifests},
		staticModulePack{manifests: inventoryCoreKernelPackManifests},
		staticModulePack{manifests: fulfillmentCoreKernelPackManifests},
		staticModulePack{manifests: deliveryCoreKernelPackManifests},
		staticModulePack{manifests: returnsCoreKernelPackManifests},
		staticModulePack{manifests: supplierReturnsCoreKernelPackManifests},
		staticModulePack{manifests: planningCoreKernelPackManifests},
		staticModulePack{manifests: productionCoreKernelPackManifests},
		staticModulePack{manifests: traceabilityCoreKernelPackManifests},
		staticModulePack{manifests: recallCoreKernelPackManifests},
		staticModulePack{manifests: analyticsKernelPackManifests},
		staticModulePack{manifests: monitoringKernelPackManifests},
		staticModulePack{manifests: integrationKernelPackManifests},
	}
}

func builtInModuleManifests() []module.Manifest {
	return collectModuleManifests(builtInModulePacks())
}

func collectModuleManifests(packs []modulePack) []module.Manifest {
	var manifests []module.Manifest
	seen := make(map[string]struct{})
	for _, pack := range packs {
		for _, manifest := range pack.Manifests() {
			if manifest.Key == "" {
				panic("built-in module pack contains manifest with empty key")
			}
			if _, exists := seen[manifest.Key]; exists {
				panic(fmt.Sprintf("built-in module pack contains duplicate manifest key %q", manifest.Key))
			}
			seen[manifest.Key] = struct{}{}
			manifests = append(manifests, manifest)
		}
	}
	return manifests
}
