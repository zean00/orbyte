import { type FormEvent, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useShellStore } from "@/stores/shellStore";
import { useAuth } from "@/hooks/useAuth";
import { useDarkMode } from "@/hooks/useDarkMode";
import { usePreferencesStore } from "@/stores/preferencesStore";
import { useToast } from "@/components/ui/Toast";
import {
  fetchAdminBootstrap,
  fetchWorkspaceBootstrap,
  normalizeShellPath,
  persistLocale,
  pickText,
  toShellRoutes,
  workspaceSurfaceTarget,
} from "@/services/bootstrap";
import { preloadSurfaceModule } from "@/services/surfaceModules";

const surfaceLabels: Record<string, string> = {
  backoffice: "Backoffice",
  worklist: "Worklist",
  self_service: "Self-Service",
  agent: "Agent",
  pos: "POS",
  dashboard: "Dashboard",
};

export function Header() {
  const navigate = useNavigate();
  const {
    shellKind,
    availableSurfaces,
    currentSurface,
    currentRoute,
    locale,
    supportedLocales,
    adminAccess,
    adminPath,
    uiAccess,
    uiPath,
    workspaceBootstrap,
    setWorkspaceBootstrap,
    setAdminBootstrap,
    setLocale,
    setRoutes,
    setNavigationPending,
    toggleMobileNav,
    actions,
    routes,
  } = useShellStore();
  const { setLocale: setPreferredLocale } = usePreferencesStore();
  const { darkMode, toggleDarkMode } = useDarkMode();
  const { user, logout } = useAuth();
  const { addToast } = useToast();
  const [command, setCommand] = useState("");

  const handleLogout = async () => {
    try {
      await logout();
    } catch (error) {
      addToast({
        message:
          error instanceof Error
            ? error.message
            : "Logout failed. Please try again.",
        variant: "error",
      });
      return;
    }
    if (shellKind === "admin") {
      window.location.href = "/ui/login";
      return;
    }
    navigate("/login", { replace: true });
  };

  const commandOptions = useMemo(() => {
    const unique = new Map<string, { label: string; path: string }>();
    for (const route of routes) {
      unique.set(route.path, { label: route.label, path: route.path });
    }
    for (const action of actions) {
      const path = normalizeShellPath(action.route_path, shellKind);
      if (!path) continue;
      unique.set(path, {
        label: pickText(action, "label", locale) || action.key,
        path,
      });
    }
    if (shellKind === "workspace") {
      unique.set("/notifications", {
        label: "Notifications",
        path: "/notifications",
      });
    }
    return [...unique.values()];
  }, [actions, locale, routes, shellKind]);

  const currentRouteLabel = useMemo(() => {
    if (currentRoute) {
      const match = routes.find((route) => route.path === currentRoute);
      if (match) return match.label;
    }
    if (shellKind === "workspace") {
      return surfaceLabels[currentSurface] || "Workspace";
    }
    return "Admin Console";
  }, [currentRoute, currentSurface, routes, shellKind]);

  const handleSurfaceChange = async (surface: string) => {
    if (surface === currentSurface) return;
    setNavigationPending(true, "surface");
    try {
      const [bootstrap] = await Promise.all([
        fetchWorkspaceBootstrap(surface),
        preloadSurfaceModule(surface),
      ]);
      const target = workspaceSurfaceTarget(bootstrap, surface);
      navigate(target || "/", { replace: true });
    } finally {
      setNavigationPending(false);
    }
  };

  const handleLocaleChange = async (newLocale: string) => {
    setNavigationPending(true, "locale");
    try {
      const activeLocale = await persistLocale(newLocale);
      setLocale(activeLocale);
      setPreferredLocale(activeLocale);
      if (shellKind === "workspace") {
        const bootstrap = await fetchWorkspaceBootstrap(currentSurface);
        setWorkspaceBootstrap(bootstrap);
        setRoutes(
          toShellRoutes(
            bootstrap.menus,
            bootstrap.actions,
            bootstrap.locale,
            "workspace",
          ),
        );
        return;
      }
      const bootstrap = await fetchAdminBootstrap();
      setAdminBootstrap(bootstrap);
      setRoutes(
        toShellRoutes(
          bootstrap.menus,
          bootstrap.actions,
          bootstrap.locale,
          "admin",
        ),
      );
    } finally {
      setNavigationPending(false);
    }
  };

  const handleCommandSubmit = (event: FormEvent) => {
    event.preventDefault();
    const raw = command.trim();
    if (!raw) return;
    const lowered = raw.toLowerCase();
    const match =
      commandOptions.find(
        (item) => item.path === raw || item.label.toLowerCase() === lowered,
      ) ||
      commandOptions.find(
        (item) =>
          item.label.toLowerCase().includes(lowered) ||
          item.path.toLowerCase().includes(lowered),
      );
    navigate(
      (match?.path || raw).startsWith("/")
        ? match?.path || raw
        : `/${match?.path || raw}`,
    );
    setNavigationPending(true, "command");
    setCommand("");
  };

  return (
    <header className="sticky top-0 z-30 border-b border-line bg-surface/90 backdrop-blur">
      <div className="flex min-h-16 items-center gap-3 px-4 py-3 sm:px-6">
        <button
          type="button"
          onClick={toggleMobileNav}
          className="inline-flex rounded-xl border border-line p-2 text-muted transition-colors hover:bg-shell hover:text-body md:hidden"
          aria-label="Open navigation"
        >
          <MenuIcon className="h-5 w-5" />
        </button>

        <div className="min-w-0 flex-1">
          <div className="truncate text-[11px] font-semibold uppercase tracking-[0.18em] text-muted">
            {shellKind === "admin" ? "Admin Console" : "Orbyte Workspace"}
          </div>
          <div className="truncate text-lg font-semibold text-body">{currentRouteLabel}</div>
        </div>

        {shellKind === "workspace" && availableSurfaces.length > 0 && (
          <div className="hidden items-center rounded-2xl border border-line bg-shell/80 p-1 md:flex">
            {availableSurfaces.map((surface) => (
              <button
                key={surface}
                onClick={() => void handleSurfaceChange(surface)}
                onMouseEnter={() => {
                  void preloadSurfaceModule(surface);
                  void fetchWorkspaceBootstrap(surface);
                }}
                onFocus={() => {
                  void preloadSurfaceModule(surface);
                  void fetchWorkspaceBootstrap(surface);
                }}
                className={`rounded-xl px-3 py-1.5 text-xs font-medium transition-colors whitespace-nowrap ${
                  currentSurface === surface
                    ? "bg-accent text-white"
                    : "text-muted hover:text-body"
                }`}
              >
                {surfaceLabels[surface] || surface}
              </button>
            ))}
          </div>
        )}

        <div className="hidden items-center gap-2 xl:flex">
        <form
          onSubmit={handleCommandSubmit}
          className="flex items-center gap-2"
        >
          <input
            id="shell-command"
            value={command}
            onChange={(e) => setCommand(e.target.value)}
            list="shell-route-options"
            placeholder={
              shellKind === "admin"
                ? "Search admin sections or jump to a route"
                : "Search pages or jump to a route"
            }
            className="w-72 rounded-2xl border border-line bg-surface px-3 py-2 text-sm text-body"
            name="shell_command"
          />
          <datalist id="shell-route-options">
            {commandOptions.map((item) => (
              <option key={item.path} value={item.path}>
                {item.label}
              </option>
            ))}
          </datalist>
          <button
            type="submit"
            className="rounded-2xl border border-line px-3 py-2 text-sm text-body transition-colors hover:bg-shell"
          >
            Go
          </button>
        </form>
        </div>

        <div className="flex items-center gap-2">
        {shellKind === "workspace" && availableSurfaces.length > 0 && (
          <select
            value={currentSurface}
            onChange={(e) => void handleSurfaceChange(e.target.value)}
            className="min-w-0 rounded-2xl border border-line bg-surface px-3 py-2 text-xs text-body md:hidden"
            name="surface"
          >
            {availableSurfaces.map((surface) => (
              <option key={surface} value={surface}>
                {surfaceLabels[surface] || surface}
              </option>
            ))}
          </select>
        )}
        {supportedLocales.length > 1 && (
          <select
            id="locale"
            value={locale}
            onChange={(e) => void handleLocaleChange(e.target.value)}
            className="rounded-2xl border border-line bg-surface px-3 py-2 text-xs text-body"
            name="locale"
          >
            {supportedLocales.map((loc) => (
              <option key={loc} value={loc}>
                {loc.toUpperCase()}
              </option>
            ))}
          </select>
        )}

        {shellKind === "workspace" && (
          <button
            onClick={() => navigate("/settings")}
            className="rounded-xl p-2 text-muted transition-colors hover:bg-shell hover:text-body"
            title="Settings"
          >
            <SettingsIcon className="w-4 h-4" />
          </button>
        )}

        {shellKind === "workspace" && (
          <button
            onClick={() => navigate("/notifications")}
            className="rounded-xl p-2 text-muted transition-colors hover:bg-shell hover:text-body"
            title="Notifications"
          >
            <BellIcon className="w-4 h-4" />
          </button>
        )}

        <button
          onClick={toggleDarkMode}
          className="rounded-xl p-2 text-muted transition-colors hover:bg-shell hover:text-body"
          title={darkMode ? "Light mode" : "Dark mode"}
        >
          {darkMode ? (
            <SunIcon className="w-4 h-4" />
          ) : (
            <MoonIcon className="w-4 h-4" />
          )}
        </button>

        {user && (
          <div className="hidden items-center gap-2 border-l border-line pl-3 lg:flex">
            <span className="max-w-32 truncate text-sm font-medium text-body">{user.name}</span>
            <button
              onClick={() => {
                void handleLogout();
              }}
              className="rounded-xl p-2 text-muted transition-colors hover:bg-shell hover:text-warn"
              title="Log out"
            >
              <LogoutIcon className="w-4 h-4" />
            </button>
          </div>
        )}

        {shellKind === "workspace" && adminAccess && workspaceBootstrap && (
          <button
            type="button"
            onClick={() => {
              window.location.href = `/admin${adminPath === "/" ? "" : adminPath}`;
            }}
            className="rounded-2xl bg-accent px-3 py-2 text-xs font-semibold text-white transition-colors hover:bg-accent-dark"
          >
            Admin
          </button>
        )}

        {shellKind === "admin" && uiAccess && (
          <button
            type="button"
            onClick={() => {
              window.location.href = `/ui${uiPath === "/" ? "" : uiPath}`;
            }}
            className="rounded-2xl bg-accent px-3 py-2 text-xs font-semibold text-white transition-colors hover:bg-accent-dark"
          >
            Workspace
          </button>
        )}
        {user && (
          <button
            onClick={() => {
              void handleLogout();
            }}
            className="rounded-xl p-2 text-muted transition-colors hover:bg-shell hover:text-warn lg:hidden"
            title="Log out"
          >
            <LogoutIcon className="h-4 w-4" />
          </button>
        )}
        </div>
      </div>
    </header>
  );
}

