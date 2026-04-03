import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import type {
  ActionDefinition,
  AdminBootstrapResponse,
  CustomEntryDefinition,
  DocumentFlowDefinition,
  ShellKind,
  ShellRoute,
  ViewDefinition,
  WorkspaceBootstrapResponse,
} from '@/services/bootstrap'
import { normalizeShellPath } from '@/services/bootstrap'

interface AuthContext {
  actor_user_id: string
  effective_user_id: string
  delegation_active: boolean
  delegation_grant_id?: string
}

interface ShellState {
  sidebarOpen: boolean
  mobileNavOpen: boolean
  shellKind: ShellKind
  currentRoute: string
  routes: ShellRoute[]
  currentSurface: string
  availableSurfaces: string[]
  adminAccess: boolean
  adminPath: string
  uiAccess: boolean
  uiPath: string
  locale: string
  supportedLocales: string[]
  authContext: AuthContext | null
  defaultPath: string
  actions: ActionDefinition[]
  views: ViewDefinition[]
  flows: DocumentFlowDefinition[]
  customEntries: CustomEntryDefinition[]
  workspaceBootstrap: WorkspaceBootstrapResponse | null
  adminBootstrap: AdminBootstrapResponse | null
  navigationPending: boolean
  toggleSidebar: () => void
  setSidebarOpen: (open: boolean) => void
  toggleMobileNav: () => void
  closeMobileNav: () => void
  setCurrentRoute: (route: string) => void
  setWorkspaceBootstrap: (data: WorkspaceBootstrapResponse) => void
  setAdminBootstrap: (data: AdminBootstrapResponse) => void
  setRoutes: (routes: ShellRoute[]) => void
  setLocale: (locale: string) => void
  setNavigationPending: (pending: boolean) => void
}

export const useShellStore = create<ShellState>()(
  persist(
    (set) => ({
      sidebarOpen: true,
      mobileNavOpen: false,
      shellKind: 'workspace',
      currentRoute: '',
      routes: [],
      currentSurface: 'backoffice',
      availableSurfaces: [],
      adminAccess: false,
      adminPath: '/admin',
      uiAccess: false,
      uiPath: '/ui',
      locale: 'en',
      supportedLocales: ['en', 'id'],
      authContext: null,
      defaultPath: '/',
      actions: [],
      views: [],
      flows: [],
      customEntries: [],
      workspaceBootstrap: null,
      adminBootstrap: null,
      navigationPending: false,

      toggleSidebar: () => set((state) => ({ sidebarOpen: !state.sidebarOpen })),
      setSidebarOpen: (open) => set({ sidebarOpen: open }),
      toggleMobileNav: () => set((state) => ({ mobileNavOpen: !state.mobileNavOpen })),
      closeMobileNav: () => set({ mobileNavOpen: false }),
      setCurrentRoute: (route) => set({ currentRoute: route }),
      setWorkspaceBootstrap: (data) =>
        set({
          shellKind: 'workspace',
          currentSurface: data.surface,
          availableSurfaces: data.available_surfaces,
          adminAccess: data.admin_access,
          adminPath: normalizeShellPath(data.admin_path, 'admin'),
          locale: data.locale,
          supportedLocales: data.supported_locales,
          authContext: data.auth_context,
          defaultPath: normalizeShellPath(data.default_path, 'workspace'),
          actions: data.actions,
          views: data.views,
          flows: data.flows,
          customEntries: [],
          workspaceBootstrap: data,
        }),
      setAdminBootstrap: (data) =>
        set({
          shellKind: 'admin',
          uiAccess: data.ui_access,
          uiPath: normalizeShellPath(data.ui_path, 'workspace'),
          locale: data.locale,
          supportedLocales: data.supported_locales,
          defaultPath: normalizeShellPath(data.default_path, 'admin'),
          actions: data.actions,
          views: data.views,
          flows: [],
          customEntries: data.custom_entries,
          adminBootstrap: data,
        }),
      setRoutes: (routes) => set({ routes }),
      setLocale: (locale) => set({ locale }),
      setNavigationPending: (pending) => set({ navigationPending: pending }),
    }),
    {
      name: 'orbyte-shell',
      partialize: (state) => ({
        sidebarOpen: state.sidebarOpen,
        locale: state.locale,
      }),
    }
  )
)
