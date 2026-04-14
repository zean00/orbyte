export const pageModuleLoaders = {
  login: () => import("@/features/auth/LoginPage"),
  workspace: () => import("@/features/workspace/WorkspacePage"),
  agent: () => import("@/features/agent/AgentSurfacePage"),
  dashboard: () => import("@/features/dashboard/DashboardSurfacePage"),
  pos: () => import("@/features/pos/POSSurfacePage"),
} as const;

export function preloadSurfaceModule(surface?: string): Promise<unknown> {
  switch (surface) {
    case "agent":
      return pageModuleLoaders.agent();
    case "dashboard":
      return pageModuleLoaders.dashboard();
    case "pos":
      return pageModuleLoaders.pos();
    case "backoffice":
    case "worklist":
    case "self_service":
    default:
      return pageModuleLoaders.workspace();
  }
}

export function preloadVisibleSurfaceModules(
  surfaces: string[],
): Promise<unknown[]> {
  const seen = new Set<string>();
  const jobs: Promise<unknown>[] = [];
  for (const surface of surfaces) {
    const key =
      surface === "agent" || surface === "dashboard" || surface === "pos"
        ? surface
        : "workspace";
    if (seen.has(key)) continue;
    seen.add(key);
    jobs.push(preloadSurfaceModule(surface));
  }
  return Promise.all(jobs);
}