function MenuIcon({ className }: { className?: string }) {
  return (
    <svg
      className={className}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      strokeWidth={2}
    >
      <path strokeLinecap="round" strokeLinejoin="round" d="M4 6h16M4 12h16M4 18h16" />
    </svg>
  )
}

function SunIcon({ className }: { className?: string }) {
  return (
    <svg
      className={className}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      strokeWidth={2}
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z"
      />
    </svg>
  );
}

function MoonIcon({ className }: { className?: string }) {
  return (
    <svg
      className={className}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      strokeWidth={2}
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z"
      />
    </svg>
  );
}

function BellIcon({ className }: { className?: string }) {
  return (
    <svg
      className={className}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      strokeWidth={2}
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9"
      />
    </svg>
  );
}

function LogoutIcon({ className }: { className?: string }) {
  return (
    <svg
      className={className}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      strokeWidth={2}
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1"
      />
    </svg>
  );
}

function SettingsIcon({ className }: { className?: string }) {
  return (
    <svg
      className={className}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      strokeWidth={2}
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M10.325 4.317a1 1 0 011.35-.936l.821.328a1 1 0 00.748 0l.821-.328a1 1 0 011.35.936l.062.883a1 1 0 00.512.815l.746.43a1 1 0 01.365 1.366l-.43.746a1 1 0 000 .748l.43.746a1 1 0 01-.365 1.366l-.746.43a1 1 0 00-.512.815l-.062.883a1 1 0 01-1.35.936l-.821-.328a1 1 0 00-.748 0l-.821.328a1 1 0 01-1.35-.936l-.062-.883a1 1 0 00-.512-.815l-.746-.43a1 1 0 01-.365-1.366l.43-.746a1 1 0 000-.748l-.43-.746a1 1 0 01.365-1.366l.746-.43a1 1 0 00.512-.815l.062-.883z"
      />
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M12 15a3 3 0 100-6 3 3 0 000 6z"
      />
    </svg>
  );
}
