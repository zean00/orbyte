import {
  Routes,
  Route,
  Navigate,
  useLocation,
  useNavigate,
} from "react-router-dom";
import { ToastProvider } from "@/components/ui/Toast";
import { PageLoading } from "@/components/feedback/Loading";
import { lazy, Suspense, useEffect, useState } from "react";
import { useDarkMode } from "@/hooks/useDarkMode";
import { useAuth } from "@/hooks/useAuth";
import { fetchWorkspaceBootstrap, toShellRoutes } from "@/services/bootstrap";
import { useShellStore } from "@/stores/shellStore";

const LoginPage = lazy(() => import("@/features/auth/LoginPage"));
const AgentSurfacePage = lazy(
  () => import("@/features/agent/AgentSurfacePage"),
);
const DashboardSurfacePage = lazy(
  () => import("@/features/dashboard/DashboardSurfacePage"),
);
const POSSurfacePage = lazy(() => import("@/features/pos/POSSurfacePage"));
const WorkspacePage = lazy(() => import("@/features/workspace/WorkspacePage"));

function PageLoader() {
  return (
    <div className="flex h-full items-center justify-center">
      <PageLoading />
    </div>
  );
}

function BootstrapLoader({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, hasCheckedAuth, isLoading, checkAuth } = useAuth();
  const location = useLocation();
  const [ready, setReady] = useState(false);
  const setWorkspaceBootstrap = useShellStore(
    (state) => state.setWorkspaceBootstrap,
  );
  const setRoutes = useShellStore((state) => state.setRoutes);

  useEffect(() => {
    let mounted = true;

    async function run() {
      if (!hasCheckedAuth) {
        if (!isLoading) {
          await checkAuth();
        }
        return;
      }
      if (!hasCheckedAuth) return;
      if (!isAuthenticated) {
        setReady(true);
        return;
      }
      try {
        let surface: string | undefined;
        if (location.pathname.startsWith("/pos")) {
          surface = "pos";
        } else if (location.pathname.startsWith("/agent")) {
          surface = "agent";
        } else if (location.pathname.startsWith("/dashboard")) {
          surface = "dashboard";
        } else if (
          location.pathname === "/worklist" ||
          location.pathname.startsWith("/worklist/")
        ) {
          surface = "worklist";
        } else if (
          location.pathname === "/self-service" ||
          location.pathname.startsWith("/self-service/")
        ) {
          surface = "self_service";
        }
        const bootstrap = await fetchWorkspaceBootstrap(surface);
        if (!mounted) return;
        setWorkspaceBootstrap(bootstrap);
        setRoutes(
          toShellRoutes(
            bootstrap.menus,
            bootstrap.actions,
            bootstrap.locale,
            "workspace",
          ),
        );
        setReady(true);
      } catch (error) {
        if (!mounted) return;
        if ((error as { status?: number }).status === 401) {
          window.location.href = "/ui/login";
          return;
        }
        throw error;
      }
    }

    void run();
    return () => {
      mounted = false;
    };
  }, [
    checkAuth,
    hasCheckedAuth,
    isAuthenticated,
    isLoading,
    location.pathname,
    setRoutes,
    setWorkspaceBootstrap,
  ]);

  if (!hasCheckedAuth || !ready) return <PageLoader />;
  return <>{children}</>;
}

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isLoading, hasCheckedAuth, checkAuth } = useAuth();
  const location = useLocation();

  useEffect(() => {
    void checkAuth();
  }, [checkAuth]);

  if (isLoading || !hasCheckedAuth) return <PageLoader />;
  if (!isAuthenticated) {
    return (
      <Navigate
        to="/login"
        replace
        state={{ from: location.pathname + location.search }}
      />
    );
  }
  return <>{children}</>;
}

function AgentProtectedRoute({ children }: { children: React.ReactNode }) {
  const availableSurfaces = useShellStore((state) => state.availableSurfaces);
  const defaultPath = useShellStore((state) => state.defaultPath);

  if (!availableSurfaces.includes("agent")) {
    return <Navigate to={defaultPath || "/"} replace />;
  }
  return <>{children}</>;
}

function SurfaceProtectedRoute({
  surface,
  children,
}: {
  surface: string;
  children: React.ReactNode;
}) {
  const availableSurfaces = useShellStore((state) => state.availableSurfaces);
  const defaultPath = useShellStore((state) => state.defaultPath);

  if (!availableSurfaces.includes(surface)) {
    return <Navigate to={defaultPath || "/"} replace />;
  }
  return <>{children}</>;
}

function LoginGate() {
  const { isAuthenticated, isLoading, hasCheckedAuth, checkAuth } = useAuth();
  const navigate = useNavigate();

  useEffect(() => {
    if (!hasCheckedAuth) {
      void checkAuth();
    }
  }, [checkAuth, hasCheckedAuth]);

  useEffect(() => {
    if (hasCheckedAuth && isAuthenticated) navigate("/", { replace: true });
  }, [hasCheckedAuth, isAuthenticated, navigate]);

  if (isLoading || !hasCheckedAuth) return <PageLoader />;

  return (
    <Suspense fallback={<PageLoader />}>
      <LoginPage />
    </Suspense>
  );
}

export default function App() {
  useDarkMode();

  return (
    <ToastProvider>
      <BootstrapLoader>
        <Routes>
          <Route path="/login" element={<LoginGate />} />
          <Route
            path="/agent/workspace"
            element={
              <ProtectedRoute>
                <AgentProtectedRoute>
                <Suspense fallback={<PageLoader />}>
                  <AgentSurfacePage />
                </Suspense>
                </AgentProtectedRoute>
              </ProtectedRoute>
            }
          />
          <Route
            path="/dashboard"
            element={
              <ProtectedRoute>
                <SurfaceProtectedRoute surface="dashboard">
                  <Suspense fallback={<PageLoader />}>
                    <DashboardSurfacePage />
                  </Suspense>
                </SurfaceProtectedRoute>
              </ProtectedRoute>
            }
          />
          <Route
            path="/pos/terminal"
            element={
              <ProtectedRoute>
                <SurfaceProtectedRoute surface="pos">
                  <Suspense fallback={<PageLoader />}>
                    <POSSurfacePage />
                  </Suspense>
                </SurfaceProtectedRoute>
              </ProtectedRoute>
            }
          />
          <Route
            path="*"
            element={
              <ProtectedRoute>
                <Suspense fallback={<PageLoader />}>
                  <WorkspacePage />
                </Suspense>
              </ProtectedRoute>
            }
          />
        </Routes>
      </BootstrapLoader>
    </ToastProvider>
  );
}
